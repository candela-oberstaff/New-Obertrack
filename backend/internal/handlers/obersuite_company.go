package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// ObersuiteCompanyHandler sirve la ficha de una empresa, bloque a bloque, para
// la pantalla de detalle de Obersuite.
//
// Por qué existe habiéndose descartado antes: se aparcó cuando nadie la
// necesitaba. Obersuite construyó una ficha con las mismas ocho pestañas que la
// nuestra y las ocho salían vacías, así que ya hay quien la necesita.
//
// Y por qué no rompe lo que se acaba de arreglar: los campos de operación se
// retiraron del padrón porque rotaban el ETag de una lista que se pide entera y
// se cachea. Esto es por empresa y bajo demanda —solo se llama al abrir una
// ficha, y cada bloque solo al abrir su pestaña—, así que no toca esa caché.
//
// Un bloque por ruta, y no una respuesta gorda con todo dentro, por la misma
// razón: la pestaña que nadie abre no se pide, y entonces no cuesta nada. Fue lo
// que falló al revés con los diez campos que hubo que retirar del padrón.
type ObersuiteCompanyHandler struct {
	admin      service.AdminService
	employment service.EmploymentService
	usage      repository.UsageRepository
}

func NewObersuiteCompanyHandler(
	admin service.AdminService,
	employment service.EmploymentService,
	usage repository.UsageRepository,
) *ObersuiteCompanyHandler {
	return &ObersuiteCompanyHandler{admin: admin, employment: employment, usage: usage}
}

// company resuelve el :id de la ruta y comprueba que exista y sea una empresa.
//
// Se valida SIEMPRE, en todos los bloques, aunque el id venga de un padrón que
// acabamos de servirle a quien llama: entre que Obersuite cachea el padrón y
// abre una ficha pueden pasar horas, y la empresa puede haberse dado de baja.
// Un 404 limpio dice qué pasó; consultar por un id inexistente devolvería
// listas vacías, que se leen como "esta empresa no tiene nada".
func (h *ObersuiteCompanyHandler) company(c *gin.Context) (*repository.TenantSummary, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el id de empresa no es válido"})
		return nil, false
	}
	tenant, err := h.admin.GetTenant(uint(id))
	if err != nil || tenant == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "la empresa no existe o ya no está dada de alta"})
		return nil, false
	}
	return tenant, true
}

// Detail devuelve la ficha completa de una empresa: lo mismo que el padrón más
// lo que allí se retiró por caro o por volátil.
//
// Aquí sí va: el padrón se pide entero y se cachea, esto se pide de una empresa
// cuando alguien abre su ficha.
func (h *ObersuiteCompanyHandler) Detail(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	status := "suspended"
	if tenant.IsActive {
		status = "active"
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                tenant.ID,
		"name":              tenant.CompanyName,
		"status":            status,
		"responsible_name":  tenant.OwnerName,
		"responsible_email": tenant.OwnerEmail,
		"phone_number":      tenant.PhoneNumber,
		"industry":          tenant.Industry,
		"country":           tenant.Country,
		"state":             tenant.State,
		"city":              tenant.City,
		"address":           tenant.Address,

		"professionals_count": tenant.UserCount,
		"boards_count":        tenant.BoardCount,
		"tasks_count":         tenant.TaskCount,
		"open_tickets":        tenant.OpenTickets,

		// La operación del mes en curso. Es lo que se quitó del padrón por
		// rotar el ETag; en la ficha no hay ETag que romper.
		"hours_this_month": tenant.HoursThisMonth,
		"pending_hours":    tenant.PendingHours,
		"pending_count":    tenant.PendingCount,
		"rejected_count":   tenant.RejectedCount,

		"created_at":       tenant.CreatedAt,
		"client_since":     tenant.ClientSince,
		"last_contact_at":  tenant.LastContactAt,
		"last_activity_at": tenant.LastActivityAt,
	})
}

// Professionals alimenta DOS pestañas de Obersuite: "Profesionales" y
// "Horarios". No son dos consultas: la jornada de cada persona
// (schedule_type / schedule_days) viaja en su propia fila, así que Horarios es
// una proyección de esta lista, no un bloque aparte.
func (h *ObersuiteCompanyHandler) Professionals(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	people, err := h.admin.GetTenantEmployees(tenant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar la plantilla"})
		return
	}
	if people == nil {
		people = []repository.EmployeeSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"company_id": tenant.ID, "professionals": people})
}

// OrgChart devuelve el organigrama de la empresa.
//
// El servicio recorta el árbol según quién mire —un manager no ve lo mismo que
// un superadmin—. Aquí no hay persona que mire: es un servicio autenticado con
// token que pinta la ficha de la empresa entera, así que se pide sin recorte.
func (h *ObersuiteCompanyHandler) OrgChart(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	nodes, err := h.employment.OrgChart(tenant.ID, 0, "", true, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el organigrama"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"company_id": tenant.ID, "nodes": nodes})
}

// obersuiteTimelinePageSize es el tamaño de página del expediente. Una empresa
// con año y pico de vida acumula cientos de entradas; devolverlas todas haría
// que la pestaña tardara en abrir por movimientos que nadie va a leer.
const obersuiteTimelinePageSize = 50

// Timeline es el EXPEDIENTE: la cronología de la empresa —notas de Customer
// Success, contactos, altas y bajas, jornadas, testimonios—.
//
// Ojo con el nombre de la pestaña: en nuestra pantalla "Actividad" NO es esto,
// es el panel de inactividad y ausencias (ver Attention). Obersuite clonó las
// etiquetas, así que es fácil cablear una en la otra y que parezca que funciona.
//
// Las categorías van EN LA RESPUESTA y no se dejan a que el cliente las escriba
// a mano. Cambian: esta misma semana "staff" pasó a vivir dentro de
// "lifecycle", y una lista repetida del otro lado se habría quedado
// desincronizada sin que fallara nada.
func (h *ObersuiteCompanyHandler) Timeline(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}

	category := c.Query("category")
	personID, _ := strconv.ParseUint(c.Query("person_id"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * obersuiteTimelinePageSize

	entries, total, err := h.admin.GetTenantActivities(
		tenant.ID, category, uint(personID), offset, obersuiteTimelinePageSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el expediente"})
		return
	}
	if entries == nil {
		entries = []repository.TenantActivity{}
	}

	// counts trae MÁS claves que categories, y es a propósito: "staff" y
	// "management" ya no se ofrecen como filtro —el primero se fusionó dentro de
	// "lifecycle" y el segundo se retiró—, pero sus movimientos siguen ahí y se
	// pueden consultar por ?category=. Se mandan en crudo porque son el número
	// real.
	//
	// Los chips se pintan desde `categories`, NUNCA recorriendo las claves de
	// `counts`: eso produciría dos filtros que nuestra pantalla no tiene.
	counts, err := h.admin.GetTenantActivityCounts(tenant.ID, uint(personID))
	if err != nil {
		// El contador es para pintar los chips; sin él la cronología sigue
		// siendo útil, así que no se tumba la respuesta entera por eso.
		counts = map[string]int64{}
	}

	c.JSON(http.StatusOK, gin.H{
		"company_id": tenant.ID,
		"entries":    entries,
		"categories": obersuiteTimelineCategories(),
		"counts":     counts,
		"page":       page,
		"page_size":  obersuiteTimelinePageSize,
		"total":      total,
	})
}

// obersuiteTimelineCategories son los filtros del expediente CON su etiqueta.
//
// Viajan en la respuesta a propósito. Si Obersuite las escribe en su código,
// cada vez que aquí se renombre o se fusione una —como acaba de pasar— su
// pantalla queda mostrando algo que ya no existe, y no falla nada: simplemente
// enseña un filtro vacío. Un espejo se desincroniza en silencio, y esta es la
// forma barata de que no lo haga.
// Tiene que decir lo MISMO que ACTIVITY_CATEGORIES en
// frontend/src/hooks/useTenantActivity.ts, en el mismo orden: son los filtros
// que ofrece nuestra pantalla, y la gracia es que Obersuite enseñe los suyos.
// Al tocar una hay que tocar la otra (lo protege una prueba).
func obersuiteTimelineCategories() []gin.H {
	return []gin.H{
		{"value": "", "label": "Todo"},
		{"value": repository.TenantActivityLifecycle, "label": "Escala de tiempo"},
		{"value": repository.TenantActivityWork, "label": "Jornadas"},
		// Comunicaciones antes que notas: es lo primero que se mira al abrir
		// una ficha ("¿ya hablamos con ellos?"), y las notas son su detalle.
		{"value": repository.TenantActivityContact, "label": "Comunicaciones"},
		{"value": repository.TenantActivityNote, "label": "Notas"},
		{"value": repository.TenantActivityTestimonial, "label": "Testimonios"},
	}
}

// Attention es la pestaña "Actividad": a quién hay que mirar en esta empresa.
//
// No es la cronología (eso es Timeline). Son dos cosas distintas juntas porque
// se miran juntas: quién lleva días sin aparecer, y quién ha faltado este mes.
func (h *ObersuiteCompanyHandler) Attention(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days <= 0 {
		days = 7
	}
	inactive, err := h.admin.GetInactiveUsers(tenant.ID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar la inactividad"})
		return
	}
	if inactive == nil {
		inactive = []repository.InactiveUser{}
	}

	// Sin mes/año se toma el corriente, que es lo que se mira al abrir.
	month, _ := strconv.Atoi(c.Query("month"))
	year, _ := strconv.Atoi(c.Query("year"))
	if month == 0 || year == 0 {
		now := time.Now()
		month, year = int(now.Month()), now.Year()
	}
	absence, err := h.admin.GetAbsenceReport(tenant.ID, month, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el informe de ausencias"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"company_id":     tenant.ID,
		"inactive_days":  days,
		"inactive":       inactive,
		"absence_month":  month,
		"absence_year":   year,
		"absence_report": absence,
	})
}

// Tickets son los de soporte de esta empresa, del origen que sea (internos,
// WhatsApp o Zoho).
func (h *ObersuiteCompanyHandler) Tickets(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	tickets, err := h.admin.GetTenantTickets(tenant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudieron cargar los tickets"})
		return
	}
	if tickets == nil {
		tickets = []repository.TenantTicket{}
	}
	c.JSON(http.StatusOK, gin.H{"company_id": tenant.ID, "tickets": tickets})
}

// Archived son las bajas: empleos terminados y cuentas desactivadas.
//
// Es lo contrario del padrón, donde una empresa borrada simplemente desaparece.
// Aquí las personas que se fueron SÍ se conservan, porque el historial de quién
// pasó por una empresa es justo lo que se consulta al recontratar.
func (h *ObersuiteCompanyHandler) Archived(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	entries, err := h.admin.GetArchived(tenant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudieron cargar los archivados"})
		return
	}
	if entries == nil {
		entries = []repository.ArchivedEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"company_id": tenant.ID, "archived": entries})
}

// obersuiteUsageDays es la ventana por defecto del bloque de uso.
const obersuiteUsageDays = 30

// Usage es cuánto se usa realmente la aplicación en esta empresa: el resumen,
// el desglose por módulo y la lista de personas.
//
// El dato sale de nuestro contador propio (user_activity_daily) y no de los
// registros de auditoría, que solo anotan escrituras: por auditoría, alguien que
// entra todos los días a mirar sus tareas y no toca nada figura como inactivo.
func (h *ObersuiteCompanyHandler) Usage(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", strconv.Itoa(obersuiteUsageDays)))
	if days <= 0 {
		days = obersuiteUsageDays
	}
	scope := repository.UsageScope{Days: days, CompanyID: tenant.ID}

	overview, err := h.usage.Overview(scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el uso"})
		return
	}
	modules, err := h.usage.ModuleUsage(scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el uso por módulo"})
		return
	}
	if modules == nil {
		modules = []repository.ModuleUsage{}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	people, total, err := h.usage.PeopleUsage(repository.PeopleFilter{
		Days:      days,
		CompanyID: tenant.ID,
		Search:    c.Query("search"),
		Status:    c.Query("status"),
		Limit:     obersuiteTimelinePageSize,
		Offset:    (page - 1) * obersuiteTimelinePageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el uso por persona"})
		return
	}
	if people == nil {
		people = []repository.PersonUsage{}
	}

	c.JSON(http.StatusOK, gin.H{
		"company_id": tenant.ID,
		"days":       days,
		"overview":   overview,
		"modules":    modules,
		"people":     people,
		"page":       page,
		"page_size":  obersuiteTimelinePageSize,
		"total":      total,
		// Online se resuelve desde el hub de WebSockets de cada proceso, que
		// solo sabe quién tiene la pestaña abierta CONTRA ÉL. Se dice que no
		// viene en vez de mandar false para todos, que se leería como "no hay
		// nadie conectado" en lugar de "esto no se contesta por aquí".
		"online_available": false,
	})
}
