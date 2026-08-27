package service

import (
	"log"
	"sync"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// activityKey es la granularidad de la medición: una persona, un día, un
// módulo. Todo lo que pase dentro de esa casilla se suma en memoria y llega a
// la base de datos como una sola fila.
type activityKey struct {
	UserID uint
	Day    string
	Module string
}

type activityCell struct {
	TenantID uint
	Hits     int
	LastAt   time.Time
}

const (
	// activityFlushEvery es cada cuánto se vuelca el buffer. Treinta segundos
	// es más de lo que cualquier pantalla de métricas necesita —se miran días,
	// no minutos— y a cambio agrupa decenas de peticiones de la misma persona
	// en un solo UPDATE.
	activityFlushEvery = 30 * time.Second
	// activityMaxBuffer es el tope de casillas distintas en memoria. Se vuelca
	// antes de tiempo al alcanzarlo para que un pico de tráfico no crezca sin
	// límite entre dos tics del reloj.
	activityMaxBuffer = 5000
)

// ActivityService cuenta el uso real de la app en memoria y lo vuelca por
// lotes.
//
// Nunca escribe en la ruta de la petición: medir el uso no puede hacer más
// lenta ni más frágil la app que se mide. Si la base de datos está caída, el
// lote se pierde y la vida sigue —un hueco en una métrica de adopción no vale
// un error en la cara del usuario—.
type ActivityService struct {
	repo repository.UsageRepository

	mu   sync.Mutex
	buf  map[activityKey]*activityCell
	stop chan struct{}
	once sync.Once
}

func NewActivityService(repo repository.UsageRepository) *ActivityService {
	return &ActivityService{
		repo: repo,
		buf:  make(map[activityKey]*activityCell, 256),
		stop: make(chan struct{}),
	}
}

// Track suma una petición a la casilla correspondiente. Es lo único que corre
// dentro del ciclo de vida de la petición: un lock y un incremento.
func (s *ActivityService) Track(userID, tenantID uint, module string, at time.Time) {
	if userID == 0 || module == "" {
		return
	}
	key := activityKey{UserID: userID, Day: at.Format("2006-01-02"), Module: module}

	s.mu.Lock()
	cell, ok := s.buf[key]
	if !ok {
		cell = &activityCell{}
		s.buf[key] = cell
	}
	cell.Hits++
	cell.TenantID = tenantID
	if at.After(cell.LastAt) {
		cell.LastAt = at
	}
	full := len(s.buf) >= activityMaxBuffer
	s.mu.Unlock()

	if full {
		go s.Flush()
	}
}

// Start arranca el volcado periódico. Se llama una vez al levantar el servidor.
func (s *ActivityService) Start() {
	go func() {
		ticker := time.NewTicker(activityFlushEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.Flush()
			case <-s.stop:
				// Último volcado al cerrar: lo acumulado en los últimos
				// segundos de vida del proceso también es uso.
				s.Flush()
				return
			}
		}
	}()
}

// Stop detiene el volcado periódico tras vaciar lo pendiente.
func (s *ActivityService) Stop() {
	s.once.Do(func() { close(s.stop) })
}

// Flush vacía el buffer contra la base de datos. Es seguro llamarlo desde
// varias goroutines: el buffer se cambia por uno nuevo bajo lock y el volcado
// ocurre fuera de él, así que una escritura lenta no bloquea las peticiones
// que siguen llegando.
func (s *ActivityService) Flush() {
	s.mu.Lock()
	if len(s.buf) == 0 {
		s.mu.Unlock()
		return
	}
	pending := s.buf
	s.buf = make(map[activityKey]*activityCell, len(pending))
	s.mu.Unlock()

	entries := make([]models.UserActivityDaily, 0, len(pending))
	for key, cell := range pending {
		day, err := time.Parse("2006-01-02", key.Day)
		if err != nil {
			continue
		}
		entries = append(entries, models.UserActivityDaily{
			UserID:   key.UserID,
			Day:      day,
			Module:   key.Module,
			TenantID: cell.TenantID,
			Hits:     cell.Hits,
			LastAt:   cell.LastAt,
		})
	}

	if err := s.repo.UpsertActivity(entries); err != nil {
		// No se reintenta ni se devuelve el lote al buffer: reencolar en un
		// fallo persistente haría crecer la memoria sin fin por una métrica.
		log.Printf("[Activity] no se pudo volcar %d contadores de uso: %v", len(entries), err)
	}
}
