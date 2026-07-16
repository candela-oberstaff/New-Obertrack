package service

import (
	"log"
	"time"
)

// ChatImportWatcher watches the WAHA session and, on the transition to WORKING
// (connected), imports the existing conversations once. Re-import is idempotent
// (external_id unique index), so a reconnect safely re-syncs without duplicates.
type ChatImportWatcher struct {
	wahaSvc   *WahaService
	ticketSvc TicketService
	imported  bool // whether we already imported for the current WORKING streak
}

func NewChatImportWatcher(wahaSvc *WahaService, ticketSvc TicketService) *ChatImportWatcher {
	return &ChatImportWatcher{wahaSvc: wahaSvc, ticketSvc: ticketSvc}
}

// Start launches the watcher loop. `interval` is how often the session status is
// polled to detect a (re)connection.
func (w *ChatImportWatcher) Start(interval time.Duration) {
	go func() {
		time.Sleep(startupJitter()) // stagger boot-time load (shared with ContactSync)
		w.tick()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			w.tick()
		}
	}()
}

// tick checks the session status and imports once when it becomes connected.
func (w *ChatImportWatcher) tick() {
	status, err := w.wahaSvc.GetSessionStatusAndQR(w.wahaSvc.GetSession())
	if err != nil {
		return
	}

	working := status.Status == "WORKING" || status.Status == "CONNECTED"
	if !working {
		w.imported = false // reset so the next reconnection re-imports
		return
	}
	if w.imported {
		return
	}

	n, err := w.ticketSvc.ImportWhatsAppHistory()
	if err != nil {
		log.Printf("[ChatImport] import failed: %v", err)
		return // leave imported=false so it retries on the next tick
	}
	w.imported = true
	log.Printf("[ChatImport] imported %d message(s) from existing WhatsApp chats", n)
}
