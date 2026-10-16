package mcp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

// Tools de lectura. Cada uno se registra solo si el rol tiene acceso: un rol
// sin acceso no lo ve ni en tools/list. Las consultas pasan SIEMPRE por
// rt.RO.Query (READ ONLY + timeout) y por los filtros de scope.go.

// Quién puede qué.
func (s *Scope) canCompanies() bool {
	return s.CrossCompany || s.RoleKey == RoleEmployer
}
func (s *Scope) canPeople() bool {
	return s.CrossCompany || s.RoleKey == RoleEmployer || s.IsManager
}
func (s *Scope) canHours() bool   { return s.RoleKey != RoleIT && s.CanModule("hours") }
func (s *Scope) canTasks() bool   { return s.RoleKey != RoleIT && s.CanModule("tasks") }
func (s *Scope) canTickets() bool { return s.RoleKey == RoleSuperadmin }
func (s *Scope) canIncidents() bool {
	return s.CrossCompany
}
func (s *Scope) canOverview() bool {
	return s.CrossCompany || s.RoleKey == RoleIT
}

func registerTools(srv *gomcp.Server, rt *Runtime) {
	s := rt.Scope
	registerReadTool(srv, rt, "whoami", "Quién soy",
		"Quién está conectado: nombre, rol, empresa, qué puede ver (alcance) y cuántas consultas lleva hoy. No consume cuota.",
		true, rt.toolWhoami)
	if s.canOverview() {
		registerReadTool(srv, rt, "general_overview", "Resumen general",
			"Totales agregados de la plataforma (empresas, profesionales, jornadas y horas de los últimos 30 días, tareas abiertas, incidentes). Sin datos personales.",
			false, rt.toolOverview)
	}
	if s.canCompanies() {
		registerReadTool(srv, rt, "list_companies", "Listar empresas",
			"Lista las empresas cliente visibles para tu rol, con su número de profesionales activos.", false, rt.toolListCompanies)
		registerReadTool(srv, rt, "get_company", "Ver empresa",
			"Ficha de una empresa: datos, totales de equipo, jornadas pendientes, horas de los últimos 30 días, tareas abiertas y, para plataforma, los últimos eventos del expediente.",
			false, rt.toolGetCompany)
	}
	if s.canPeople() {
		registerReadTool(srv, rt, "list_professionals", "Listar profesionales",
			"Profesionales con empleo activo visibles para tu rol (empresa: su equipo; manager: su equipo). Los teléfonos llegan enmascarados.",
			false, rt.toolListProfessionals)
	}
	if s.canHours() {
		registerReadTool(srv, rt, "list_work_hours", "Listar jornadas",
			"Jornadas (horas registradas) visibles para tu rol, por defecto de los últimos 30 días. Filtra por empresa, profesional, fechas y estado.",
			false, rt.toolListWorkHours)
		registerReadTool(srv, rt, "hours_summary", "Resumen de horas",
			"Horas totales por profesional en un rango de fechas (por defecto, los últimos 30 días): días, horas, aprobadas, pendientes, rechazadas y ausencias.",
			false, rt.toolHoursSummary)
	}
	if s.canTasks() {
		registerReadTool(srv, rt, "list_boards", "Listar tableros",
			"Tableros de tareas visibles para tu rol, con número de tareas abiertas y miembros.", false, rt.toolListBoards)
		registerReadTool(srv, rt, "list_tasks", "Listar tareas",
			"Tareas visibles para tu rol. Filtra por tablero, estado, prioridad, responsable, texto o vencidas.", false, rt.toolListTasks)
		registerReadTool(srv, rt, "get_task", "Ver tarea",
			"Detalle de una tarea: descripción, responsables y últimos comentarios.", false, rt.toolGetTask)
	}
	if s.canTickets() {
		registerReadTool(srv, rt, "list_tickets", "Listar tickets",
			"Tickets de soporte (bandeja interna). Filtra por estado, etapa o texto.", false, rt.toolListTickets)
	}
	if s.canIncidents() {
		registerReadTool(srv, rt, "list_incidents", "Listar incidentes",
			"Incidentes de emergencia con cuántos profesionales respondieron.", false, rt.toolListIncidents)
	}
	if s.canSearch() {
		registerReadTool(srv, rt, "search", "Buscar",
			"Busca tareas, empresas, profesionales y tickets visibles para tu rol. Devuelve {results:[{id,title,url}]}; usa fetch con el id para ver el detalle.",
			false, rt.toolSearch)
		registerReadTool(srv, rt, "fetch", "Ver resultado",
			"Devuelve el detalle de un resultado de search por su id (por ejemplo task-12, company-3, professional-8, ticket-5).",
			false, rt.toolFetch)
	}
}

func (s *Scope) canSearch() bool {
	return s.canTasks() || s.canCompanies() || s.canPeople() || s.canTickets()
}

// ---------------------------------------------------------------------------
// Utilidades

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// dateRange valida AAAA-MM-DD y, si no viene nada, usa los últimos 30 días.
func dateRange(from, to string) (string, string, error) {
	for _, d := range []string{from, to} {
		if d != "" {
			if !dateRe.MatchString(d) {
				return "", "", userErr("Las fechas van en formato AAAA-MM-DD (recibí %q).", d)
			}
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return "", "", userErr("Fecha inválida: %q.", d)
			}
		}
	}
	if to == "" {
		to = time.Now().UTC().Format("2006-01-02")
	}
	if from == "" {
		t, _ := time.Parse("2006-01-02", to)
		from = t.AddDate(0, 0, -30).Format("2006-01-02")
	}
	if from > to {
		return "", "", userErr("from (%s) es posterior a to (%s).", from, to)
	}
	return from, to, nil
}

// likeArg escapa los comodines de ILIKE en el texto de búsqueda.
func likeArg(q string) string {
	q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(q))
	return "%" + q + "%"
}

// list consulta filas con LIMIT n+1 para saber si hubo más.
func (rt *Runtime) list(ctx context.Context, limit int, sql string, args ...any) ([]map[string]any, bool, error) {
	n := rt.maxRows(limit)
	var rows []map[string]any
	err := rt.RO.Query(ctx, func(tx *gorm.DB) error {
		rows = nil
		return tx.Raw(sql+fmt.Sprintf(" LIMIT %d", n+1), args...).Scan(&rows).Error
	})
	if err != nil {
		return nil, false, err
	}
	truncated := len(rows) > n
	if truncated {
		rows = rows[:n]
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	return rows, truncated, nil
}

// listResult arma la respuesta estándar de un listado.
func listResult(rows []map[string]any, truncated, untrusted bool) *result {
	out := map[string]any{"items": rows, "count": len(rows), "truncated": truncated}
	if truncated {
		out["hint"] = "Hay más resultados que el máximo por respuesta de tu rol: acota con filtros."
	}
	return &result{Data: out, Rows: len(rows), Untrusted: untrusted}
}

// ---------------------------------------------------------------------------
// whoami y resumen

type noInput struct{}

func (rt *Runtime) toolWhoami(ctx context.Context, _ noInput) (*result, error) {
	s := rt.Scope
	out := map[string]any{
		"user":           map[string]any{"id": s.UserID, "name": s.Name, "email": s.Email},
		"role":           RoleLabels[s.RoleKey],
		"scope":          s.Describe(),
		"access":         "solo lectura",
		"tools":          rt.toolNames,
		"calls_today":    rt.Svc.CallsToday(rt.User.ID),
		"daily_limit":    rt.Quota.DailyCalls,
		"max_rows":       rt.maxRows(0),
		"minimal_data":   rt.Settings.MinimalData,
		"connected_with": rt.Conn.ClientName,
	}
	if s.TenantID > 0 {
		out["company"] = map[string]any{"id": s.TenantID, "name": s.TenantName}
	}
	return &result{Data: out, Rows: 1}, nil
}

func (rt *Runtime) toolOverview(ctx context.Context, _ noInput) (*result, error) {
	var row map[string]any
	err := rt.RO.Query(ctx, func(tx *gorm.DB) error {
		row = map[string]any{}
		return tx.Raw(`SELECT
			(SELECT count(*) FROM users WHERE deleted_at IS NULL AND user_type = 'empleador') AS companies,
			(SELECT count(*) FROM users WHERE deleted_at IS NULL AND user_type = 'empleador' AND is_active) AS active_companies,
			(SELECT count(*) FROM users WHERE deleted_at IS NULL AND user_type = 'profesional' AND is_active) AS active_professionals,
			(SELECT count(*) FROM work_hours WHERE deleted_at IS NULL AND work_date >= current_date - 30) AS workdays_last_30d,
			(SELECT COALESCE(sum(hours_worked), 0)::float8 FROM work_hours WHERE deleted_at IS NULL AND work_date >= current_date - 30) AS hours_last_30d,
			(SELECT count(*) FROM work_hours WHERE deleted_at IS NULL AND NOT approved AND NOT rejected) AS workdays_pending_approval,
			(SELECT count(*) FROM tasks WHERE deleted_at IS NULL AND NOT completed) AS open_tasks,
			(SELECT count(*) FROM tasks WHERE deleted_at IS NULL AND created_at >= now() - interval '30 days') AS tasks_created_last_30d,
			(SELECT count(*) FROM incidents WHERE deleted_at IS NULL AND status = 'open') AS open_incidents`).Scan(&row).Error
	})
	if err != nil {
		return nil, err
	}
	return &result{Data: row, Rows: 1}, nil
}

// ---------------------------------------------------------------------------
// Empresas

type listCompaniesIn struct {
	Search     string `json:"search,omitempty" jsonschema:"Texto a buscar en el nombre de la empresa"`
	ActiveOnly bool   `json:"active_only,omitempty" jsonschema:"Solo empresas activas (no suspendidas)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Máximo de filas (lo acota la cuota del rol)"`
}

func (rt *Runtime) toolListCompanies(ctx context.Context, in listCompaniesIn) (*result, error) {
	w, err := rt.Scope.CompaniesFilter()
	if err != nil {
		return nil, err
	}
	if in.Search != "" {
		w.add("u.company_name ILIKE ? OR u.name ILIKE ?", likeArg(in.Search), likeArg(in.Search))
	}
	if in.ActiveOnly {
		w.add("u.is_active")
	}
	rows, trunc, err := rt.list(ctx, in.Limit, `SELECT u.id, COALESCE(NULLIF(u.company_name, ''), u.name) AS name, u.industry,
		u.is_active AS active, to_char(u.client_since, 'YYYY-MM-DD') AS client_since, cs.name AS assigned_cs,
		(SELECT count(*) FROM employments e WHERE e.company_id = u.id AND e.status = 'active' AND e.deleted_at IS NULL) AS active_professionals
		FROM users u LEFT JOIN users cs ON cs.id = u.assigned_cs_id
		WHERE `+w.SQL()+` ORDER BY 2`, w.args...)
	if err != nil {
		return nil, err
	}
	return listResult(rows, trunc, false), nil
}

type companyIn struct {
	CompanyID uint `json:"company_id" jsonschema:"Id de la empresa (ver list_companies)"`
}

func (rt *Runtime) companyDetail(ctx context.Context, companyID uint) (map[string]any, error) {
	w, err := rt.Scope.CompaniesFilter()
	if err != nil {
		return nil, err
	}
	w.add("u.id = ?", companyID)
	var company map[string]any
	var timeline []map[string]any
	err = rt.RO.Query(ctx, func(tx *gorm.DB) error {
		company, timeline = map[string]any{}, nil
		res := tx.Raw(`SELECT u.id, COALESCE(NULLIF(u.company_name, ''), u.name) AS name, u.email, u.phone_number AS phone,
			u.industry, u.country, u.is_active AS active, to_char(u.client_since, 'YYYY-MM-DD') AS client_since, cs.name AS assigned_cs,
			(SELECT count(*) FROM employments e WHERE e.company_id = u.id AND e.status = 'active' AND e.deleted_at IS NULL) AS active_professionals,
			(SELECT count(*) FROM employments e JOIN users p ON p.id = e.user_id
				WHERE e.company_id = u.id AND e.status = 'active' AND e.deleted_at IS NULL AND p.is_manager) AS managers,
			(SELECT count(*) FROM work_hours wh WHERE wh.tenant_id = u.id AND wh.deleted_at IS NULL AND NOT wh.approved AND NOT wh.rejected) AS workdays_pending_approval,
			(SELECT COALESCE(sum(wh.hours_worked), 0)::float8 FROM work_hours wh WHERE wh.tenant_id = u.id AND wh.deleted_at IS NULL AND wh.work_date >= current_date - 30) AS hours_last_30d,
			(SELECT count(*) FROM tasks t WHERE t.tenant_id = u.id AND t.deleted_at IS NULL AND NOT t.completed) AS open_tasks,
			(SELECT count(*) FROM boards b WHERE b.tenant_id = u.id AND b.deleted_at IS NULL) AS boards
			FROM users u LEFT JOIN users cs ON cs.id = u.assigned_cs_id WHERE `+w.SQL(), w.args...).Scan(&company)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			company = nil
			return nil
		}
		// El expediente es material interno de plataforma (CS y superadmin).
		if rt.Scope.CrossCompany {
			return tx.Raw(`SELECT ev.type, ev.detail, ev.channel, ev.pinned, to_char(ev.created_at, 'YYYY-MM-DD HH24:MI') AS created_at, by_u.name AS by
				FROM company_events ev LEFT JOIN users by_u ON by_u.id = ev.by_user_id
				WHERE ev.company_id = ? ORDER BY ev.pinned DESC, ev.created_at DESC LIMIT 10`, companyID).Scan(&timeline).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if company == nil {
		return nil, userErr("No existe la empresa %d o no está dentro de tu alcance.", companyID)
	}
	if rt.Scope.CrossCompany {
		if timeline == nil {
			timeline = []map[string]any{}
		}
		company["timeline"] = timeline
	}
	return company, nil
}

func (rt *Runtime) toolGetCompany(ctx context.Context, in companyIn) (*result, error) {
	c, err := rt.companyDetail(ctx, in.CompanyID)
	if err != nil {
		return nil, err
	}
	return &result{Data: c, Rows: 1, Untrusted: rt.Scope.CrossCompany}, nil
}

// ---------------------------------------------------------------------------
// Profesionales

type listProfessionalsIn struct {
	CompanyID uint   `json:"company_id,omitempty" jsonschema:"Id de la empresa (opcional para plataforma; el resto queda en la suya)"`
	Search    string `json:"search,omitempty" jsonschema:"Texto a buscar en nombre o correo"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Máximo de filas (lo acota la cuota del rol)"`
}

const professionalSelect = `SELECT u.id, u.name, u.email, COALESCE(NULLIF(e.job_title, ''), u.job_title) AS job_title,
	u.country, u.state, u.phone_number AS phone, u.is_manager, u.is_active AS active,
	e.company_id, COALESCE(NULLIF(c.company_name, ''), c.name) AS company, m.name AS manager,
	to_char(e.started_at, 'YYYY-MM-DD') AS started_at
	FROM users u
	JOIN employments e ON e.user_id = u.id
	LEFT JOIN users c ON c.id = e.company_id
	LEFT JOIN users m ON m.id = e.manager_id
	WHERE `

func (rt *Runtime) toolListProfessionals(ctx context.Context, in listProfessionalsIn) (*result, error) {
	w, err := rt.Scope.PeopleFilter(in.CompanyID)
	if err != nil {
		return nil, err
	}
	if in.Search != "" {
		w.add("u.name ILIKE ? OR u.email ILIKE ?", likeArg(in.Search), likeArg(in.Search))
	}
	rows, trunc, err := rt.list(ctx, in.Limit, professionalSelect+w.SQL()+` ORDER BY u.name, e.company_id`, w.args...)
	if err != nil {
		return nil, err
	}
	return listResult(rows, trunc, false), nil
}

// ---------------------------------------------------------------------------
// Jornadas

type listWorkHoursIn struct {
	CompanyID uint   `json:"company_id,omitempty" jsonschema:"Id de la empresa (obligatorio para plataforma)"`
	UserID    uint   `json:"user_id,omitempty" jsonschema:"Id del profesional (acota dentro de tu alcance)"`
	From      string `json:"from,omitempty" jsonschema:"Desde (AAAA-MM-DD); por defecto hace 30 días"`
	To        string `json:"to,omitempty" jsonschema:"Hasta (AAAA-MM-DD); por defecto hoy"`
	Status    string `json:"status,omitempty" jsonschema:"pending, approved o rejected"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Máximo de filas (lo acota la cuota del rol)"`
}

func (rt *Runtime) toolListWorkHours(ctx context.Context, in listWorkHoursIn) (*result, error) {
	w, err := rt.Scope.HoursFilter(in.CompanyID)
	if err != nil {
		return nil, err
	}
	from, to, err := dateRange(in.From, in.To)
	if err != nil {
		return nil, err
	}
	w.add("wh.work_date BETWEEN ? AND ?", from, to)
	if in.UserID != 0 {
		w.add("wh.user_id = ?", in.UserID)
	}
	switch in.Status {
	case "":
	case "pending":
		w.add("NOT wh.approved AND NOT wh.rejected")
	case "approved":
		w.add("wh.approved")
	case "rejected":
		w.add("wh.rejected")
	default:
		return nil, userErr("status admite pending, approved o rejected.")
	}
	rows, trunc, err := rt.list(ctx, in.Limit, `SELECT wh.id, to_char(wh.work_date, 'YYYY-MM-DD') AS date, wh.user_id, u.name AS professional,
		wh.tenant_id AS company_id, wh.work_type, wh.hours_worked::float8 AS hours, wh.absence_hours::float8 AS absence_hours, wh.absence_reason,
		CASE WHEN wh.approved THEN 'aprobada' WHEN wh.rejected THEN 'rechazada' ELSE 'pendiente' END AS status,
		wh.activities, wh.comments, wh.rejection_reason
		FROM work_hours wh JOIN users u ON u.id = wh.user_id
		WHERE `+w.SQL()+` ORDER BY wh.work_date DESC, u.name`, w.args...)
	if err != nil {
		return nil, err
	}
	res := listResult(rows, trunc, true)
	res.Data.(map[string]any)["range"] = map[string]string{"from": from, "to": to}
	return res, nil
}

type hoursSummaryIn struct {
	CompanyID uint   `json:"company_id,omitempty" jsonschema:"Id de la empresa (obligatorio para plataforma)"`
	From      string `json:"from,omitempty" jsonschema:"Desde (AAAA-MM-DD); por defecto hace 30 días"`
	To        string `json:"to,omitempty" jsonschema:"Hasta (AAAA-MM-DD); por defecto hoy"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Máximo de profesionales (lo acota la cuota del rol)"`
}

func (rt *Runtime) toolHoursSummary(ctx context.Context, in hoursSummaryIn) (*result, error) {
	w, err := rt.Scope.HoursFilter(in.CompanyID)
	if err != nil {
		return nil, err
	}
	from, to, err := dateRange(in.From, in.To)
	if err != nil {
		return nil, err
	}
	w.add("wh.work_date BETWEEN ? AND ?", from, to)
	const agg = `count(*) AS days, COALESCE(sum(wh.hours_worked), 0)::float8 AS hours,
		count(*) FILTER (WHERE wh.approved) AS approved_days,
		count(*) FILTER (WHERE NOT wh.approved AND NOT wh.rejected) AS pending_days,
		count(*) FILTER (WHERE wh.rejected) AS rejected_days,
		count(*) FILTER (WHERE wh.work_type = 'absence') AS absence_days`
	rows, trunc, err := rt.list(ctx, in.Limit, `SELECT wh.user_id, u.name AS professional, `+agg+`
		FROM work_hours wh JOIN users u ON u.id = wh.user_id
		WHERE `+w.SQL()+` GROUP BY wh.user_id, u.name ORDER BY hours DESC, u.name`, w.args...)
	if err != nil {
		return nil, err
	}
	var totals map[string]any
	if err := rt.RO.Query(ctx, func(tx *gorm.DB) error {
		totals = map[string]any{}
		return tx.Raw(`SELECT count(DISTINCT wh.user_id) AS professionals, `+agg+`
			FROM work_hours wh WHERE `+w.SQL(), w.args...).Scan(&totals).Error
	}); err != nil {
		return nil, err
	}
	res := listResult(rows, trunc, false)
	res.Data.(map[string]any)["range"] = map[string]string{"from": from, "to": to}
	res.Data.(map[string]any)["totals"] = totals
	return res, nil
}

// ---------------------------------------------------------------------------
// Tableros y tareas

type listBoardsIn struct {
	CompanyID uint `json:"company_id,omitempty" jsonschema:"Id de la empresa (obligatorio para plataforma)"`
	Limit     int  `json:"limit,omitempty" jsonschema:"Máximo de filas (lo acota la cuota del rol)"`
}

func (rt *Runtime) toolListBoards(ctx context.Context, in listBoardsIn) (*result, error) {
	w, err := rt.Scope.BoardsFilter(in.CompanyID)
	if err != nil {
		return nil, err
	}
	rows, trunc, err := rt.list(ctx, in.Limit, `SELECT b.id, b.name, b.description, b.tenant_id AS company_id,
		(SELECT count(*) FROM tasks t WHERE t.board_id = b.id AND t.deleted_at IS NULL) AS tasks,
		(SELECT count(*) FROM tasks t WHERE t.board_id = b.id AND t.deleted_at IS NULL AND NOT t.completed) AS open_tasks,
		(SELECT count(*) FROM board_members bm WHERE bm.board_id = b.id) AS members
		FROM boards b WHERE `+w.SQL()+` ORDER BY b.name`, w.args...)
	if err != nil {
		return nil, err
	}
	return listResult(rows, trunc, true), nil
}

type listTasksIn struct {
	CompanyID  uint   `json:"company_id,omitempty" jsonschema:"Id de la empresa (obligatorio para plataforma)"`
	BoardID    uint   `json:"board_id,omitempty" jsonschema:"Id del tablero"`
	Status     string `json:"status,omitempty" jsonschema:"Estado (por_hacer, en_proceso, finalizado o el de una fase propia)"`
	Priority   string `json:"priority,omitempty" jsonschema:"low, medium, high o urgent"`
	AssigneeID uint   `json:"assignee_id,omitempty" jsonschema:"Id del responsable"`
	Search     string `json:"search,omitempty" jsonschema:"Texto a buscar en título o descripción"`
	Overdue    bool   `json:"overdue,omitempty" jsonschema:"Solo tareas vencidas sin completar"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Máximo de filas (lo acota la cuota del rol)"`
}

const taskSelect = `SELECT t.id, t.title, t.status, t.priority, t.completed,
	to_char(t.start_date, 'YYYY-MM-DD') AS start_date, to_char(t.end_date, 'YYYY-MM-DD') AS due_date,
	t.board_id, b.name AS board, t.tenant_id AS company_id, cr.name AS created_by,
	(SELECT string_agg(au.name, ', ' ORDER BY au.name) FROM task_users tu JOIN users au ON au.id = tu.user_id WHERE tu.task_id = t.id) AS assignees,
	to_char(t.created_at, 'YYYY-MM-DD') AS created_at
	FROM tasks t LEFT JOIN boards b ON b.id = t.board_id LEFT JOIN users cr ON cr.id = t.created_by
	WHERE `

func (rt *Runtime) toolListTasks(ctx context.Context, in listTasksIn) (*result, error) {
	w, err := rt.Scope.TasksFilter(in.CompanyID)
	if err != nil {
		return nil, err
	}
	if in.BoardID != 0 {
		w.add("t.board_id = ?", in.BoardID)
	}
	if in.Status != "" {
		w.add("t.status = ?", in.Status)
	}
	if in.Priority != "" {
		w.add("t.priority = ?", in.Priority)
	}
	if in.AssigneeID != 0 {
		w.add("t.id IN (SELECT task_id FROM task_users WHERE user_id = ?)", in.AssigneeID)
	}
	if in.Search != "" {
		w.add("t.title ILIKE ? OR t.description ILIKE ?", likeArg(in.Search), likeArg(in.Search))
	}
	if in.Overdue {
		w.add("t.end_date < current_date AND NOT t.completed")
	}
	rows, trunc, err := rt.list(ctx, in.Limit, taskSelect+w.SQL()+` ORDER BY t.created_at DESC`, w.args...)
	if err != nil {
		return nil, err
	}
	return listResult(rows, trunc, true), nil
}

// taskByIDFilter: plataforma puede abrir una tarea de cualquier empresa (en
// la web también: authorizeTaskByID no la ata a un tenant); el resto, con el
// mismo filtro del listado.
func (s *Scope) taskByIDFilter() (*where, error) {
	if s.CrossCompany {
		w := &where{}
		w.add("t.deleted_at IS NULL")
		return w, nil
	}
	return s.TasksFilter(0)
}

type taskIn struct {
	TaskID uint `json:"task_id" jsonschema:"Id de la tarea"`
}

func (rt *Runtime) taskDetail(ctx context.Context, taskID uint) (map[string]any, error) {
	w, err := rt.Scope.taskByIDFilter()
	if err != nil {
		return nil, err
	}
	w.add("t.id = ?", taskID)
	var task map[string]any
	var comments []map[string]any
	err = rt.RO.Query(ctx, func(tx *gorm.DB) error {
		task, comments = map[string]any{}, nil
		res := tx.Raw(strings.Replace(taskSelect, "SELECT t.id,", "SELECT t.id, t.description,", 1)+w.SQL(), w.args...).Scan(&task)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			task = nil
			return nil
		}
		return tx.Raw(`SELECT cu.name AS author, c.content, to_char(c.created_at, 'YYYY-MM-DD HH24:MI') AS created_at
			FROM comments c LEFT JOIN users cu ON cu.id = c.user_id
			WHERE c.task_id = ? AND c.deleted_at IS NULL ORDER BY c.created_at DESC LIMIT 15`, taskID).Scan(&comments).Error
	})
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, userErr("No existe la tarea %d o no está dentro de tu alcance.", taskID)
	}
	if comments == nil {
		comments = []map[string]any{}
	}
	task["last_comments"] = comments
	return task, nil
}

func (rt *Runtime) toolGetTask(ctx context.Context, in taskIn) (*result, error) {
	t, err := rt.taskDetail(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	return &result{Data: t, Rows: 1, Untrusted: true}, nil
}

// ---------------------------------------------------------------------------
// Tickets e incidentes

type listTicketsIn struct {
	Status string `json:"status,omitempty" jsonschema:"open o closed"`
	Stage  string `json:"stage,omitempty" jsonschema:"Etapa del ticket"`
	Search string `json:"search,omitempty" jsonschema:"Texto a buscar en título, empresa o correo"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Máximo de filas (lo acota la cuota del rol)"`
}

const ticketSelect = `SELECT tk.id, tk.title, tk.origin, tk.stage, tk.status, tk.company_name, tk.professional_email,
	tk.professional_phone, a.name AS assigned_to, to_char(tk.created_at, 'YYYY-MM-DD HH24:MI') AS created_at, tk.description
	FROM tickets tk LEFT JOIN users a ON a.id = tk.assigned_to
	WHERE `

func (rt *Runtime) toolListTickets(ctx context.Context, in listTicketsIn) (*result, error) {
	if !rt.Scope.canTickets() {
		return nil, userErr("Tu rol no tiene acceso a tickets.")
	}
	w := &where{}
	w.add("tk.deleted_at IS NULL")
	if in.Status != "" {
		w.add("tk.status = ?", in.Status)
	}
	if in.Stage != "" {
		w.add("tk.stage = ?", in.Stage)
	}
	if in.Search != "" {
		l := likeArg(in.Search)
		w.add("tk.title ILIKE ? OR tk.company_name ILIKE ? OR tk.professional_email ILIKE ?", l, l, l)
	}
	rows, trunc, err := rt.list(ctx, in.Limit, ticketSelect+w.SQL()+` ORDER BY tk.created_at DESC`, w.args...)
	if err != nil {
		return nil, err
	}
	return listResult(rows, trunc, true), nil
}

type listIncidentsIn struct {
	Status string `json:"status,omitempty" jsonschema:"open o closed"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Máximo de filas (lo acota la cuota del rol)"`
}

func (rt *Runtime) toolListIncidents(ctx context.Context, in listIncidentsIn) (*result, error) {
	if !rt.Scope.canIncidents() {
		return nil, userErr("Tu rol no tiene acceso a incidentes.")
	}
	w := &where{}
	w.add("i.deleted_at IS NULL")
	if in.Status != "" {
		w.add("i.status = ?", in.Status)
	}
	rows, trunc, err := rt.list(ctx, in.Limit, `SELECT i.id, i.title, i.kind, i.country, i.state, i.status, i.description,
		to_char(i.created_at, 'YYYY-MM-DD HH24:MI') AS created_at, to_char(i.closed_at, 'YYYY-MM-DD HH24:MI') AS closed_at,
		(SELECT count(*) FROM incident_responses r WHERE r.incident_id = i.id) AS people,
		(SELECT count(*) FROM incident_responses r WHERE r.incident_id = i.id AND r.status <> 'pendiente') AS answered
		FROM incidents i WHERE `+w.SQL()+` ORDER BY i.created_at DESC`, w.args...)
	if err != nil {
		return nil, err
	}
	return listResult(rows, trunc, true), nil
}
