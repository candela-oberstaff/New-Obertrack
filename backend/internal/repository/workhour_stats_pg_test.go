package repository

import (
	"os"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Corre contra un Postgres de verdad solo si STATS_PG_DSN apunta a una base
// desechable (crea y borra sus tablas). Sin la variable se salta.
func TestStatsEnPostgres(t *testing.T) {
	dsn := os.Getenv("STATS_PG_DSN")
	if dsn == "" {
		t.Skip("STATS_PG_DSN sin definir")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Migrator().DropTable(&models.WorkHour{}, &models.User{})
	if err := db.AutoMigrate(&models.User{}, &models.WorkHour{}); err != nil {
		t.Fatal(err)
	}

	ana := models.User{Name: "Ana", Email: "ana@x.test"}
	beto := models.User{Name: "Beto", Email: "beto@x.test"}
	db.Create(&ana)
	db.Create(&beto)
	d := func(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC) }
	rows := []models.WorkHour{
		{UserID: ana.ID, TenantID: 7, WorkDate: d(1), HoursWorked: 8, Approved: true},
		{UserID: ana.ID, TenantID: 7, WorkDate: d(2), HoursWorked: 8},
		{UserID: beto.ID, TenantID: 7, WorkDate: d(2), HoursWorked: 4, Rejected: true},
		{UserID: beto.ID, TenantID: 7, WorkDate: d(3), HoursWorked: 6},
		// Otra empresa: no cuenta.
		{UserID: beto.ID, TenantID: 9, WorkDate: d(3), HoursWorked: 8},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := &workHourRepository{db: db}
	today := d(3)
	s, err := repo.Stats(map[string]interface{}{
		"tenant_id":  uint(7),
		"start_date": d(1),
		"end_date":   d(31),
	}, &today)
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalHours != 26 || s.ApprovedHours != 8 || s.RejectedHours != 4 || s.PendingHours != 14 {
		t.Fatalf("horas = %+v", s)
	}
	if s.Entries != 4 || s.PendingEntries != 2 || s.DaysWorked != 3 || s.People != 2 {
		t.Fatalf("cuentas = %+v", s)
	}
	if len(s.Daily) != 3 || s.Daily[1].Date != "2026-10-02" || s.Daily[1].Hours != 12 {
		t.Fatalf("por día = %+v", s.Daily)
	}
	if len(s.ByPerson) != 2 || s.ByPerson[0].Name != "Ana" || s.ByPerson[0].Hours != 16 {
		t.Fatalf("por persona = %+v", s.ByPerson)
	}
	if len(s.TodayUserIDs) != 1 || s.TodayUserIDs[0] != beto.ID {
		t.Fatalf("hoy = %v", s.TodayUserIDs)
	}
	_ = db.Migrator().DropTable(&models.WorkHour{}, &models.User{})
}
