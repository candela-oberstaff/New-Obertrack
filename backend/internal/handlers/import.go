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
	"cargo", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "es_manager", "es_supervisor", "reporta_a",
}

var importEmployerProfHeaders = []string{
	"nombre *", "email *",
	"cargo", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "es_manager", "es_supervisor", "reporta_a",
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
		"3) Borrá las filas de EJEMPLO (en gris) antes de importar.",
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
		"   • es_manager: escribe 'Sí' si el profesional es un manager (puede tener gente a cargo);",
		"     'No' o vacío en caso contrario.",
		"   • es_supervisor: escribe 'Sí' si además tiene MANAGERS a su cargo, no solo profesionales.",
		"     Un supervisor ve y aprueba todo lo que cuelga de él hacia abajo. Marcarlo implica",
		"     es_manager: no hace falta poner las dos, pero tampoco estorba.",
		"   • reporta_a: EMAIL del manager de esa persona. Dejalo vacío para quien no le reporta a nadie.",
		"",
		"Cómo armar el organigrama con reporta_a:",
		"   • Se usa el EMAIL del manager, no su nombre (el email es único y evita confusiones",
		"     entre homónimos).",
		"   • El manager puede estar en otra fila de esta misma hoja (arriba o abajo, da igual) o",
		"     ya existir en el sistema. Debe ser de la MISMA empresa que quien le reporta.",
		"   • Si alguien aparece en reporta_a pero su fila dice es_manager 'No', lo marcamos como",
		"     manager automáticamente y te lo avisamos en la previsualización.",
		"   • Se admiten varios niveles (dirección → gerencia → equipo). Lo único que se rechaza es",
		"     un círculo: que dos personas terminen siendo manager una de la otra.",
		"",
		"Reglas importantes:",
		"   • Contraseña: NO la pongas en el Excel. El sistema genera una temporal por cada fila",
		"     y te la entrega al finalizar para que la compartas.",
		"   • Email ya existente: te avisaremos a quién pertenece y podrás elegir SOBREESCRIBIR u OMITIR,",
		"     fila por fila. Por defecto se OMITE.",
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

	// Tres filas de ejemplo que arman una cadena completa: la supervisora, la
	// manager que le reporta y alguien del equipo de esa manager. Así se ve de
	// una sola lectura cómo se encadenan con reporta_a y en qué se diferencian
	// es_manager y es_supervisor.
	writeImportSheet(f, "Profesionales", importProfessionalHeaders, headerStyle, exampleStyle, []string{
		"Ana Torres", "ana@miempresa.com", "Mi Empresa S.A.",
		"Directora de Operaciones", "+58 412 000 1111", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "Sí", "Sí", "",
	}, []string{
		"María González", "maria@miempresa.com", "Mi Empresa S.A.",
		"Líder de Desarrollo", "+58 412 111 1111", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "Sí", "No", "ana@miempresa.com",
	}, []string{
		"Pedro Ruiz", "pedro@miempresa.com", "Mi Empresa S.A.",
		"Desarrollador Backend", "+58 412 222 2222", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "No", "No", "maria@miempresa.com",
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

func writeImportSheet(f *excelize.File, sheet string, headers []string, headerStyle, exampleStyle int, exampleRows ...[]string) {
	_, _ = f.NewSheet(sheet)
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s1", col), h)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s1", col), fmt.Sprintf("%s1", col), headerStyle)
		_ = f.SetColWidth(sheet, col, col, 22)
	}
	for r, exampleRow := range exampleRows {
		for i, v := range exampleRow {
			col, _ := excelize.ColumnNumberToName(i + 1)
			cell := fmt.Sprintf("%s%d", col, r+2)
			_ = f.SetCellValue(sheet, cell, v)
			_ = f.SetCellStyle(sheet, cell, cell, exampleStyle)
		}
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
	// Warning es informativo: la fila se importa igual (p.ej. alguien marcado
	// como manager automáticamente porque otra fila le reporta).
	Warning string `json:"warning,omitempty"`
}

// Claves que se leen de cada hoja de profesionales, en el orden de la plantilla.
var profDataKeys = []string{
	"nombre", "email", "empresa", "cargo", "telefono",
	"pais", "estado_provincia", "ciudad", "ubicacion", "es_manager", "es_supervisor", "reporta_a",
}

var employerProfDataKeys = []string{
	"nombre", "email", "cargo", "telefono",
	"pais", "estado_provincia", "ciudad", "ubicacion", "es_manager", "es_supervisor", "reporta_a",
}

// profSheetRow es una fila ya parseada de la hoja Profesionales. Conserva el
// número de fila del Excel para poder señalarla en la previsualización.
type profSheetRow struct {
	row  int
	data map[string]string
}

// parseProfSheet lee la hoja completa antes de validar nada. Hace falta porque
// reporta_a puede apuntar a una fila que está más abajo en el mismo archivo.
func parseProfSheet(xl *excelize.File, sheet string, keys []string) []profSheetRow {
	idx, rows := readImportSheet(xl, sheet)
	if idx == nil {
		return nil
	}
	out := make([]profSheetRow, 0, len(rows))
	for i, row := range rows {
		data := make(map[string]string, len(keys))
		for _, k := range keys {
			data[k] = cellVal(idx, row, k)
		}
		if emptyRow(data) {
			continue
		}
		out = append(out, profSheetRow{row: i + 2, data: data})
	}
	return out
}

// flagOrphanedManagerRows avisa cuando el manager de una fila está en el
// archivo pero su propia fila no se va a importar por tener un error: el
// profesional se crea igual, solo que quedará sin manager. Se resuelve acá
// porque en la ejecución ya sería tarde para que el usuario corrija el Excel.
func flagOrphanedManagerRows(reports []importRow) {
	byEmail := map[string]int{}
	for i, r := range reports {
		if e := strings.ToLower(strings.TrimSpace(r.Data["email"])); e != "" {
			if _, dup := byEmail[e]; !dup {
				byEmail[e] = i
			}
		}
	}
	for i := range reports {
		if reports[i].Status == "error" {
			continue
		}
		mgrEmail := strings.ToLower(strings.TrimSpace(reports[i].Data["reporta_a"]))
		if mgrEmail == "" {
			continue
		}
		j, inFile := byEmail[mgrEmail]
		if !inFile || reports[j].Status != "error" {
			continue
		}
		msg := fmt.Sprintf("Su manager (fila %d) no se va a importar por un error, así que quedará sin manager asignado.", reports[j].Row)
		if reports[i].Warning == "" {
			reports[i].Warning = msg
		} else {
			reports[i].Warning += " " + msg
		}
	}
}

// managerResolver reúne lo que cambia entre la plantilla de superadmin (la
// empresa viene en una columna) y la de empresa (la empresa es el tenant).
type managerResolver struct {
	// findUser busca un usuario ya existente por email; nil si no hay.
	findUser func(email string) *models.User
	// companyKey devuelve un identificador comparable de la empresa de una fila.
	companyKey func(data map[string]string) string
	// existingUsable dice si un usuario ya existente puede ser manager de esa
	// fila; el string es el motivo cuando no.
	existingUsable func(data map[string]string, u *models.User) (bool, string)
}

// resolveProfManagers valida la columna reporta_a de una hoja ya parseada y
// devuelve, por índice de fila, el error que impide la asignación y el aviso a
// mostrar. Solo puede hacerse con la hoja entera a la vista: el manager puede
// ser otra fila del archivo, que todavía no existe en la base.
//
// Efecto lateral deliberado: marca es_manager='Sí' en las filas a las que
// alguien le reporta y no venían marcadas, para que la importación no falle por
// un olvido. Esa data es la que el frontend devuelve al ejecutar.
func resolveProfManagers(rows []profSheetRow, r managerResolver) (map[int]string, map[int]string) {
	errs := map[int]string{}
	warns := map[int]string{}

	byEmail := map[string]int{}
	for i, pr := range rows {
		if e := strings.ToLower(strings.TrimSpace(pr.data["email"])); e != "" {
			if _, dup := byEmail[e]; !dup {
				byEmail[e] = i
			}
		}
	}

	// edges[i] = índice de la fila del manager, cuando está en el mismo archivo.
	edges := map[int]int{}

	for i, pr := range rows {
		mgrEmail := strings.ToLower(strings.TrimSpace(pr.data["reporta_a"]))
		if mgrEmail == "" {
			continue
		}
		if mgrEmail == strings.ToLower(strings.TrimSpace(pr.data["email"])) {
			errs[i] = "Un profesional no puede ser su propio manager (reporta_a)."
			continue
		}
		if j, inFile := byEmail[mgrEmail]; inFile {
			if r.companyKey(pr.data) != r.companyKey(rows[j].data) {
				errs[i] = fmt.Sprintf("El manager %q está en el archivo pero pertenece a otra empresa.", mgrEmail)
				continue
			}
			if !managerFlag(rows[j].data["es_manager"]) {
				rows[j].data["es_manager"] = "Sí"
				warns[j] = fmt.Sprintf("Se marcará como manager automáticamente: la fila %d le reporta.", pr.row)
			}
			edges[i] = j
			continue
		}
		u := r.findUser(mgrEmail)
		if u == nil {
			errs[i] = fmt.Sprintf("El manager %q no existe en el sistema ni está en este archivo.", mgrEmail)
			continue
		}
		if ok, why := r.existingUsable(pr.data, u); !ok {
			errs[i] = why
		}
	}

	// Segunda deducción, ya con todos los es_manager resueltos: si a alguien le
	// reporta un MANAGER, esa persona es supervisora — es literalmente la
	// definición del rol. Se marca sola, igual que es_manager, para que el
	// organigrama salga del archivo sin depender de que alguien se acuerde de la
	// columna. Se recorre por orden de fila y no por el mapa de aristas para que
	// el aviso sea siempre el mismo ante el mismo archivo.
	for i := range rows {
		j, inFile := edges[i]
		if !inFile || !managerFlag(rows[i].data["es_manager"]) {
			continue
		}
		if managerFlag(rows[j].data["es_supervisor"]) {
			continue
		}
		rows[j].data["es_supervisor"] = "Sí"
		// Reemplaza al aviso de "se marcará como manager": decir las dos cosas de
		// la misma fila confunde más de lo que informa, y supervisor ya implica
		// manager.
		warns[j] = fmt.Sprintf("Se marcará como supervisor automáticamente: la fila %d, que es manager, le reporta.", rows[i].row)
	}

	// Ciclos dentro del archivo. Las cadenas que salen hacia usuarios ya
	// existentes no pueden cerrarse (nadie existente reporta a alguien que
	// todavía no se creó), así que basta con recorrer las aristas internas.
	for i := range rows {
		cur, ok := edges[i]
		// El tope de pasos evita quedar dando vueltas cuando la cadena de i
		// desemboca en un círculo del que i no forma parte.
		for steps := 0; ok && steps <= len(rows); steps++ {
			if cur == i {
				if errs[i] == "" {
					errs[i] = "La cadena de reporta_a forma un círculo: revisa quién le reporta a quién."
				}
				break
			}
			cur, ok = edges[cur]
		}
	}

	return errs, warns
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
	companyByName := map[string]uint{}
	companyByID := map[uint]bool{}
	for _, comp := range companies {
		companyByID[comp.ID] = true
		if comp.CompanyName != "" {
			companyByName[strings.ToLower(strings.TrimSpace(comp.CompanyName))] = comp.ID
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
				} else if companyByName[lname] > 0 {
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

	profRows := parseProfSheet(xl, "Profesionales", profDataKeys)

	// La empresa de una fila puede ser un ID, un nombre existente o uno que se
	// crea en la hoja Empresas; se normaliza para poder comparar dos filas.
	companyKey := func(data map[string]string) string {
		val := strings.TrimSpace(data["empresa"])
		if id := resolveCompanyID(val, companyByName, companyByID); id > 0 {
			return fmt.Sprintf("id:%d", id)
		}
		return "name:" + strings.ToLower(val)
	}
	mgrErrs, mgrWarns := resolveProfManagers(profRows, managerResolver{
		findUser: func(email string) *models.User {
			u, err := h.service.FindUserByEmail(email)
			if err != nil {
				return nil
			}
			return u
		},
		companyKey: companyKey,
		existingUsable: func(data map[string]string, u *models.User) (bool, string) {
			if !u.IsManager {
				return false, fmt.Sprintf("%s ya existe pero no está marcado como manager. Marcalo en su ficha o incluilo en el archivo con es_manager 'Sí'.", u.Email)
			}
			if !u.IsActive {
				return false, fmt.Sprintf("El manager %s está inactivo.", u.Email)
			}
			cid := resolveCompanyID(data["empresa"], companyByName, companyByID)
			if cid == 0 || models.TenantForUser(u) != cid {
				return false, fmt.Sprintf("El manager %s no pertenece a la empresa %q.", u.Email, data["empresa"])
			}
			return true, ""
		},
	})

	profReports := []importRow{}
	for i, pr := range profRows {
		data := pr.data
		rep := importRow{Row: pr.row, Data: data, Status: "ok", Warning: mgrWarns[i]}
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
		case mgrErrs[i] != "":
			rep.Status, rep.Message = "error", mgrErrs[i]
		default:
			seen[email] = rep.Row
			if existing, err := h.service.FindUserByEmail(data["email"]); err == nil && existing != nil {
				rep.Status, rep.Message = "conflict", "Este correo ya existe."
				rep.Existing = &importExisting{ID: existing.ID, Name: existing.Name, Email: existing.Email}
			}
		}
		profReports = append(profReports, rep)
	}
	flagOrphanedManagerRows(profReports)

	c.JSON(http.StatusOK, gin.H{
		"companies":     compReports,
		"professionals": profReports,
		"summary":       importSummary(compReports, profReports),
	})
}

func companyResolvable(val string, byName map[string]uint, byID map[uint]bool, pending map[string]bool) bool {
	val = strings.TrimSpace(val)
	if val == "" {
		return false
	}
	if id, err := strconv.ParseUint(val, 10, 32); err == nil && byID[uint(id)] {
		return true
	}
	l := strings.ToLower(val)
	return byName[l] > 0 || pending[l]
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

type importRowErr struct {
	Email string `json:"email"`
	Error string `json:"error"`
}

// importManagerLink es una asignación pendiente de la segunda pasada: el
// profesional ya existe (recién creado o sobreescrito) y falta colgarlo del
// manager que indicaba su columna reporta_a.
type importManagerLink struct {
	profID       uint
	managerEmail string
}

// applyImportManagers resuelve reporta_a una vez creadas todas las filas. Tiene
// que ser una segunda pasada: el manager puede estar más abajo en el archivo y
// no existir todavía cuando se procesa a quien le reporta.
//
// Agrupa por manager para delegar en BulkAssignManager, que es el guard
// compartido con el panel: valida empresa, rol, estado y que la asignación no
// cierre un ciclo en la cadena de mando, y mantiene el espejo de
// employments.manager_id y employment_managers. Escribir manager_id a mano
// dejaría esas tablas desincronizadas.
func (h *AdminHandler) applyImportManagers(links []importManagerLink, tenantID uint) (int, []importRowErr) {
	errs := []importRowErr{}
	if len(links) == 0 {
		return 0, errs
	}

	byManager := map[string][]uint{}
	order := []string{}
	for _, l := range links {
		email := strings.ToLower(strings.TrimSpace(l.managerEmail))
		if email == "" || l.profID == 0 {
			continue
		}
		if _, ok := byManager[email]; !ok {
			order = append(order, email)
		}
		byManager[email] = append(byManager[email], l.profID)
	}

	assigned := 0
	for _, email := range order {
		ids := byManager[email]
		mgr, err := h.service.FindUserByEmail(email)
		if err != nil || mgr == nil {
			errs = append(errs, importRowErr{email, fmt.Sprintf("Manager no encontrado: %d fila(s) quedaron sin asignar", len(ids))})
			continue
		}
		mgrID := mgr.ID

		var n, skipped int
		var aerr error
		if tenantID > 0 {
			n, skipped, aerr = h.service.BulkAssignManagerScoped(ids, &mgrID, tenantID)
		} else {
			n, skipped, aerr = h.service.BulkAssignManager(ids, &mgrID)
		}
		if aerr != nil {
			errs = append(errs, importRowErr{email, aerr.Error()})
			continue
		}
		assigned += n
		if skipped > 0 {
			errs = append(errs, importRowErr{email, fmt.Sprintf("%d fila(s) no se pudieron asignar a este manager (otra empresa, o la asignación crearía un ciclo en la cadena de mando)", skipped)})
		}
	}
	return assigned, errs
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
	type rowErr = importRowErr
	creds := []cred{}

	compCreated, compUpdated, compSkipped := 0, 0, 0
	profCreated, profUpdated, profSkipped := 0, 0, 0
	compErrors := []rowErr{}
	profErrors := []rowErr{}
	mgrLinks := []importManagerLink{}

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
			// El servicio ya sabe que supervisor implica manager, así que aquí
			// solo se traslada la columna tal cual venga.
			if strings.TrimSpace(d["es_supervisor"]) != "" {
				updates["is_supervisor"] = managerFlag(d["es_supervisor"])
			}
			u, err := h.service.UpdateUser(existing.ID, updates)
			if err != nil {
				profErrors = append(profErrors, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			if d["reporta_a"] != "" {
				mgrLinks = append(mgrLinks, importManagerLink{u.ID, d["reporta_a"]})
			}
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
				"is_manager":    managerFlag(d["es_manager"]),
				"is_supervisor": managerFlag(d["es_supervisor"]),
			}
			u, err := h.service.CreateUser(payload)
			if err != nil {
				profErrors = append(profErrors, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			if d["reporta_a"] != "" {
				mgrLinks = append(mgrLinks, importManagerLink{u.ID, d["reporta_a"]})
			}
			profCreated++
			creds = append(creds, cred{u.Name, u.Email, d["empresa"], temp})
		}
	}

	// Segunda pasada: ya existen todas las filas, así que ahora sí se puede
	// resolver reporta_a (el manager podía estar más abajo en el archivo).
	mgrAssigned, mgrErrors := h.applyImportManagers(mgrLinks, 0)

	c.JSON(http.StatusOK, gin.H{
		"companies":     gin.H{"created": compCreated, "updated": compUpdated, "skipped": compSkipped, "errors": compErrors},
		"professionals": gin.H{"created": profCreated, "updated": profUpdated, "skipped": profSkipped, "errors": profErrors},
		"managers":      gin.H{"assigned": mgrAssigned, "errors": mgrErrors},
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
		"2) No cambies los nombres de los encabezados. Borrá las filas de EJEMPLO (gris) antes de importar.",
		"3) Los profesionales se crean automáticamente en TU empresa (no se incluye columna de empresa).",
		"",
		"Reglas:",
		"   • Contraseña: NO la pongas; se genera una temporal por fila y te la entregamos al finalizar.",
		"   • Email ya existente: si pertenece a TU empresa podrás SOBREESCRIBIR u OMITIR; si pertenece a",
		"     otra empresa, no se podrá importar (usá otro correo).",
		"   • es_manager: escribe 'Sí' si el profesional es un manager; 'No' o vacío en caso contrario.",
		"   • es_supervisor: escribe 'Sí' si además tiene MANAGERS a su cargo, no solo profesionales.",
		"     Ve y aprueba todo lo que cuelga de él hacia abajo. Marcarlo implica es_manager.",
		"   • País y ubicación: texto libre.",
		"",
		"Organigrama (columna reporta_a):",
		"   • Poné el EMAIL del manager de esa persona; vacío si no le reporta a nadie.",
		"   • El manager puede estar en otra fila de esta hoja (arriba o abajo) o ya existir en tu empresa.",
		"   • Si alguien aparece en reporta_a pero su fila dice es_manager 'No', lo marcamos como manager",
		"     automáticamente y te lo avisamos en la previsualización.",
		"   • Se admiten varios niveles. Solo se rechaza el círculo: que dos personas terminen siendo",
		"     manager una de la otra.",
	}
	for i, line := range instructions {
		_ = f.SetCellValue("Instrucciones", fmt.Sprintf("A%d", i+1), line)
	}
	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14, Color: "6D28D9"}})
	_ = f.SetCellStyle("Instrucciones", "A1", "A1", titleStyle)
	_ = f.SetColWidth("Instrucciones", "A", "A", 95)

	writeImportSheet(f, "Profesionales", importEmployerProfHeaders, headerStyle, exampleStyle, []string{
		"Ana Torres", "ana@miempresa.com",
		"Directora de Operaciones", "+58 412 000 1111", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "Sí", "Sí", "",
	}, []string{
		"María González", "maria@miempresa.com",
		"Líder de Desarrollo", "+58 412 111 1111", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "Sí", "No", "ana@miempresa.com",
	}, []string{
		"Pedro Ruiz", "pedro@miempresa.com",
		"Desarrollador Backend", "+58 412 222 2222", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "No", "No", "maria@miempresa.com",
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
	rows := parseProfSheet(xl, "Profesionales", employerProfDataKeys)
	if len(rows) == 0 {
		return reports
	}

	// Todas las filas son de la misma empresa (el tenant de quien importa), así
	// que la comparación por empresa es trivial y solo hay que acotar los
	// managers que ya existen en el sistema.
	mgrErrs, mgrWarns := resolveProfManagers(rows, managerResolver{
		findUser: func(email string) *models.User {
			u, err := h.service.FindUserByEmail(email)
			if err != nil {
				return nil
			}
			return u
		},
		companyKey: func(map[string]string) string { return "tenant" },
		existingUsable: func(_ map[string]string, u *models.User) (bool, string) {
			if models.TenantForUser(u) != tenantID {
				return false, fmt.Sprintf("El manager %s no pertenece a tu empresa.", u.Email)
			}
			if !u.IsManager {
				return false, fmt.Sprintf("%s ya existe pero no está marcado como manager. Marcalo en su ficha o incluilo en el archivo con es_manager 'Sí'.", u.Email)
			}
			if !u.IsActive {
				return false, fmt.Sprintf("El manager %s está inactivo.", u.Email)
			}
			return true, ""
		},
	})

	for i, pr := range rows {
		data := pr.data
		rep := importRow{Row: pr.row, Data: data, Status: "ok", Warning: mgrWarns[i]}
		email := strings.ToLower(data["email"])
		switch {
		case data["nombre"] == "" || data["email"] == "":
			rep.Status, rep.Message = "error", "Faltan campos obligatorios (nombre, email)."
		case !validEmail(data["email"]):
			rep.Status, rep.Message = "error", "Email inválido."
		case seen[email] != 0:
			rep.Status, rep.Message = "error", fmt.Sprintf("Email repetido en el archivo (ya está en la fila %d).", seen[email])
		case mgrErrs[i] != "":
			rep.Status, rep.Message = "error", mgrErrs[i]
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
	flagOrphanedManagerRows(reports)
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
	type rowErr = importRowErr
	creds := []cred{}
	created, updated, skipped := 0, 0, 0
	errs := []rowErr{}
	mgrLinks := []importManagerLink{}

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
			// El servicio ya sabe que supervisor implica manager, así que aquí
			// solo se traslada la columna tal cual venga.
			if strings.TrimSpace(d["es_supervisor"]) != "" {
				updates["is_supervisor"] = managerFlag(d["es_supervisor"])
			}
			u, err := h.service.UpdateUserScoped(existing.ID, updates, tenantID)
			if err != nil {
				errs = append(errs, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			if d["reporta_a"] != "" {
				mgrLinks = append(mgrLinks, importManagerLink{u.ID, d["reporta_a"]})
			}
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
				"is_manager":    managerFlag(d["es_manager"]),
				"is_supervisor": managerFlag(d["es_supervisor"]),
			}
			u, err := h.service.CreateUser(payload)
			if err != nil {
				errs = append(errs, rowErr{d["email"], err.Error()})
				continue
			}
			_ = h.employmentSvc.SyncActiveForUser(u)
			if d["reporta_a"] != "" {
				mgrLinks = append(mgrLinks, importManagerLink{u.ID, d["reporta_a"]})
			}
			created++
			creds = append(creds, cred{u.Name, u.Email, "", temp})
		}
	}

	// Segunda pasada, acotada al tenant: reporta_a puede apuntar a una fila que
	// recién se creó en este mismo lote.
	mgrAssigned, mgrErrors := h.applyImportManagers(mgrLinks, tenantID)

	c.JSON(http.StatusOK, gin.H{
		"professionals": gin.H{"created": created, "updated": updated, "skipped": skipped, "errors": errs},
		"managers":      gin.H{"assigned": mgrAssigned, "errors": mgrErrors},
		"credentials":   creds,
	})
}
