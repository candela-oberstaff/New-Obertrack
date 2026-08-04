package service

import (
	"errors"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// WhatsAppOutbox entrega los mensajes salientes de WhatsApp fuera del request.
//
// Antes el handler llamaba a WAHA en línea: si el contenedor de WAHA estaba
// caído, o el backend se reiniciaba a mitad del envío, la respuesta del agente se
// perdía sin rastro. Además el request cargaba con el espaciado antibaneo y la
// simulación de tecleo (~6s), y bastaban dos agentes respondiendo a la vez para
// que se hicieran cola entre ellos.
//
// Ahora el mensaje se persiste como 'pending' dentro del request (rápido y
// duradero) y este worker lo entrega con reintentos escalonados. Un reinicio no
// pierde nada: los pendientes siguen en la tabla y se retoman al arrancar.
type WhatsAppOutbox struct {
	repo repository.TicketRepository
	waha *WahaService
	// nudge despierta al worker en cuanto se encola algo, para no esperar al
	// siguiente tick: el agente espera ver su mensaje salir ya.
	nudge chan struct{}
}

func NewWhatsAppOutbox(repo repository.TicketRepository, waha *WahaService) *WhatsAppOutbox {
	return &WhatsAppOutbox{repo: repo, waha: waha, nudge: make(chan struct{}, 1)}
}

// outboxBatchSize acota cuántos mensajes se toman por vuelta. Cada envío consume
// varios segundos por el antibaneo, así que un lote grande solo alargaría la
// vuelta sin entregar antes: el resto se toma en la siguiente.
const outboxBatchSize = 25

// Start lanza el bucle del worker. Idempotente frente a reinicios: los pendientes
// sobreviven en la BD y se retoman en la primera vuelta.
func (o *WhatsAppOutbox) Start() {
	go o.run()
	log.Println("WhatsApp outbox: worker iniciado")
}

func (o *WhatsAppOutbox) run() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		o.processBatch()
		select {
		case <-ticker.C:
		case <-o.nudge:
		}
	}
}

// Signal despierta al worker tras encolar un mensaje.
func (o *WhatsAppOutbox) Signal() {
	select {
	case o.nudge <- struct{}{}:
	default: // ya hay un aviso pendiente; no hace falta otro.
	}
}

func (o *WhatsAppOutbox) processBatch() {
	msgs, err := o.repo.ClaimPendingOutbound(outboxBatchSize, time.Now())
	if err != nil {
		log.Printf("WhatsApp outbox: no se pudieron leer mensajes pendientes: %v", err)
		return
	}
	for i := range msgs {
		o.processMessage(&msgs[i])
	}
}

// dbRetryDelay es lo que se espera antes de volver a intentar un mensaje cuya
// entrega ni siquiera llegó a empezar por un fallo de base de datos.
const dbRetryDelay = 30 * time.Second

func (o *WhatsAppOutbox) processMessage(msg *models.TicketMessage) {
	ticket, err := o.repo.GetWithContact(msg.TicketID)
	// Un fallo de base NO es lo mismo que un ticket sin contacto: si el pool se
	// agota o hay un failover, darlo por agotado aquí marcaría como no entregado
	// —para siempre y sin reintento— todo el lote, por un parpadeo. Se pospone.
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		o.postpone(msg, dbRetryDelay, err)
		return
	}
	if err != nil || ticket == nil || ticket.Contact == nil {
		// Sin destinatario no hay envío posible y esperar no lo arregla: se agota
		// de inmediato en vez de gastar seis intentos en lo mismo.
		o.fail(msg, msg.DeliveryAttempts+1, "el ticket no tiene contacto asociado", nil)
		return
	}

	dest := ticket.Contact.WaID
	if dest == "" {
		dest = ticket.Contact.Phone
	}

	session := o.waha.GetSession()
	// Marca de tiempo previa al envío: acota la búsqueda de la reconciliación a
	// mensajes posteriores a este instante.
	startedAt := time.Now()
	sentID, err := o.waha.SendMessage(session, dest, msg.Content)

	// Envío de resultado desconocido (timeout). Reenviarlo a ciegas se lo dejaría
	// duplicado al contacto, así que primero se le pregunta al propio chat si el
	// mensaje llegó a salir.
	if errors.Is(err, apperrors.ErrSendUncertain) {
		if id, ok := o.waha.FindDeliveredMessage(session, dest, msg.Content, startedAt); ok {
			log.Printf("WhatsApp outbox: el mensaje %d sí se había entregado pese al timeout; no se reenvía", msg.ID)
			sentID, err = id, nil
		}
	}

	if err == nil {
		o.markSent(msg, sentID)
		return
	}

	// El límite antibaneo no es un fallo: la cola está funcionando y este mensaje
	// simplemente todavía no tiene turno. Se pospone sin gastar un intento, de lo
	// contrario una ráfaga legítima agotaría los reintentos sin haber tocado WAHA.
	if errors.Is(err, apperrors.ErrRateLimited) {
		o.postpone(msg, 5*time.Second, nil)
		return
	}

	attempts := msg.DeliveryAttempts + 1
	var retryAt *time.Time
	if attempts < models.DeliveryMaxAttempts {
		at := time.Now().Add(models.DeliveryRetryDelay(attempts))
		retryAt = &at
	}
	o.fail(msg, attempts, err.Error(), retryAt)
}

// markSentAttempts: WAHA ya aceptó el mensaje, así que no cerrar la fila lo deja
// en 'pending' y la vuelta siguiente lo entregaría por segunda vez. Se insiste
// unas cuantas veces antes de rendirse, que cubre el parpadeo de base típico.
const markSentAttempts = 3

// markSent cierra la entrega guardando el ID de WAHA y avisa al chat.
func (o *WhatsAppOutbox) markSent(msg *models.TicketMessage, sentID string) {
	var err error
	for attempt := 1; attempt <= markSentAttempts; attempt++ {
		if err = o.repo.MarkMessageSent(msg.ID, sentID); err == nil {
			msg.DeliveryStatus = models.DeliverySent
			msg.ExternalID = sentID
			broadcastTicketMessage(msg.TicketID, msg)
			return
		}
		if attempt < markSentAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	log.Printf("WhatsApp outbox: ATENCIÓN — el mensaje %d se entregó (id de WAHA %q) pero no se pudo marcar como enviado: %v. Sigue pendiente y la próxima vuelta podría reenviarlo al contacto.",
		msg.ID, sentID, err)
}

// postpone retrasa un mensaje SIN gastar un intento: la entrega no llegó a
// fallar, solo no se pudo intentar todavía.
func (o *WhatsAppOutbox) postpone(msg *models.TicketMessage, in time.Duration, cause error) {
	if rerr := o.repo.RescheduleMessage(msg.ID, time.Now().Add(in)); rerr != nil {
		log.Printf("WhatsApp outbox: no se pudo reprogramar el mensaje %d: %v", msg.ID, rerr)
		return
	}
	if cause != nil {
		log.Printf("WhatsApp outbox: mensaje %d pospuesto %s: %v", msg.ID, in, cause)
	}
}

// fail registra el intento fallido y, cuando ya no hay reintentos, avisa por
// websocket para que el chat muestre el mensaje como no entregado.
func (o *WhatsAppOutbox) fail(msg *models.TicketMessage, attempts int, errMsg string, retryAt *time.Time) {
	if merr := o.repo.MarkMessageFailed(msg.ID, attempts, errMsg, retryAt); merr != nil {
		log.Printf("WhatsApp outbox: no se pudo marcar el mensaje %d como fallido: %v", msg.ID, merr)
		return
	}
	if retryAt != nil {
		log.Printf("WhatsApp outbox: mensaje %d falló en el intento %d, reintento en %s: %s",
			msg.ID, attempts, models.DeliveryRetryDelay(attempts), errMsg)
		return
	}
	log.Printf("WhatsApp outbox: mensaje %d agotado tras %d intentos: %s", msg.ID, attempts, errMsg)
	msg.DeliveryStatus = models.DeliveryFailed
	msg.DeliveryError = errMsg
	broadcastTicketMessage(msg.TicketID, msg)
}
