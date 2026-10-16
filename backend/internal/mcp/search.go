package mcp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// search y fetch con el formato que exige ChatGPT para conectores:
//   search → {"results":[{"id","title","url"}]}
//   fetch  → {"id","title","text","url","metadata"}
// Los ids llevan el tipo delante (task-12, company-3, professional-8,
// ticket-5) para que fetch sepa dónde buscar.

type searchIn struct {
	Query string `json:"query" jsonschema:"Texto a buscar"`
}

type searchHit struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

const perKindLimit = 8

func (rt *Runtime) toolSearch(ctx context.Context, in searchIn) (*result, error) {
	q := strings.TrimSpace(in.Query)
	if len([]rune(q)) < 2 {
		return nil, userErr("Escribe al menos 2 caracteres para buscar.")
	}
	l := likeArg(q)
	s := rt.Scope
	hits := []searchHit{}
	type rowT struct {
		ID    uint
		Title string
		Extra string
	}
	run := func(sql string, args ...any) ([]rowT, error) {
		var rows []rowT
		err := rt.RO.Query(ctx, func(tx *gorm.DB) error {
			rows = nil
			return tx.Raw(sql+fmt.Sprintf(" LIMIT %d", perKindLimit), args...).Scan(&rows).Error
		})
		return rows, err
	}

	if s.canTasks() {
		if w, err := s.taskByIDFilter(); err == nil {
			w.add("t.title ILIKE ? OR t.description ILIKE ?", l, l)
			rows, err := run(`SELECT t.id, t.title, (t.tenant_id::text || ':' || t.board_id::text) AS extra FROM tasks t WHERE `+w.SQL()+` ORDER BY t.created_at DESC`, w.args...)
			if err != nil {
				return nil, err
			}
			for _, r := range rows {
				company, board, _ := strings.Cut(r.Extra, ":")
				hits = append(hits, searchHit{ID: fmt.Sprintf("task-%d", r.ID), Title: "Tarea: " + r.Title,
					URL: fmt.Sprintf("%s/tasks?company=%s&board=%s&task=%d", rt.FrontendURL, company, board, r.ID)})
			}
		}
	}
	if s.canCompanies() {
		if w, err := s.CompaniesFilter(); err == nil {
			w.add("u.company_name ILIKE ? OR u.name ILIKE ?", l, l)
			rows, err := run(`SELECT u.id, COALESCE(NULLIF(u.company_name, ''), u.name) AS title FROM users u WHERE `+w.SQL()+` ORDER BY 2`, w.args...)
			if err != nil {
				return nil, err
			}
			for _, r := range rows {
				hits = append(hits, searchHit{ID: fmt.Sprintf("company-%d", r.ID), Title: "Empresa: " + r.Title, URL: rt.companyURL(r.ID)})
			}
		}
	}
	if s.canPeople() {
		if w, err := s.PeopleFilter(0); err == nil {
			w.add("u.name ILIKE ? OR u.email ILIKE ?", l, l)
			rows, err := run(`SELECT DISTINCT u.id, u.name AS title FROM users u JOIN employments e ON e.user_id = u.id WHERE `+w.SQL()+` ORDER BY 2`, w.args...)
			if err != nil {
				return nil, err
			}
			for _, r := range rows {
				hits = append(hits, searchHit{ID: fmt.Sprintf("professional-%d", r.ID), Title: "Profesional: " + r.Title, URL: rt.professionalURL(r.ID)})
			}
		}
	}
	if s.canTickets() {
		rows, err := run(`SELECT tk.id, tk.title FROM tickets tk WHERE tk.deleted_at IS NULL
			AND (tk.title ILIKE ? OR tk.company_name ILIKE ? OR tk.professional_email ILIKE ?) ORDER BY tk.created_at DESC`, l, l, l)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			hits = append(hits, searchHit{ID: fmt.Sprintf("ticket-%d", r.ID), Title: "Ticket: " + r.Title, URL: fmt.Sprintf("%s/tickets/%d", rt.FrontendURL, r.ID)})
		}
	}
	if max := rt.maxRows(0); len(hits) > max {
		hits = hits[:max]
	}
	return &result{Raw: map[string]any{"results": hits}, Rows: len(hits)}, nil
}

func (rt *Runtime) companyURL(id uint) string {
	if rt.Scope.CrossCompany {
		return fmt.Sprintf("%s/admin/tenants/%d", rt.FrontendURL, id)
	}
	return rt.FrontendURL + "/empresa"
}

func (rt *Runtime) professionalURL(id uint) string {
	switch {
	case rt.Scope.CrossCompany:
		return fmt.Sprintf("%s/admin/users/%d", rt.FrontendURL, id)
	case rt.Scope.RoleKey == RoleEmployer:
		return fmt.Sprintf("%s/empresa/employees/%d", rt.FrontendURL, id)
	}
	return rt.FrontendURL + "/work-hours"
}

type fetchIn struct {
	ID string `json:"id" jsonschema:"Id devuelto por search (por ejemplo task-12)"`
}

func (rt *Runtime) toolFetch(ctx context.Context, in fetchIn) (*result, error) {
	kind, idStr, ok := strings.Cut(strings.TrimSpace(in.ID), "-")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if !ok || err != nil || id == 0 {
		return nil, userErr("Id no reconocido: %q. Usa uno devuelto por search (task-12, company-3, professional-8, ticket-5).", in.ID)
	}
	s := rt.Scope
	var (
		record map[string]any
		title  string
		url    string
	)
	switch kind {
	case "task":
		if !s.canTasks() {
			return nil, userErr("Tu rol no tiene acceso a tareas.")
		}
		if record, err = rt.taskDetail(ctx, uint(id)); err != nil {
			return nil, err
		}
		title = "Tarea: " + fmt.Sprint(record["title"])
		url = fmt.Sprintf("%s/tasks?company=%v&board=%v&task=%d", rt.FrontendURL, record["company_id"], record["board_id"], id)
	case "company":
		if !s.canCompanies() {
			return nil, userErr("Tu rol no tiene acceso a empresas.")
		}
		if record, err = rt.companyDetail(ctx, uint(id)); err != nil {
			return nil, err
		}
		title = "Empresa: " + fmt.Sprint(record["name"])
		url = rt.companyURL(uint(id))
	case "professional":
		w, err := s.PeopleFilter(0)
		if err != nil {
			return nil, err
		}
		w.add("u.id = ?", id)
		rows, _, err := rt.list(ctx, 0, professionalSelect+w.SQL()+` ORDER BY e.company_id`, w.args...)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, userErr("No existe el profesional %d o no está dentro de tu alcance.", id)
		}
		// Copia: si tiene varios empleos, el registro principal no puede
		// contenerse a sí mismo dentro de "employments".
		record = make(map[string]any, len(rows[0])+1)
		for k, v := range rows[0] {
			record[k] = v
		}
		if len(rows) > 1 {
			record["employments"] = rows
		}
		title = "Profesional: " + fmt.Sprint(record["name"])
		url = rt.professionalURL(uint(id))
	case "ticket":
		if !s.canTickets() {
			return nil, userErr("Tu rol no tiene acceso a tickets.")
		}
		rows, _, err := rt.list(ctx, 1, ticketSelect+`tk.deleted_at IS NULL AND tk.id = ?`, id)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, userErr("No existe el ticket %d.", id)
		}
		record = rows[0]
		title = "Ticket: " + fmt.Sprint(record["title"])
		url = fmt.Sprintf("%s/tickets/%d", rt.FrontendURL, id)
	default:
		return nil, userErr("Tipo de id desconocido: %q.", kind)
	}
	clean, _ := RedactData(normalize(record), rt.Settings.MinimalData).(map[string]any)
	text := recordText(clean) + "\n\n" + UntrustedTextNote
	return &result{
		Raw: map[string]any{
			"id":       in.ID,
			"title":    title,
			"text":     text,
			"url":      url,
			"metadata": map[string]any{"type": kind, "source": "Obertrack", "access": "solo lectura"},
		},
		Rows:      1,
		Untrusted: true,
	}, nil
}

// recordText convierte un registro en líneas "clave: valor" legibles.
func recordText(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := m[k]
		if v == nil || v == "" {
			continue
		}
		switch t := v.(type) {
		case []any:
			fmt.Fprintf(&b, "%s:\n", k)
			for _, item := range t {
				if mm, ok := item.(map[string]any); ok {
					fmt.Fprintf(&b, "  - %s\n", strings.ReplaceAll(strings.TrimSpace(recordText(mm)), "\n", "; "))
				} else {
					fmt.Fprintf(&b, "  - %v\n", item)
				}
			}
		default:
			fmt.Fprintf(&b, "%s: %v\n", k, v)
		}
	}
	return strings.TrimSpace(b.String())
}
