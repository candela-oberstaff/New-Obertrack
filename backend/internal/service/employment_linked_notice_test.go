package service

import (
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type linkedUserRepo struct {
	repository.UserRepository
	users map[uint]models.User
}

func (f *linkedUserRepo) GetByID(id uint) (*models.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, errFakeNotFound
	}
	return &u, nil
}

type linkedNotifier struct {
	NotificationService
	userID         uint
	title, message string
}

func (f *linkedNotifier) CreateNotification(userID uint, kind, title, message string, data map[string]interface{}) error {
	f.userID, f.title, f.message = userID, title, message
	return nil
}

type linkedMailer struct {
	kind, to, subject, html string
}

func (f *linkedMailer) SendEmailKind(kind, toEmail, toName, subject, htmlContent string) error {
	f.kind, f.to, f.subject, f.html = kind, toEmail, subject, htmlContent
	return nil
}

// Al vincularlo a otra empresa, el profesional se entera: notificación y
// correo con la empresa, el cargo y su manager. Antes solo se le cerraba la
// sesión y aparecía en otra empresa sin explicación.
func TestNotifyLinked_AvisaAlProfesional(t *testing.T) {
	manager := uint(30)
	emp := models.Employment{UserID: 7, CompanyID: 20, JobTitle: "Diseñador Digital", ManagerID: &manager}
	emp.ID = 50
	notifier := &linkedNotifier{}
	mailer := &linkedMailer{}
	s := &employmentService{
		repo: &restoreEmpRepo{emp: emp},
		userRepo: &linkedUserRepo{users: map[uint]models.User{
			7:  {ID: 7, Name: "Adriana", Email: "adriana@example.test"},
			20: {ID: 20, Name: "Dueña", CompanyName: "Chess Advertising"},
			30: {ID: 30, Name: "Laura Pérez"},
		}},
		notifSvc: notifier,
	}
	s.SetMailer(mailer)

	s.NotifyLinked(50)

	if notifier.userID != 7 || !strings.Contains(notifier.title, "Chess Advertising") {
		t.Fatalf("notificación mal dirigida o sin empresa: user=%d título=%q", notifier.userID, notifier.title)
	}
	if !strings.Contains(notifier.message, "Diseñador Digital") || !strings.Contains(notifier.message, "Laura Pérez") {
		t.Fatalf("la notificación debe decir cargo y manager: %q", notifier.message)
	}
	if mailer.kind != EmailKindEmploymentLinked || mailer.to != "adriana@example.test" {
		t.Fatalf("correo con tipo o destinatario incorrecto: kind=%q to=%q", mailer.kind, mailer.to)
	}
	if !strings.Contains(mailer.html, "Chess Advertising") || !strings.Contains(mailer.subject, "Chess Advertising") {
		t.Fatal("el correo debe nombrar la empresa nueva")
	}
}
