package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// ObersuiteProfessionalHandler sirve la ficha de UNA persona dentro de UNA
// empresa, bloque a bloque, para Obersuite. Es el espejo de nuestra pantalla de
// empleado (EmployeeDetail), con sus seis pestañas.
//
// Cuelga de /companies/:id/professionals/:uid y no de /professionals/:uid a
// propósito: el expediente, las jornadas y las tareas son DEL EMPLEO, no de la
// persona. Alguien que pasó por dos empresas tiene dos expedientes, y desde la
// ficha de la empresa A solo interesa el suyo. Sin la empresa en la ruta habría
// que adivinar cuál, y se adivinaría mal justo con los recontratados.
type ObersuiteProfessionalHandler struct {
	company    *ObersuiteCompanyHandler
	users      repository.UserRepository
	admin      service.AdminService
	employment service.EmploymentService
	induction  service.InductionService
	incidents  service.IncidentService
}

func NewObersuiteProfessionalHandler(
	company *ObersuiteCompanyHandler,
	users repository.UserRepository,
	admin service.AdminService,
	employment service.EmploymentService,
	induction service.InductionService,
	incidents service.IncidentService,
) *ObersuiteProfessionalHandler {
	return &ObersuiteProfessionalHandler{
		company: company, users: users, admin: admin,
		employment: employment, induction: induction, incidents: incidents,
	}
}

// obersuitePersonPageSize es el tamaño de página de las listas largas de la
// ficha (jornadas, tareas, actividad). Más pequeño que el del expediente de
// empresa porque una jornada se lee entera y sesenta caben en una pantalla; lo
// que Obersuite necesita es no pedir dos años de jornadas para pintar un mes.
const obersuitePersonPageSize = 30

// person resuelve la empresa y la persona de la ruta y comprueba que la persona
// esté vinculada a ESA empresa. Devuelve además su empleo activo allí, que es
// lo que necesitan el expediente y el seguimiento; puede ser nil cuando la
// persona está vinculada solo como empresa principal, sin empleo escrito.
//
// Una persona real que NO trabaja en esa empresa da 404 igual que una que no
// existe. Es deliberado: contestar con datos de otra empresa sería justo el
// enlace-a-la-ficha-equivocada que Obersuite temía, y distinguir los dos casos
// en el mensaje le diría a quien pregunta que la persona existe en otro sitio.
func (h *ObersuiteProfessionalHandler) person(c *gin.Context) (*repository.TenantSummary, *repository.ObersuiteProfessional, *models.Employment, bool) {
	tenant, ok := h.company.company(c)
	if !ok {
		return nil, nil, nil, false
	}
	uid, err := strconv.ParseUint(c.Param("uid"), 10, 32)
	if err != nil || uid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el id de la persona no es válido"})
		return nil, nil, nil, false
	}
	row, err := h.users.GetObersuiteProfessional(tenant.ID, uint(uid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar la persona"})
		return nil, nil, nil, false
	}
	if row == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "la persona no existe o no trabaja en esta empresa"})
		return nil, nil, nil, false
	}

	var emp *models.Employment
	if views, err := h.employment.ListForUser(row.ID); err == nil {
		for i := range views {
			v := &views[i]
			if v.CompanyID == tenant.ID && v.Status == models.EmploymentActive {
				emp = &v.Employment
				break
			}
		}
	}
	return tenant, row, emp, true
}

// Detail es la cabecera: la misma fila que sale en /professionals, para que la
// ficha diga exactamente lo que decía la lista, más los tres números de arriba
// (los KPIs de nuestra pantalla).
func (h *ObersuiteProfessionalHandler) Detail(c *gin.Context) {
	tenant, row, emp, ok := h.person(c)
	if !ok {
		return
	}
	out := gin.H{
		"company_id":   tenant.ID,
		"company_name": tenant.CompanyName,
		"professional": row,
		// employment_id es lo que enlaza el expediente. Nulo cuando la persona
		// está vinculada sin empleo escrito: entonces record y onboarding
		// contestan con record_available=false en vez de inventar un resumen.
		"employment_id": nil,
	}
	if emp != nil {
		out["employment_id"] = emp.ID
	}
	c.JSON(http.StatusOK, out)
}

// Record es el EXPEDIENTE del empleo: resumen, ausencias, contactos, notas y
// documentos. Es lo que abre nuestro ExpedienteModal.
//
// Se pide con la audiencia de plataforma —lo ve todo, incluidas las notas
// privadas y las evaluaciones—. Está decidido así (Obersuite es interno y su
// pantalla de Empresas la ven seis superadmins), y se deja aquí escrito porque
// es la clase de decisión que no se puede reconstruir leyendo el código.
func (h *ObersuiteProfessionalHandler) Record(c *gin.Context) {
	tenant, row, emp, ok := h.person(c)
	if !ok {
		return
	}
	if emp == nil {
		c.JSON(http.StatusOK, h.noRecord(tenant, row))
		return
	}
	view, err := h.employment.GetExpediente(emp.ID, service.AudiencePlatform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el expediente"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"company_id":       tenant.ID,
		"professional_id":  row.ID,
		"employment_id":    emp.ID,
		"record_available": true,
		"summary":          view.Summary,
		"absences":         orEmpty(view.Absences),
		"contacts":         orEmpty(view.Contactos),
		"notes":            orEmpty(view.Notes),
		"documents":        orEmpty(view.Documents),
		"labels":           expedienteLabels(),
	})
}

// noRecord es la respuesta del expediente y del seguimiento cuando la persona
// no tiene un empleo escrito en la empresa. La bandera dice que NO se puede
// contestar por aquí; mandar un resumen a cero afirmaría que no ha trabajado.
func (h *ObersuiteProfessionalHandler) noRecord(tenant *repository.TenantSummary, row *repository.ObersuiteProfessional) gin.H {
	return gin.H{
		"company_id":       tenant.ID,
		"professional_id":  row.ID,
		"employment_id":    nil,
		"record_available": false,
		"reason":           "la persona está vinculada a la empresa sin un empleo registrado; el expediente cuelga del empleo",
	}
}

// Workdays son las JORNADAS de la persona en esta empresa, paginadas.
func (h *ObersuiteProfessionalHandler) Workdays(c *gin.Context) {
	tenant, row, _, ok := h.person(c)
	if !ok {
		return
	}
	page, offset := pageAndOffset(c, obersuitePersonPageSize)
	entries, total, err := h.users.GetObersuiteWorkdays(row.ID, tenant.ID, offset, obersuitePersonPageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudieron cargar las jornadas"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"company_id":      tenant.ID,
		"professional_id": row.ID,
		"entries":         orEmpty(entries),
		"page":            page,
		"page_size":       obersuitePersonPageSize,
		"total":           total,
		"labels": gin.H{
			"state":     workdayStateLabels(),
			"work_type": workTypeLabels(),
		},
	})
}

// Tasks son las TAREAS asignadas a la persona en esta empresa, paginadas.
func (h *ObersuiteProfessionalHandler) Tasks(c *gin.Context) {
	tenant, row, _, ok := h.person(c)
	if !ok {
		return
	}
	page, offset := pageAndOffset(c, obersuitePersonPageSize)
	entries, total, err := h.users.GetObersuiteTasks(row.ID, tenant.ID, offset, obersuitePersonPageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudieron cargar las tareas"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"company_id":      tenant.ID,
		"professional_id": row.ID,
		"entries":         orEmpty(entries),
		"page":            page,
		"page_size":       obersuitePersonPageSize,
		"total":           total,
		"labels":          gin.H{"status": taskStatusLabels()},
	})
}

// Onboarding es el SEGUIMIENTO: el estado de la inducción con sus intentos, y
// las gestiones (inactividad, ausencias) anotadas en el expediente.
//
// La inducción es de la PERSONA, no del empleo: se aprueba una vez. Las
// gestiones sí son del empleo. Por eso una persona sin empleo escrito recibe
// la inducción igual y las gestiones vacías con la bandera.
func (h *ObersuiteProfessionalHandler) Onboarding(c *gin.Context) {
	tenant, row, emp, ok := h.person(c)
	if !ok {
		return
	}
	out := gin.H{
		"company_id":      tenant.ID,
		"professional_id": row.ID,
		"labels": gin.H{
			"induction_status": inductionStatusLabels(),
			"followup_kind":    followUpKindLabels(),
			"followup_status":  followUpStatusLabels(),
		},
	}

	if st, err := h.induction.Status(row.ID); err == nil && st != nil {
		out["induction"] = gin.H{
			"status":        st.Status,
			"attempts":      st.Attempts,
			"max_attempts":  st.MaxAttempts,
			"best_score":    st.BestScore,
			"passing_score": st.PassingScore,
		}
		out["attempts"] = inductionAttempts(st.AttemptLog)
	} else {
		// Sin invitación de inducción no hay estado que dar. Se dice, no se
		// rellena con ceros: cero intentos y "sin inducción" no son lo mismo.
		out["induction"] = nil
		out["attempts"] = []gin.H{}
		out["induction_available"] = false
	}

	if emp == nil {
		out["employment_id"] = nil
		out["record_available"] = false
		out["follow_ups"] = []service.GestionEntry{}
		c.JSON(http.StatusOK, out)
		return
	}
	view, err := h.employment.GetExpediente(emp.ID, service.AudiencePlatform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el seguimiento"})
		return
	}
	out["employment_id"] = emp.ID
	out["record_available"] = true
	out["follow_ups"] = orEmpty(view.Gestiones)
	c.JSON(http.StatusOK, out)
}

// Support es SOPORTE: los tickets de la persona y sus incidencias.
//
// Los tickets son los mismos que en /companies/:id/tickets, filtrados por
// persona. Las incidencias son las emergencias por zona (un apagón, una
// inundación) en las que la persona figuraba y cómo respondió.
func (h *ObersuiteProfessionalHandler) Support(c *gin.Context) {
	tenant, row, _, ok := h.person(c)
	if !ok {
		return
	}
	tickets, err := h.admin.GetEmployeeTickets(row.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudieron cargar los tickets"})
		return
	}
	incidents, err := h.incidents.ListForUser(row.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudieron cargar las incidencias"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"company_id":      tenant.ID,
		"professional_id": row.ID,
		"tickets":         orEmpty(tickets),
		"incidents":       orEmpty(incidents),
	})
}

// Activity es la ACTIVIDAD de la persona en el expediente de la empresa: los
// mismos movimientos de /companies/:id/timeline, filtrados a ella. No es otra
// fuente; es la misma cronología vista desde una persona.
func (h *ObersuiteProfessionalHandler) Activity(c *gin.Context) {
	tenant, row, _, ok := h.person(c)
	if !ok {
		return
	}
	page, offset := pageAndOffset(c, obersuitePersonPageSize)
	entries, total, err := h.admin.GetTenantActivities(tenant.ID, "", row.ID, offset, obersuitePersonPageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar la actividad"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"company_id":      tenant.ID,
		"professional_id": row.ID,
		"entries":         orEmpty(entries),
		"page":            page,
		"page_size":       obersuitePersonPageSize,
		"total":           total,
	})
}

// pageAndOffset lee ?page= (desde 1) y devuelve la página y el desplazamiento.
func pageAndOffset(c *gin.Context, size int) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	return page, (page - 1) * size
}

// orEmpty convierte un slice nulo en uno vacío. Un `null` donde se espera una
// lista obliga al otro lado a defenderse en cada campo; `[]` no.
func orEmpty[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

func inductionAttempts(log []models.InductionAttempt) []gin.H {
	out := make([]gin.H, 0, len(log))
	for i, a := range log {
		out = append(out, gin.H{
			"number":     i + 1,
			"score":      a.Score,
			"passed":     a.Passed,
			"created_at": a.CreatedAt,
		})
	}
	return out
}

// Los diccionarios de etiquetas. Viajan en cada respuesta con value y label,
// resueltos aquí, por la misma razón que las categorías del expediente de
// empresa: si Obersuite los escribe, el día que aquí se renombre algo su
// pantalla sigue funcionando y enseñando un texto muerto.

func workdayStateLabels() []gin.H {
	return []gin.H{
		{"value": "approved", "label": "Aprobada"},
		{"value": "rejected", "label": "Rechazada"},
		{"value": "pending", "label": "Pendiente"},
	}
}

// Mismo diccionario que WORK_TYPE en frontend/src/utils/workHours.ts.
func workTypeLabels() []gin.H {
	return []gin.H{
		{"value": "complete", "label": "Jornada"},
		{"value": "absence", "label": "Ausencia"},
		{"value": "recover", "label": "Recuperación"},
	}
}

// Los tres estados por defecto de un tablero. Un tablero puede definir columnas
// propias, y entonces status trae un valor que no está aquí: se pinta tal cual,
// que es lo que hace nuestra propia pantalla.
func taskStatusLabels() []gin.H {
	return []gin.H{
		{"value": "por_hacer", "label": "Por hacer"},
		{"value": "en_proceso", "label": "En proceso"},
		{"value": "finalizado", "label": "Finalizado"},
	}
}

func inductionStatusLabels() []gin.H {
	return []gin.H{
		{"value": "not_required", "label": "Acceso directo"},
		{"value": "pending", "label": "Inducción pendiente"},
		{"value": "passed", "label": "Inducción aprobada"},
		{"value": "blocked", "label": "Bloqueado por intentos"},
	}
}

func followUpKindLabels() []gin.H {
	return []gin.H{
		{"value": "inactivity", "label": "Inactividad"},
		{"value": "absence", "label": "Ausencia"},
	}
}

func followUpStatusLabels() []gin.H {
	return []gin.H{
		{"value": "contacted", "label": "Contactado"},
		{"value": "justified", "label": "Justificado"},
		{"value": "escalated", "label": "Escalado"},
	}
}

func expedienteLabels() gin.H {
	return gin.H{
		"note_kind": []gin.H{
			{"value": models.NoteKindNote, "label": "Nota"},
			{"value": models.NoteKindEvaluation, "label": "Evaluación"},
			{"value": models.NoteKindTestimonial, "label": "Testimonio"},
		},
		"visibility": []gin.H{
			{"value": models.ExpedientePrivate, "label": "Solo la empresa"},
			{"value": models.ExpedienteShared, "label": "Compartida con el profesional"},
		},
		"contact_channel": []gin.H{
			{"value": "email", "label": "Correo"},
			{"value": "whatsapp", "label": "WhatsApp"},
			{"value": "chat", "label": "Chat"},
		},
	}
}
