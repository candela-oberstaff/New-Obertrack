package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/websocket"
	"github.com/xuri/excelize/v2"
)

// UsageHandler sirve las métricas de usabilidad: quién y qué empresas usan la
// app, cuánto usan cada módulo y quién está conectado ahora mismo.
type UsageHandler struct {
	repo repository.UsageRepository
}

func NewUsageHandler(repo repository.UsageRepository) *UsageHandler {
	return &UsageHandler{repo: repo}
}

// usageDays lee ?days= con un tope. El tope existe porque la tabla de
// actividad crece por persona y día: pedir "todo" sería un escaneo completo
// para responder una pregunta que nadie hace desde un panel.
func usageDays(c *gin.Context) int {
	days, err := strconv.Atoi(c.DefaultQuery("days", "30"))
	if err != nil || days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	return days
}

// usageClientsOnly: por defecto se miden CLIENTES. El equipo de casa
// (superadmin, Customer Success, analistas) entra a diario por oficio y
// contarlo dispararía la adopción sin que ningún cliente abriera nada.
// ?scope=all lo incluye, para cuando la pregunta es "quién está dentro".
func usageClientsOnly(c *gin.Context) bool {
	return c.Query("scope") != "all"
}

// GetUsageSummary devuelve portada, uso por módulo y evolución diaria en una
// sola respuesta: la pestaña las pinta juntas y partirlo en tres llamadas solo
// añadiría estados de carga desincronizados.
func (h *UsageHandler) GetUsageSummary(c *gin.Context) {
	days := usageDays(c)
	scope := repository.UsageScope{
		Days:        days,
		ClientsOnly: usageClientsOnly(c),
		// ?company_id= responde la misma pregunta para una sola empresa: es lo
		// que se pinta en la pestaña Uso de su ficha.
		CompanyID: uint(parseUintQuery(c, "company_id")),
	}

	overview, err := h.repo.Overview(scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron calcular las métricas de uso"})
		return
	}
	modules, err := h.repo.ModuleUsage(scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo calcular el uso por módulo"})
		return
	}
	trend, err := h.repo.DailyTrend(scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo calcular la evolución diaria"})
		return
	}

	if modules == nil {
		modules = []repository.ModuleUsage{}
	}
	if trend == nil {
		trend = []repository.UsageDay{}
	}

	c.JSON(http.StatusOK, gin.H{
		"overview": overview,
		"modules":  modules,
		"trend":    trend,
		"online":   len(onlineSet()),
		"days":     days,
	})
}

func (h *UsageHandler) GetCompanyUsage(c *gin.Context) {
	rows, err := h.repo.CompanyUsage(usageDays(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo calcular el uso por empresa"})
		return
	}
	if rows == nil {
		rows = []repository.CompanyUsage{}
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *UsageHandler) GetPeopleUsage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	companyID, _ := strconv.ParseUint(c.DefaultQuery("company_id", "0"), 10, 32)

	rows, total, err := h.repo.PeopleUsage(repository.PeopleFilter{
		Days:        usageDays(c),
		ClientsOnly: usageClientsOnly(c),
		CompanyID:   uint(companyID),
		Search:      strings.TrimSpace(c.Query("q")),
		Status:      normalizeStatus(c.Query("status")),
		Limit:       limit,
		Offset:      (page - 1) * limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo listar el uso por persona"})
		return
	}
	if rows == nil {
		rows = []repository.PersonUsage{}
	}

	// El "conectado ahora" no sale de la base de datos: lo sabe el hub de
	// WebSockets, y solo para esta instancia del servidor.
	online := onlineSet()
	for i := range rows {
		rows[i].Online = online[rows[i].UserID]
	}

	c.JSON(http.StatusOK, gin.H{
		"data": rows, "total": total, "page": page, "limit": limit,
		"online": len(online),
	})
}

// GetNeverActive lista las cuentas que jamás han aparecido en el contador: el
// hueco de activación. Va aparte del listado de personas porque no depende del
// período —"nunca" no se filtra por 30 días— y porque la pregunta es otra: no
// "quién dejó de usarla" sino "a quién le dimos acceso y nunca lo estrenó".
func (h *UsageHandler) GetNeverActive(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}

	rows, total, err := h.repo.NeverActive(repository.UsageScope{
		ClientsOnly: usageClientsOnly(c),
		CompanyID:   uint(parseUintQuery(c, "company_id")),
	}, limit, (page-1)*limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo listar la activación pendiente"})
		return
	}
	if rows == nil {
		rows = []repository.NeverActiveUser{}
	}

	// certain_total: cuántas de esas son un hecho y no una laguna de datos. El
	// frontend pinta ese número en la pestaña, porque es el único que se puede
	// defender en una reunión.
	var certain int64
	for _, r := range rows {
		if r.Certain {
			certain++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": rows, "total": total, "page": page, "limit": limit,
		"certain_in_page": certain,
	})
}

// ExportUsage baja la tabla que se está mirando como Excel. Customer Success
// trabaja las revisiones de cartera en hoja de cálculo con gente que no tiene
// cuenta en Obertrack, y hasta ahora eso era copiar a mano de la pantalla.
func (h *UsageHandler) ExportUsage(c *gin.Context) {
	days := usageDays(c)
	board := c.DefaultQuery("board", "companies")

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	var headers []string
	var rows [][]interface{}
	sheet := "Uso"

	switch board {
	case "people":
		people, _, err := h.repo.PeopleUsage(repository.PeopleFilter{
			Days:        days,
			ClientsOnly: usageClientsOnly(c),
			CompanyID:   uint(parseUintQuery(c, "company_id")),
			Search:      strings.TrimSpace(c.Query("q")),
			Status:      normalizeStatus(c.Query("status")),
			// El export no se pagina: una hoja recortada a 25 filas sin avisar
			// es peor que no ofrecer el export.
			Limit: 200,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el Excel"})
			return
		}
		online := onlineSet()
		headers = []string{"Persona", "Correo", "Empresa", "Rol", "Conectado ahora", "Días con uso", "Última actividad", "Módulos"}
		for _, p := range people {
			rows = append(rows, []interface{}{
				p.Name, p.Email, p.CompanyName, p.UserType, boolLabel(online[p.UserID]),
				p.ActiveDays, dateLabel(p.LastActive), p.Modules,
			})
		}
	case "activation":
		list, _, err := h.repo.NeverActive(repository.UsageScope{
			ClientsOnly: usageClientsOnly(c),
			CompanyID:   uint(parseUintQuery(c, "company_id")),
		}, 200, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el Excel"})
			return
		}
		headers = []string{"Persona", "Correo", "Empresa", "Rol", "Alta", "Días desde el alta", "Confirmado"}
		for _, u := range list {
			rows = append(rows, []interface{}{
				u.Name, u.Email, u.CompanyName, u.UserType,
				u.CreatedAt.Format("2006-01-02"), u.DaysSince, boolLabel(u.Certain),
			})
		}
	default:
		companies, err := h.repo.CompanyUsage(days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el Excel"})
			return
		}
		headers = []string{"Empresa", "Personas", "Activas", "% de uso", "% periodo anterior", "Variación (puntos)", "% usa el chat", "Última actividad"}
		for _, co := range companies {
			rows = append(rows, []interface{}{
				co.CompanyName, co.TotalUsers, co.ActiveUsers,
				round1(co.Rate), round1(co.PrevRate), round1(co.Delta), round1(co.ChatRate),
				dateLabel(co.LastActive),
			})
		}
	}

	_, _ = f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")
	for i, head := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s1", col), head)
		_ = f.SetColWidth(sheet, col, col, 22)
	}
	for r, row := range rows {
		for i, v := range row {
			col, _ := excelize.ColumnNumberToName(i + 1)
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, r+2), v)
		}
	}

	buf := new(bytes.Buffer)
	if err := f.Write(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el Excel"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=uso_%s_%s.xlsx", board, time.Now().Format("2006-01-02")))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

func parseUintQuery(c *gin.Context, key string) uint64 {
	v, _ := strconv.ParseUint(c.DefaultQuery(key, "0"), 10, 32)
	return v
}

func normalizeStatus(v string) string {
	if v == "active" || v == "inactive" {
		return v
	}
	return ""
}

func boolLabel(v bool) string {
	if v {
		return "Sí"
	}
	return "No"
}

func dateLabel(t *time.Time) string {
	if t == nil {
		return "Nunca"
	}
	return t.Format("2006-01-02 15:04")
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// GetOnlineUsers lista quién tiene la app abierta ahora mismo. Se sirve aparte
// del resto porque es el único dato en vivo de la pantalla: el frontend lo
// repregunta solo, sin recalcular las métricas del período.
func (h *UsageHandler) GetOnlineUsers(c *gin.Context) {
	ids := websocket.GlobalNotifHub.OnlineUserIDs()
	c.JSON(http.StatusOK, gin.H{"user_ids": ids, "count": len(ids)})
}

func onlineSet() map[uint]bool {
	ids := websocket.GlobalNotifHub.OnlineUserIDs()
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
