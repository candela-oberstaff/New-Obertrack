package service

import (
	"log"
	"time"
)

// defaultResyncEvery es cada cuánto se vuelve a traer el historial mientras la
// sesión sigue conectada.
//
// Importar solo al conectar dejaba la bandeja congelada en la última
// sincronización: si el webhook no llega —sesión sin webhook configurado, o una
// instancia de WAHA que no alcanza a este backend— no entraba ni un mensaje más
// hasta reiniciar la sesión a mano. Con la re-sincronización la bandeja se pone
// al día sola; el webhook sigue siendo el camino rápido, esto es la red de abajo.
const defaultResyncEvery = 5 * time.Minute

// ChatImportWatcher vigila la sesión de WAHA e importa las conversaciones
// existentes: en cuanto queda conectada (WORKING) y luego cada `resyncEvery`.
// Re-importar es idempotente (índice único de external_id), así que repetir solo
// añade lo nuevo.
type ChatImportWatcher struct {
	wahaSvc   *WahaService
	ticketSvc TicketService
	// lastImport es cuándo entró el último import de esta racha de conexión.
	// Cero = todavía no se ha importado desde que la sesión está conectada.
	lastImport  time.Time
	resyncEvery time.Duration
}

func NewChatImportWatcher(wahaSvc *WahaService, ticketSvc TicketService) *ChatImportWatcher {
	return &ChatImportWatcher{
		wahaSvc:     wahaSvc,
		ticketSvc:   ticketSvc,
		resyncEvery: time.Duration(envInt("WAHA_RESYNC_MINUTES", 5)) * time.Minute,
	}
}

// Start launches the watcher loop. `interval` is how often the session status is
// polled to detect a (re)connection.
func (w *ChatImportWatcher) Start(interval time.Duration) {
	if w.resyncEvery <= 0 {
		w.resyncEvery = defaultResyncEvery
	}
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

// tick importa al conectar y, a partir de ahí, cada `resyncEvery`.
func (w *ChatImportWatcher) tick() {
	status, err := w.wahaSvc.GetSessionStatusAndQR(w.wahaSvc.GetSession())
	if err != nil {
		return
	}

	working := status.Status == "WORKING" || status.Status == "CONNECTED"
	if !working {
		w.lastImport = time.Time{} // al reconectar se importa de inmediato
		return
	}
	if !w.lastImport.IsZero() && time.Since(w.lastImport) < w.resyncEvery {
		return
	}

	n, err := w.ticketSvc.ImportWhatsAppHistory()
	if err != nil {
		log.Printf("[ChatImport] import failed: %v", err)
		return // no se toca lastImport: se reintenta en el siguiente tick
	}
	w.lastImport = time.Now()
	if n > 0 {
		log.Printf("[ChatImport] imported %d message(s) from existing WhatsApp chats", n)
	}
}
