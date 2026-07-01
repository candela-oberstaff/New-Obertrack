package service

import (
	"errors"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type pcUserRepo struct {
	repository.UserRepository
	users       map[uint]*models.User
	lastUpdates map[string]interface{}
}

func (f *pcUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}
func (f *pcUserRepo) Update(user *models.User, updates map[string]interface{}) error {
	f.lastUpdates = updates
	return nil
}

type pcReqRepo struct {
	repository.ProfileChangeRequestRepository
	req         *models.ProfileChangeRequest
	lastUpdates map[string]interface{}
}

func (f *pcReqRepo) GetByID(_ uint) (*models.ProfileChangeRequest, error) { return f.req, nil }
func (f *pcReqRepo) Update(_ *models.ProfileChangeRequest, updates map[string]interface{}) error {
	f.lastUpdates = updates
	return nil
}

func TestUserUpdate_ProfessionalSelfEditLocksFields(t *testing.T) {
	prof := &models.User{ID: 5, UserType: models.UserTypeProfessional}
	repo := &pcUserRepo{users: map[uint]*models.User{5: prof}}
	svc := NewUserService(repo, nil)

	updates := map[string]interface{}{
		"name":      "Nombre Falso",
		"job_title": "CEO",
		"avatar":    "/api/uploads/x.png",
	}
	if _, err := svc.Update(5, 5, 0, string(models.UserTypeProfessional), false, false, updates); err != nil {
		t.Fatalf("Update falló: %v", err)
	}
	if _, ok := repo.lastUpdates["name"]; ok {
		t.Fatal("un profesional no debe poder auto-editar 'name'")
	}
	if _, ok := repo.lastUpdates["job_title"]; ok {
		t.Fatal("un profesional no debe poder auto-editar 'job_title'")
	}
	if repo.lastUpdates["avatar"] != "/api/uploads/x.png" {
		t.Fatalf("el avatar sí debe poder cambiarse; updates=%+v", repo.lastUpdates)
	}
}

func TestProfileChangeApply_WritesUserAndMarksApplied(t *testing.T) {
	target := &models.User{ID: 7, UserType: models.UserTypeProfessional, PhoneNumber: "old"}
	actor := &models.User{ID: 2, UserType: models.UserTypeCustomerSuccess}
	userRepo := &pcUserRepo{users: map[uint]*models.User{7: target, 2: actor}}

	req := &models.ProfileChangeRequest{
		ID: 10, UserID: 7,
		Changes: `{"phone_number":"+58999"}`,
		Status:  models.ProfileChangePending,
	}
	reqRepo := &pcReqRepo{req: req}
	svc := NewProfileChangeService(reqRepo, userRepo, nil, nil, nil)

	if err := svc.Apply(10, 2, map[string]string{"phone_number": "+58999"}); err != nil {
		t.Fatalf("Apply falló: %v", err)
	}
	if userRepo.lastUpdates["phone_number"] != "+58999" {
		t.Fatalf("no se aplicó phone_number al usuario; updates=%+v", userRepo.lastUpdates)
	}
	if reqRepo.lastUpdates["status"] != models.ProfileChangeApplied {
		t.Fatalf("la solicitud no quedó 'applied'; updates=%+v", reqRepo.lastUpdates)
	}
}

func TestProfileChangeApply_RejectsNonReviewer(t *testing.T) {
	target := &models.User{ID: 7, UserType: models.UserTypeProfessional}
	intruder := &models.User{ID: 3, UserType: models.UserTypeProfessional}
	userRepo := &pcUserRepo{users: map[uint]*models.User{7: target, 3: intruder}}
	req := &models.ProfileChangeRequest{ID: 10, UserID: 7, Changes: `{"city":"X"}`, Status: models.ProfileChangePending}
	svc := NewProfileChangeService(&pcReqRepo{req: req}, userRepo, nil, nil, nil)

	if err := svc.Apply(10, 3, nil); err == nil {
		t.Fatal("un profesional no debe poder aplicar solicitudes de cambio")
	}
}
