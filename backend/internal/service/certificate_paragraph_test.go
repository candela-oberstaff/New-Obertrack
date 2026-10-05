package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jung-kurt/gofpdf"

	"github.com/obertrack/backend/internal/models"
)

var paragraphData = certificateData{Name: "Mary Marín", Program: "Prueba de Inducción", Date: "30/09/26", Code: "OBT-ABCD-EFGH"}

// wordText arma una palabra marcando las variables entre corchetes.
func wordText(w paragraphWord) string {
	var b strings.Builder
	for _, sg := range w {
		if sg.variable {
			b.WriteString("[" + sg.text + "]")
		} else {
			b.WriteString(sg.text)
		}
	}
	return b.String()
}

func TestParagraphLines_SustituyeVariables(t *testing.T) {
	lines := paragraphLines("aprobado la {programa}, {nombre}\n{fecha} {codigo} {otra}", paragraphData)
	if len(lines) != 2 {
		t.Fatalf("esperaba 2 líneas escritas a mano, hay %d", len(lines))
	}
	var got []string
	for _, w := range lines[0] {
		got = append(got, wordText(w))
	}
	// Cada palabra del programa es partible; la coma va pegada pero sin resaltar.
	want := []string{"aprobado", "la", "[Prueba]", "[de]", "[Inducción],", "[Mary]", "[Marín]"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("palabras = %q\n esperaba %q", got, want)
	}
	var second []string
	for _, w := range lines[1] {
		second = append(second, wordText(w))
	}
	// Una llave desconocida se deja tal cual, sin resaltar.
	if strings.Join(second, " ") != "[30/09/26] [OBT-ABCD-EFGH] {otra}" {
		t.Fatalf("segunda línea = %q", second)
	}
}

func TestLayoutParagraph_AjustaAlAncho(t *testing.T) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pageW, _ := pdf.GetPageSize()
	st := paragraphStyle{pdf: pdf, font: "Helvetica", size: 13, tr: pdf.UnicodeTranslatorFromDescriptor("")}
	f := models.CertificateField{
		Key:  models.CertificateFieldText,
		Text: "Por haber completado y aprobado satisfactoriamente la {programa}, demostrando las competencias, conocimientos y alineación requeridos.",
	}

	f.Wrap = 0
	if n := len(layoutParagraph(st, f, paragraphData, pageW)); n != 1 {
		t.Fatalf("sin ancho debe ir en una línea, salen %d", n)
	}

	f.Wrap = 55
	lines := layoutParagraph(st, f, paragraphData, pageW)
	if len(lines) < 2 || len(lines) > 4 {
		t.Fatalf("con 55%% del ancho esperaba 2-4 líneas, salen %d", len(lines))
	}
	for i, ln := range lines {
		if ln.width > pageW*0.55+0.01 {
			t.Errorf("la línea %d mide %.1f mm, más que el ancho del párrafo (%.1f)", i, ln.width, pageW*0.55)
		}
	}
}

// El párrafo sale en el PDF también con Poppins y sus variables.
func TestRender_ParrafoConVariables(t *testing.T) {
	svc, _, _, _, _ := newCertSvc(t)
	tpl := &models.CertificateTemplate{ImageFilename: "diseno.png", Orientation: "L", Fields: []models.CertificateField{
		{Key: models.CertificateFieldText, X: 50, Y: 60, Size: 13, Color: "#0f172a", Align: "C", Font: "Poppins", Wrap: 55, Highlight: "#fa3ab4",
			Text: "Por haber completado y aprobado satisfactoriamente la {programa}, demostrando las competencias requeridas."},
	}}
	pdf, err := svc.render(tpl, paragraphData)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Contains(pdf, []byte("/FontName /utf8poppinsB")) {
		t.Fatal("las variables van en semibold: falta Poppins semibold en el PDF")
	}
}

func TestCreateTemplate_TextoLibreAnchoYResaltado(t *testing.T) {
	svc, _, _, _, _ := newCertSvc(t)
	tpl, err := svc.CreateTemplate(1, TemplateInput{Name: "x", ImageFilename: "diseno.png", Fields: []models.CertificateField{
		{Key: "text", Text: "la {programa}", X: 50, Y: 60, Size: 13, Wrap: 200, Highlight: "#fa3ab4"},
		{Key: "text", Text: "otro", X: 50, Y: 70, Size: 13, Wrap: 3, Highlight: "rosa"},
		{Key: "name", X: 50, Y: 48, Size: 32, Wrap: 50, Highlight: "#fa3ab4"},
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	a, b, name := tpl.Fields[0], tpl.Fields[1], tpl.Fields[2]
	if a.Wrap != paragraphMaxWrap || a.Highlight != "#fa3ab4" || a.Text != "la {programa}" {
		t.Errorf("ancho acotado arriba y variables conservadas: %+v", a)
	}
	if b.Wrap != paragraphMinWrap || b.Highlight != "" {
		t.Errorf("ancho acotado abajo y color inválido descartado: %+v", b)
	}
	if name.Wrap != 0 || name.Highlight != "" {
		t.Errorf("solo el texto libre lleva ancho y resaltado: %+v", name)
	}
}
