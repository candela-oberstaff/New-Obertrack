package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type cleanupEmpRepo struct {
	repository.EmploymentRepository
	target  models.Employment
	updated bool
}

func (f *cleanupEmpRepo) ListByUser(userID uint) ([]models.Employment, error) {
	return []models.Employment{f.target}, nil
}
func (f *cleanupEmpRepo) CountActiveByManagerInCompany(managerID, companyID uint) (int64, error) {
	return 0, nil
}
func (f *cleanupEmpRepo) CountActiveByManagerInCompanyViaLinks(managerID, companyID uint) (int64, error) {
	return 0, nil
}
func (f *cleanupEmpRepo) CountTasks(userID, tenantID uint) (int64, int64, error) { return 0, 0, nil }
func (f *cleanupEmpRepo) ListNotes(employmentID uint) ([]models.EmploymentNote, error) {
	return nil, nil
}
func (f *cleanupEmpRepo) ListFollowUps(userID, companyID uint, start, end time.Time) ([]models.FollowUp, error) {
	return nil, nil
}
func (f *cleanupEmpRepo) ListDocuments(employmentID uint) ([]models.EmploymentDocument, error) {
	return nil, nil
}
func (f *cleanupEmpRepo) ListContacts(userID, companyID uint, start, end time.Time) ([]models.ContactLog, error) {
	return nil, nil
}
func (f *cleanupEmpRepo) Update(_ *models.Employment, _ map[string]interface{}) error {
	f.updated = true
	return nil
}

type cleanupWHRepo struct {
	repository.WorkHourRepository
}

func (f *cleanupWHRepo) GetSummary(_ map[string]interface{}) (map[string]float64, error) {
	return map[string]float64{}, nil
}
func (f *cleanupWHRepo) ListAbsences(_, _ uint, _, _ time.Time) ([]models.WorkHour, error) {
	return nil, nil
}

type cleanupUserRepo struct {
	repository.UserRepository
	user models.User
}

func (f *cleanupUserRepo) GetByID(_ uint) (*models.User, error)                  { u := f.user; return &u, nil }
func (f *cleanupUserRepo) Update(_ *models.User, _ map[string]interface{}) error { return nil }

func TestEndEmployment_RevokesChannelMembership(t *testing.T) {
	const empID, userID, companyID = uint(50), uint(7), uint(20)
	target := models.Employment{UserID: userID, CompanyID: companyID, Status: models.EmploymentActive, StartedAt: time.Now().Add(-30 * 24 * time.Hour)}
	target.ID = empID

	empRepo := &cleanupEmpRepo{target: target}
	s := &employmentService{
		repo:         empRepo,
		userRepo:     &cleanupUserRepo{user: models.User{}},
		workHourRepo: &cleanupWHRepo{},
	}

	var gotUser, gotCompany uint
	called := false
	s.SetChannelCleaner(func(u, c uint) error {
		called = true
		gotUser, gotCompany = u, c
		return nil
	})

	if err := s.EndEmployment(userID, empID, "fin de contrato"); err != nil {
		t.Fatalf("EndEmployment falló: %v", err)
	}
	if !empRepo.updated {
		t.Fatal("la baja del empleo debió persistirse (Update no se llamó)")
	}
	if !called {
		t.Fatal("al dar de baja se debe revocar la membresía de canales (cleaner no invocado)")
	}
	if gotUser != userID || gotCompany != companyID {
		t.Fatalf("cleaner invocado con (user=%d, company=%d); se esperaba (%d, %d)", gotUser, gotCompany, userID, companyID)
	}
}

func TestEndEmployment_NilChannelCleanerIsSafe(t *testing.T) {
	const empID, userID, companyID = uint(51), uint(8), uint(21)
	target := models.Employment{UserID: userID, CompanyID: companyID, Status: models.EmploymentActive, StartedAt: time.Now().Add(-10 * 24 * time.Hour)}
	target.ID = empID

	s := &employmentService{
		repo:         &cleanupEmpRepo{target: target},
		userRepo:     &cleanupUserRepo{user: models.User{}},
		workHourRepo: &cleanupWHRepo{},
	}

	if err := s.EndEmployment(userID, empID, ""); err != nil {
		t.Fatalf("EndEmployment con cleaner nil debió tener éxito: %v", err)
	}
}
