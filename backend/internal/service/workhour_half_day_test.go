package service

import (
	"errors"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// fakeDayWHRepo guarda en memoria lo que crea y actualiza el servicio de
// jornadas, para comprobar las horas que calcula el servidor.
type fakeDayWHRepo struct {
	repository.WorkHourRepository
	created *models.WorkHour
	stored  *models.WorkHour
	updated *models.WorkHour
}

func (f *fakeDayWHRepo) FindByUserDateKind(uint, time.Time, uint, bool) (*models.WorkHour, error) {
	return nil, errors.New("not found")
}
func (f *fakeDayWHRepo) Create(wh *models.WorkHour) error {
	wh.ID = 1
	f.created = wh
	return nil
}
func (f *fakeDayWHRepo) FindByID(uint) (*models.WorkHour, error) {
	if f.stored != nil {
		return f.stored, nil
	}
	if f.created != nil {
		return f.created, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeDayWHRepo) FindByIDAndTenant(id, _ uint) (*models.WorkHour, error) {
	return f.FindByID(id)
}
func (f *fakeDayWHRepo) Update(wh *models.WorkHour) error {
	f.updated = wh
	return nil
}

const dayUserID, dayTenantID = uint(4), uint(20)

func newDayService(scheduleType string) (*workHourService, *fakeDayWHRepo) {
	tenant := dayTenantID
	repo := &fakeDayWHRepo{}
	users := &fakeUserRepo{getByID: map[uint]*models.User{
		dayUserID: {ID: dayUserID, UserType: models.UserTypeProfessional, EmpleadorID: &tenant},
	}}
	emps := &fakeEmploymentRepo{activeByKey: map[[2]uint]*models.Employment{
		{dayUserID, dayTenantID}: {UserID: dayUserID, CompanyID: dayTenantID, ScheduleType: scheduleType},
	}}
	return &workHourService{repo: repo, userRepo: users, employmentRepo: emps, notifSvc: &fakeNotifSvc{}}, repo
}

func TestCreate_DayLengthFollowsScheduleType(t *testing.T) {
	cases := []struct {
		name         string
		scheduleType string
		req          map[string]interface{}
		wantWorked   float64
		wantAbsence  float64
	}{
		{"completa sin configurar = 8h", "", map[string]interface{}{"work_type": "complete"}, 8, 0},
		{"completa Full-time = 8h", models.ScheduleFullTime, map[string]interface{}{"work_type": "complete"}, 8, 0},
		{"media jornada = 4h", models.SchedulePartTime, map[string]interface{}{"work_type": "complete"}, 4, 0},
		{"ausencia en media jornada se acota a 4h", models.SchedulePartTime, map[string]interface{}{"work_type": "absence", "absence_hours": 6.0}, 0, 4},
		{"ausencia parcial en media jornada", models.SchedulePartTime, map[string]interface{}{"work_type": "absence", "absence_hours": 1.5}, 2.5, 1.5},
		{"recuperación en media jornada se acota a 4h", models.SchedulePartTime, map[string]interface{}{"work_type": "recover", "hours_worked": 7.0}, 4, 0},
		{"recuperación en jornada completa se acota a 8h", models.ScheduleFullTime, map[string]interface{}{"work_type": "recover", "hours_worked": 12.0}, 8, 0},
		// El cliente no decide las horas de una jornada normal.
		{"hours_worked del cliente se ignora", models.SchedulePartTime, map[string]interface{}{"work_type": "complete", "hours_worked": 8.0}, 4, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, repo := newDayService(tc.scheduleType)
			tc.req["work_date"] = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			if _, err := s.Create(dayUserID, tc.req); err != nil {
				t.Fatalf("Create: %v", err)
			}
			if repo.created.HoursWorked != tc.wantWorked || repo.created.AbsenceHours != tc.wantAbsence {
				t.Fatalf("esperaba %.2fh trabajadas y %.2fh de ausencia, got %.2f / %.2f",
					tc.wantWorked, tc.wantAbsence, repo.created.HoursWorked, repo.created.AbsenceHours)
			}
		})
	}
}

// Editar una jornada conserva la duración con la que se registró: si la
// persona pasó de completa a media jornada, sus días de 8h siguen siendo de 8h.
func TestUpdate_KeepsOriginalDayLength(t *testing.T) {
	s, repo := newDayService(models.SchedulePartTime)
	repo.stored = &models.WorkHour{
		ID: 1, UserID: dayUserID, TenantID: dayTenantID,
		WorkType: models.WorkTypeComplete, HoursWorked: 8,
	}

	_, err := s.Update(1, dayTenantID, dayUserID, "profesional", false, false, map[string]interface{}{
		"work_type": "absence", "absence_hours": 2.0,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updated.HoursWorked != 6 || repo.updated.AbsenceHours != 2 {
		t.Fatalf("la jornada de 8h debió seguir valiendo 8h (6 + 2), got %.2f + %.2f",
			repo.updated.HoursWorked, repo.updated.AbsenceHours)
	}
}

func TestDailyHoursForSchedule(t *testing.T) {
	if got := models.DailyHoursForSchedule(models.SchedulePartTime); got != 4 {
		t.Fatalf("Part-time debe ser 4h, got %v", got)
	}
	for _, st := range []string{"", models.ScheduleFullTime, "otra cosa"} {
		if got := models.DailyHoursForSchedule(st); got != 8 {
			t.Fatalf("%q debe ser jornada completa (8h), got %v", st, got)
		}
	}
}
