package models

import (
	"time"

	"gorm.io/gorm"
)

// Estados de una paylist local. Reflejan el ciclo de vida agregado del lote
// según el estado de sus pagos individuales en Ontop.
const (
	PaylistStatusPending    = "pending"    // creada localmente, aún sin confirmar en Ontop
	PaylistStatusSubmitted  = "submitted"  // aceptada por Ontop (tiene ontop_paylist_id)
	PaylistStatusProcessing = "processing" // hay pagos aún en PENDING
	PaylistStatusCompleted  = "completed"  // todos los pagos SUCCESS
	PaylistStatusPartial    = "partial"    // mezcla de SUCCESS y FAILURE/REJECTED
	PaylistStatusFailed     = "failed"     // ningún pago exitoso
)

// Estados de un pago individual. Coinciden con los que expone Ontop
// (PENDING/SUCCESS/FAILURE/REJECTED) más "error" para fallos locales de envío.
const (
	PaymentStatusPending  = "PENDING"
	PaymentStatusSuccess  = "SUCCESS"
	PaymentStatusFailure  = "FAILURE"
	PaymentStatusRejected = "REJECTED"
	PaymentStatusError    = "ERROR"
)

// WalletPaylist es el espejo LOCAL de una paylist creada en Ontop. El backend
// mantiene su propia fuente de verdad para poder reconciliar contra Ontop sin
// depender de consultar su API en cada operación, garantizar idempotencia y
// auditar quién disparó cada lote de pagos.
type WalletPaylist struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// OntopPaylistID es el ID que devuelve Ontop al aceptar el lote. Nulo hasta
	// que la creación remota se confirma.
	OntopPaylistID *int64 `gorm:"index" json:"ontop_paylist_id,omitempty"`
	// IdempotenceKey es la clave única de 32 chars (hash MD5) que exige Ontop
	// para evitar duplicar paylists en reintentos. Único a nivel local también.
	IdempotenceKey string `gorm:"size:32;uniqueIndex;not null" json:"idempotence_key"`
	ClientID       string `gorm:"size:64;not null" json:"client_id"`
	Description    string `gorm:"size:255" json:"description"`
	Status         string `gorm:"size:20;not null;default:'pending'" json:"status"`
	// TotalAmount es la suma de los montos de los pagos del lote.
	TotalAmount float64 `gorm:"type:numeric(18,2);not null;default:0" json:"total_amount"`
	Currency    string  `gorm:"size:8" json:"currency"`
	// LastError guarda el mensaje del último fallo al crear/reconciliar en Ontop.
	LastError string `gorm:"type:text" json:"last_error,omitempty"`
	CreatedBy uint   `json:"created_by"`

	Payments []WalletPayment `gorm:"foreignKey:PaylistID" json:"payments,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (WalletPaylist) TableName() string {
	return "wallet_paylists"
}

// WalletPayment es el espejo LOCAL de un pago individual dentro de una paylist.
type WalletPayment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	PaylistID      uint      `gorm:"index;not null" json:"paylist_id"`
	OntopPaymentID *int64    `gorm:"index" json:"ontop_payment_id,omitempty"`
	WorkerEmail    string    `gorm:"size:255;not null" json:"worker_email"`
	Amount         float64   `gorm:"type:numeric(18,2);not null" json:"amount"`
	Description    string    `gorm:"size:255" json:"description"`
	Status         string    `gorm:"size:20;not null;default:'PENDING'" json:"status"`
	Cause          string    `gorm:"type:text" json:"cause,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (WalletPayment) TableName() string {
	return "wallet_payments"
}
