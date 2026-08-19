package service

import (
	"log"
	"time"
)

const (
	// Cadencia del reloj. Un minuto de resolución es de sobra para un anuncio
	// programado y no pesa: la consulta busca por índice y casi siempre vuelve
	// vacía.
	tutorialScheduleInterval = time.Minute
	// Espera tras el arranque: deja pasar las migraciones antes de tocar nada.
	tutorialScheduleFirstRunDelay = 90 * time.Second
)

// TutorialScheduleWatcher publica las novedades programadas cuando llega su
// hora y retira las que caducaron. Es lo que permite dejar un anuncio listo el
// viernes para que salga el lunes a las nueve.
type TutorialScheduleWatcher struct {
	svc TutorialService
}

func NewTutorialScheduleWatcher(svc TutorialService) *TutorialScheduleWatcher {
	return &TutorialScheduleWatcher{svc: svc}
}

// Start lanza el reloj en segundo plano.
func (w *TutorialScheduleWatcher) Start() {
	go func() {
		time.Sleep(tutorialScheduleFirstRunDelay)
		for {
			published, expired, err := w.svc.RunSchedule()
			if err != nil {
				log.Printf("[tutorial-schedule] pasada fallida: %v", err)
			} else if published > 0 || expired > 0 {
				log.Printf("[tutorial-schedule] %d novedades publicadas, %d retiradas", published, expired)
			}
			time.Sleep(tutorialScheduleInterval)
		}
	}()
}
