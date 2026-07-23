package service

import (
	"errors"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// WalletService es el servicio de dominio del módulo Wallet en su enfoque
// PERSONAL y de SOLO LECTURA: cada profesional consulta únicamente sus propios
// pagos/ganancias. El backend usa la cuenta Ontop del cliente para listar los
// pagos y los filtra por el email del usuario autenticado, de modo que un
// profesional nunca ve los pagos de otro ni la empresa ve las ganancias
// individuales. No se crean pagos desde la app (los pagos se originan en Ontop).
type WalletService interface {
	Enabled() bool
	MyEarnings(email string) (*EarningsSummary, error)
}

// MyPayment es un pago recibido por el profesional (vista personal).
type MyPayment struct {
	ID          int64   `json:"id"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Status      string  `json:"status"`
	Cause       string  `json:"cause,omitempty"`
	PaylistID   int64   `json:"paylist_id,omitempty"`
	Date        string  `json:"date,omitempty"`
}

// EarningsSummary agrega las ganancias del profesional.
type EarningsSummary struct {
	TotalPaid float64     `json:"total_paid"` // suma de pagos SUCCESS
	Pending   float64     `json:"pending"`    // suma de pagos PENDING
	Count     int         `json:"count"`
	Currency  string      `json:"currency"`
	Payments  []MyPayment `json:"payments"`
}

type walletService struct {
	ontop *OntopService
}

func NewWalletService(ontop *OntopService) WalletService {
	return &walletService{ontop: ontop}
}

func (s *walletService) Enabled() bool { return s.ontop.Configured() }

// MyEarnings lista los pagos del profesional (por email) en los últimos 12 meses
// y los agrega. El filtrado por email ocurre en el backend: la respuesta de Ontop
// nunca se expone cruda al cliente.
func (s *walletService) MyEarnings(email string) (*EarningsSummary, error) {
	if !s.ontop.Configured() {
		return nil, errors.New("La billetera no está disponible por el momento")
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, errors.New("El usuario no tiene un email asociado")
	}

	end := time.Now().UTC()
	start := end.AddDate(-1, 0, 0)
	startISO := start.Format(time.RFC3339)
	endISO := end.Format(time.RFC3339)

	const pageSize = 200
	const maxPages = 50 // tope de seguridad (~10k pagos) para no paginar sin fin

	mine := make([]MyPayment, 0)
	for page := 0; page < maxPages; page++ {
		items, last, err := s.ontop.ListPayments(startISO, endISO, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, p := range items {
			if !strings.EqualFold(strings.TrimSpace(p.WorkerEmail), email) {
				continue
			}
			cause := ""
			if p.Cause != nil {
				cause = *p.Cause
			}
			mine = append(mine, MyPayment{
				ID:          p.ID,
				Description: p.Description,
				Amount:      p.Amount,
				Status:      p.Status,
				Cause:       cause,
				PaylistID:   p.PaylistID,
				Date:        p.PaymentDate(),
			})
		}
		if last || len(items) < pageSize {
			break
		}
	}

	var total, pending float64
	for _, p := range mine {
		switch p.Status {
		case models.PaymentStatusSuccess:
			total += p.Amount
		case models.PaymentStatusPending:
			pending += p.Amount
		}
	}

	return &EarningsSummary{
		TotalPaid: total,
		Pending:   pending,
		Count:     len(mine),
		Currency:  "USD",
		Payments:  mine,
	}, nil
}
