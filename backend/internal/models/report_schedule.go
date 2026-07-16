package models

import "time"

const (
	ReportFreqDaily   = "daily"
	ReportFreqWeekly  = "weekly"
	ReportFreqMonthly = "monthly"

	ReportRunSent   = "sent"
	ReportRunFailed = "failed"
)

func IsValidReportFrequency(f string) bool {
	return f == ReportFreqDaily || f == ReportFreqWeekly || f == ReportFreqMonthly
}

// ReportSchedule guarda la configuración global (fila única, id = 1) del envío
// automático de reportes de jornadas a cada empresa.
type ReportSchedule struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Enabled    bool      `gorm:"not null;default:false" json:"enabled"`
	Frequency  string    `gorm:"size:20;not null;default:'monthly'" json:"frequency"`
	Hour       int       `gorm:"not null;default:8" json:"hour"`
	Minute     int       `gorm:"not null;default:0" json:"minute"`
	Timezone   string    `gorm:"size:64;not null;default:'UTC'" json:"timezone"`
	Weekday    int       `gorm:"not null;default:1" json:"weekday"`
	DayOfMonth int       `gorm:"not null;default:1" json:"day_of_month"`
	UpdatedBy  *uint     `json:"updated_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ReportSchedule) TableName() string { return "report_schedules" }

// ReportRun es bitácora y a la vez la deduplicación del worker: un índice único
// parcial sobre (tenant_id, period_key) WHERE status='sent' impide reenviar un
// período ya entregado, incluso después de reiniciar el backend. Las filas
// 'failed' no ocupan el índice, así que un fallo se puede reintentar.
type ReportRun struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	TenantID       uint      `gorm:"not null;index" json:"tenant_id"`
	PeriodKey      string    `gorm:"size:32;not null;index" json:"period_key"`
	Frequency      string    `gorm:"size:20" json:"frequency"`
	RecipientEmail string    `gorm:"size:255" json:"recipient_email"`
	RecipientName  string    `gorm:"size:255" json:"recipient_name"`
	Status         string    `gorm:"size:20;not null" json:"status"`
	Error          string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (ReportRun) TableName() string { return "report_runs" }
