package service

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/repository"
)

const (
	// chatDigestPendingFor: cuánto tiempo debe llevar pendiente el mensaje sin
	// leer MÁS ANTIGUO antes de avisar por correo. Corto sería spam (lo iban a
	// leer igual); esto cubre al que de verdad no se enteró.
	chatDigestPendingFor = 2 * time.Hour
	// chatDigestResendAfter: máximo un correo por usuario por esta ventana.
	chatDigestResendAfter = 24 * time.Hour
	// chatDigestCheckInterval: cadencia del chequeo.
	chatDigestCheckInterval = 30 * time.Minute
	// chatDigestFirstRunDelay: espera tras el arranque (migraciones, estabilizar).
	chatDigestFirstRunDelay = 3 * time.Minute
)

// ChatDigestWatcher cierra el ciclo de la comunicación interna: si alguien
// acumula mensajes de chat sin leer y no se conecta, le llega un correo de
// respaldo "tienes mensajes esperando en Obertrack". Sin esto, escribirle a un
// desconectado era esperar a que se le ocurriera entrar.
type ChatDigestWatcher struct {
	repo  repository.ChannelRepository
	brevo *BrevoService
}

func NewChatDigestWatcher(repo repository.ChannelRepository, brevo *BrevoService) *ChatDigestWatcher {
	return &ChatDigestWatcher{repo: repo, brevo: brevo}
}

// Start lanza el chequeo periódico en segundo plano.
func (w *ChatDigestWatcher) Start() {
	go func() {
		time.Sleep(chatDigestFirstRunDelay)
		for {
			if err := w.RunOnce(); err != nil {
				log.Printf("[chat-digest] chequeo fallido: %v", err)
			}
			time.Sleep(chatDigestCheckInterval)
		}
	}()
}

// RunOnce envía el correo a quienes lo necesitan y registra el envío. El
// registro se escribe ANTES de enviar: si Brevo acepta y el proceso muere, no
// se duplica el aviso; si Brevo falla, el reintento llega en la próxima
// ventana (perder un aviso es más barato que duplicarlo).
func (w *ChatDigestWatcher) RunOnce() error {
	if w.brevo == nil {
		return nil
	}
	candidates, err := w.repo.ListUsersNeedingChatDigest(chatDigestPendingFor, chatDigestResendAfter)
	if err != nil {
		return fmt.Errorf("listando destinatarios: %w", err)
	}
	for _, c := range candidates {
		if err := w.repo.MarkChatDigestSent(c.UserID); err != nil {
			log.Printf("[chat-digest] no se pudo registrar el envío a %d: %v", c.UserID, err)
			continue
		}
		if err := w.brevo.SendEmail(c.Email, c.Name, "💬 Tienes mensajes esperando en Obertrack", buildChatDigestHTML(c.Name, c.Unread)); err != nil {
			log.Printf("[chat-digest] no se pudo enviar a %s: %v", c.Email, err)
		}
	}
	if len(candidates) > 0 {
		log.Printf("[chat-digest] avisados %d usuario(s) con mensajes pendientes", len(candidates))
	}
	return nil
}

func buildChatDigestHTML(name string, unread int64) string {
	link := "/chat"
	base := os.Getenv("FRONTEND_URL")
	if base == "" {
		base = os.Getenv("SERVICE_URL_FRONTEND")
	}
	if base != "" {
		link = strings.TrimRight(base, "/") + link
	}

	plural := "mensajes nuevos"
	if unread == 1 {
		plural = "mensaje nuevo"
	}
	return fmt.Sprintf(`<h2 style="margin:0 0 16px 0;color:#060b23;">💬 Tienes %d %s</h2>
<p style="margin:0 0 16px 0;">Hola %s: tu equipo te escribió en el chat de Obertrack y aún no lo has visto.</p>
<div style="margin-top:24px;">
	<a href="%s" style="display:inline-block;background:#cc33cc;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:8px;font-weight:600;">Abrir el chat</a>
</div>
<p style="margin:16px 0 0;color:#8880a8;font-size:12px;">Recibes este aviso porque llevas un rato sin conectarte y hay conversaciones esperándote. Como máximo te enviaremos uno al día.</p>`,
		unread, plural, name, link)
}
