package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Certificados: render sobre el diseño, emisión idempotente y verificación.

type fakeCertRepo struct {
	repository.CertificateRepository
	templates map[uint]*models.CertificateTemplate
	certs     []models.Certificate
	usedBy    map[uint][]string
}

func (f *fakeCertRepo) GetTemplate(id uint) (*models.CertificateTemplate, error) {
	if t, ok := f.templates[id]; ok {
		t.ProgramNames = f.usedBy[id]
		return t, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeCertRepo) CreateTemplate(t *models.CertificateTemplate) error {
	t.ID = uint(len(f.templates) + 1)
	// El repositorio real desempaqueta los campos al leer (AfterFind).
	_ = json.Unmarshal([]byte(t.FieldsJSON), &t.Fields)
	f.templates[t.ID] = t
	return nil
}

func (f *fakeCertRepo) DeleteTemplate(id uint) error {
	delete(f.templates, id)
	return nil
}

func (f *fakeCertRepo) CreateCertificate(c *models.Certificate) error {
	c.ID = uint(len(f.certs) + 1)
	f.certs = append(f.certs, *c)
	return nil
}

func (f *fakeCertRepo) GetByInvite(inviteID uint) (*models.Certificate, error) {
	for i := range f.certs {
		if f.certs[i].InviteID == inviteID {
			return &f.certs[i], nil
		}
	}
	return nil, nil
}

func (f *fakeCertRepo) GetByCode(code string) (*models.Certificate, error) {
	for i := range f.certs {
		if f.certs[i].Code == code {
			return &f.certs[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeCertRepo) GetCertificate(id uint) (*models.Certificate, error) {
	for i := range f.certs {
		if f.certs[i].ID == id {
			return &f.certs[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeCertRepo) UpdateCertificate(id uint, updates map[string]interface{}) error {
	for i := range f.certs {
		if f.certs[i].ID == id {
			if v, ok := updates["reissued_at"].(time.Time); ok {
				f.certs[i].ReissuedAt = &v
			}
		}
	}
	return nil
}

// writeDesign deja un PNG apaisado en la carpeta de uploads de prueba.
func writeDesign(t *testing.T, dir, name string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 235, B: 250, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func newCertSvc(t *testing.T) (*certificateService, *fakeCertRepo, *fakeInductionRepo, *fakeInductionUserRepo, string) {
	t.Helper()
	dir := t.TempDir()
	writeDesign(t, dir, "diseno.png", 400, 280)
	certRepo := &fakeCertRepo{templates: map[uint]*models.CertificateTemplate{}, usedBy: map[uint][]string{}}
	indRepo := &fakeInductionRepo{programs: map[uint]*models.InductionProgram{}, blocks: map[uint]*models.InductionBlock{}, surveys: map[uint]*models.Survey{}}
	userRepo := &fakeInductionUserRepo{users: map[uint]*models.User{5: {ID: 5, Name: "Ana María Pérez", Email: "ana@x.com"}}}
	svc := &certificateService{repo: certRepo, inductionRepo: indRepo, userRepo: userRepo, uploadPath: dir}
	return svc, certRepo, indRepo, userRepo, dir
}

func TestCreateTemplate_DeduceOrientacionYCamposPorDefecto(t *testing.T) {
	svc, _, _, _, dir := newCertSvc(t)
	writeDesign(t, dir, "vertical.png", 200, 300)

	horizontal, err := svc.CreateTemplate(1, TemplateInput{Name: "Diseño A", ImageFilename: "diseno.png"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if horizontal.Orientation != "L" || len(horizontal.Fields) != 4 {
		t.Fatalf("apaisada con campos por defecto: %+v", horizontal)
	}
	vertical, err := svc.CreateTemplate(1, TemplateInput{Name: "Diseño B", ImageFilename: "vertical.png"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if vertical.Orientation != "P" {
		t.Fatalf("una imagen más alta que ancha es vertical: %+v", vertical)
	}
}

func TestCreateTemplate_Valida(t *testing.T) {
	svc, _, _, _, _ := newCertSvc(t)
	cases := []TemplateInput{
		{Name: "", ImageFilename: "diseno.png"},
		{Name: "x", ImageFilename: ""},
		{Name: "x", ImageFilename: "no-existe.png"},
		{Name: "x", ImageFilename: "../diseno.png"},
		{Name: "x", ImageFilename: "diseno.png", Fields: []models.CertificateField{{Key: "rarisimo", X: 10, Y: 10, Size: 12}}},
		{Name: "x", ImageFilename: "diseno.png", Fields: []models.CertificateField{{Key: "name", X: 150, Y: 10, Size: 12}}},
	}
	for i, in := range cases {
		if _, err := svc.CreateTemplate(1, in); err == nil {
			t.Errorf("caso %d: se esperaba error de validación", i)
		}
	}
}

func TestCreateTemplate_NormalizaCampos(t *testing.T) {
	svc, _, _, _, _ := newCertSvc(t)
	tpl, err := svc.CreateTemplate(1, TemplateInput{Name: "x", ImageFilename: "diseno.png", Fields: []models.CertificateField{
		{Key: "name", X: 50, Y: 50, Size: 20, Color: "rojo", Align: "X", Font: "Comic"},
		{Key: "text", Text: "  por haber completado  ", X: 50, Y: 60, Size: 12},
		{Key: "date", Text: "no debería quedar", X: 50, Y: 70, Size: 12},
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f := tpl.Fields
	if f[0].Color != "#0f172a" || f[0].Align != "C" || f[0].Font != "Helvetica" {
		t.Fatalf("color, alineación y fuente inválidos deben caer al respaldo: %+v", f[0])
	}
	if f[1].Text != "por haber completado" || f[2].Text != "" {
		t.Fatalf("solo el campo libre conserva texto: %+v", f)
	}
}

func TestRender_ProduceUnPDF(t *testing.T) {
	svc, _, _, _, _ := newCertSvc(t)
	tpl := &models.CertificateTemplate{ImageFilename: "diseno.png", Orientation: "L", Fields: DefaultCertificateFields()}
	pdf, err := svc.render(tpl, certificateData{Name: "Ana María Pérez", Program: "Inducción", Date: "25 de septiembre de 2026", Code: "OBT-ABCD-EFGH"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("el resultado debe ser un PDF")
	}
	// Vista previa con datos de ejemplo, sin plantilla guardada.
	preview, err := svc.Preview(TemplateInput{Name: "x", ImageFilename: "diseno.png"})
	if err != nil || !bytes.HasPrefix(preview, []byte("%PDF")) {
		t.Fatalf("la vista previa debe ser un PDF: %v", err)
	}
}

func TestRender_SinDisenoFalla(t *testing.T) {
	svc, _, _, _, _ := newCertSvc(t)
	tpl := &models.CertificateTemplate{ImageFilename: "borrado.png", Orientation: "L", Fields: DefaultCertificateFields()}
	if _, err := svc.render(tpl, certificateData{}); err == nil {
		t.Fatal("sin la imagen no hay certificado")
	}
}

func passedInvite(programID uint) *models.InductionInvite {
	done := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	return &models.InductionInvite{ID: 7, UserID: 5, Status: models.InductionPassed, ProgramID: &programID, ProgramName: "Inducción", CompletedAt: &done}
}

func TestIssueForCompletion_EmiteUnaVezYGuardaElPDF(t *testing.T) {
	svc, certRepo, indRepo, _, dir := newCertSvc(t)
	tplID := uint(1)
	certRepo.templates[1] = &models.CertificateTemplate{ID: 1, Name: "Diseño A", ImageFilename: "diseno.png", Orientation: "L", Fields: DefaultCertificateFields()}
	indRepo.programs[3] = &models.InductionProgram{ID: 3, Name: "Inducción", IsActive: true, CertificateTemplateID: &tplID}
	invite := passedInvite(3)
	user := &models.User{ID: 5, Name: "Ana María Pérez", Email: "ana@x.com"}

	cert, err := svc.IssueForCompletion(user, invite)
	if err != nil || cert == nil {
		t.Fatalf("emitir: %v %+v", err, cert)
	}
	if cert.Code == "" || len(cert.Code) != 13 || cert.Code[:4] != "OBT-" {
		t.Fatalf("código mal formado: %q", cert.Code)
	}
	if cert.ProgramName != "Inducción" || cert.TemplateName != "Diseño A" || !cert.IssuedAt.Equal(*invite.CompletedAt) {
		t.Fatalf("certificado mal formado: %+v", cert)
	}
	if _, err := os.Stat(filepath.Join(dir, "certificates", cert.Code+".pdf")); err != nil {
		t.Fatalf("el PDF debe quedar guardado: %v", err)
	}
	// Idempotente: emitir de nuevo devuelve el mismo.
	again, err := svc.IssueForCompletion(user, invite)
	if err != nil || again.ID != cert.ID || len(certRepo.certs) != 1 {
		t.Fatalf("no debe emitirse dos veces: %v %+v (%d)", err, again, len(certRepo.certs))
	}
}

// Un programa sin plantilla no certifica, y no es un error.
func TestIssueForCompletion_SinPlantillaNoEmite(t *testing.T) {
	svc, certRepo, indRepo, _, _ := newCertSvc(t)
	indRepo.programs[3] = &models.InductionProgram{ID: 3, Name: "Inducción", IsActive: true}
	cert, err := svc.IssueForCompletion(&models.User{ID: 5, Name: "Ana"}, passedInvite(3))
	if err != nil || cert != nil || len(certRepo.certs) != 0 {
		t.Fatalf("sin plantilla no hay certificado: %v %+v", err, cert)
	}
}

func TestIssueForInvite_SoloAprobadasYConPlantilla(t *testing.T) {
	svc, certRepo, indRepo, _, _ := newCertSvc(t)
	indRepo.programs[3] = &models.InductionProgram{ID: 3, Name: "Inducción", IsActive: true}
	pending := passedInvite(3)
	pending.Status = models.InductionPending
	indRepo.invite = pending
	if _, err := svc.IssueForInvite(7); err == nil {
		t.Fatal("una pendiente no se certifica")
	}
	indRepo.invite = passedInvite(3)
	if _, err := svc.IssueForInvite(7); err == nil {
		t.Fatal("sin plantilla en el programa debe explicarlo")
	}
	tplID := uint(1)
	certRepo.templates[1] = &models.CertificateTemplate{ID: 1, Name: "Diseño A", ImageFilename: "diseno.png", Orientation: "L", Fields: DefaultCertificateFields()}
	indRepo.programs[3].CertificateTemplateID = &tplID
	cert, err := svc.IssueForInvite(7)
	if err != nil || cert == nil {
		t.Fatalf("con plantilla debe emitir: %v", err)
	}
}

func TestVerify_ResuelveElCodigo(t *testing.T) {
	svc, certRepo, _, _, _ := newCertSvc(t)
	certRepo.certs = []models.Certificate{{ID: 1, UserID: 5, Code: "OBT-ABCD-EFGH", ProgramName: "Inducción", IssuedAt: time.Now()}}

	ok, err := svc.Verify(" obt-abcd-efgh ")
	if err != nil || !ok.Valid || ok.ProfessionalName != "Ana María Pérez" || ok.ProgramName != "Inducción" {
		t.Fatalf("debe verificar sin importar mayúsculas ni espacios: %+v %v", ok, err)
	}
	bad, err := svc.Verify("OBT-NADA-NADA")
	if err != nil || bad.Valid {
		t.Fatalf("un código inexistente no verifica: %+v", bad)
	}
}

func TestDeleteTemplate_RechazaSiEstaEnUso(t *testing.T) {
	svc, certRepo, _, _, _ := newCertSvc(t)
	certRepo.templates[1] = &models.CertificateTemplate{ID: 1, Name: "A", ImageFilename: "diseno.png"}
	certRepo.usedBy[1] = []string{"Inducción"}
	if err := svc.DeleteTemplate(1); err == nil {
		t.Fatal("en uso no se borra")
	}
	certRepo.usedBy[1] = nil
	if err := svc.DeleteTemplate(1); err != nil {
		t.Fatalf("sin uso debe borrarse: %v", err)
	}
}

func TestSubmit_CompletarEmiteCertificadoSiHayServicio(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	issuer := &fakeIssuer{}
	svc.certSvc = issuer
	pendingInvite(repo, 3)
	programID := uint(1)
	repo.invite.ProgramID = &programID
	repo.inviteBlocks[0].Status = models.InductionPassed

	res, err := svc.Submit("tok", 12, []SubmittedAnswer{{QuestionID: 81, Value: "b"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !res.Completed || res.Certificate == nil || res.Certificate.Code != "OBT-TEST-0001" {
		t.Fatalf("al completar debe viajar el certificado: %+v", res.Certificate)
	}
	if issuer.issuedFor != 1 {
		t.Fatalf("debe emitirse para la invitación 1, got %d", issuer.issuedFor)
	}
}

type fakeIssuer struct {
	issuedFor uint
}

func (f *fakeIssuer) IssueForCompletion(user *models.User, invite *models.InductionInvite) (*models.Certificate, error) {
	f.issuedFor = invite.ID
	return &models.Certificate{ID: 1, Code: "OBT-TEST-0001", ProgramName: invite.ProgramName, IssuedAt: time.Now()}, nil
}

func (f *fakeIssuer) GetForInvite(inviteID uint) (*models.Certificate, error) {
	if f.issuedFor == inviteID {
		return &models.Certificate{ID: 1, Code: "OBT-TEST-0001", IssuedAt: time.Now()}, nil
	}
	return nil, nil
}
