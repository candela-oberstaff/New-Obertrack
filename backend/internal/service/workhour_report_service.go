package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"

	"github.com/obertrack/backend/internal/models"
)

// Las actividades vienen del editor de texto enriquecido como HTML (tiptap:
// <p data-path-to-node…>, <div style="--tw-…">). Para los reportes se
// convierten a texto plano legible: saltos de línea en bloques, viñetas en
// listas y entidades decodificadas.
var (
	reHTMLBreak = regexp.MustCompile(`(?i)<br\s*/?>`)
	reHTMLLi    = regexp.MustCompile(`(?i)<li[^>]*>`)
	// Apertura y cierre de bloques generan salto de línea (los <div> anidados de
	// tiptap venían pegados: "llamadas<div>PS</div>" debe ser dos líneas).
	reHTMLBlock = regexp.MustCompile(`(?i)</?(p|div|li|ul|ol|h[1-6]|tr|blockquote)(\s[^>]*)?>`)
	reHTMLTag   = regexp.MustCompile(`<[^>]*>`)
	reSpaces    = regexp.MustCompile(`[ \t]+`)
	reBlankNL   = regexp.MustCompile(`\s*\n\s*`)
)

func htmlToPlainText(s string) string {
	if strings.ContainsAny(s, "<&") {
		s = reHTMLBreak.ReplaceAllString(s, "\n")
		s = reHTMLLi.ReplaceAllString(s, "\n- ")
		s = reHTMLBlock.ReplaceAllString(s, "\n")
		s = reHTMLTag.ReplaceAllString(s, "")
		s = html.UnescapeString(s)
	}
	s = reSpaces.ReplaceAllString(s, " ")
	s = reBlankNL.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}

// toLatin1Safe descarta las runas fuera de Latin-1 (emojis, símbolos): gofpdf
// las pintaría como "." y SplitText entra en pánico con runas altas al indexar
// la tabla de anchos de 256 entradas de las fuentes core.
func toLatin1Safe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r <= 0xFF {
			b.WriteRune(r)
		}
	}
	// Al quitar un emoji quedan espacios dobles; se colapsan de nuevo.
	return strings.TrimSpace(reSpaces.ReplaceAllString(b.String(), " "))
}

// humanizeReason convierte el slug del motivo de ausencia ("cita_medica") en
// texto presentable ("Cita medica") para los reportes.
func humanizeReason(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "_", " "))
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func (s *workHourService) getReportWorkHours(userID uint, role string, isSuperadmin, isManager bool, tenantID uint, month int, year int, companyFilter uint) ([]models.WorkHour, string, error) {
	monthsEs := []string{
		"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
		"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre",
	}
	monthName := monthsEs[month-1]

	startDateStr := fmt.Sprintf("%d-%02d-01", year, month)
	t := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
	endDateStr := fmt.Sprintf("%d-%02d-%02d", year, month, t.Day())

	filters := make(map[string]interface{})
	switch {
	case isSuperadmin:
		// Superadmin must scope to a company; without it, no data so we never mix
		// tenants in a report.
		if companyFilter == 0 {
			return []models.WorkHour{}, monthName, nil
		}
		filters["tenant_id"] = companyFilter
	case isManager:
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
		if MultiManagerReadsEnabled() {
			filters["manager_or_user_links_id"] = userID
		} else {
			filters["manager_or_user_id"] = userID
		}
	default:
		filters["tenant_id"] = tenantID
	}

	if start, err := time.Parse("2006-01-02", startDateStr); err == nil {
		filters["start_date"] = start
	}
	if end, err := time.Parse("2006-01-02", endDateStr); err == nil {
		filters["end_date"] = end
	}

	workHours, _, err := s.repo.FindAll(filters, 0, 1000)
	if err != nil {
		return nil, "", err
	}

	return workHours, monthName, nil
}

func (s *workHourService) GetPDFReportBytes(userID uint, role string, isSuperadmin, isManager bool, tenantID uint, month int, year int, companyFilter uint) ([]byte, string, error) {
	workHours, monthName, err := s.getReportWorkHours(userID, role, isSuperadmin, isManager, tenantID, month, year, companyFilter)
	if err != nil {
		return nil, "", err
	}
	pdfBytes, err := generatePDFReport(workHours, fmt.Sprintf("%s %d", monthName, year))
	if err != nil {
		return nil, "", err
	}
	return pdfBytes, monthName, nil
}

func (s *workHourService) GetExcelReportBytes(userID uint, role string, isSuperadmin, isManager bool, tenantID uint, month int, year int, companyFilter uint) ([]byte, string, error) {
	workHours, monthName, err := s.getReportWorkHours(userID, role, isSuperadmin, isManager, tenantID, month, year, companyFilter)
	if err != nil {
		return nil, "", err
	}
	excelBytes, err := generateExcelReport(workHours, fmt.Sprintf("%s %d", monthName, year))
	if err != nil {
		return nil, "", err
	}
	return excelBytes, monthName, nil
}

// SendReportEmail mantiene la firma que usa el botón manual de /work-hours.
// Resuelve los bordes del mes y delega en SendPeriodReport para no duplicar la
// construcción del correo ni de los adjuntos.
func (s *workHourService) SendReportEmail(userID uint, role string, isSuperadmin, isManager bool, tenantID uint, month int, year int, companyFilter uint) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	workHours, monthName, err := s.getReportWorkHours(userID, role, isSuperadmin, isManager, tenantID, month, year, companyFilter)
	if err != nil {
		return err
	}

	label := fmt.Sprintf("%s %d", monthName, year)
	return s.sendReportWithAttachments(user, workHours, "Reporte Mensual de Jornadas", label)
}

// reportWorkHoursInRange trae las jornadas de una empresa en un rango de fechas
// arbitrario. El repositorio ya soporta start_date/end_date.
func (s *workHourService) reportWorkHoursInRange(tenantID uint, start, end time.Time) ([]models.WorkHour, error) {
	filters := map[string]interface{}{
		"tenant_id":  tenantID,
		"start_date": start,
		"end_date":   end,
	}
	workHours, _, err := s.repo.FindAll(filters, 0, 1000)
	return workHours, err
}

// SendPeriodReport envía el reporte de una empresa para un rango arbitrario.
// Lo usa el worker de envíos automáticos (diario / semanal / mensual).
func (s *workHourService) SendPeriodReport(recipient *models.User, tenantID uint, periodTitle, periodLabel string, start, end time.Time) error {
	if recipient == nil {
		return fmt.Errorf("destinatario inválido")
	}
	workHours, err := s.reportWorkHoursInRange(tenantID, start, end)
	if err != nil {
		return err
	}
	return s.sendReportWithAttachments(recipient, workHours, periodTitle, periodLabel)
}

// sendReportWithAttachments arma el correo branded, genera PDF + Excel y envía.
// periodTitle es el encabezado ("Reporte Diario de Jornadas") y periodLabel el
// período legible ("08/07/2026", "Julio 2026").
func (s *workHourService) sendReportWithAttachments(user *models.User, workHours []models.WorkHour, periodTitle, periodLabel string) error {
	var totalHours float64
	var approvedHours float64
	var totalAbsences int
	var totalAbsenceHours float64

	for _, wh := range workHours {
		totalHours += wh.HoursWorked
		if wh.Approved {
			approvedHours += wh.HoursWorked
		}
		if wh.WorkType == models.WorkTypeAbsence {
			totalAbsences++
			totalAbsenceHours += wh.AbsenceHours
		}
	}

	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
</head>
<body style="font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f2fb; margin: 0; padding: 20px; color: #060b23;">
	<div style="max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 20px; overflow: hidden; box-shadow: 0 10px 15px -3px rgba(6, 11, 35, 0.1), 0 4px 6px -2px rgba(6, 11, 35, 0.05); border: 1px solid #ddd9ef;">
		
		<!-- Banner Superior con Degradado de Obertrack (Prussian Blue a Orchid) -->
		<div style="background: linear-gradient(135deg, #060b23 0%%, #cc33cc 100%%); padding: 32px 24px; color: #ffffff; text-align: center;">
			<img src="https://obertrack.com/logos/Horizontal_Blanco.png" alt="Obertrack Logo" height="40" style="display: block; margin: 0 auto 12px auto; height: 40px; border: 0; outline: none;" />
			<h1 style="font-size: 20px; font-weight: 700; opacity: 0.95; margin: 0; color: #ffffff; font-family: sans-serif; letter-spacing: -0.01em;">%s</h1>
			<div style="font-size: 14px; opacity: 0.85; margin-top: 6px; color: #f5f2fb; font-family: sans-serif;">%s</div>
		</div>

		<!-- Contenido Principal -->
		<div style="padding: 32px 24px;">
			<p style="font-size: 16px; line-height: 1.6; margin-bottom: 24px; color: #060b23; font-family: sans-serif;">Hola <strong>%s</strong>,</p>
			<p style="font-size: 15px; line-height: 1.6; margin-bottom: 24px; color: #5c5680; font-family: sans-serif;">Aquí tienes el informe de horas y asistencia consolidado de tu equipo correspondiente a <strong>%s</strong>.</p>
			
			<!-- Rejilla de Estadísticas compatible con Outlook (Tablas) -->
			<table cellpadding="0" cellspacing="0" border="0" width="100%%" style="width: 100%%; margin-bottom: 32px; table-layout: fixed;">
				<tr>
					<td width="32%%" style="background: #f5f2fb; border: 1px solid #ddd9ef; border-radius: 12px; padding: 16px; text-align: center; font-family: sans-serif;">
						<span style="font-size: 11px; font-weight: 700; color: #8880a8; text-transform: uppercase; letter-spacing: 0.05em; display: block; margin-bottom: 4px;">Horas Totales</span>
						<h2 style="font-size: 22px; font-weight: 800; color: #8a2be2; margin: 0;">%.1f h</h2>
					</td>
					<td width="2%%">&nbsp;</td>
					<td width="32%%" style="background: #f5f2fb; border: 1px solid #ddd9ef; border-radius: 12px; padding: 16px; text-align: center; font-family: sans-serif;">
						<span style="font-size: 11px; font-weight: 700; color: #8880a8; text-transform: uppercase; letter-spacing: 0.05em; display: block; margin-bottom: 4px;">Aprobadas</span>
						<h2 style="font-size: 22px; font-weight: 800; color: #10b981; margin: 0;">%.1f h</h2>
					</td>
					<td width="2%%">&nbsp;</td>
					<td width="32%%" style="background: #f5f2fb; border: 1px solid #ddd9ef; border-radius: 12px; padding: 16px; text-align: center; font-family: sans-serif;">
						<span style="font-size: 11px; font-weight: 700; color: #8880a8; text-transform: uppercase; letter-spacing: 0.05em; display: block; margin-bottom: 4px;">Ausencias</span>
						<h2 style="font-size: 22px; font-weight: 800; color: #ef4444; margin: 0;">%d <span style="font-size: 14px; font-weight: 600; color: #8880a8;">(%.1f h)</span></h2>
					</td>
				</tr>
			</table>

			<p style="font-size: 13px; line-height: 1.6; margin: 0; color: #8880a8; font-family: sans-serif;">El detalle completo de actividades va en los archivos adjuntos (PDF y Excel).</p>
		</div>

		<!-- Footer con Estilos Inline de la Marca -->
		<div style="background: #f5f2fb; padding: 24px; text-align: center; font-size: 12px; color: #8880a8; border-top: 1px solid #ddd9ef; font-family: sans-serif;">
			Este es un informe automático generado de forma segura para ti por <strong>Obertrack</strong>.<br>
			&copy; 2026 Obertrack. Todos los derechos reservados.
		</div>
	</div>
</body>
</html>`, periodTitle, periodLabel, html.EscapeString(user.Name), periodLabel, totalHours, approvedHours, totalAbsences, totalAbsenceHours)

	subject := fmt.Sprintf("Obertrack - Reporte de Jornadas (%s)", periodLabel)

	pdfBytes, err := generatePDFReport(workHours, periodLabel)
	if err != nil {
		return fmt.Errorf("failed to generate PDF attachment: %w", err)
	}

	excelBytes, err := generateExcelReport(workHours, periodLabel)
	if err != nil {
		return fmt.Errorf("failed to generate Excel attachment: %w", err)
	}

	slug := reportFileSlug(periodLabel)
	attachments := []BrevoAttachment{
		{
			Name:    fmt.Sprintf("reporte_jornadas_%s.pdf", slug),
			Content: base64.StdEncoding.EncodeToString(pdfBytes),
		},
		{
			Name:    fmt.Sprintf("reporte_jornadas_%s.xlsx", slug),
			Content: base64.StdEncoding.EncodeToString(excelBytes),
		},
	}

	return s.brevoSvc.SendEmailWithAttachments(user.Email, user.Name, subject, htmlContent, attachments)
}

// reportFileSlug convierte "01/07 al 07/07/2026" en algo apto para un nombre de
// archivo adjunto.
func reportFileSlug(label string) string {
	repl := strings.NewReplacer(" ", "_", "/", "-", ":", "-", "\\", "-")
	return repl.Replace(strings.TrimSpace(label))
}

func generateExcelReport(workHours []models.WorkHour, periodLabel string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Reporte de Horas"
	f.SetSheetName("Sheet1", sheetName)

	// A1 Title styled with brand color (Prussian Blue)
	f.SetCellValue(sheetName, "A1", fmt.Sprintf("Reporte de Actividades - %s", periodLabel))
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Plus Jakarta Sans",
			Bold:   true,
			Size:   16,
			Color:  "060B23",
		},
		Alignment: &excelize.Alignment{
			Vertical: "center",
		},
	})
	f.SetCellStyle(sheetName, "A1", "H1", titleStyle)
	f.SetRowHeight(sheetName, 1, 35)

	// A3 Headers
	headers := []string{"Profesional", "Fecha", "Tipo de Jornada", "Jornada (Horas)", "Ausencias (Horas)", "Motivo de Ausencia", "Actividades Realizadas", "Estado"}
	for i, h := range headers {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetCellValue(sheetName, fmt.Sprintf("%s3", colName), h)
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Plus Jakarta Sans",
			Bold:   true,
			Size:   10,
			Color:  "5C5680",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"F5F2FB"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "top", Color: "DDD9EF", Style: 1},
			{Type: "bottom", Color: "DDD9EF", Style: 1},
			{Type: "left", Color: "DDD9EF", Style: 1},
			{Type: "right", Color: "DDD9EF", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	f.SetCellStyle(sheetName, "A3", "H3", headerStyle)
	f.SetRowHeight(sheetName, 3, 26)

	rowIdx := 4
	for _, wh := range workHours {
		userLabel := ""
		if wh.User.ID != 0 {
			userLabel = wh.User.Name
		}
		
		workTypeLabel := "Día Completo"
		if wh.WorkType == models.WorkTypeAbsence {
			workTypeLabel = "Ausencia"
		}
		
		statusLabel := "Pendiente"
		if wh.Approved {
			statusLabel = "Aprobado"
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowIdx), userLabel)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowIdx), wh.WorkDate.Format("2006-01-02"))
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowIdx), workTypeLabel)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowIdx), wh.HoursWorked)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowIdx), wh.AbsenceHours)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowIdx), humanizeReason(htmlToPlainText(wh.AbsenceReason)))
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowIdx), htmlToPlainText(wh.Activities))
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowIdx), statusLabel)

		// Stylings for cells
		var fillCol string
		if rowIdx%2 == 0 {
			fillCol = "F0EEF8" // alternating Lavender Gray background
		} else {
			fillCol = "FFFFFF"
		}

		cellStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Family: "Plus Jakarta Sans",
				Size:   9,
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{fillCol},
				Pattern: 1,
			},
			Border: []excelize.Border{
				{Type: "bottom", Color: "DDD9EF", Style: 1},
			},
			Alignment: &excelize.Alignment{
				Vertical: "center",
				WrapText: true,
			},
		})
		
		// Status-specific cell styling for cell H
		var statusFontColor, statusBgColor string
		if wh.Approved {
			statusFontColor = "10B981" // Green
			statusBgColor = "DCFCE7"
		} else {
			statusFontColor = "F59E0B" // Orange
			statusBgColor = "FEF3C7"
		}

		statusStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Family: "Plus Jakarta Sans",
				Bold:   true,
				Size:   9,
				Color:  statusFontColor,
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{statusBgColor},
				Pattern: 1,
			},
			Border: []excelize.Border{
				{Type: "bottom", Color: "DDD9EF", Style: 1},
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
		})

		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", rowIdx), fmt.Sprintf("G%d", rowIdx), cellStyle)
		f.SetCellStyle(sheetName, fmt.Sprintf("H%d", rowIdx), fmt.Sprintf("H%d", rowIdx), statusStyle)
		f.SetRowHeight(sheetName, rowIdx, 20)

		rowIdx++
	}

	cols, _ := f.GetCols(sheetName)
	for i, col := range cols {
		maxLen := 0
		for _, cell := range col {
			if len(cell) > maxLen {
				maxLen = len(cell)
			}
		}
		// Tope de ancho: las actividades largas se envuelven (WrapText) en vez de
		// estirar la columna a lo absurdo.
		if maxLen > 60 {
			maxLen = 60
		}
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, colName, colName, float64(maxLen+4))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func generatePDFReport(workHours []models.WorkHour, periodLabel string) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 15, 10)
	// El salto de página lo manejamos manualmente (para re-dibujar el encabezado
	// de la tabla); el auto page break de gofpdf saltaría antes (y=190) y dejaría
	// páginas de continuación sin encabezado.
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	// Try to locate the white logo on the filesystem
	logoPath := "../frontend/public/logos/Horizontal_Blanco.png"
	if _, err := os.Stat(logoPath); err != nil {
		logoPath = "frontend/public/logos/Horizontal_Blanco.png"
		if _, err := os.Stat(logoPath); err != nil {
			logoPath = "public/logos/Horizontal_Blanco.png"
			if _, err := os.Stat(logoPath); err != nil {
				logoPath = ""
			}
		}
	}

	// Banner Superior con Degradado de Obertrack (Prussian Blue)
	pdf.SetFillColor(6, 11, 35) // #060b23 (Prussian Blue)
	pdf.Rect(0, 0, 297, 30, "F")
	
	if logoPath != "" {
		pdf.Image(logoPath, 15, 7, 45, 0, false, "", 0, "")
	} else {
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 18)
		pdf.Text(15, 18, "OBERTRACK")
	}
	
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 14)
	pdf.Text(155, 14, "REPORTE MENSUAL DE JORNADAS")
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(245, 242, 251) // Lavender Mist
	pdf.Text(155, 21, fmt.Sprintf("Periodo: %s", periodLabel))

	// Calcular estadísticas
	var totalHours float64
	var approvedHours float64
	var totalAbsences int
	var totalAbsenceHours float64
	for _, wh := range workHours {
		totalHours += wh.HoursWorked
		if wh.Approved {
			approvedHours += wh.HoursWorked
		}
		if wh.WorkType == models.WorkTypeAbsence {
			totalAbsences++
			totalAbsenceHours += wh.AbsenceHours
		}
	}

	// Dibujar tarjetas de estadísticas
	pdf.SetFillColor(245, 242, 251) // Lavender Mist
	pdf.SetDrawColor(221, 217, 239) // Gray 200 #ddd9ef
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(15, 38, 80, 22, 3.0, "1234", "FD")
	pdf.RoundedRect(105, 38, 80, 22, 3.0, "1234", "FD")
	pdf.RoundedRect(195, 38, 87, 22, 3.0, "1234", "FD")
	
	// Textos tarjetas
	pdf.SetTextColor(136, 128, 168) // Gray 400
	pdf.SetFont("Arial", "B", 8)
	pdf.Text(20, 44, "HORAS TOTALES")
	pdf.SetTextColor(138, 43, 226) // Blue Violet
	pdf.SetFont("Arial", "B", 14)
	pdf.Text(20, 53, fmt.Sprintf("%.1f h", totalHours))

	pdf.SetTextColor(136, 128, 168)
	pdf.SetFont("Arial", "B", 8)
	pdf.Text(110, 44, "APROBADAS")
	pdf.SetTextColor(16, 185, 129) // Success
	pdf.SetFont("Arial", "B", 14)
	pdf.Text(110, 53, fmt.Sprintf("%.1f h", approvedHours))

	pdf.SetTextColor(136, 128, 168)
	pdf.SetFont("Arial", "B", 8)
	pdf.Text(200, 44, "AUSENCIAS")
	pdf.SetTextColor(239, 68, 68) // Danger
	pdf.SetFont("Arial", "B", 14)
	pdf.Text(200, 53, fmt.Sprintf("%d (%.1f h)", totalAbsences, totalAbsenceHours))

	pdf.SetY(68)

	// Traduce UTF-8 a CP1252 (fuentes core de gofpdf): sin esto los acentos
	// salen como "reuniÃ³n".
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	drawTableHeader := func() {
		pdf.SetFillColor(245, 242, 251) // Lavender Mist
		pdf.SetTextColor(92, 86, 128)   // Gray 500
		pdf.SetDrawColor(221, 217, 239)
		pdf.SetLineWidth(0.2)
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(50, 7, "Profesional", "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 7, "Fecha", "1", 0, "C", true, 0, "")
		pdf.CellFormat(30, 7, "Tipo Jornada", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, "Jornada", "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 7, "Ausencias", "1", 0, "C", true, 0, "")
		pdf.CellFormat(77, 7, "Actividades / Motivo", "1", 0, "L", true, 0, "")
		pdf.CellFormat(30, 7, "Estado", "1", 1, "C", true, 0, "")
		pdf.SetFont("Arial", "", 8)
	}
	drawTableHeader()

	drawBadge := func(x, y, rowH float64, text string, bgR, bgG, bgB, fgR, fgG, fgB int) {
		badgeY := y + rowH/2 - 2
		pdf.SetFillColor(bgR, bgG, bgB)
		pdf.RoundedRect(x+3, badgeY, 24, 4, 1.0, "1234", "F")
		pdf.SetTextColor(fgR, fgG, fgB)
		pdf.SetFont("Arial", "B", 7)
		pdf.Text(x+6, badgeY+3, text)
		pdf.SetFont("Arial", "", 8)
	}

	fill := false
	for _, wh := range workHours {
		userLabel := ""
		if wh.User.ID != 0 {
			userLabel = wh.User.Name
		}

		detailsText := wh.Activities
		if wh.WorkType == models.WorkTypeAbsence {
			detailsText = humanizeReason(wh.AbsenceReason)
		}

		// Texto plano, envuelto en hasta 4 líneas; la fila crece con el contenido.
		// SplitText recibe UTF-8 crudo (ya filtrado a Latin-1) y tr() se aplica
		// línea a línea al dibujar: pasarle texto ya traducido (bytes CP1252, no
		// UTF-8 válido) lo hace entrar en pánico con cualquier acento.
		lines := pdf.SplitText(toLatin1Safe(htmlToPlainText(detailsText)), 71)
		if len(lines) > 4 {
			lines = lines[:4]
			lines[3] += "..."
		}
		lineH := 3.6
		rowH := 6.0
		if len(lines) > 1 {
			rowH = float64(len(lines))*lineH + 2.4
		}

		// Salto de página: A4 apaisado mide 210 mm de alto; corta antes del margen
		// y vuelve a dibujar el encabezado de la tabla.
		if pdf.GetY()+rowH > 195 {
			pdf.AddPage()
			pdf.SetY(15)
			drawTableHeader()
		}

		rowFill := func() {
			pdf.SetFillColor(255, 255, 255)
			if fill {
				pdf.SetFillColor(245, 242, 251)
			}
			pdf.SetTextColor(6, 11, 35)
		}
		rowFill()

		name := toLatin1Safe(userLabel)
		if pdf.GetStringWidth(tr(name)) > 46 {
			r := []rune(name)
			for len(r) > 0 && pdf.GetStringWidth(tr(string(r))+"...") > 46 {
				r = r[:len(r)-1]
			}
			name = string(r) + "..."
		}
		pdf.CellFormat(50, rowH, tr(name), "1", 0, "LM", true, 0, "")
		pdf.CellFormat(25, rowH, wh.WorkDate.Format("02-01-2006"), "1", 0, "CM", true, 0, "")

		typeX, typeY := pdf.GetX(), pdf.GetY()
		pdf.CellFormat(30, rowH, "", "1", 0, "C", true, 0, "")
		if wh.WorkType == models.WorkTypeAbsence {
			drawBadge(typeX, typeY, rowH, "AUSENCIA", 254, 226, 226, 239, 68, 68)
		} else {
			drawBadge(typeX, typeY, rowH, "COMPLETO", 220, 252, 231, 16, 185, 129)
		}
		rowFill()

		pdf.CellFormat(25, rowH, fmt.Sprintf("%.1f h", wh.HoursWorked), "1", 0, "CM", true, 0, "")
		pdf.CellFormat(35, rowH, fmt.Sprintf("%.1f h", wh.AbsenceHours), "1", 0, "CM", true, 0, "")

		actX, actY := pdf.GetX(), pdf.GetY()
		pdf.CellFormat(77, rowH, "", "1", 0, "L", true, 0, "")
		pdf.SetTextColor(6, 11, 35)
		textTop := actY + rowH/2 - float64(len(lines))*lineH/2
		for i, ln := range lines {
			pdf.Text(actX+2, textTop+float64(i+1)*lineH-0.9, tr(ln))
		}

		statusX, statusY := pdf.GetX(), pdf.GetY()
		pdf.CellFormat(30, rowH, "", "1", 1, "C", true, 0, "")
		if wh.Approved {
			drawBadge(statusX, statusY, rowH, "APROBADO", 220, 252, 231, 16, 185, 129)
		} else {
			drawBadge(statusX, statusY, rowH, "PENDIENTE", 254, 243, 199, 245, 158, 11)
		}
		pdf.SetTextColor(6, 11, 35)

		fill = !fill
	}

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
