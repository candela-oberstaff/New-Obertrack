package models

import "time"

// MonthCloseRun es el libro del CIERRE DE MES de una empresa: la aprobación
// automática de las jornadas que quedaron pendientes al terminar el mes. Una
// fila por (empresa, período) hace la aprobación idempotente — el correo del
// reporte tiene su propia deduplicación (report_runs), así que un reintento de
// envío jamás re-aprueba. Lo dispara ReportMailWatcher en el envío MENSUAL,
// según la configuración de /admin/settings.
type MonthCloseRun struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	TenantID uint `gorm:"not null;uniqueIndex:idx_month_close_tenant_period" json:"tenant_id"`
	// Period es el mes cerrado en formato "2026-07".
	Period string `gorm:"size:7;not null;uniqueIndex:idx_month_close_tenant_period" json:"period"`
	// ApprovedCount: cuántas jornadas pendientes aprobó el cierre.
	ApprovedCount int64     `json:"approved_count"`
	ApprovedAt    time.Time `json:"approved_at"`
	CreatedAt     time.Time `json:"created_at"`
}

func (MonthCloseRun) TableName() string {
	return "month_close_runs"
}
