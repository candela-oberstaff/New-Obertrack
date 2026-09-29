package service

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // dimensiones del diseño al subirlo
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

// CertificateService emite certificados al completar un programa de
// inducción: un PDF inmutable con el diseño que subió el equipo y, encima, el
// nombre, el programa, la fecha y un código de verificación. No se envía por
// correo: se descarga desde la plataforma y se verifica por su código.
type CertificateService interface {
	// --- Plantillas (diseños) ---
	ListTemplates() ([]models.CertificateTemplate, error)
	GetTemplate(id uint) (*models.CertificateTemplate, error)
	CreateTemplate(actorID uint, in TemplateInput) (*models.CertificateTemplate, error)
	UpdateTemplate(id uint, in TemplateInput) (*models.CertificateTemplate, error)
	DeleteTemplate(id uint) error
	// Preview renderiza una plantilla (guardada o no) con datos de ejemplo.
	Preview(in TemplateInput) ([]byte, error)

	// --- Emisión ---
	// IssueForCompletion emite el certificado de una invitación recién
	// aprobada, si su programa tiene plantilla. Idempotente. Best-effort para
	// quien llama: un certificado que no se pudo emitir no frena la aprobación.
	IssueForCompletion(user *models.User, invite *models.InductionInvite) (*models.Certificate, error)
	// IssueForInvite emite (acción de Soporte) el certificado de una
	// invitación aprobada que no lo tiene: la que aprobó antes de que el
	// programa tuviera plantilla.
	IssueForInvite(inviteID uint) (*models.Certificate, error)
	// Reissue vuelve a generar el PDF con la plantilla actual del programa,
	// conservando el código.
	Reissue(certificateID uint) (*models.Certificate, error)
	GetForInvite(inviteID uint) (*models.Certificate, error)
	GetByID(id uint) (*models.Certificate, error)
	GetByCode(code string) (*models.Certificate, error)
	ListForUser(userID uint) ([]models.Certificate, error)
	// ListForProgram devuelve el total emitido en un programa y los últimos.
	ListForProgram(programID uint) (*models.ProgramCertificates, error)
	// Verify resuelve un código para la página pública.
	Verify(code string) (*CertificateVerification, error)
	// FilePath es la ruta en disco del PDF emitido.
	FilePath(c *models.Certificate) string
}

// TemplateInput es lo que llega del editor de plantillas.
type TemplateInput struct {
	Name          string                    `json:"name"`
	ImageFilename string                    `json:"image_filename"`
	Fields        []models.CertificateField `json:"fields"`
}

// CertificateVerification es lo que muestra la página pública.
type CertificateVerification struct {
	Valid            bool      `json:"valid"`
	Code             string    `json:"code"`
	ProfessionalName string    `json:"professional_name"`
	ProgramName      string    `json:"program_name"`
	IssuedAt         time.Time `json:"issued_at"`
	DownloadURL      string    `json:"download_url"`
}

// certificateData son los valores que se imprimen sobre el diseño.
type certificateData struct {
	Name    string
	Program string
	Date    string
	Code    string
}

const certificatesDir = "certificates"

var certificateFonts = map[string]bool{"Helvetica": true, "Times": true, "Courier": true}

type certificateService struct {
	repo          repository.CertificateRepository
	inductionRepo repository.InductionRepository
	userRepo      repository.UserRepository
	notifSvc      NotificationService
	uploadPath    string
}

func NewCertificateService(
	repo repository.CertificateRepository,
	inductionRepo repository.InductionRepository,
	userRepo repository.UserRepository,
	notifSvc NotificationService,
	uploadPath string,
) CertificateService {
	if uploadPath == "" {
		uploadPath = "./uploads"
	}
	return &certificateService{
		repo:          repo,
		inductionRepo: inductionRepo,
		userRepo:      userRepo,
		notifSvc:      notifSvc,
		uploadPath:    uploadPath,
	}
}

// --- Plantillas ---------------------------------------------------------------

func (s *certificateService) ListTemplates() ([]models.CertificateTemplate, error) {
	templates, err := s.repo.ListTemplates()
	if err != nil {
		return nil, err
	}
	if templates == nil {
		templates = []models.CertificateTemplate{}
	}
	return templates, nil
}

func (s *certificateService) GetTemplate(id uint) (*models.CertificateTemplate, error) {
	t, err := s.repo.GetTemplate(id)
	if err != nil {
		return nil, errors.New("plantilla no encontrada")
	}
	return t, nil
}

// DefaultCertificateFields es la disposición inicial de una plantilla nueva:
// el nombre al centro, el programa debajo, la fecha y el código al pie.
func DefaultCertificateFields() []models.CertificateField {
	return []models.CertificateField{
		{Key: models.CertificateFieldName, X: 50, Y: 48, Size: 32, Color: "#0f172a", Align: "C", Bold: true, Font: "Helvetica"},
		{Key: models.CertificateFieldProgram, X: 50, Y: 62, Size: 18, Color: "#334155", Align: "C", Font: "Helvetica"},
		{Key: models.CertificateFieldDate, X: 50, Y: 74, Size: 12, Color: "#64748b", Align: "C", Font: "Helvetica"},
		{Key: models.CertificateFieldCode, X: 50, Y: 93, Size: 9, Color: "#94a3b8", Align: "C", Font: "Courier"},
	}
}

// validateTemplateInput normaliza nombre, imagen y campos. La imagen tiene que
// existir ya en uploads (se sube por /api/uploads) y ser PNG o JPG.
func (s *certificateService) validateTemplateInput(in *TemplateInput) (orientation string, err error) {
	in.Name = strings.TrimSpace(utils.SanitizeHTML(in.Name))
	if in.Name == "" {
		return "", errors.New("la plantilla necesita un nombre")
	}
	in.ImageFilename = strings.TrimSpace(in.ImageFilename)
	if in.ImageFilename == "" {
		return "", errors.New("sube el diseño del certificado (PNG o JPG)")
	}
	if strings.ContainsAny(in.ImageFilename, "/\\") || strings.Contains(in.ImageFilename, "..") || in.ImageFilename != filepath.Base(in.ImageFilename) {
		return "", errors.New("nombre de archivo inválido")
	}
	ext := strings.ToLower(filepath.Ext(in.ImageFilename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return "", errors.New("el diseño debe ser una imagen PNG o JPG")
	}
	orientation, err = imageOrientation(filepath.Join(s.uploadPath, in.ImageFilename))
	if err != nil {
		return "", err
	}
	if len(in.Fields) == 0 {
		in.Fields = DefaultCertificateFields()
	}
	for i := range in.Fields {
		f := &in.Fields[i]
		if !models.IsValidCertificateFieldKey(f.Key) {
			return "", fmt.Errorf("campo desconocido: %q", f.Key)
		}
		if f.X < 0 || f.X > 100 || f.Y < 0 || f.Y > 100 {
			return "", errors.New("la posición de un campo debe estar entre 0 y 100")
		}
		if f.Size < 6 || f.Size > 120 {
			return "", errors.New("el tamaño de letra debe estar entre 6 y 120")
		}
		if _, ok := hexToRGB(f.Color); !ok {
			f.Color = "#0f172a"
		}
		switch f.Align {
		case "L", "C", "R":
		default:
			f.Align = "C"
		}
		if !certificateFonts[f.Font] {
			f.Font = "Helvetica"
		}
		if f.Key == models.CertificateFieldText {
			f.Text = strings.TrimSpace(utils.SanitizeHTML(f.Text))
		} else {
			f.Text = ""
		}
	}
	return orientation, nil
}

// imageOrientation lee las dimensiones del diseño: apaisado = L, vertical = P.
func imageOrientation(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("no se encontró el diseño subido; vuelve a subirlo")
	}
	defer file.Close()
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return "", errors.New("el diseño no es una imagen válida")
	}
	if cfg.Height > cfg.Width {
		return "P", nil
	}
	return "L", nil
}

func (s *certificateService) CreateTemplate(actorID uint, in TemplateInput) (*models.CertificateTemplate, error) {
	orientation, err := s.validateTemplateInput(&in)
	if err != nil {
		return nil, err
	}
	t := &models.CertificateTemplate{
		Name:          in.Name,
		ImageFilename: in.ImageFilename,
		Orientation:   orientation,
		FieldsJSON:    encodeFields(in.Fields),
		CreatedBy:     actorID,
	}
	if err := s.repo.CreateTemplate(t); err != nil {
		return nil, err
	}
	return s.repo.GetTemplate(t.ID)
}

func (s *certificateService) UpdateTemplate(id uint, in TemplateInput) (*models.CertificateTemplate, error) {
	if _, err := s.repo.GetTemplate(id); err != nil {
		return nil, errors.New("plantilla no encontrada")
	}
	orientation, err := s.validateTemplateInput(&in)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTemplate(id, map[string]interface{}{
		"name":           in.Name,
		"image_filename": in.ImageFilename,
		"orientation":    orientation,
		"fields_json":    encodeFields(in.Fields),
	}); err != nil {
		return nil, err
	}
	return s.repo.GetTemplate(id)
}

func (s *certificateService) DeleteTemplate(id uint) error {
	t, err := s.repo.GetTemplate(id)
	if err != nil {
		return errors.New("plantilla no encontrada")
	}
	// Los certificados ya emitidos no dependen de la plantilla (el PDF está
	// guardado), pero un programa que la usa dejaría de emitir sin avisar.
	if len(t.ProgramNames) > 0 {
		return fmt.Errorf("esta plantilla está en uso en: %s. Quítala de esos programas antes de borrarla", strings.Join(t.ProgramNames, ", "))
	}
	return s.repo.DeleteTemplate(id)
}

func (s *certificateService) Preview(in TemplateInput) ([]byte, error) {
	orientation, err := s.validateTemplateInput(&in)
	if err != nil {
		return nil, err
	}
	t := &models.CertificateTemplate{
		Name: in.Name, ImageFilename: in.ImageFilename, Orientation: orientation, Fields: in.Fields,
	}
	return s.render(t, certificateData{
		Name:    "María Fernanda Pérez",
		Program: "Programa de ejemplo",
		Date:    formatCertificateDate(time.Now()),
		Code:    "OBT-EJEM-PLO1",
	})
}

// --- Emisión ------------------------------------------------------------------

func (s *certificateService) IssueForCompletion(user *models.User, invite *models.InductionInvite) (*models.Certificate, error) {
	if user == nil || invite == nil || invite.ProgramID == nil || *invite.ProgramID == 0 {
		return nil, nil
	}
	program, err := s.inductionRepo.GetProgram(*invite.ProgramID)
	if err != nil || program == nil || program.CertificateTemplateID == nil || *program.CertificateTemplateID == 0 {
		// Sin plantilla no hay certificado, y no es un error.
		return nil, nil
	}
	return s.issue(user, invite, program.Name, *program.CertificateTemplateID)
}

func (s *certificateService) IssueForInvite(inviteID uint) (*models.Certificate, error) {
	invite, err := s.inductionRepo.GetInviteByID(inviteID)
	if err != nil {
		return nil, errors.New("capacitación no encontrada")
	}
	if invite.Status != models.InductionPassed {
		return nil, errors.New("solo se certifica una capacitación aprobada")
	}
	if invite.ProgramID == nil || *invite.ProgramID == 0 {
		return nil, errors.New("esta capacitación no tiene programa asociado")
	}
	program, err := s.inductionRepo.GetProgram(*invite.ProgramID)
	if err != nil {
		return nil, errors.New("el programa de esta capacitación ya no existe")
	}
	if program.CertificateTemplateID == nil || *program.CertificateTemplateID == 0 {
		return nil, errors.New("el programa no tiene plantilla de certificado; asígnale una primero")
	}
	user, err := s.userRepo.GetByID(invite.UserID)
	if err != nil {
		return nil, errors.New("usuario no encontrado")
	}
	return s.issue(user, invite, program.Name, *program.CertificateTemplateID)
}

// issue genera y guarda el certificado. Si la invitación ya tiene uno, lo
// devuelve tal cual: emitir dos veces no debe producir dos documentos.
func (s *certificateService) issue(user *models.User, invite *models.InductionInvite, programName string, templateID uint) (*models.Certificate, error) {
	if existing, err := s.repo.GetByInvite(invite.ID); err == nil && existing != nil {
		return existing, nil
	}
	template, err := s.repo.GetTemplate(templateID)
	if err != nil {
		return nil, errors.New("la plantilla del programa ya no existe")
	}
	code, err := s.uniqueCode()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if invite.CompletedAt != nil {
		now = *invite.CompletedAt
	}
	pdf, err := s.render(template, certificateData{
		Name:    user.Name,
		Program: programName,
		Date:    formatCertificateDate(now),
		Code:    code,
	})
	if err != nil {
		return nil, err
	}
	filename, err := s.writePDF(code, pdf)
	if err != nil {
		return nil, err
	}
	cert := &models.Certificate{
		UserID:       user.ID,
		InviteID:     invite.ID,
		ProgramID:    *invite.ProgramID,
		ProgramName:  programName,
		TemplateID:   template.ID,
		TemplateName: template.Name,
		Code:         code,
		Filename:     filename,
		IssuedAt:     now,
	}
	if err := s.repo.CreateCertificate(cert); err != nil {
		return nil, err
	}
	if s.notifSvc != nil {
		_ = s.notifSvc.CreateNotification(user.ID, "certificado",
			"Tu certificado está listo: "+programName,
			"Puedes descargarlo desde tu perfil y compartir su código de verificación.",
			map[string]interface{}{"link": "/profile", "certificate_id": cert.ID})
	}
	return cert, nil
}

func (s *certificateService) Reissue(certificateID uint) (*models.Certificate, error) {
	cert, err := s.repo.GetCertificate(certificateID)
	if err != nil {
		return nil, errors.New("certificado no encontrado")
	}
	user, err := s.userRepo.GetByID(cert.UserID)
	if err != nil {
		return nil, errors.New("usuario no encontrado")
	}
	// La plantilla ACTUAL del programa, que es lo que se corrigió; si el
	// programa ya no la tiene, la que se usó al emitir.
	templateID := cert.TemplateID
	programName := cert.ProgramName
	if program, err := s.inductionRepo.GetProgram(cert.ProgramID); err == nil && program != nil {
		programName = program.Name
		if program.CertificateTemplateID != nil && *program.CertificateTemplateID > 0 {
			templateID = *program.CertificateTemplateID
		}
	}
	template, err := s.repo.GetTemplate(templateID)
	if err != nil {
		return nil, errors.New("la plantilla ya no existe")
	}
	pdf, err := s.render(template, certificateData{
		Name:    user.Name,
		Program: programName,
		Date:    formatCertificateDate(cert.IssuedAt),
		Code:    cert.Code,
	})
	if err != nil {
		return nil, err
	}
	filename, err := s.writePDF(cert.Code, pdf)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.repo.UpdateCertificate(cert.ID, map[string]interface{}{
		"filename":      filename,
		"template_id":   template.ID,
		"template_name": template.Name,
		"program_name":  programName,
		"reissued_at":   now,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetCertificate(cert.ID)
}

func (s *certificateService) GetForInvite(inviteID uint) (*models.Certificate, error) {
	return s.repo.GetByInvite(inviteID)
}

func (s *certificateService) GetByID(id uint) (*models.Certificate, error) {
	c, err := s.repo.GetCertificate(id)
	if err != nil {
		return nil, errors.New("certificado no encontrado")
	}
	return c, nil
}

func (s *certificateService) GetByCode(code string) (*models.Certificate, error) {
	code = normalizeCode(code)
	if code == "" {
		return nil, errors.New("certificado no encontrado")
	}
	c, err := s.repo.GetByCode(code)
	if err != nil {
		return nil, errors.New("certificado no encontrado")
	}
	return c, nil
}

func (s *certificateService) ListForUser(userID uint) ([]models.Certificate, error) {
	certs, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	if certs == nil {
		certs = []models.Certificate{}
	}
	return certs, nil
}

func (s *certificateService) ListForProgram(programID uint) (*models.ProgramCertificates, error) {
	total, err := s.repo.CountByProgram(programID)
	if err != nil {
		return nil, err
	}
	recent, err := s.repo.ListByProgram(programID, 12)
	if err != nil {
		return nil, err
	}
	if recent == nil {
		recent = []models.IssuedCertificate{}
	}
	return &models.ProgramCertificates{Total: total, Recent: recent}, nil
}

func (s *certificateService) Verify(code string) (*CertificateVerification, error) {
	code = normalizeCode(code)
	if code == "" {
		return &CertificateVerification{Valid: false}, nil
	}
	cert, err := s.repo.GetByCode(code)
	if err != nil {
		return &CertificateVerification{Valid: false, Code: code}, nil
	}
	name := ""
	if user, err := s.userRepo.GetByID(cert.UserID); err == nil {
		name = user.Name
	}
	return &CertificateVerification{
		Valid:            true,
		Code:             cert.Code,
		ProfessionalName: name,
		ProgramName:      cert.ProgramName,
		IssuedAt:         cert.IssuedAt,
		DownloadURL:      cert.DownloadURL(),
	}, nil
}

func (s *certificateService) FilePath(c *models.Certificate) string {
	return filepath.Join(s.uploadPath, certificatesDir, filepath.Base(c.Filename))
}

// --- Internos -----------------------------------------------------------------

// render dibuja el diseño a página completa y los campos encima.
func (s *certificateService) render(t *models.CertificateTemplate, data certificateData) ([]byte, error) {
	imagePath := filepath.Join(s.uploadPath, filepath.Base(t.ImageFilename))
	if _, err := os.Stat(imagePath); err != nil {
		return nil, errors.New("no se encontró el diseño de la plantilla")
	}
	orientation := "L"
	if t.Orientation == "P" {
		orientation = "P"
	}
	pdf := gofpdf.New(orientation, "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetMargins(0, 0, 0)
	pdf.AddPage()
	pageW, pageH := pdf.GetPageSize()

	imageType := "PNG"
	if ext := strings.ToLower(filepath.Ext(imagePath)); ext == ".jpg" || ext == ".jpeg" {
		imageType = "JPG"
	}
	pdf.ImageOptions(imagePath, 0, 0, pageW, pageH, false, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: false}, 0, "")

	// Traduce UTF-8 a CP1252 (fuentes core): sin esto los acentos salen mal.
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	for _, f := range t.Fields {
		text := fieldValue(f, data)
		if strings.TrimSpace(text) == "" {
			continue
		}
		style := ""
		if f.Bold {
			style = "B"
		}
		font := f.Font
		if !certificateFonts[font] {
			font = "Helvetica"
		}
		pdf.SetFont(font, style, f.Size)
		if rgb, ok := hexToRGB(f.Color); ok {
			pdf.SetTextColor(rgb[0], rgb[1], rgb[2])
		} else {
			pdf.SetTextColor(15, 23, 42)
		}
		txt := tr(text)
		width := pdf.GetStringWidth(txt)
		x := pageW * f.X / 100
		y := pageH * f.Y / 100
		switch f.Align {
		case "C":
			x -= width / 2
		case "R":
			x -= width
		}
		// Alto de línea en mm (puntos × 0.3528) con un poco de aire.
		lineH := f.Size * 0.3528 * 1.2
		pdf.SetXY(x, y-lineH/2)
		pdf.CellFormat(width+1, lineH, txt, "", 0, "L", false, 0, "")
	}
	if pdf.Err() {
		return nil, fmt.Errorf("no se pudo generar el certificado: %v", pdf.Error())
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fieldValue(f models.CertificateField, data certificateData) string {
	switch f.Key {
	case models.CertificateFieldName:
		return data.Name
	case models.CertificateFieldProgram:
		return data.Program
	case models.CertificateFieldDate:
		return data.Date
	case models.CertificateFieldCode:
		return data.Code
	case models.CertificateFieldText:
		return f.Text
	}
	return ""
}

func (s *certificateService) writePDF(code string, pdf []byte) (string, error) {
	dir := filepath.Join(s.uploadPath, certificatesDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	filename := code + ".pdf"
	if err := os.WriteFile(filepath.Join(dir, filename), pdf, 0644); err != nil {
		log.Printf("[Certificates] no se pudo guardar %s: %v", filename, err)
		return "", errors.New("no se pudo guardar el certificado")
	}
	return filename, nil
}

// uniqueCode genera un código legible y único: OBT-XXXX-XXXX con un alfabeto
// sin caracteres ambiguos (0/O, 1/I).
func (s *certificateService) uniqueCode() (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		code, err := generateCertificateCode()
		if err != nil {
			return "", err
		}
		if _, err := s.repo.GetByCode(code); err != nil {
			return code, nil
		}
	}
	return "", errors.New("no se pudo generar un código único")
}

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateCertificateCode() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", errors.New("no se pudo generar el código")
	}
	var b strings.Builder
	b.WriteString("OBT-")
	for i, r := range raw {
		if i == 4 {
			b.WriteByte('-')
		}
		b.WriteByte(codeAlphabet[int(r)%len(codeAlphabet)])
	}
	return b.String(), nil
}

func normalizeCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "")
	if len(code) > 32 {
		return ""
	}
	return code
}

var spanishMonths = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

func formatCertificateDate(t time.Time) string {
	return fmt.Sprintf("%d de %s de %d", t.Day(), spanishMonths[t.Month()-1], t.Year())
}

func encodeFields(fields []models.CertificateField) string {
	if len(fields) == 0 {
		return ""
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return ""
	}
	return string(raw)
}

// hexToRGB acepta #rrggbb (y #rgb).
func hexToRGB(hex string) ([3]int, bool) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return [3]int{}, false
	}
	var out [3]int
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = int(v)
	}
	return out, true
}
