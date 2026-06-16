package service

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/obertrack/backend/internal/models"
)

// Paleta Obertrack para los PDFs del expediente.
var (
	pdfPrussian = [3]int{6, 11, 35}     // banner
	pdfViolet   = [3]int{138, 43, 226}  // acentos
	pdfGray     = [3]int{100, 116, 139} // texto secundario
	pdfDark     = [3]int{15, 23, 42}    // texto principal
	pdfCardBg   = [3]int{245, 242, 251} // fondo tarjetas
	pdfBorder   = [3]int{221, 217, 239} // bordes
)

func pdfLocateLogo() string {
	for _, p := range []string{
		"../frontend/public/logos/Horizontal_Blanco.png",
		"frontend/public/logos/Horizontal_Blanco.png",
		"public/logos/Horizontal_Blanco.png",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// pdfBanner dibuja el encabezado común (logo + título + subtítulo).
func pdfBanner(pdf *gofpdf.Fpdf, tr func(string) string, title, subtitle string) {
	pdf.SetFillColor(pdfPrussian[0], pdfPrussian[1], pdfPrussian[2])
	pdf.Rect(0, 0, 210, 32, "F")

	if logo := pdfLocateLogo(); logo != "" {
		pdf.Image(logo, 15, 9, 42, 0, false, "", 0, "")
	} else {
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 16)
		pdf.Text(15, 20, "OBERTRACK")
	}

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 15)
	pdf.Text(75, 16, tr(title))
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(245, 242, 251)
	pdf.Text(75, 24, tr(subtitle))

	pdf.SetY(40)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
}

// pdfSection dibuja un título de sección con una barra de color.
func pdfSection(pdf *gofpdf.Fpdf, tr func(string) string, text string) {
	pdf.Ln(3)
	y := pdf.GetY()
	pdf.SetFillColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
	pdf.Rect(15, y+1, 2.5, 5, "F")
	pdf.SetX(20)
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.CellFormat(0, 7, tr(text), "", 1, "L", false, 0, "")
	pdf.Ln(1)
}

func pdfDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("02/01/2006")
}

func pdfHours(n float64) string { return fmt.Sprintf("%.1f h", n) }

// generateCVPDF arma el PDF del CV vivo del profesional (trayectoria + lo
// compartido por cada empresa).
func generateCVPDF(cv *CVView, name, email string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdfBanner(pdf, tr, "TRAYECTORIA PROFESIONAL", "Curriculum vivo - Obertrack")

	// Identidad
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.CellFormat(0, 9, tr(name), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.CellFormat(0, 6, tr(email), "", 1, "L", false, 0, "")

	// Agregados
	pdf.Ln(2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 7, tr(fmt.Sprintf("%d empresas  -  %d en curso  -  %s de trayectoria",
		cv.TotalCompanies, cv.ActiveCompanies, pdfHumanDuration(cv.TotalDays))), "", 1, "L", false, 0, "")

	for _, e := range cv.Entries {
		emp := e.Employment
		estado := "Finalizado"
		periodoFin := pdfDate(deref(emp.EndedAt))
		if emp.Status == models.EmploymentActive {
			estado = "Actual"
			periodoFin = "actualidad"
		}

		pdfSection(pdf, tr, fmt.Sprintf("%s  [%s]", emp.CompanyName, estado))

		pdf.SetX(20)
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
		linea := fmt.Sprintf("%s  -  %s a %s  -  %s",
			orDash(emp.JobTitle), pdfDate(emp.StartedAt), periodoFin, pdfHumanDuration(e.Summary.DaysEmployed))
		pdf.MultiCell(0, 5, tr(linea), "", "L", false)

		pdf.SetX(20)
		pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
		pdf.CellFormat(0, 6, tr(fmt.Sprintf("Horas aprobadas: %s   |   Tareas: %d/%d",
			pdfHours(e.Summary.ApprovedHours), e.Summary.TasksCompleted, e.Summary.TasksAssigned)), "", 1, "L", false, 0, "")

		// Evaluaciones compartidas
		for _, n := range e.Notes {
			pdf.SetX(20)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
			stars := ""
			if n.Rating != nil && *n.Rating > 0 {
				for i := 0; i < *n.Rating; i++ {
					stars += "*"
				}
				stars = " (" + stars + ")"
			}
			pdf.CellFormat(0, 5, tr(fmt.Sprintf("Evaluacion%s - %s", stars, n.AuthorName)), "", 1, "L", false, 0, "")
			pdf.SetX(20)
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.MultiCell(0, 5, tr(n.Content), "", "L", false)
		}

		// Documentos compartidos
		if len(e.Documents) > 0 {
			pdf.SetX(20)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
			pdf.CellFormat(0, 5, tr("Documentos compartidos:"), "", 1, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			for _, d := range e.Documents {
				pdf.SetX(22)
				pdf.MultiCell(0, 5, tr("- "+orDash(firstNonEmpty(d.Title, d.FileName))), "", "L", false)
			}
		}
	}

	pdfFooter(pdf, tr)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// generateExpedientePDF arma el PDF del expediente completo de un empleo
// (audiencia empresa: incluye todo).
func generateExpedientePDF(exp *ExpedienteView, professionalName string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	emp := exp.Employment
	pdfBanner(pdf, tr, "EXPEDIENTE LABORAL", emp.CompanyName)

	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.CellFormat(0, 9, tr(professionalName), "", 1, "L", false, 0, "")

	estado := "Empleo activo"
	if emp.Status == models.EmploymentEnded {
		estado = "Empleo finalizado"
	}
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("%s  -  %s", orDash(emp.JobTitle), estado)), "", 1, "L", false, 0, "")

	// Datos del empleo
	pdfSection(pdf, tr, "Datos del empleo")
	pdfKV(pdf, tr, "Empresa", emp.CompanyName)
	pdfKV(pdf, tr, "Cargo", orDash(emp.JobTitle))
	pdfKV(pdf, tr, "Manager", orDash(emp.ManagerName))
	pdfKV(pdf, tr, "Inicio", pdfDate(emp.StartedAt))
	if emp.StartReason != "" {
		pdfKV(pdf, tr, "Motivo de ingreso", emp.StartReason)
	}
	if emp.Status == models.EmploymentEnded {
		pdfKV(pdf, tr, "Fin", pdfDate(deref(emp.EndedAt)))
		pdfKV(pdf, tr, "Motivo de salida", orDash(emp.EndReason))
	}

	// Resumen
	titulo := "Resumen (en vivo)"
	if exp.Summary.FrozenAt != nil {
		titulo = "Resumen (congelado al salir)"
	}
	pdfSection(pdf, tr, titulo)
	pdfKV(pdf, tr, "Antiguedad", pdfHumanDuration(exp.Summary.DaysEmployed))
	pdfKV(pdf, tr, "Horas totales", pdfHours(exp.Summary.TotalHours))
	pdfKV(pdf, tr, "Horas aprobadas", pdfHours(exp.Summary.ApprovedHours))
	pdfKV(pdf, tr, "Tareas", fmt.Sprintf("%d / %d completadas", exp.Summary.TasksCompleted, exp.Summary.TasksAssigned))
	pdfKV(pdf, tr, "Ausencias", fmt.Sprintf("%d", exp.Summary.Absences))

	// Evaluaciones y notas
	pdfSection(pdf, tr, "Evaluaciones y notas")
	if len(exp.Notes) == 0 {
		pdfMuted(pdf, tr, "Sin evaluaciones ni notas.")
	}
	for _, n := range exp.Notes {
		vis := "privada"
		if n.Visibility == models.ExpedienteShared {
			vis = "compartida"
		}
		stars := ""
		if n.Rating != nil && *n.Rating > 0 {
			for i := 0; i < *n.Rating; i++ {
				stars += "*"
			}
			stars = " (" + stars + ")"
		}
		kind := "Nota"
		if n.Kind == models.NoteKindEvaluation {
			kind = "Evaluacion"
		}
		pdf.SetX(20)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
		pdf.CellFormat(0, 5, tr(fmt.Sprintf("%s%s - %s - %s [%s]", kind, stars, n.AuthorName, pdfDate(n.CreatedAt), vis)), "", 1, "L", false, 0, "")
		pdf.SetX(20)
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
		pdf.MultiCell(0, 5, tr(n.Content), "", "L", false)
	}

	// Ausencias
	if len(exp.Absences) > 0 {
		pdfSection(pdf, tr, "Ausencias")
		for _, a := range exp.Absences {
			estado := "pendiente"
			if a.Approved {
				estado = "justificada"
			}
			pdfBullet(pdf, tr, fmt.Sprintf("%s - %s - %.1f h [%s]", pdfDate(a.Date), orDash(a.Reason), a.Hours, estado))
		}
	}

	// Gestiones de CS
	if len(exp.Gestiones) > 0 {
		pdfSection(pdf, tr, "Gestiones de seguimiento")
		for _, g := range exp.Gestiones {
			pdfBullet(pdf, tr, fmt.Sprintf("%s: %s - %s - %s", g.Kind, g.Status, orDash(g.ByName), pdfDate(g.CreatedAt)))
		}
	}

	// Contactos
	if len(exp.Contactos) > 0 {
		pdfSection(pdf, tr, "Contactos")
		for _, c := range exp.Contactos {
			pdfBullet(pdf, tr, fmt.Sprintf("%s - %s - %s", c.Channel, orDash(c.ByName), pdfDate(c.CreatedAt)))
		}
	}

	// Documentos
	pdfSection(pdf, tr, "Documentos")
	if len(exp.Documents) == 0 {
		pdfMuted(pdf, tr, "Sin documentos adjuntos.")
	}
	for _, d := range exp.Documents {
		vis := "privado"
		if d.Visibility == models.ExpedienteShared {
			vis = "compartido"
		}
		pdfBullet(pdf, tr, fmt.Sprintf("%s [%s] - %s", orDash(firstNonEmpty(d.Title, d.FileName)), vis, pdfDate(d.CreatedAt)))
	}

	pdfFooter(pdf, tr)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- helpers de layout ---

func pdfKV(pdf *gofpdf.Fpdf, tr func(string) string, k, v string) {
	pdf.SetX(20)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.CellFormat(45, 5.5, tr(k), "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.MultiCell(0, 5.5, tr(v), "", "L", false)
}

func pdfBullet(pdf *gofpdf.Fpdf, tr func(string) string, text string) {
	pdf.SetX(20)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.MultiCell(0, 5, tr("- "+text), "", "L", false)
}

func pdfMuted(pdf *gofpdf.Fpdf, tr func(string) string, text string) {
	pdf.SetX(20)
	pdf.SetFont("Arial", "I", 9)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.CellFormat(0, 5, tr(text), "", 1, "L", false, 0, "")
}

func pdfFooter(pdf *gofpdf.Fpdf, tr func(string) string) {
	pdf.Ln(6)
	pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(2)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.CellFormat(0, 5, tr(fmt.Sprintf("Generado por Obertrack el %s", time.Now().Format("02/01/2006 15:04"))), "", 1, "L", false, 0, "")
}

func pdfHumanDuration(days int) string {
	if days >= 365 {
		y := days / 365
		m := (days % 365) / 30
		if m > 0 {
			return fmt.Sprintf("%d a %d m", y, m)
		}
		return fmt.Sprintf("%d ano(s)", y)
	}
	if days >= 30 {
		return fmt.Sprintf("%d mes(es)", days/30)
	}
	return fmt.Sprintf("%d dia(s)", days)
}

func deref(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

