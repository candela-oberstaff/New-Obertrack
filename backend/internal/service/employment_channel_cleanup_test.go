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
	s.SetChannelCleaner(func(u, c uint) ([]uint, error) {
		called = true
		gotUser, gotCompany = u, c
		return []uint{3, 9}, nil
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

type restoreEmpRepo struct {
	cleanupEmpRepo
	emp models.Employment
}

func (f *restoreEmpRepo) GetByID(_ uint) (*models.Employment, error) { e := f.emp; return &e, nil }

// Update imita a GORM: Updates sobre el struct le copia los valores nuevos.
// Sin esto el test no veía que reactivar vaciaba la lista antes de leerla.
func (f *restoreEmpRepo) Update(e *models.Employment, updates map[string]interface{}) error {
	if v, ok := updates["removed_channels"].(string); ok {
		e.RemovedChannels = v
	}
	return nil
}
func (f *restoreEmpRepo) GetActive(_, _ uint) (*models.Employment, error) {
	return nil, nil
}

// Reactivar devuelve a la persona a los canales de los que salió con la baja.
// Antes no: quedaba fuera de sus chats de esa empresa.
func TestReactivateEmployment_RestoresChannels(t *testing.T) {
	emp := models.Employment{UserID: 7, CompanyID: 20, Status: models.EmploymentEnded, RemovedChannels: "[3,9]"}
	emp.ID = 50
	s := &employmentService{
		repo:     &restoreEmpRepo{emp: emp},
		userRepo: &cleanupUserRepo{user: models.User{}},
	}
	var gotUser, gotCompany uint
	var gotChannels []uint
	s.SetChannelRestorer(func(u, c uint, ids []uint) error {
		gotUser, gotCompany, gotChannels = u, c, ids
		return nil
	})

	if err := s.ReactivateEmployment(50); err != nil {
		t.Fatalf("reactivar falló: %v", err)
	}
	if gotUser != 7 || gotCompany != 20 || len(gotChannels) != 2 || gotChannels[0] != 3 || gotChannels[1] != 9 {
		t.Fatalf("restorer con (user=%d, company=%d, canales=%v); esperaba (7, 20, [3 9])", gotUser, gotCompany, gotChannels)
	}
}
