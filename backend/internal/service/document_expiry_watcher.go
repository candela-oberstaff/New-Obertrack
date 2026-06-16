package service

import (
	"fmt"
	"log"
	"time"

	"github.com/obertrack/backend/internal/repository"
)

const (
	// Avisar cuando un documento vence dentro de esta ventana.
	docExpiryWindow = 30 * 24 * time.Hour
	// Cadencia del chequeo.
	docExpiryCheckInterval = 24 * time.Hour
	// Espera tras el arranque antes del primer chequeo.
	docExpiryFirstRunDelay = 3 * time.Minute
)

// DocumentExpiryWatcher revisa a diario los documentos del expediente que están
// por vencer y notifica a la empresa (RR.HH.) para que los renueve. Cada
// documento se alerta una sola vez (se rearma al cambiar su vencimiento).
type DocumentExpiryWatcher struct {
	repo     repository.EmploymentRepository
	userRepo repository.UserRepository
	notifSvc NotificationService
}

func NewDocumentExpiryWatcher(repo repository.EmploymentRepository, userRepo repository.UserRepository, notifSvc NotificationService) *DocumentExpiryWatcher {
	return &DocumentExpiryWatcher{repo: repo, userRepo: userRepo, notifSvc: notifSvc}
}

// Start lanza el chequeo periódico en segundo plano.
func (w *DocumentExpiryWatcher) Start() {
	go func() {
		time.Sleep(docExpiryFirstRunDelay)
		for {
			if _, err := w.RunOnce(); err != nil {
				log.Printf("[doc-expiry-watcher] chequeo fallido: %v", err)
			}
			time.Sleep(docExpiryCheckInterval)
		}
	}()
}

// RunOnce notifica los documentos que vencen dentro de la ventana y aún no se
// alertaron. Devuelve la cantidad de alertas enviadas.
func (w *DocumentExpiryWatcher) RunOnce() (int, error) {
	now := time.Now()
	docs, err := w.repo.ListDocumentsExpiringSoon(now.Add(docExpiryWindow))
	if err != nil {
		return 0, err
	}

	sent := 0
	for i := range docs {
		d := docs[i]
		emp, err := w.repo.GetByID(d.EmploymentID)
		if err != nil {
			continue
		}

		// Nombre del profesional y título del documento para el mensaje.
		who := "un profesional"
		if u, err := w.userRepo.GetByID(emp.UserID); err == nil && u.Name != "" {
			who = u.Name
		}
		title := d.Title
		if title == "" {
			title = d.FileName
		}

		vencido := d.ExpiresAt != nil && d.ExpiresAt.Before(now)
		var msg string
		if vencido {
			msg = fmt.Sprintf("El documento \"%s\" de %s está vencido (%s). Conviene renovarlo.",
				title, who, d.ExpiresAt.Format("02/01/2006"))
		} else if d.ExpiresAt != nil {
			dias := int(d.ExpiresAt.Sub(now).Hours() / 24)
			msg = fmt.Sprintf("El documento \"%s\" de %s vence en %d día(s) (%s).",
				title, who, dias, d.ExpiresAt.Format("02/01/2006"))
		}

		// Avisar a la empresa (cuenta empleador = CompanyID) para que actúe.
		if err := w.notifSvc.CreateNotification(
			emp.CompanyID,
			"document_expiry",
			"Documento por vencer",
			msg,
			map[string]interface{}{"employment_id": emp.ID, "document_id": d.ID},
		); err != nil {
			log.Printf("[doc-expiry-watcher] no se pudo notificar doc %d: %v", d.ID, err)
			continue
		}

		if err := w.repo.MarkDocumentAlerted(d.ID, now); err != nil {
			log.Printf("[doc-expiry-watcher] no se pudo marcar doc %d: %v", d.ID, err)
			continue
		}
		sent++
	}
	if sent > 0 {
		log.Printf("[doc-expiry-watcher] %d alerta(s) de vencimiento enviada(s)", sent)
	}
	return sent, nil
}
