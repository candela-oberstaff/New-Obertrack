package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Las métricas cuentan envíos y personas por separado: a quien se le reenvió
// el programa suma dos envíos pero una sola persona.
func TestSummarizeSends_CuentaEnviosYPersonas(t *testing.T) {
	now := time.Now()
	sends := []repository.ProgramSend{
		{UserID: 1, Status: models.InductionPassed, SentAt: now.Add(-48 * time.Hour)},
		{UserID: 1, Status: "expired", SentAt: now.Add(-96 * time.Hour)},
		{UserID: 2, Status: models.InductionPending, SentAt: now},
		{UserID: 3, Status: models.InductionBlocked, SentAt: now.Add(-time.Hour)},
	}
	stats := summarizeSends(sends)
	if stats.Sent != 4 || stats.People != 3 {
		t.Fatalf("envíos/personas = %d/%d, esperaba 4/3", stats.Sent, stats.People)
	}
	if stats.Passed != 1 || stats.Pending != 1 || stats.Expired != 1 || stats.Blocked != 1 {
		t.Fatalf("por estado: %+v", stats)
	}
	if stats.LastSentAt == nil || !stats.LastSentAt.Equal(now) {
		t.Fatalf("último envío = %v, esperaba %v", stats.LastSentAt, now)
	}
	if empty := summarizeSends(nil); empty.Sent != 0 || empty.LastSentAt != nil {
		t.Fatalf("sin envíos: %+v", empty)
	}
}
