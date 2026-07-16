package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

var importCompanyHeaders = []string{
	"nombre_responsable *", "email *", "nombre_empresa *",
	"industria", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "direccion",
}

var importProfessionalHeaders = []string{
	"nombre *", "email *", "empresa *",
	"cargo", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "es_manager",
}

var importEmployerProfHeaders = []string{
	"nombre *", "email *",
	"cargo", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "es_manager",
}

func (h *AdminHandler) DownloadImportTemplate(c *gin.Context) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"6D28D9"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "DDD9EF", Style: 1},
		},
	})
	exampleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Italic: true, Color: "94A3B8", Size: 10},
	})

	f.SetSheetName("Sheet1", "Instrucciones")
	instructions := []string{
		"IMPORTACIÓN MASIVA — OBERTRACK",
		"",
		"Cómo usar esta plantilla:",
		"1) Completá la hoja EMPRESAS y/o la hoja PROFESIONALES. Cada hoja se importa por separado.",
		"2) Las columnas marcadas con * son OBLIGATORIAS. No cambies los nombres de los encabezados.",
		"3) Borrá la fila de EJEMPLO (en gris) antes de importar.",
		"",
		"EMPRESAS — crea cuentas de empresa (empleador):",
		"   • nombre_responsable*: nombre del dueño o administrador de la cuenta.",
		"   • email*: correo de acceso (único en todo el sistema).",
		"   • nombre_empresa*: razón social / nombre comercial.",
		"   • industria, telefono, pais, estado_provincia, ciudad, ubicacion, direccion: opcionales.",
		"",
		"PROFESIONALES — crea profesionales y los vincula a una empresa existente:",
		"   • nombre*, email*.",
		"   • empresa*: nombre EXACTO de la empresa, o su ID. La empresa debe existir",
		"     (si la estás creando en la hoja Empresas, esa se procesa primero).",
		"   • cargo, telefono, pais, estado_provincia, ciudad, ubicacion: opcionales.",
		"   • es_manager: escribí 'Sí' si el profesional es un manager (puede tener gente a cargo);",
		"     'No' o vacío en caso contrario.",
		"",
		"Reglas importantes:",
		"   • Contraseña: NO la pongas en el Excel. El sistema genera una temporal por cada fila",
		"     y te la entrega al finalizar para que la compartas.",
		"   • Email ya existente: te avisaremos a quién pertenece y podrás elegir SOBREESCRIBIR u OMITIR,",
		"     fila por fila. Por defecto se OMITE.",
		"   • Managers: marcá 'Sí' en es_manager para identificarlos. A quién reporta cada profesional",
		"     se asigna después, desde el detalle del usuario.",
		"   • País y ubicación: texto libre.",
	}
	for i, line := range instructions {
		cell := fmt.Sprintf("A%d", i+1)
		_ = f.SetCellValue("Instrucciones", cell, line)
	}
	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14, Color: "6D28D9"}})
	_ = f.SetCellStyle("Instrucciones", "A1", "A1", titleStyle)
	_ = f.SetColWidth("Instrucciones", "A", "A", 95)

	writeImportSheet(f, "Empresas", importCompanyHeaders, headerStyle, exampleStyle, []string{
		"Juan Pérez", "juan@miempresa.com", "Mi Empresa S.A.",
		"Tecnología", "+58 412 000 0000", "Venezuela", "Distrito Capital", "Caracas", "Las Mercedes", "Av. Principal 123",
	})

	writeImportSheet(f, "Profesionales", importProfessionalHeaders, headerStyle, exampleStyle, []string{
		"María González", "maria@miempresa.com", "Mi Empresa S.A.",
		"Desarrolladora Backend", "+58 412 111 1111", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "No",
	})

	if idx, err := f.GetSheetIndex("Instrucciones"); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar la plantilla"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=plantilla_importacion_obertrack.xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

func writeImportSheet(f *excelize.File, sheet string, headers []string, headerStyle, exampleStyle int, exampleRow []string) {
	_, _ = f.NewSheet(sheet)
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s1", col), h)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s1", col), fmt.Sprintf("%s1", col), headerStyle)
		_ = f.SetColWidth(sheet, col, col, 22)
	}
	for i, v := range exampleRow {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s2", col), v)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s2", col), fmt.Sprintf("%s2", col), exampleStyle)
	}
	_ = f.SetRowHeight(sheet, 1, 22)
}

type importExisting struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type importRow struct {
	Row      int               `json:"row"`
	Data     map[string]string `json:"data"`
	Status   string            `json:"status"`
	Message  string            `json:"message,omitempty"`
	Existing *importExisting   `json:"existing,omitempty"`
}

func normHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSpace(strings.TrimRight(s, " *"))
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", " ", "_")
	return r.Replace(s)
}

func readImportSheet(xl *excelize.File, sheet string) (map[string]int, [][]string) {
	rows, err := xl.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil, nil
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		if key := normHeader(h); key != "" {
			idx[key] = i
		}
	}
	return idx, rows[1:]
}

func cellVal(idx map[string]int, row []string, key string) string {
	i, ok := idx[key]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func emptyRow(m map[string]string) bool {
	for _, v := range m {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func validEmail(e string) bool {
	e = strings.TrimSpace(e)
	at := strings.Index(e, "@")
	return at > 0 && strings.Contains(e[at:], ".") && !strings.ContainsAny(e, " \t")
}

func openUploadedXlsx(c *gin.Context) (*excelize.File, func(), bool) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Subí un archivo .xlsx en el campo 'file'."})
		return nil, nil, false
	}
	file, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo abrir el archivo."})
		return nil, nil, false
	}
	xl, err := excelize.OpenReader(file)
	if err != nil {
		_ = file.Close()
		c.JSON(http.StatusBadRequest, gin.H{"error": "El archivo no es un Excel (.xlsx) válido."})
		return nil, nil, false
	}
	return xl, func() { _ = xl.Close(); _ = file.Close() }, true
}

func (h *AdminHandler) ImportPreview(c *gin.Context) {
	xl, closer, ok := openUploadedXlsx(c)
	if !ok {
		return
	}
	defer closer()

	companies, _, _ := h.service.GetAllUsers(string(models.UserTypeEmployer), "", "", "", 0, 100000)
	companyByName := map[string]bool{}
	companyByID := map[uint]bool{}
	for _, comp := range companies {
		companyByID[comp.ID] = true
		if comp.CompanyName != "" {
			companyByName[strings.ToLower(strings.TrimSpace(comp.CompanyName))] = true
		}
	}

	seen := map[string]int{}

	compReports := []importRow{}
	pendingCompanyNames := map[string]bool{}
	seenCompanyNames := map[string]int{}
	if cIdx, cRows := readImportSheet(xl, "Empresas"); cIdx != nil {
		for i, row := range cRows {
			data := map[string]string{
				"nombre_responsable": cellVal(cIdx, row, "nombre_responsable"),
				"email":              cellVal(cIdx, row, "email"),
				"nombre_empresa":     cellVal(cIdx, row, "nombre_empresa"),
				"industria":          cellVal(cIdx, row, "industria"),
				"telefono":           cellVal(cIdx, row, "telefono"),
				"pais":               cellVal(cIdx, row, "pais"),
				"estado_provincia":   cellVal(cIdx, row, "estado_provincia"),
				"ciudad":             cellVal(cIdx, row, "ciudad"),
				"ubicacion":          cellVal(cIdx, row, "ubicacion"),
				"direccion":          cellVal(cIdx, row, "direccion"),
			}
			if emptyRow(data) {
				continue
			}
			rep := importRow{Row: i + 2, Data: data, Status: "ok"}
			email := strings.ToLower(data["email"])
			switch {
			case data["nombre_responsable"] == "" || data["email"] == "" || data["nombre_empresa"] == "":
				rep.Status, rep.Message = "error", "Faltan campos obligatorios (nombre_responsable, email, nombre_empresa)."
			case !validEmail(data["email"]):
				rep.Status, rep.Message = "error", "Email inválido."
			case seen[email] != 0:
				rep.Status, rep.Message = "error", fmt.Sprintf("Email repetido en el archivo (ya está en la fila %d).", seen[email])
			default:
				seen[email] = rep.Row
				lname := strings.ToLower(strings.TrimSpace(data["nombre_empresa"]))
				if existing, err := h.service.FindUserByEmail(data["email"]); err == nil && existing != nil {
					rep.Status, rep.Message = "conflict", "Este correo ya existe."
					rep.Existing = &importExisting{ID: existing.ID, Name: existing.Name, Email: existing.Email}
				} else if companyByName[lname] {
					rep.Status, rep.Message = "conflict", "Ya existe una empresa con ese nombre en el sistema."
				} else if prev, dup := seenCompanyNames[lname]; dup {
					rep.Status, rep.Message = "conflict", fmt.Sprintf("Empresa repetida en el archivo (ya está en la fila %d).", prev)
				} else {
					seenCompanyNames[lname] = rep.Row
					pendingCompanyNames[lname] = true
				}
			}
			compReports = append(compReports, rep)
		}
	}

	profReports := []importRow{}
	if pIdx, pRows := readImportSheet(xl, "Profesionales"); pIdx != nil {
		for i, row := range pRows {
			data := map[string]string{
				"nombre":           cellVal(pIdx, row, "nombre"),
				"email":            cellVal(pIdx, row, "email"),
				"empresa":          cellVal(pIdx, row, "empresa"),
				"cargo":            cellVal(pIdx, row, "cargo"),
				"telefono":         cellVal(pIdx, row, "telefono"),
				"pais":             cellVal(pIdx, row, "pais"),
				"estado_provincia": cellVal(pIdx, row, "estado_provincia"),
				"ciudad":           cellVal(pIdx, row, "ciudad"),
				"ubicacion":        cellVal(pIdx, row, "ubicacion"),
				"es_manager":       cellVal(pIdx, row, "es_manager"),
			}
			if emptyRow(data) {
				continue
			}
			rep := importRow{Row: i + 2, Data: data, Status: "ok"}
			email := strings.ToLower(data["email"])
			switch {
			case data["nombre"] == "" || data["email"] == "" || data["empresa"] == "":
				rep.Status, rep.Message = "error", "Faltan campos obligatorios (nombre, email, empresa)."
			case !validEmail(data["email"]):
				rep.Status, rep.Message = "error", "Email inválido."
			case seen[email] != 0:
				rep.Status, rep.Message = "error", fmt.Sprintf("Email repetido en el archivo (ya está en la fila %d).", seen[email])
			case !companyResolvable(data["empresa"], companyByName, companyByID, pendingCompanyNames):
				rep.Status, rep.Message = "error", fmt.Sprintf("Empresa %q no encontrada (ni en la hoja Empresas).", data["empresa"])
			default:
				seen[email] = rep.Row
				if existing, err := h.service.FindUserByEmail(data["email"]); err == nil && existing != nil {
					rep.Status, rep.Message = "conflict", "Este correo ya existe."
					rep.Existing = &importExisting{ID: existing.ID, Name: existing.Name, Email: existing.Email}
				}
			}
			profReports = append(profReports, rep)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"companies":     compReports,
		"professionals": profReports,
		"summary":       importSummary(compReports, profReports),
	})
}

func companyResolvable(val string, byName map[string]bool, byID map[uint]bool, pending map[string]bool) bool {
	val = strings.TrimSpace(val)
	if val == "" {
		return false
	}
	if id, err := strconv.ParseUint(val, 10, 32); err == nil && byID[uint(id)] {
		return true
	}
	l := strings.ToLower(val)
	return byName[l] || pending[l]
}

func importSummary(comp, prof []importRow) gin.H {
	count := func(rows []importRow) gin.H {
		var okc, errc, conf int
		for _, r := range rows {
			switch r.Status {
			case "ok":
				okc++
			case "error":
				errc++
			case "conflict":
				conf++
			}
		}
		return gin.H{"ok": okc, "error": errc, "conflict": conf, "total": len(rows)}
	}
	return gin.H{"companies": count(comp), "professionals": count(prof)}
}

type importExecRow struct {
	Action string            `json:"action"`
	Data   map[string]string `json:"data"`
}

type importExecReq struct {
	Companies     []importExecRow `json:"companies"`
	Professionals []importExecRow `json:"professionals"`
}

func putIf(m map[string]interface{}, key, val string) {
	if strings.TrimSpace(val) != "" {
		m[key] = val
	}
}

func managerFlag(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "si", "sí", "yes", "y", "true", "1", "x", "verdadero":
		return true
	}
	return false
}

func (h *AdminHandler) ImportExecute(c *gin.Context) {
	var req importExecReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type cred struct {
		Name         string `json:"name"`
		Email        string `json:"email"`
		Company      string `json:"company"`
		TempPassword string `json:"temp_password"`
	}
	type rowErr struct {
		Email string `json:"email"`
		Error string `json:"error"`
	}
	creds := []cred{}

	compCreated, compUpdated, compSkipped := 0, 0, 0
	profCreated, profUpdated, profSkipped := 0, 0, 0
	compErrors := []rowErr{}
	profErrors := []rowErr{}

	for _, r := range req.Companies {
		d := r.Data
		switch r.Action {
		case "skip":
			compSkipped++
		case "overwrite":
			existing, err := h.service.FindUserByEmail(d["email"])
			if err != nil || existing == nil {
				compErrors = append(compErrors, rowErr{d["email"], "No se encontró el usuario a sobreescribir"})
				continue
			}
			updates := map[string]interface{}{}
			putIf(updates, "name", d["nombre_responsable"])
			putIf(updates, "company_name", d["nombre_empresa"])
			putIf(updates, "industry", d["industria"])
			putIf(updates, "phone_number", d["telefono"])
			putIf(updates, "country", d["pais"])
			putIf(updates, "state", d["estado_provincia"])
			putIf(updates, "city", d["ciudad"])
			putIf(updates, "location", d["ubicacion"])
			putIf(updates, "address", d["direccion"])
			if _, err := h.service.UpdateUser(existing.ID, updates); err != nil {
				compErrors = append(compErrors, rowErr{d["email"], err.Error()})
				continue
			}
			compUpdated++
		default:
			temp, err := generateTempPassword(12)
			if err != nil {
				compErrors = append(compErrors, rowErr{d["email"], "No se pudo generar la contraseña temporal"})
				continue
			}
			payload := map[string]interface{}{
				"name": d["nombre_responsable"], "email": d["email"], "password": temp,
				"user_type": string(models.UserTypeEmployer), "company_name": d["nombre_empresa"],
				"industry": d["industria"], "phone_number": d["telefono"], "country": d["pais"],
				"state": d["estado_provincia"], "city": d["ciudad"], "location": d["ubicacion"], "address": d["direccion"],
			}
			u, err := h.service.CreateUser(payload)
			if err != nil {
				compErrors = append(compErrors, rowErr{d["email"], err.Error()})
				continue
			}
			compCreated++
			creds = append(creds, cred{u.Name, u.Email, d["nombre_empresa"], temp})
		}
	}

	companies, _, _ := h.service.GetAllUsers(string(models.UserTypeEmployer), "", "", "", 0, 100000)
	byName := map[string]uint{}
	byID := map[uint]bool{}
	for _, comp := range companies {
		byID[comp.ID] = true
		if comp.CompanyName != "" {
			byName[strings.ToLower(strings.TrimSpace(comp.CompanyName))] = comp.ID
		}
	}

	for _, r := range req.Professionals {
		d := r.Data
		switch r.Action {
		case "skip":
			profSkipped++
		case "overwrite":
			existing, err := h.service.FindUserByEmail(d["email"])
			if err != nil || existing == nil {
				profErrors = append(profErrors, rowErr{d["email"], "No se encontró el usuario a sobreescribir"})
				continue
			}
			updates := map[string]interface{}{}
			putIf(updates, "name", d["nombre"])
			putIf(updates, "job_title", d["cargo"])
			putIf(updates, "phone_number", d["telefono"])
			putIf(updates, "country", d["pais"])
			putIf(updates, "state", d["estado_provincia"])
			putIf(updates, "city", d["ciudad"])
			putIf(updates, "location", d["ubicacion"])
			if strings.TrimSpace(d["es_manager"]) != "" {
				updates["is_manager"] = managerFlag(d["es_manager"])
			}
			u, err := h.service.UpdateUser(existing.ID, updates)
			if err != nil {
				profErrors = append(profErrors, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			profUpdated++
		default:
			empID := resolveCompanyID(d["empresa"], byName, byID)
			if empID == 0 {
				profErrors = append(profErrors, rowErr{d["email"], fmt.Sprintf("Empresa %q no encontrada", d["empresa"])})
				continue
			}
			temp, err := generateTempPassword(12)
			if err != nil {
				profErrors = append(profErrors, rowErr{d["email"], "No se pudo generar la contraseña temporal"})
				continue
			}
			payload := map[string]interface{}{
				"name": d["nombre"], "email": d["email"], "password": temp,
				"user_type": string(models.UserTypeProfessional), "empleador_id": empID,
				"job_title": d["cargo"], "phone_number": d["telefono"], "country": d["pais"],
				"state": d["estado_provincia"], "city": d["ciudad"], "location": d["ubicacion"],
				"is_manager": managerFlag(d["es_manager"]),
			}
			u, err := h.service.CreateUser(payload)
			if err != nil {
				profErrors = append(profErrors, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			profCreated++
			creds = append(creds, cred{u.Name, u.Email, d["empresa"], temp})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"companies":     gin.H{"created": compCreated, "updated": compUpdated, "skipped": compSkipped, "errors": compErrors},
		"professionals": gin.H{"created": profCreated, "updated": profUpdated, "skipped": profSkipped, "errors": profErrors},
		"credentials":   creds,
	})
}

func resolveCompanyID(val string, byName map[string]uint, byID map[uint]bool) uint {
	val = strings.TrimSpace(val)
	if id, err := strconv.ParseUint(val, 10, 32); err == nil && byID[uint(id)] {
		return uint(id)
	}
	if id, ok := byName[strings.ToLower(val)]; ok {
		return id
	}
	return 0
}

func (h *AdminHandler) DownloadEmployerImportTemplate(c *gin.Context) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"6D28D9"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	exampleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Italic: true, Color: "94A3B8", Size: 10}})

	f.SetSheetName("Sheet1", "Instrucciones")
	instructions := []string{
		"IMPORTACIÓN DE PROFESIONALES — OBERTRACK",
		"",
		"1) Completá la hoja PROFESIONALES. Las columnas con * son OBLIGATORIAS.",
		"2) No cambies los nombres de los encabezados. Borrá la fila de EJEMPLO (gris) antes de importar.",
		"3) Los profesionales se crean automáticamente en TU empresa (no se incluye columna de empresa).",
		"",
		"Reglas:",
		"   • Contraseña: NO la pongas; se genera una temporal por fila y te la entregamos al finalizar.",
		"   • Email ya existente: si pertenece a TU empresa podrás SOBREESCRIBIR u OMITIR; si pertenece a",
		"     otra empresa, no se podrá importar (usá otro correo).",
		"   • es_manager: escribí 'Sí' si el profesional es un manager; 'No' o vacío en caso contrario.",
		"   • País y ubicación: texto libre.",
	}
	for i, line := range instructions {
		_ = f.SetCellValue("Instrucciones", fmt.Sprintf("A%d", i+1), line)
	}
	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14, Color: "6D28D9"}})
	_ = f.SetCellStyle("Instrucciones", "A1", "A1", titleStyle)
	_ = f.SetColWidth("Instrucciones", "A", "A", 95)

	writeImportSheet(f, "Profesionales", importEmployerProfHeaders, headerStyle, exampleStyle, []string{
		"María González", "maria@miempresa.com",
		"Desarrolladora Backend", "+58 412 111 1111", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "No",
	})

	if idx, err := f.GetSheetIndex("Instrucciones"); err == nil {
		f.SetActiveSheet(idx)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar la plantilla"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=plantilla_profesionales_obertrack.xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

func employerProfRows(xl *excelize.File, h *AdminHandler, tenantID uint) []importRow {
	seen := map[string]int{}
	reports := []importRow{}
	idx, rows := readImportSheet(xl, "Profesionales")
	if idx == nil {
		return reports
	}
	for i, row := range rows {
		data := map[string]string{
			"nombre":           cellVal(idx, row, "nombre"),
			"email":            cellVal(idx, row, "email"),
			"cargo":            cellVal(idx, row, "cargo"),
			"telefono":         cellVal(idx, row, "telefono"),
			"pais":             cellVal(idx, row, "pais"),
			"estado_provincia": cellVal(idx, row, "estado_provincia"),
			"ciudad":           cellVal(idx, row, "ciudad"),
			"ubicacion":        cellVal(idx, row, "ubicacion"),
			"es_manager":       cellVal(idx, row, "es_manager"),
		}
		if emptyRow(data) {
			continue
		}
		rep := importRow{Row: i + 2, Data: data, Status: "ok"}
		email := strings.ToLower(data["email"])
		switch {
		case data["nombre"] == "" || data["email"] == "":
			rep.Status, rep.Message = "error", "Faltan campos obligatorios (nombre, email)."
		case !validEmail(data["email"]):
			rep.Status, rep.Message = "error", "Email inválido."
		case seen[email] != 0:
			rep.Status, rep.Message = "error", fmt.Sprintf("Email repetido en el archivo (ya está en la fila %d).", seen[email])
		default:
			seen[email] = rep.Row
			if existing, err := h.service.FindUserByEmail(data["email"]); err == nil && existing != nil {
				if models.TenantForUser(existing) == tenantID && existing.UserType == models.UserTypeProfessional {
					rep.Status, rep.Message = "conflict", "Este correo ya existe en tu empresa."
					rep.Existing = &importExisting{ID: existing.ID, Name: existing.Name, Email: existing.Email}
				} else {
					rep.Status, rep.Message = "error", "Este correo ya está registrado y no pertenece a tu empresa."
				}
			}
		}
		reports = append(reports, rep)
	}
	return reports
}

func (h *AdminHandler) EmployerImportPreview(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tu cuenta no está asociada a una empresa"})
		return
	}
	xl, closer, ok := openUploadedXlsx(c)
	if !ok {
		return
	}
	defer closer()

	reports := employerProfRows(xl, h, tenantID)
	c.JSON(http.StatusOK, gin.H{
		"professionals": reports,
		"summary":       importSummary(nil, reports),
	})
}

func (h *AdminHandler) EmployerImportExecute(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tu cuenta no está asociada a una empresa"})
		return
	}
	var req importExecReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type cred struct {
		Name         string `json:"name"`
		Email        string `json:"email"`
		Company      string `json:"company"`
		TempPassword string `json:"temp_password"`
	}
	type rowErr struct {
		Email string `json:"email"`
		Error string `json:"error"`
	}
	creds := []cred{}
	created, updated, skipped := 0, 0, 0
	errs := []rowErr{}

	for _, r := range req.Professionals {
		d := r.Data
		switch r.Action {
		case "skip":
			skipped++
		case "overwrite":
			existing, err := h.service.FindUserByEmail(d["email"])
			if err != nil || existing == nil {
				errs = append(errs, rowErr{d["email"], "No se encontró el usuario a sobreescribir"})
				continue
			}
			updates := map[string]interface{}{}
			putIf(updates, "name", d["nombre"])
			putIf(updates, "job_title", d["cargo"])
			putIf(updates, "phone_number", d["telefono"])
			putIf(updates, "country", d["pais"])
			putIf(updates, "state", d["estado_provincia"])
			putIf(updates, "city", d["ciudad"])
			putIf(updates, "location", d["ubicacion"])
			if strings.TrimSpace(d["es_manager"]) != "" {
				updates["is_manager"] = managerFlag(d["es_manager"])
			}
			u, err := h.service.UpdateUserScoped(existing.ID, updates, tenantID)
			if err != nil {
				errs = append(errs, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			updated++
		default:
			temp, err := generateTempPassword(12)
			if err != nil {
				errs = append(errs, rowErr{d["email"], "No se pudo generar la contraseña temporal"})
				continue
			}
			payload := map[string]interface{}{
				"name": d["nombre"], "email": d["email"], "password": temp,
				"user_type": string(models.UserTypeProfessional), "empleador_id": tenantID,
				"job_title": d["cargo"], "phone_number": d["telefono"], "country": d["pais"],
				"state": d["estado_provincia"], "city": d["ciudad"], "location": d["ubicacion"],
				"is_manager": managerFlag(d["es_manager"]),
			}
			u, err := h.service.CreateUser(payload)
			if err != nil {
				errs = append(errs, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			created++
			creds = append(creds, cred{u.Name, u.Email, "", temp})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"professionals": gin.H{"created": created, "updated": updated, "skipped": skipped, "errors": errs},
		"credentials":   creds,
	})
}
