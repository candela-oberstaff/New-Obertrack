package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Cierre de mes automático (tarjeta "Aprobación de horas al final del mes"):
// en el envío MENSUAL del reporte, ReportMailWatcher aprueba primero las
// jornadas pendientes del período — una sola vez por (empresa, período),
// aunque el correo se reintente. Estos tests fijan ese contrato sobre
// autoApproveMonth.

type fakeMonthCloseWHRepo struct {
	repository.WorkHourRepository

	runs        map[string]*models.MonthCloseRun
	approvedN   int64
	approveHits int
}

func newFakeMonthCloseWHRepo() *fakeMonthCloseWHRepo {
	return &fakeMonthCloseWHRepo{runs: map[string]*models.MonthCloseRun{}, approvedN: 3}
}

func (f *fakeMonthCloseWHRepo) GetMonthCloseRun(tenantID uint, period string) (*models.MonthCloseRun, error) {
	return f.runs[period], nil
}

func (f *fakeMonthCloseWHRepo) CreateMonthCloseRun(run *models.MonthCloseRun) error {
	run.ID = uint(len(f.runs) + 1)
	f.runs[run.Period] = run
	return nil
}

func (f *fakeMonthCloseWHRepo) ApprovePendingInRange(tenantID uint, start, end time.Time, approvedBy uint, approvedAt time.Time) (int64, error) {
	f.approveHits++
	return f.approvedN, nil
}

func julio2026() (time.Time, time.Time) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	return start, time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
}

func TestMonthClose_ApprovesOncePerPeriod(t *testing.T) {
	wh := newFakeMonthCloseWHRepo()
	notif := &fakeNotifSvc{}
	w := &ReportMailWatcher{whRepo: wh, notifSvc: notif}
	emp := &models.User{ID: 50, UserType: models.UserTypeEmployer, CompanyName: "Acme"}

	start, end := julio2026()
	w.autoApproveMonth(emp, start, end)

	if wh.approveHits != 1 {
		t.Fatalf("debió aprobar pendientes una vez, hits: %d", wh.approveHits)
	}
	run := wh.runs["2026-07"]
	if run == nil || run.ApprovedCount != 3 || run.TenantID != 50 {
		t.Fatalf("el cierre debió registrarse, run: %+v", run)
	}

	// Idempotencia: el reintento del correo (u otro tick) NO re-aprueba.
	w.autoApproveMonth(emp, start, end)
	if wh.approveHits != 1 {
		t.Fatalf("el segundo intento no debía re-aprobar, hits: %d", wh.approveHits)
	}
}

// Sin jornadas pendientes el cierre se registra igual (para no re-evaluar) pero
// no molesta a la empresa con una campanita vacía.
func TestMonthClose_ZeroPendingRegistersWithoutNotifying(t *testing.T) {
	wh := newFakeMonthCloseWHRepo()
	wh.approvedN = 0
	notif := &fakeNotifSvc{}
	w := &ReportMailWatcher{whRepo: wh, notifSvc: notif}

	start, end := julio2026()
	w.autoApproveMonth(&models.User{ID: 50, UserType: models.UserTypeEmployer}, start, end)

	if run := wh.runs["2026-07"]; run == nil || run.ApprovedCount != 0 {
		t.Fatalf("el cierre en cero debió registrarse, run: %+v", run)
	}
	if len(notif.notified) != 0 {
		t.Fatalf("sin aprobaciones no debe haber campanita, creadas: %d", len(notif.notified))
	}
}

// Sin whRepo cableado (tests viejos u otros usos del watcher), autoApproveMonth
// es un no-op seguro.
func TestMonthClose_NoRepoIsNoop(t *testing.T) {
	w := &ReportMailWatcher{}
	start, end := julio2026()
	w.autoApproveMonth(&models.User{ID: 50}, start, end) // no debe panicar
}
