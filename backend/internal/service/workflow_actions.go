package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/obertrack/backend/internal/models"
)

// stepConfig es lo que guarda cada paso. Los textos llevan {{variables}} sin
// resolver; se interpolan al ejecutar, contra el snapshot de la ejecución.
type stepConfig struct {
	// Recipient es la CLASE de destinatario (ver models.Recipient*), no un id.
	Recipient string `json:"recipient"`
	// UserID sólo se usa con recipient = "usuario_fijo".
	UserID uint `json:"user_id,omitempty"`

	// Priority, Status y Content son de las acciones que mutan la tarea.
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
	Content  string `json:"content,omitempty"`

	Title   string `json:"title,omitempty"`
	Message string `json:"message,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

// variablePattern captura {{lo.que.sea}} tolerando espacios interiores.
var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// interpolate sustituye las variables del texto por su valor en el snapshot.
//
// Una variable desconocida se deja TAL CUAL, visible, en vez de borrarse: un aviso
// que dice "vence {{tarea.fecha_finn}}" delata la errata de quien escribió la
// plantilla; uno que dice "vence " parece un fallo del sistema y nadie sabe dónde
// mirar.
func interpolate(text string, ctx WorkflowContext) string {
	if text == "" || !strings.Contains(text, "{{") {
		return text
	}
	vars := flattenContext(ctx)
	return variablePattern.ReplaceAllStringFunc(text, func(match string) string {
		key := variablePattern.FindStringSubmatch(match)[1]
		if v, ok := vars[key]; ok {
			return v
		}
		return match
	})
}

// flattenContext aplana el snapshot a rutas con punto ("tarea.titulo"), que es como
// se escriben en las plantillas. Se hace pasando por JSON para que el aplanado siga
// automáticamente a la estructura y no haya que mantener una lista a mano que se
// desincronice en cuanto alguien añada un campo.
func flattenContext(ctx WorkflowContext) map[string]string {
	raw, err := json.Marshal(ctx)
	if err != nil {
		return map[string]string{}
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return map[string]string{}
	}
	out := make(map[string]string)
	flattenInto("", tree, out)
	return out
}

func flattenInto(prefix string, node any, out map[string]string) {
	switch t := node.(type) {
	case map[string]any:
		for k, v := range t {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flattenInto(key, v, out)
		}
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		out[prefix] = strings.Join(parts, ", ")
	case float64:
		// Los números del JSON son float64; sin esto un id 7 se imprimiría "7e+00".
		if t == float64(int64(t)) {
			out[prefix] = fmt.Sprintf("%d", int64(t))
		} else {
			out[prefix] = fmt.Sprintf("%v", t)
		}
	case nil:
		out[prefix] = ""
	default:
		out[prefix] = fmt.Sprintf("%v", t)
	}
}

// resolveRecipients traduce una clase de destinatario a ids concretos, EN EL
// MOMENTO DE EJECUTAR. Devuelve también el motivo cuando no resuelve a nadie, para
// que el paso quede 'skipped' explicando por qué y no en silencio.
func (s *WorkflowService) resolveRecipients(cfg stepConfig, run *models.WorkflowRun, ctx WorkflowContext) ([]uint, string) {
	switch cfg.Recipient {
	case models.RecipientAssignees:
		return s.activeOnly(ctx.Task.AsignadosIDs), "la tarea no tiene responsables activos"

	case models.RecipientNewAssignees:
		return s.activeOnly(ctx.Task.NuevosAsignados), "en este cambio no se sumó ningún responsable"

	case models.RecipientTaskCreator:
		// El creador no viaja en el snapshot (no es un dato que las condiciones
		// usen), así que se lee de la tarea. Es una lectura por ejecución, no por
		// request.
		if task, err := s.taskRepo.GetByID(ctx.Task.ID); err == nil && task != nil {
			return s.activeOnly([]uint{task.CreatedBy}), "el creador de la tarea ya no está activo"
		}
		return nil, "no se pudo leer la tarea para resolver a su creador"

	case models.RecipientBoardCreator:
		return s.activeOnly([]uint{ctx.Board.CreadorID}), "el creador del tablero ya no está activo"

	case models.RecipientActor:
		return s.activeOnly([]uint{ctx.Actor.ID}), "el cambio no lo hizo una persona"

	case models.RecipientFixedUser:
		return s.activeOnly([]uint{cfg.UserID}), "el usuario configurado ya no está activo"

	case models.RecipientEmployer:
		// El tenant ES la cuenta empleador: su id y el del tenant coinciden.
		return s.activeOnly([]uint{run.TenantID}), "la empresa no tiene cuenta activa"

	case models.RecipientAssigneeManager:
		return s.activeOnly(s.managersOfAssignees(ctx, run.TenantID)),
			"los responsables no tienen manager asignado"

	case models.RecipientBoardSupervisor:
		return s.activeOnly(s.boardSupervisors(ctx.Board.ID)),
			"el tablero no tiene supervisores entre sus miembros"

	case models.RecipientLeastLoaded:
		return s.leastLoadedMember(ctx, run.TenantID)

	case models.RecipientProjectLead:
		return s.projectLead(ctx, run.TenantID)
	}

	return nil, fmt.Sprintf("clase de destinatario desconocida: %q", cfg.Recipient)
}

// leastLoadedMember elige al miembro del tablero que está MÁS LIBRE.
//
// La carga se mide en tareas VIVAS —ni completadas ni en finalizado— y en toda la
// empresa, no sólo en este tablero: quien está hasta arriba en otro sitio está
// ocupado igual. Que lo terminado no cuente es lo que hace que quien va al día
// vuelva a la cola: acabar el trabajo tiene que abrir hueco, no cerrarlo.
//
// El desempate es en cascada y a propósito determinista: menos tareas abiertas →
// menos atrasadas → id más bajo. Sin el último criterio dos personas empatadas se
// turnarían al azar y la misma regla daría resultados distintos en cada ejecución,
// que en un reintento significaría asignar a alguien diferente del primer intento.
func (s *WorkflowService) leastLoadedMember(ctx WorkflowContext, tenantID uint) ([]uint, string) {
	board, err := s.boardRepo.GetByID(ctx.Board.ID)
	if err != nil || board == nil {
		return nil, "no se pudo leer el tablero para repartir por carga"
	}

	candidatos := make([]uint, 0, len(board.Members))
	for _, m := range board.Members {
		// La cuenta de la empresa es miembro de sus tableros pero no ejecuta tareas:
		// darle trabajo sería esconderlo, no repartirlo.
		if m.ID == tenantID {
			continue
		}
		candidatos = append(candidatos, m.ID)
	}
	candidatos = s.activeOnly(candidatos)
	if len(candidatos) == 0 {
		return nil, "el tablero no tiene miembros activos entre los que repartir"
	}

	cargas, err := s.taskRepo.OpenLoadByUser(tenantID, candidatos)
	if err != nil {
		return nil, "no se pudo calcular la carga de trabajo del equipo"
	}

	sort.Slice(candidatos, func(i, j int) bool {
		// Quien no está en el mapa no tiene ninguna tarea: el valor cero ES la
		// respuesta correcta, y por eso no hace falta comprobar la existencia.
		a, b := cargas[candidatos[i]], cargas[candidatos[j]]
		if a.Abiertas != b.Abiertas {
			return a.Abiertas < b.Abiertas
		}
		if a.Vencidas != b.Vencidas {
			return a.Vencidas < b.Vencidas
		}
		return candidatos[i] < candidatos[j]
	})
	return candidatos[:1], ""
}

// projectLead es la cadena de respaldo del "líder del proyecto". El modelo no tiene
// un rol de líder por tablero, así que se baja por aproximaciones hasta dar con
// alguien: manager del asignado → supervisor del tablero → creador del tablero →
// empleador. Si ningún nivel resuelve, se dice cuál fue el recorrido en vez de
// callar.
func (s *WorkflowService) projectLead(ctx WorkflowContext, tenantID uint) ([]uint, string) {
	if ids := s.activeOnly(s.managersOfAssignees(ctx, tenantID)); len(ids) > 0 {
		return ids, ""
	}
	if ids := s.activeOnly(s.boardSupervisors(ctx.Board.ID)); len(ids) > 0 {
		return ids, ""
	}
	if ids := s.activeOnly([]uint{ctx.Board.CreadorID}); len(ids) > 0 {
		return ids, ""
	}
	if ids := s.activeOnly([]uint{tenantID}); len(ids) > 0 {
		return ids, ""
	}
	return nil, "no hay líder de proyecto: ni manager del responsable, ni supervisor, ni creador del tablero, ni cuenta de empresa activos"
}

func (s *WorkflowService) managersOfAssignees(ctx WorkflowContext, tenantID uint) []uint {
	seen := make(map[uint]bool)
	out := []uint{}
	for _, assignee := range ctx.Task.AsignadosIDs {
		for _, mid := range resolveManagersFor(s.userRepo, s.empRepo, assignee, tenantID) {
			if !seen[mid] {
				seen[mid] = true
				out = append(out, mid)
			}
		}
	}
	return out
}

func (s *WorkflowService) boardSupervisors(boardID uint) []uint {
	board, err := s.boardRepo.GetByID(boardID)
	if err != nil || board == nil {
		return nil
	}
	out := []uint{}
	for _, m := range board.Members {
		if m.IsSupervisor {
			out = append(out, m.ID)
		}
	}
	return out
}

// activeOnly descarta ceros, repetidos, cuentas inactivas y cuentas de sistema. El
// bot no recibe sus propios avisos, y avisar a alguien que ya no trabaja aquí es
// justo lo que la resolución en ejecución existe para evitar.
func (s *WorkflowService) activeOnly(ids []uint) []uint {
	seen := make(map[uint]bool)
	out := []uint{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		u, err := s.userRepo.GetByID(id)
		if err != nil || u == nil || !u.IsActive || u.IsSystem {
			continue
		}
		out = append(out, id)
	}
	return out
}

// runAction ejecuta un paso y devuelve lo que deja para los siguientes. Un motivo
// no vacío significa 'skipped'; un error significa 'failed' y reintento.
//
// Las acciones se reparten en dos familias. Las de AVISO necesitan destinatarios y
// no tocan el dato. Las que MUTAN la tarea se resuelven aparte porque la mayoría no
// tiene destinatario alguno: exigirles uno las dejaría saltadas para siempre.
func (s *WorkflowService) runAction(step models.WorkflowStep, run *models.WorkflowRun, ctx WorkflowContext) (map[string]any, string, error) {
	var cfg stepConfig
	if err := json.Unmarshal([]byte(nonEmptyJSON(step.Config)), &cfg); err != nil {
		// Configuración ilegible: reintentar no la va a arreglar, así que se salta
		// en vez de gastar los seis intentos.
		return nil, fmt.Sprintf("la configuración del paso no se pudo interpretar: %v", err), nil
	}

	switch step.ActionType {
	case models.ActionSetPriority, models.ActionSetStatus, models.ActionComment, models.ActionAssign:
		return s.runMutation(step, cfg, run, ctx)
	}

	recipients, why := s.resolveRecipients(cfg, run, ctx)
	if len(recipients) == 0 {
		if why == "" {
			why = "no hay destinatarios para este paso"
		}
		return nil, why, nil
	}

	switch step.ActionType {
	case models.ActionNotify:
		return s.actionNotify(cfg, ctx, recipients)
	case models.ActionChatDM:
		return s.actionChatDM(cfg, ctx, recipients)
	case models.ActionEmail:
		return s.actionEmail(cfg, ctx, recipients)
	}
	return nil, fmt.Sprintf("acción desconocida: %q", step.ActionType), nil
}

func (s *WorkflowService) actionNotify(cfg stepConfig, ctx WorkflowContext, recipients []uint) (map[string]any, string, error) {
	title := interpolate(defaultText(cfg.Title, "Automatización de Obertrack"), ctx)
	message := interpolate(defaultText(cfg.Message, "La tarea {{tarea.titulo}} requiere tu atención"), ctx)

	entregados := make([]uint, 0, len(recipients))
	omitidos := make([]uint, 0)
	for _, uid := range recipients {
		creado, err := s.notifSvc.CreateNotificationChecked(uid, "workflow", title, message, map[string]interface{}{
			"task_id":  ctx.Task.ID,
			"board_id": ctx.Board.ID,
			// Al aviso de una automatización se llega sin contexto: nadie estaba
			// mirando esa tarjeta cuando saltó. Es donde más falta hace que el enlace
			// lleve a la tarea y no a la pantalla.
			"link": taskDeepLink(ctx.Task.ID, ctx.Board.ID, ctx.Empresa),
		})
		if errors.Is(err, ErrNotificationSuppressed) {
			// La deduplicación se lo tragó porque Tareas ya avisó de esta tarea hace
			// menos de un minuto. No es un fallo y no se reintenta —reintentar no
			// haría llegar nada— pero se registra: apuntar como "notificado" a quien
			// no recibió nada es lo que convierte el registro en un adorno.
			omitidos = append(omitidos, uid)
			continue
		}
		if err != nil {
			// Un destinatario que falla aborta el paso y lo reintenta entero. Los
			// avisos ya entregados no se duplican porque la deduplicación los atrapa.
			return nil, "", fmt.Errorf("notificando al usuario %d: %w", uid, err)
		}
		if creado {
			entregados = append(entregados, uid)
		}
	}

	out := map[string]any{"notificados": entregados}
	if len(omitidos) > 0 {
		out["omitidos_por_duplicado"] = omitidos
	}
	// Todos omitidos: el paso no entregó nada, y decirlo como "hecho" es mentir en el
	// único sitio donde alguien va a mirar cuando pregunte por qué no le llegó.
	if len(entregados) == 0 && len(omitidos) > 0 {
		return out, "Tareas ya había avisado de esta tarea hace menos de un minuto", nil
	}
	return out, "", nil
}

func (s *WorkflowService) actionChatDM(cfg stepConfig, ctx WorkflowContext, recipients []uint) (map[string]any, string, error) {
	if s.postSystemDM == nil {
		return nil, "el chat interno no está disponible en esta instancia", nil
	}
	message := interpolate(defaultText(cfg.Message, "⚙️ {{tarea.titulo}} — {{tarea.estado}}"), ctx)
	for _, uid := range recipients {
		// PostSystemDM es best-effort por diseño (traga sus errores y los loguea):
		// el aviso que cuenta es la campanita, y un DM perdido no debe hacer que
		// toda la ejecución se reintente y reenvíe lo demás.
		s.postSystemDM(uid, message)
	}
	return map[string]any{"dm_enviados": recipients}, "", nil
}

func (s *WorkflowService) actionEmail(cfg stepConfig, ctx WorkflowContext, recipients []uint) (map[string]any, string, error) {
	if s.brevoSvc == nil {
		return nil, "el envío de correo no está configurado en esta instancia", nil
	}
	subject := interpolate(defaultText(cfg.Subject, "Obertrack · {{tarea.titulo}}"), ctx)
	body := interpolate(defaultText(cfg.Body, "<p>La tarea <strong>{{tarea.titulo}}</strong> está en {{tarea.estado}}.</p>"), ctx)

	sent := []uint{}
	for _, uid := range recipients {
		u, err := s.userRepo.GetByID(uid)
		if err != nil || u == nil || u.Email == "" {
			continue
		}
		// La CONSTANTE, no la cadena: escrita a mano, la clave no estaba en el
		// catálogo y una clave sin fila se da por encendida, así que este correo no
		// se podía apagar desde Configuración → Correos por mucho que el comentario
		// de antes dijera lo contrario.
		if err := s.brevoSvc.SendEmailKind(EmailKindWorkflow, u.Email, u.Name, subject, body); err != nil {
			return nil, "", fmt.Errorf("enviando correo a %s: %w", u.Email, err)
		}
		sent = append(sent, uid)
	}
	if len(sent) == 0 {
		return nil, "ningún destinatario tiene correo utilizable", nil
	}
	return map[string]any{"correos_enviados": sent}, "", nil
}

func defaultText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func nonEmptyJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	return raw
}
