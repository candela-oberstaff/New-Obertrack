package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"

	"github.com/obertrack/backend/internal/models"
)

func (s *workHourService) getReportWorkHours(userID uint, month int, year int, companyFilter uint) ([]models.WorkHour, string, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, "", err
	}

	monthsEs := []string{
		"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
		"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre",
	}
	monthName := monthsEs[month-1]

	var employerID uint
	if user.IsSuperadmin {
		employerID = 0
	} else if user.UserType == models.UserTypeEmployer {
		employerID = user.ID
	} else if user.EmpleadorID != nil {
		employerID = *user.EmpleadorID
	}

	startDateStr := fmt.Sprintf("%d-%02d-01", year, month)
	t := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
	endDateStr := fmt.Sprintf("%d-%02d-%02d", year, month, t.Day())

	filters := make(map[string]interface{})
	if user.IsSuperadmin {
		// Superadmin must scope to a company; without it, no data so we never mix
		// tenants in a report.
		if companyFilter == 0 {
			return []models.WorkHour{}, monthName, nil
		}
		filters["tenant_id"] = companyFilter
	} else {
		filters["tenant_id"] = employerID
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

func (s *workHourService) GetPDFReportBytes(userID uint, month int, year int, companyFilter uint) ([]byte, string, error) {
	workHours, monthName, err := s.getReportWorkHours(userID, month, year, companyFilter)
	if err != nil {
		return nil, "", err
	}
	pdfBytes, err := generatePDFReport(workHours, monthName, year)
	if err != nil {
		return nil, "", err
	}
	return pdfBytes, monthName, nil
}

func (s *workHourService) GetExcelReportBytes(userID uint, month int, year int, companyFilter uint) ([]byte, string, error) {
	workHours, monthName, err := s.getReportWorkHours(userID, month, year, companyFilter)
	if err != nil {
		return nil, "", err
	}
	excelBytes, err := generateExcelReport(workHours, monthName, year)
	if err != nil {
		return nil, "", err
	}
	return excelBytes, monthName, nil
}

func (s *workHourService) SendReportEmail(employerID uint, month int, year int, companyFilter uint) error {
	user, err := s.userRepo.GetByID(employerID)
	if err != nil {
		return err
	}

	workHours, monthName, err := s.getReportWorkHours(employerID, month, year, companyFilter)
	if err != nil {
		return err
	}

	// Calculate summary stats
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

	// Format a beautiful, premium, branded email matching the Obertrack theme
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
			<h1 style="font-size: 20px; font-weight: 700; opacity: 0.95; margin: 0; color: #ffffff; font-family: sans-serif; letter-spacing: -0.01em;">Reporte Mensual de Jornadas</h1>
			<div style="font-size: 14px; opacity: 0.85; margin-top: 6px; color: #f5f2fb; font-family: sans-serif;">%s %d</div>
		</div>
		
		<!-- Contenido Principal -->
		<div style="padding: 32px 24px;">
			<p style="font-size: 16px; line-height: 1.6; margin-bottom: 24px; color: #060b23; font-family: sans-serif;">Hola <strong>%s</strong>,</p>
			<p style="font-size: 15px; line-height: 1.6; margin-bottom: 24px; color: #5c5680; font-family: sans-serif;">Aquí tienes el informe de horas y asistencia consolidado de tu equipo correspondiente a <strong>%s de %d</strong>.</p>
			
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

			<div style="font-size: 15px; font-weight: 700; color: #512868; border-bottom: 2px solid #f0eef8; padding-bottom: 8px; margin-bottom: 16px; text-transform: uppercase; letter-spacing: 0.025em; font-family: sans-serif;">Detalle de Actividades</div>
			
			<table cellpadding="0" cellspacing="0" border="0" width="100%%" style="width: 100%%; border-collapse: collapse; margin-bottom: 24px;">
				<thead>
					<tr>
						<th style="background-color: #f5f2fb; border-bottom: 2px solid #ddd9ef; padding: 12px 8px; text-align: left; font-size: 11px; font-weight: 700; color: #5c5680; text-transform: uppercase; font-family: sans-serif;">Fecha</th>
						<th style="background-color: #f5f2fb; border-bottom: 2px solid #ddd9ef; padding: 12px 8px; text-align: left; font-size: 11px; font-weight: 700; color: #5c5680; text-transform: uppercase; font-family: sans-serif;">Profesional</th>
						<th style="background-color: #f5f2fb; border-bottom: 2px solid #ddd9ef; padding: 12px 8px; text-align: left; font-size: 11px; font-weight: 700; color: #5c5680; text-transform: uppercase; font-family: sans-serif;">Tipo</th>
						<th style="background-color: #f5f2fb; border-bottom: 2px solid #ddd9ef; padding: 12px 8px; text-align: left; font-size: 11px; font-weight: 700; color: #5c5680; text-transform: uppercase; font-family: sans-serif;">Horas</th>
						<th style="background-color: #f5f2fb; border-bottom: 2px solid #ddd9ef; padding: 12px 8px; text-align: left; font-size: 11px; font-weight: 700; color: #5c5680; text-transform: uppercase; font-family: sans-serif;">Detalles</th>
					</tr>
				</thead>
				<tbody>`, monthName, year, user.Name, monthName, year, totalHours, approvedHours, totalAbsences, totalAbsenceHours)

	for _, wh := range workHours {
		typeStr := "Completo"
		badgeStyle := "background-color: #dcfce7; color: #10b981; border: 1px solid rgba(16, 185, 129, 0.2);"
		detailStr := wh.Activities
		if wh.WorkType == models.WorkTypeAbsence {
			typeStr = "Ausencia"
			badgeStyle = "background-color: #fee2e2; color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.2);"
			detailStr = fmt.Sprintf("Faltantes: %.1f h<br><span style='font-size: 11px; color: #8880a8;'>Motivo: %s</span>", wh.AbsenceHours, wh.AbsenceReason)
		} else {
			if len(detailStr) > 60 {
				detailStr = detailStr[:57] + "..."
			}
		}

		htmlContent += fmt.Sprintf(`
					<tr>
						<td style="border-bottom: 1px solid #f0eef8; padding: 12px 8px; font-size: 13px; color: #060b23; white-space: nowrap; font-family: sans-serif;">%s</td>
						<td style="border-bottom: 1px solid #f0eef8; padding: 12px 8px; font-size: 13px; color: #060b23; font-family: sans-serif;"><strong>%s</strong></td>
						<td style="border-bottom: 1px solid #f0eef8; padding: 12px 8px; font-size: 13px; color: #060b23; font-family: sans-serif;">
							<span style="display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 10px; font-weight: 600; text-transform: uppercase; %s">%s</span>
						</td>
						<td style="border-bottom: 1px solid #f0eef8; padding: 12px 8px; font-size: 13px; color: #060b23; font-family: sans-serif;"><strong>%.1f h</strong></td>
						<td style="border-bottom: 1px solid #f0eef8; padding: 12px 8px; font-size: 13px; color: #5c5680; font-family: sans-serif;">%s</td>
					</tr>`, wh.WorkDate.Format("02-01-2006"), wh.User.Name, badgeStyle, typeStr, wh.HoursWorked, detailStr)
	}

	htmlContent += `
				</tbody>
			</table>
		</div>

		<!-- Footer con Estilos Inline de la Marca -->
		<div style="background: #f5f2fb; padding: 24px; text-align: center; font-size: 12px; color: #8880a8; border-top: 1px solid #ddd9ef; font-family: sans-serif;">
			Este es un informe automático generado de forma segura para ti por <strong>Obertrack</strong>.<br>
			&copy; 2026 Obertrack. Todos los derechos reservados.
		</div>
	</div>
</body>
</html>`

	subject := fmt.Sprintf("Obertrack - Reporte de Jornadas (%s %d)", monthName, year)

	pdfBytes, err := generatePDFReport(workHours, monthName, year)
	if err != nil {
		return fmt.Errorf("failed to generate PDF attachment: %w", err)
	}

	excelBytes, err := generateExcelReport(workHours, monthName, year)
	if err != nil {
		return fmt.Errorf("failed to generate Excel attachment: %w", err)
	}

	attachments := []BrevoAttachment{
		{
			Name:    fmt.Sprintf("reporte_jornadas_%s_%d.pdf", monthName, year),
			Content: base64.StdEncoding.EncodeToString(pdfBytes),
		},
		{
			Name:    fmt.Sprintf("reporte_jornadas_%s_%d.xlsx", monthName, year),
			Content: base64.StdEncoding.EncodeToString(excelBytes),
		},
	}

	return s.brevoSvc.SendEmailWithAttachments(user.Email, user.Name, subject, htmlContent, attachments)
}

func generateExcelReport(workHours []models.WorkHour, monthName string, year int) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Reporte de Horas"
	f.SetSheetName("Sheet1", sheetName)

	// A1 Title styled with brand color (Prussian Blue)
	f.SetCellValue(sheetName, "A1", fmt.Sprintf("Reporte de Actividades - %s %d", monthName, year))
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
	headers := []string{"Profesional", "Fecha", "Tipo de Jornada", "Horas Trabajadas", "Ausencia (Horas)", "Motivo de Ausencia", "Actividades Realizadas", "Estado"}
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
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowIdx), wh.AbsenceReason)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowIdx), wh.Activities)
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
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, colName, colName, float64(maxLen+4))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func generatePDFReport(workHours []models.WorkHour, monthName string, year int) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 15, 10)
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
	pdf.Text(155, 21, fmt.Sprintf("Periodo: %s de %d", monthName, year))

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
	
	// Table headers
	pdf.SetFillColor(245, 242, 251) // Lavender Mist
	pdf.SetTextColor(92, 86, 128)  // Gray 500
	pdf.SetDrawColor(221, 217, 239)
	pdf.SetLineWidth(0.2)
	pdf.SetFont("Arial", "B", 9)
	
	pdf.CellFormat(50, 7, "Profesional", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 7, "Fecha", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 7, "Tipo Jornada", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "Horas Trab.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(35, 7, "Horas Faltantes", "1", 0, "C", true, 0, "")
	pdf.CellFormat(77, 7, "Actividades / Motivo", "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 7, "Estado", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 8)
	fill := false
	for _, wh := range workHours {
		userLabel := ""
		if wh.User.ID != 0 {
			userLabel = wh.User.Name
		}
		
		detailsText := wh.Activities
		if wh.WorkType == models.WorkTypeAbsence {
			detailsText = wh.AbsenceReason
		}

		pdf.SetFillColor(255, 255, 255)
		if fill {
			pdf.SetFillColor(245, 242, 251)
		}
		pdf.SetTextColor(6, 11, 35)

		pdf.CellFormat(50, 6, userLabel, "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 6, wh.WorkDate.Format("2006-01-02"), "1", 0, "C", true, 0, "")
		
		// Draw Type Badge
		typeX := pdf.GetX()
		typeY := pdf.GetY()
		pdf.CellFormat(30, 6, "", "1", 0, "C", true, 0, "")
		if wh.WorkType == models.WorkTypeAbsence {
			pdf.SetFillColor(254, 226, 226) // #fee2e2
			pdf.RoundedRect(typeX+3, typeY+1, 24, 4, 1.0, "1234", "F")
			pdf.SetTextColor(239, 68, 68) // #ef4444
			pdf.SetFont("Arial", "B", 7)
			pdf.Text(typeX+6, typeY+4, "AUSENCIA")
		} else {
			pdf.SetFillColor(220, 252, 231) // #dcfce7
			pdf.RoundedRect(typeX+3, typeY+1, 24, 4, 1.0, "1234", "F")
			pdf.SetTextColor(16, 185, 129) // #10b981
			pdf.SetFont("Arial", "B", 7)
			pdf.Text(typeX+6, typeY+4, "COMPLETO")
		}
		
		// Restore row styles
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(6, 11, 35)
		pdf.SetFillColor(255, 255, 255)
		if fill {
			pdf.SetFillColor(245, 242, 251)
		}

		pdf.CellFormat(25, 6, fmt.Sprintf("%.1f h", wh.HoursWorked), "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 6, fmt.Sprintf("%.1f h", wh.AbsenceHours), "1", 0, "C", true, 0, "")
		
		if len(detailsText) > 42 {
			detailsText = detailsText[:39] + "..."
		}
		pdf.CellFormat(77, 6, detailsText, "1", 0, "L", true, 0, "")
		
		// Draw Status Badge
		statusX := pdf.GetX()
		statusY := pdf.GetY()
		pdf.CellFormat(30, 6, "", "1", 1, "C", true, 0, "")
		if wh.Approved {
			pdf.SetFillColor(220, 252, 231) // #dcfce7
			pdf.RoundedRect(statusX+3, statusY+1, 24, 4, 1.0, "1234", "F")
			pdf.SetTextColor(16, 185, 129) // #10b981
			pdf.SetFont("Arial", "B", 7)
			pdf.Text(statusX+6, statusY+4, "APROBADO")
		} else {
			pdf.SetFillColor(254, 243, 199) // #fef3c7
			pdf.RoundedRect(statusX+3, statusY+1, 24, 4, 1.0, "1234", "F")
			pdf.SetTextColor(245, 158, 11) // #f59e0b
			pdf.SetFont("Arial", "B", 7)
			pdf.Text(statusX+6, statusY+4, "PENDIENTE")
		}
		
		// Restore row styles
		pdf.SetFont("Arial", "", 8)
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
