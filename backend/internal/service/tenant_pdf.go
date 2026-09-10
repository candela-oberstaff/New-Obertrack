package service

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/obertrack/backend/internal/repository"
)

// generateTenantReportPDF crea un informe completo en PDF con el estado de salud,
// métricas, expediente, profesionales, horarios, tickets, encuestas y archivados de una empresa.
func generateTenantReportPDF(
	tenant *repository.TenantSummary,
	employees []repository.EmployeeSummary,
	tickets []repository.TenantTicket,
	activities []repository.TenantActivity,
	archived []repository.ArchivedEntry,
	absence *repository.AbsenceReport,
	inactives []repository.InactiveUser,
	surveys []repository.TenantSurveyReportItem,
) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AliasNbPages("{nb}")

	tr := pdf.UnicodeTranslatorFromDescriptor("")

	// Pie de página institucional
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
		pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
		pdf.Ln(2)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
		pdf.CellFormat(90, 5, tr("Obertrack  *  Informe de Auditoría y Salud Empresarial"), "", 0, "L", false, 0, "")
		pdf.CellFormat(90, 5, tr(fmt.Sprintf("Pág. %d de {nb}", pdf.PageNo())), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()

	// --- 1. BANNER INSTITUCIONAL ---
	pdf.SetFillColor(pdfPrussian[0], pdfPrussian[1], pdfPrussian[2])
	pdf.Rect(0, 0, 210, 36, "F")

	if logo := pdfLocateLogo(); logo != "" {
		pdf.Image(logo, 15, 8, 45, 0, false, "", 0, "")
	} else {
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 17)
		pdf.Text(15, 20, "OBERTRACK")
	}

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 14)
	pdf.Text(75, 16, tr("AUDITORÍA Y SALUD EMPRESARIAL"))
	pdf.SetFont("Arial", "", 9.5)
	pdf.SetTextColor(245, 242, 251)
	companyTitle := tenant.CompanyName
	if len(companyTitle) > 48 {
		companyTitle = companyTitle[:45] + "..."
	}
	pdf.Text(75, 23, tr(companyTitle))
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(180, 185, 210)
	pdf.Text(75, 29, tr(fmt.Sprintf("Emitido el %s", time.Now().Format("02/01/2006 15:04"))))

	pdf.SetY(42)

	// --- 2. INFORMACIÓN GENERAL Y SEÑALES DE SALUD ---
	pdfTenantSection(pdf, tr, "1. Información General y Salud de la Cuenta")

	// Tarjeta de datos básicos con dimensiones holgadas
	pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
	pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
	cardY := pdf.GetY()
	cardH := 45.0
	pdf.RoundedRect(15, cardY, 180, cardH, 2, "1234", "FD")
	pdf.SetY(cardY + 3)

	statusStr := "Activa"
	if !tenant.IsActive {
		statusStr = "Suspendida"
	}

	altaStr := pdfDate(tenant.CreatedAt)
	if tenant.ClientSince != nil && !tenant.ClientSince.IsZero() {
		altaStr = pdfDate(*tenant.ClientSince)
	}

	lastContact := "Sin contacto"
	if tenant.LastContactAt != nil && !tenant.LastContactAt.IsZero() {
		lastContact = tenant.LastContactAt.Format("02/01/2006")
	}
	lastAct := "Sin actividad"
	if tenant.LastActivityAt != nil && !tenant.LastActivityAt.IsZero() {
		lastAct = tenant.LastActivityAt.Format("02/01/2006")
	}

	loc := strings.TrimSpace(fmt.Sprintf("%s, %s, %s", orDash(tenant.City), orDash(tenant.State), orDash(tenant.Country)))
	loc = strings.Trim(loc, ", -")
	if loc == "" {
		loc = "-"
	}

	pdfTwoColKV(pdf, tr, "Razón Social", orDash(tenant.CompanyName), "Estado de Cuenta", statusStr)
	pdfTwoColKV(pdf, tr, "Propietario", orDash(tenant.OwnerName), "Email Contacto", orDash(tenant.OwnerEmail))
	pdfTwoColKV(pdf, tr, "Customer Success", orDash(tenant.AssignedCSName), "Rubro / Industria", orDash(tenant.Industry))
	pdfTwoColKV(pdf, tr, "Ubicación", loc, "Teléfono", orDash(tenant.PhoneNumber))
	pdfTwoColKV(pdf, tr, "Cliente Desde", altaStr, "Último Contacto", lastContact)
	pdfTwoColKV(pdf, tr, "Última Actividad", lastAct, "ID de Registro", fmt.Sprintf("#%d", tenant.ID))

	pdf.SetY(cardY + cardH + 4)

	// --- 3. TARJETAS DE MÉTRICAS / KPIS ---
	pdfTenantSection(pdf, tr, "2. Indicadores Clave de Desempeño (KPIs)")

	kpis := []struct {
		Label string
		Value string
	}{
		{"Profesionales", fmt.Sprintf("%d", tenant.UserCount)},
		{"Horas este Mes", fmt.Sprintf("%.1f h", tenant.HoursThisMonth)},
		{"Horas por Aprobar", fmt.Sprintf("%.1f h", tenant.PendingHours)},
		{"Tareas Registradas", fmt.Sprintf("%d", tenant.TaskCount)},
		{"Tableros Activos", fmt.Sprintf("%d", tenant.BoardCount)},
		{"Tickets Abiertos", fmt.Sprintf("%d", tenant.OpenTickets)},
	}

	kpiY := pdf.GetY()
	boxW := 28.0
	gap := 2.4
	for i, k := range kpis {
		bx := 15.0 + float64(i)*(boxW+gap)
		pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
		pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
		pdf.RoundedRect(bx, kpiY, boxW, 17, 1.5, "1234", "FD")

		pdf.SetXY(bx, kpiY+2)
		pdf.SetFont("Arial", "B", 12)
		pdf.SetTextColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
		pdf.CellFormat(boxW, 6, tr(k.Value), "", 2, "C", false, 0, "")

		pdf.SetFont("Arial", "", 6.8)
		pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
		pdf.CellFormat(boxW, 4.5, tr(k.Label), "", 2, "C", false, 0, "")
	}

	pdf.SetY(kpiY + 21)

	// --- 4. USO, RENDIMIENTO, AUSENCIAS E INACTIVIDAD EN FORMATO TABULAR / INDICADORES ---
	pdfTenantSection(pdf, tr, "3. Horas, Ausencias y Salud Operativa")

	totAbs := 0
	totAbsH := 0.0
	pendAbs := 0
	appAbs := 0
	rejAbs := 0
	if absence != nil {
		totAbs = absence.TotalAbsences
		totAbsH = absence.AbsenceHours
		pendAbs = absence.PendingReview
		appAbs = absence.Approved
		rejAbs = absence.Rejected
	}

	opKpis := []struct {
		Label string
		Value string
	}{
		{"Horas este Mes", fmt.Sprintf("%.1f h", tenant.HoursThisMonth)},
		{"Jornadas Pendientes", fmt.Sprintf("%d", tenant.PendingCount)},
		{"Jornadas Rechazadas", fmt.Sprintf("%d", tenant.RejectedCount)},
		{"Total Ausencias", fmt.Sprintf("%d (%.1fh)", totAbs, totAbsH)},
		{"Ausencias Pendientes", fmt.Sprintf("%d", pendAbs)},
		{"Ausencias Justificadas", fmt.Sprintf("%d (rech: %d)", appAbs, rejAbs)},
	}

	opY := pdf.GetY()
	for i, k := range opKpis {
		bx := 15.0 + float64(i)*(boxW+gap)
		pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
		pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
		pdf.RoundedRect(bx, opY, boxW, 17, 1.5, "1234", "FD")

		pdf.SetXY(bx, opY+2)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
		pdf.CellFormat(boxW, 6, tr(k.Value), "", 2, "C", false, 0, "")

		pdf.SetFont("Arial", "", 6.8)
		pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
		pdf.CellFormat(boxW, 4.5, tr(k.Label), "", 2, "C", false, 0, "")
	}

	pdf.SetY(opY + 20)

	// Tabla de inactividad o mensaje de conformidad
	if len(inactives) > 0 {
		pdfCheckPageBreak(pdf, 18)
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
		pdf.Text(16, pdf.GetY()+3, tr(fmt.Sprintf("Profesionales con Alerta de Inactividad (%d)", len(inactives))))
		pdf.SetY(pdf.GetY() + 5)

		colI := []float64{60, 60, 35, 25}
		pdf.SetFillColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 7.5)
		pdf.CellFormat(colI[0], 5.5, tr(" Profesional"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colI[1], 5.5, tr(" Correo Electrónico"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colI[2], 5.5, tr(" Cargo"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colI[3], 5.5, tr(" Inactividad"), "1", 1, "L", true, 0, "")

		pdf.SetFont("Arial", "", 7.5)
		maxInact := 5
		if len(inactives) < maxInact {
			maxInact = len(inactives)
		}
		for idx := 0; idx < maxInact; idx++ {
			inact := inactives[idx]
			bg := false
			if idx%2 == 1 {
				pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
				bg = true
			}
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])

			inactStr := fmt.Sprintf("%d días", inact.DaysInactive)
			pdf.CellFormat(colI[0], 5, tr(" "+inact.Name), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colI[1], 5, tr(" "+inact.Email), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colI[2], 5, tr(" "+orDash(inact.JobTitle)), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colI[3], 5, tr(" "+inactStr), "1", 1, "L", bg, 0, "")
		}
	} else {
		pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
		pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
		bxY := pdf.GetY()
		pdf.RoundedRect(15, bxY, 180, 8, 1.5, "1234", "FD")
		pdf.SetXY(18, bxY+2)
		pdf.SetFont("Arial", "I", 7.5)
		pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
		pdf.CellFormat(174, 4, tr("Sin alertas de inactividad: Todo el equipo mantiene actividad y registros regulares."), "", 1, "L", false, 0, "")
		pdf.SetY(bxY + 10)
	}

	pdf.Ln(2)

	// --- 5. PROFESIONALES, HORARIOS Y ESTRUCTURA (ORGANIGRAMA) ---
	pdfCheckPageBreak(pdf, 25)
	pdfTenantSection(pdf, tr, fmt.Sprintf("4. Estructura de Profesionales y Horarios (%d)", len(employees)))

	if len(employees) == 0 {
		pdfMuted(pdf, tr, "No hay profesionales registrados en esta empresa.")
	} else {
		colW := []float64{45, 30, 48, 30, 27} // total 180
		pdf.SetFillColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 8)

		pdf.CellFormat(colW[0], 6.5, tr(" Profesional"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colW[1], 6.5, tr(" Rol / Jerarquía"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colW[2], 6.5, tr(" Horario / Jornada"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colW[3], 6.5, tr(" Rendimiento"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colW[4], 6.5, tr(" Estado / Alta"), "1", 1, "L", true, 0, "")

		pdf.SetFont("Arial", "", 7.5)
		for idx, emp := range employees {
			pdfCheckPageBreak(pdf, 8)

			bg := false
			if idx%2 == 1 {
				pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
				bg = true
			}
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])

			// Rol
			rol := "Profesional"
			if emp.IsManager {
				rol = "Mánager"
			} else if emp.IsSupervisor {
				rol = "Supervisor"
			} else if emp.UserType == "employer" {
				rol = "Administrador"
			}

			// Horario
			sched := "No asignado"
			if emp.ScheduleType != "" {
				sDays := emp.ScheduleDays
				if sDays == "" {
					sDays = "L-V"
				}
				sTime := ""
				if emp.ScheduleStartTime != "" && emp.ScheduleEndTime != "" {
					sTime = fmt.Sprintf(" %s-%s", emp.ScheduleStartTime, emp.ScheduleEndTime)
				}
				sched = fmt.Sprintf("%s (%s%s)", emp.ScheduleType, sDays, sTime)
			}
			if len(sched) > 28 {
				sched = sched[:26] + ".."
			}

			// Rendimiento
			rend := fmt.Sprintf("%.1fh | %d/%d tar.", emp.HoursThisMonth, emp.TasksCompleted, emp.TasksAssigned)

			// Estado y Alta
			st := "Activo"
			if !emp.IsActive {
				st = "Inactivo"
			}
			stAlta := fmt.Sprintf("%s (%s)", st, pdfDate(emp.StartedAt))

			nameStr := emp.Name
			if len(nameStr) > 24 {
				nameStr = nameStr[:22] + ".."
			}

			pdf.CellFormat(colW[0], 5.8, tr(" "+nameStr), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colW[1], 5.8, tr(" "+rol), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colW[2], 5.8, tr(" "+sched), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colW[3], 5.8, tr(" "+rend), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colW[4], 5.8, tr(" "+stAlta), "1", 1, "L", bg, 0, "")
		}
	}

	pdf.Ln(3)

	// --- 6. EXPEDIENTE, COMUNICACIONES Y NOTAS (CON SEPARADORES Y TARJETAS ESTRUCTURADAS) ---
	pdfCheckPageBreak(pdf, 30)
	pdfTenantSection(pdf, tr, fmt.Sprintf("5. Expediente, Comunicaciones y Notas (%d)", len(activities)))

	if len(activities) == 0 {
		pdfMuted(pdf, tr, "Sin registros de comunicaciones o notas en el expediente.")
	} else {
		maxAct := 25
		if len(activities) < maxAct {
			maxAct = len(activities)
		}
		for i := 0; i < maxAct; i++ {
			act := activities[i]
			pdfCheckPageBreak(pdf, 16)

			tag := "[MOVIMIENTO]"
			switch act.Category {
			case repository.TenantActivityNote:
				tag = "[NOTA INTERNA]"
				if act.Pinned {
					tag = "[NOTA FIJADA]"
				}
			case repository.TenantActivityContact:
				channelName := act.Channel
				if channelName == "" {
					channelName = "Contacto"
				}
				tag = fmt.Sprintf("[%s]", strings.ToUpper(channelName))
			case repository.TenantActivityLifecycle:
				tag = "[CICLO DE VIDA]"
			case repository.TenantActivityStaff:
				tag = "[PERSONAL]"
			case repository.TenantActivityManagement:
				tag = "[GESTIÓN CS]"
			}

			actorStr := act.User
			if actorStr == "" {
				actorStr = "Sistema"
			}

			dateStr := act.Timestamp.Format("02/01/2006 15:04")

			// Encabezado de la nota con fondo suave
			entryY := pdf.GetY()
			pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
			pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
			pdf.Rect(15, entryY, 180, 6, "F")

			pdf.SetXY(18, entryY+0.8)
			pdf.SetFont("Arial", "B", 7.5)
			pdf.SetTextColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
			pdf.CellFormat(32, 4.5, tr(tag), "", 0, "L", false, 0, "")

			pdf.SetFont("Arial", "B", 7.5)
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.CellFormat(70, 4.5, tr(actorStr), "", 0, "L", false, 0, "")

			pdf.SetFont("Arial", "I", 7)
			pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
			pdf.CellFormat(74, 4.5, tr(dateStr), "", 1, "R", false, 0, "")

			// Contenido / detalle
			pdf.SetX(18)
			pdf.SetFont("Arial", "", 8)
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.MultiCell(174, 4.2, tr(act.Details), "", "L", false)

			// Línea separadora limpia
			pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
			pdf.Line(15, pdf.GetY()+1.5, 195, pdf.GetY()+1.5)
			pdf.SetY(pdf.GetY() + 3)
		}
	}

	pdf.Ln(2)

	// --- 7. TICKETS DE SOPORTE E INCIDENCIAS ---
	pdfCheckPageBreak(pdf, 25)
	pdfTenantSection(pdf, tr, fmt.Sprintf("6. Historial de Tickets e Incidencias (%d)", len(tickets)))

	if len(tickets) == 0 {
		pdfMuted(pdf, tr, "No hay tickets de soporte registrados para esta empresa.")
	} else {
		colT := []float64{18, 65, 30, 35, 32} // total 180
		pdf.SetFillColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 8)

		pdf.CellFormat(colT[0], 6, tr(" ID"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colT[1], 6, tr(" Asunto / Título"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colT[2], 6, tr(" Origen / Etapa"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colT[3], 6, tr(" Asignado a"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colT[4], 6, tr(" Estado / Fecha"), "1", 1, "L", true, 0, "")

		pdf.SetFont("Arial", "", 7.5)
		maxT := 15
		if len(tickets) < maxT {
			maxT = len(tickets)
		}
		for idx := 0; idx < maxT; idx++ {
			t := tickets[idx]
			pdfCheckPageBreak(pdf, 8)

			bg := false
			if idx%2 == 1 {
				pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
				bg = true
			}
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])

			titleStr := t.Title
			if len(titleStr) > 40 {
				titleStr = titleStr[:38] + ".."
			}
			origEtapa := fmt.Sprintf("%s / %s", orDash(t.Origin), orDash(t.Stage))
			assignee := orDash(t.Assignee)
			if len(assignee) > 18 {
				assignee = assignee[:16] + ".."
			}
			stDate := fmt.Sprintf("%s (%s)", orDash(t.Status), pdfDate(t.CreatedAt))

			pdf.CellFormat(colT[0], 5.5, tr(fmt.Sprintf(" #%d", t.ID)), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colT[1], 5.5, tr(" "+titleStr), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colT[2], 5.5, tr(" "+origEtapa), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colT[3], 5.5, tr(" "+assignee), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colT[4], 5.5, tr(" "+stDate), "1", 1, "L", bg, 0, "")
		}
	}

	pdf.Ln(2)

	// --- 8. ENCUESTAS Y EVALUACIONES DE PROFESIONALES ---
	pdfCheckPageBreak(pdf, 25)
	pdfTenantSection(pdf, tr, fmt.Sprintf("7. Encuestas y Evaluaciones de Profesionales (%d)", len(surveys)))

	if len(surveys) == 0 {
		pdfMuted(pdf, tr, "Sin encuestas completadas por el personal de la empresa.")
	} else {
		maxS := 10
		if len(surveys) < maxS {
			maxS = len(surveys)
		}
		for i := 0; i < maxS; i++ {
			s := surveys[i]
			pdfCheckPageBreak(pdf, 14)

			pdf.SetX(18)
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetTextColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
			pdf.CellFormat(0, 5, tr(fmt.Sprintf("* %s  *  Respondido por: %s (%s)", s.SurveyTitle, s.UserName, pdfDate(s.CompletedAt))), "", 1, "L", false, 0, "")

			// Muestra hasta 3 respuestas destacadas
			for aIdx, ans := range s.Answers {
				if aIdx < 3 && ans.QuestionText != "" {
					pdf.SetX(22)
					pdf.SetFont("Arial", "I", 7.5)
					pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
					val := ans.TextValue
					if val == "" && ans.NumberValue > 0 {
						val = fmt.Sprintf("%d pts", ans.NumberValue)
					}
					qText := ans.QuestionText
					if len(qText) > 60 {
						qText = qText[:58] + ".."
					}
					pdf.CellFormat(0, 4.2, tr(fmt.Sprintf("- Pregunta: %s -> R: %s", qText, orDash(val))), "", 1, "L", false, 0, "")
				}
			}
			pdf.Ln(1)
		}
	}

	pdf.Ln(2)

	// --- 9. PERSONAL ARCHIVADO Y BAJAS LABORALES ---
	pdfCheckPageBreak(pdf, 20)
	pdfTenantSection(pdf, tr, fmt.Sprintf("8. Registro de Bajas y Personal Archivado (%d)", len(archived)))

	if len(archived) == 0 {
		pdfMuted(pdf, tr, "No se registran bajas ni cuentas desactivadas en esta empresa.")
	} else {
		colA := []float64{45, 45, 30, 35, 25} // total 180
		pdf.SetFillColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 8)

		pdf.CellFormat(colA[0], 6, tr(" Profesional"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colA[1], 6, tr(" Email"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colA[2], 6, tr(" Tipo de Registro"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colA[3], 6, tr(" Motivo"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colA[4], 6, tr(" Fecha"), "1", 1, "L", true, 0, "")

		pdf.SetFont("Arial", "", 7.5)
		maxArch := 10
		if len(archived) < maxArch {
			maxArch = len(archived)
		}
		for idx := 0; idx < maxArch; idx++ {
			a := archived[idx]
			pdfCheckPageBreak(pdf, 7)

			bg := false
			if idx%2 == 1 {
				pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
				bg = true
			}
			pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
			pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])

			kindStr := "Baja laboral"
			if a.Kind == "deactivated_user" {
				kindStr = "Cuenta inactiva"
			}
			reasonStr := orDash(a.Reason)
			if len(reasonStr) > 22 {
				reasonStr = reasonStr[:20] + ".."
			}

			pdf.CellFormat(colA[0], 5.5, tr(" "+a.Name), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colA[1], 5.5, tr(" "+a.Email), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colA[2], 5.5, tr(" "+kindStr), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colA[3], 5.5, tr(" "+reasonStr), "1", 0, "L", bg, 0, "")
			pdf.CellFormat(colA[4], 5.5, tr(" "+pdfDate(a.ArchivedAt)), "1", 1, "L", bg, 0, "")
		}
	}

	pdf.Ln(4)

	// Cuadro de cierre de auditoría
	pdfCheckPageBreak(pdf, 16)
	pdf.SetFillColor(pdfCardBg[0], pdfCardBg[1], pdfCardBg[2])
	pdf.SetDrawColor(pdfBorder[0], pdfBorder[1], pdfBorder[2])
	boxEnd := pdf.GetY()
	pdf.RoundedRect(15, boxEnd, 180, 12, 1.5, "1234", "FD")
	pdf.SetXY(18, boxEnd+2)
	pdf.SetFont("Arial", "I", 7.5)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.MultiCell(174, 3.8, tr("Este documento certifica el estado consolidado de la empresa y su equipo en la plataforma Obertrack a la fecha de emisión. Documento de respaldo y auditoría interna."), "", "C", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func pdfTenantSection(pdf *gofpdf.Fpdf, tr func(string) string, text string) {
	pdf.Ln(2)
	y := pdf.GetY()
	pdf.SetFillColor(pdfViolet[0], pdfViolet[1], pdfViolet[2])
	pdf.Rect(15, y+1, 2.5, 4.5, "F")
	pdf.SetX(19)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	pdf.CellFormat(0, 6, tr(text), "", 1, "L", false, 0, "")
	pdf.Ln(1)
}

func pdfTwoColKV(pdf *gofpdf.Fpdf, tr func(string) string, k1, v1, k2, v2 string) {
	// Columna izquierda: X=18, Key=30, Value=54 (total 84, termina en 102)
	pdf.SetX(18)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.CellFormat(30, 5.5, tr(k1+":"), "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	v1Str := v1
	if len(v1Str) > 30 {
		v1Str = v1Str[:28] + ".."
	}
	pdf.CellFormat(54, 5.5, tr(v1Str), "", 0, "L", false, 0, "")

	// Columna derecha: X=108, Key=30, Value=54 (total 84, termina en 192)
	pdf.SetX(108)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetTextColor(pdfGray[0], pdfGray[1], pdfGray[2])
	pdf.CellFormat(30, 5.5, tr(k2+":"), "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(pdfDark[0], pdfDark[1], pdfDark[2])
	v2Str := v2
	if len(v2Str) > 30 {
		v2Str = v2Str[:28] + ".."
	}
	pdf.CellFormat(54, 5.5, tr(v2Str), "", 1, "L", false, 0, "")
}

func pdfCheckPageBreak(pdf *gofpdf.Fpdf, neededHeight float64) {
	if pdf.GetY()+neededHeight > 275 {
		pdf.AddPage()
		pdf.SetY(20)
	}
}
