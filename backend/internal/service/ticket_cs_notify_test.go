package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type csUserRepo struct {
	repository.UserRepository
	byID    map[uint]*models.User
	byPhone map[string]*models.User
	byEmail map[string]*models.User
}

func (f *csUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (f *csUserRepo) FindActiveByPhoneDigits(digits string) (*models.User, error) {
	for suffix, u := range f.byPhone {
		if strings.HasSuffix(digits, suffix) {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *csUserRepo) GetByEmail(email string) (*models.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

type csNotif struct {
	NotificationService
	to    []uint
	title []string
	data  []map[string]interface{}
	msg   []string
}

func (f *csNotif) CreateNotification(userID uint, _ string, title, message string, data map[string]interface{}) error {
	f.to = append(f.to, userID)
	f.title = append(f.title, title)
	f.msg = append(f.msg, message)
	f.data = append(f.data, data)
	return nil
}

// Una empresa (10) con su CS (7) y un profesional suyo (20).
func csFixture() (*ticketService, *csNotif) {
	cs := &models.User{ID: 7, Name: "Yeilyn", UserType: models.UserTypeCustomerSuccess, IsActive: true}
	company := &models.User{ID: 10, Name: "Hermann", CompanyName: "360BIM", UserType: models.UserTypeEmployer, AssignedCSID: uptr(7)}
	pro := &models.User{ID: 20, Name: "Marta Solís", UserType: models.UserTypeProfessional, EmpleadorID: uptr(10)}
	repo := &csUserRepo{
		byID:    map[uint]*models.User{7: cs, 10: company, 20: pro},
		byPhone: map[string]*models.User{"4125550000": pro, "34670306848": company},
		byEmail: map[string]*models.User{"marta@x.com": pro},
	}
	notif := &csNotif{}
	return &ticketService{userRepo: repo, notifSvc: notif}, notif
}

func TestNotifyCompanyCS_AlertaDeUnProfesional(t *testing.T) {
	svc, notif := csFixture()
	svc.notifyCompanyCS(&models.Ticket{ID: 5, Origin: models.OriginInternal, Title: "Inducción no aprobada", UserID: uptr(20)}, "", "")
	if len(notif.to) != 1 || notif.to[0] != 7 {
		t.Fatalf("debe avisar al CS 7, got %v", notif.to)
	}
	if notif.title[0] != "Nuevo ticket de tu cliente 360BIM" || !strings.Contains(notif.msg[0], "Marta Solís") {
		t.Fatalf("el aviso debe nombrar la empresa y al profesional: %q / %q", notif.title[0], notif.msg[0])
	}
	if notif.data[0]["link"] != "/tickets/internal/5" {
		t.Fatalf("enlace al detalle interno, got %v", notif.data[0]["link"])
	}
}

func TestNotifyCompanyCS_WhatsAppDeLaEmpresaPorTelefono(t *testing.T) {
	svc, notif := csFixture()
	svc.notifyCompanyCS(&models.Ticket{ID: 64, Origin: "whatsapp", Title: "WA: 34670306848"}, "+34 670 306 848", "")
	if len(notif.to) != 1 || notif.to[0] != 7 || notif.data[0]["link"] != "/tickets/wa/64" {
		t.Fatalf("un WhatsApp de la empresa debe avisar a su CS con enlace al chat: %v %v", notif.to, notif.data)
	}
}

func TestNotifyCompanyCS_CorreoDeUnProfesional(t *testing.T) {
	svc, notif := csFixture()
	svc.notifyCompanyCS(&models.Ticket{ID: 9, Origin: "email", Title: "Duda"}, "", "marta@x.com")
	if len(notif.to) != 1 || notif.to[0] != 7 {
		t.Fatalf("un correo de un profesional debe avisar al CS de su empresa: %v", notif.to)
	}
}

func TestNotifyCompanyCS_SinAvisoCuandoNoCorresponde(t *testing.T) {
	svc, notif := csFixture()
	// Desconocido: no hay empresa.
	svc.notifyCompanyCS(&models.Ticket{ID: 1, Origin: "whatsapp", Title: "WA"}, "+1 999 000 1111", "")
	// Ya asignado a su CS: ya lo sabe.
	svc.notifyCompanyCS(&models.Ticket{ID: 2, Origin: models.OriginInternal, Title: "X", UserID: uptr(20), AssignedTo: uptr(7)}, "", "")
	// Empresa sin CS.
	svc.userRepo.(*csUserRepo).byID[10].AssignedCSID = nil
	svc.notifyCompanyCS(&models.Ticket{ID: 3, Origin: models.OriginInternal, Title: "X", UserID: uptr(20)}, "", "")
	if len(notif.to) != 0 {
		t.Fatalf("no debe avisar a nadie, got %v", notif.to)
	}
}

func TestNotifyCompanyCS_CSInactivoNoRecibe(t *testing.T) {
	svc, notif := csFixture()
	svc.userRepo.(*csUserRepo).byID[7].IsActive = false
	svc.notifyCompanyCS(&models.Ticket{ID: 5, Origin: models.OriginInternal, Title: "X", UserID: uptr(20)}, "", "")
	if len(notif.to) != 0 {
		t.Fatalf("un CS inactivo no recibe avisos, got %v", notif.to)
	}
}
