package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/obertrack/backend/internal/models"
)

// ConsentPDF emite la constancia de autorización firmada. Es el documento que
// se archiva y el que se enseña si alguien pregunta con qué permiso se publicó
// un testimonio.
//
// Se genera al vuelo desde la evidencia guardada en la fila, no se almacena:
// así no hay un archivo que pueda quedar desincronizado de los datos, y la
// constancia siempre refleja el estado real del registro.
func (s *testimonialService) ConsentPDF(id uint) ([]byte, string, error) {
	t, err := s.repo.GetByID(id)
	if err != nil {
		return nil, "", errors.New("testimonio no encontrado")
	}
	if !t.Signed() {
		return nil, "", errors.New("este testimonio todavía no está firmado")
	}

	// El trazo se incrusta desde el archivo. Si falta (volumen recreado, borrado
	// manual), la constancia se emite igual dejando constancia de la ausencia:
	// el resto de la evidencia sigue siendo válida y ocultarla sería peor.
	var signature []byte
	if t.SignatureImage != "" {
		if raw, err := os.ReadFile(s.signaturePath(t.SignatureImage)); err == nil {
			signature = raw
		}
	}

	pdf, err := buildTestimonialConsentPDF(t, signature)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("consentimiento-testimonio-%d-%s.pdf", t.ID, pdfSlug(t.RecipientName))
	return pdf, filename, nil
}

func buildTestimonialConsentPDF(t *models.Testimonial, signature []byte) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdfBanner(pdf, tr, "AUTORIZACIÓN DE TESTIMONIO", "Constancia de firma electrónica")

	// --- Quién firma ---
	pdfSection(pdf, tr, "Firmante")
	pdfKV(pdf, tr, "Nombre", orDash(t.RecipientName))
	pdfKV(pdf, tr, "Correo", orDash(t.RecipientEmail))
	if t.RecipientRole != "" {
		pdfKV(pdf, tr, "Cargo", t.RecipientRole)
	}
	if t.RecipientCompany != "" {
		pdfKV(pdf, tr, "Empresa", t.RecipientCompany)
	}
	pdfKV(pdf, tr, "Tipo", testimonialAudienceLabel(t.Audience))

	// --- El texto autorizado ---
	pdfSection(pdf, tr, "Autorización otorgada")
	pdf.SetX(20)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.MultiCell(0, 5, tr(t.ConsentText), "", "J", false)
	pdf.Ln(1)
	pdfMuted(pdf, tr, fmt.Sprintf("Redacción %s", orDash(t.ConsentVersion)))

	// --- Qué autorizó a mostrar ---
	pdfSection(pdf, tr, "Alcance del permiso")
	for _, item := range []struct {
		label   string
		granted bool
	}{
		{"Publicar su nombre", t.AllowPublicName},
		{"Publicar su cargo y empresa", t.AllowRole},
		{"Publicar su fotografía", t.AllowPhoto},
		{"Publicar el logo de la empresa", t.AllowLogo},
	} {
		mark := "NO"
		if item.granted {
			mark = "SÍ"
		}
		pdfBullet(pdf, tr, fmt.Sprintf("%s: %s", item.label, mark))
	}

	// --- El testimonio tal como se recibió ---
	pdfSection(pdf, tr, "Testimonio recibido")
	if t.Rating > 0 {
		pdfKV(pdf, tr, "Calificación", fmt.Sprintf("%d de 5", t.Rating))
	}
	pdf.SetX(20)
	pdf.SetFont("Arial", "I", 10)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.MultiCell(0, 5.5, tr(pdfQuoted(t.Quote)), "", "J", false)

	// Las respuestas a las preguntas guía son el material de origen de la cita:
	// van en la constancia para que se vea de dónde salió cada frase.
	if answers := decodeTestimonialAnswers(t.Answers); len(answers) > 0 {
		pdf.Ln(2)
		for _, a := range answers {
			if strings.TrimSpace(a.Answer) == "" {
				continue
			}
			pdf.SetX(20)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
			pdf.MultiCell(0, 5, tr(a.Prompt), "", "L", false)
			pdf.SetX(20)
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.MultiCell(0, 5, tr(a.Answer), "", "L", false)
			pdf.Ln(1)
		}
	}

	// --- Firma y evidencia ---
	pdfSection(pdf, tr, "Firma electrónica")

	if len(signature) > 0 {
		// gofpdf registra la imagen desde un lector con nombre propio; el nombre
		// solo identifica el recurso dentro del PDF.
		pdf.RegisterImageOptionsReader(
			"firma", gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(signature),
		)
		y := pdf.GetY()
		pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
		pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
		pdf.Rect(20, y, 90, 30, "FD")
		// Alto 0 = gofpdf calcula la proporción a partir del ancho.
		pdf.ImageOptions("firma", 23, y+3, 84, 24, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		pdf.SetY(y + 32)
	} else {
		pdfMuted(pdf, tr, "El trazo de la firma no está disponible en el almacenamiento.")
	}

	pdfKV(pdf, tr, "Firmado por", orDash(t.SignatureName))
	pdfKV(pdf, tr, "Modalidad", t.SignatureModeLabel())
	pdfKV(pdf, tr, "Fecha y hora", pdfDateTime(t.SignedAt))
	pdfKV(pdf, tr, "Dirección IP", orDash(t.SignerIP))
	pdfKV(pdf, tr, "Navegador", orDash(t.SignerUserAgent))
	pdfKV(pdf, tr, "Referencia", fmt.Sprintf("TST-%06d", t.ID))

	pdf.Ln(2)
	pdfMuted(pdf, tr,
		"Firma electrónica simple. La autorización se envió al correo registrado del firmante y")
	pdfMuted(pdf, tr,
		"se aceptó desde el enlace personal de ese envío. Los datos de arriba son su evidencia.")

	pdfFooter(pdf, tr)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeTestimonialAnswers lee el JSON de respuestas. Un JSON corrupto no debe
// impedir emitir la constancia: se devuelve vacío y el resto se imprime igual.
func decodeTestimonialAnswers(raw string) []TestimonialAnswer {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []TestimonialAnswer
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func testimonialAudienceLabel(audience string) string {
	if tpl, ok := testimonialTemplateFor(audience); ok {
		return tpl.Label
	}
	return audience
}

func pdfDateTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("02/01/2006 15:04")
}

func pdfQuoted(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return "“" + s + "”"
}

// pdfSlug arma un trozo de nombre de archivo seguro a partir del nombre del
// firmante.
func pdfSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "firmante"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}
