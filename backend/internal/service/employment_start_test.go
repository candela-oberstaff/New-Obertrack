package service

import (
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type startEmpRepo struct {
	repository.EmploymentRepository
	target  models.Employment
	updates map[string]interface{}
}

func (f *startEmpRepo) ListByUser(userID uint) ([]models.Employment, error) {
	return []models.Employment{f.target}, nil
}

func (f *startEmpRepo) Update(_ *models.Employment, updates map[string]interface{}) error {
	f.updates = updates
	return nil
}

func newStartService(target models.Employment) (*employmentService, *startEmpRepo) {
	repo := &startEmpRepo{target: target}
	return &employmentService{repo: repo}, repo
}

func TestUpdateEmploymentStart_CorrigeLaFecha(t *testing.T) {
	const empID, userID = uint(50), uint(7)
	target := models.Employment{UserID: userID, Status: models.EmploymentActive, StartedAt: time.Now()}
	target.ID = empID
	s, repo := newStartService(target)

	real := time.Date(2023, 4, 17, 0, 0, 0, 0, time.UTC)
	if err := s.UpdateEmploymentStart(userID, empID, real); err != nil {
		t.Fatalf("UpdateEmploymentStart falló: %v", err)
	}
	got, ok := repo.updates["started_at"].(time.Time)
	if !ok {
		t.Fatalf("se esperaba started_at en el update; llegó %#v", repo.updates)
	}
	if !got.Equal(real) {
		t.Fatalf("started_at guardado = %v; se esperaba %v", got, real)
	}
}

// Se contrata a alguien hoy para que se incorpore la semana que viene: esa
// fecha tiene que poder cargarse en el momento del alta y no el día que empieza.
func TestUpdateEmploymentStart_AceptaIncorporacionFutura(t *testing.T) {
	const empID, userID = uint(51), uint(8)
	target := models.Employment{UserID: userID, Status: models.EmploymentActive, StartedAt: time.Now()}
	target.ID = empID
	s, repo := newStartService(target)

	proxima := time.Now().Add(7 * 24 * time.Hour)
	if err := s.UpdateEmploymentStart(userID, empID, proxima); err != nil {
		t.Fatalf("una incorporación futura debió aceptarse: %v", err)
	}
	got, ok := repo.updates["started_at"].(time.Time)
	if !ok || !got.Equal(proxima) {
		t.Fatalf("started_at = %v, se esperaba %v", repo.updates["started_at"], proxima)
	}
}

// El tope no está para frenar incorporaciones futuras sino el año tecleado de
// más, que nadie revisa después y descoloca el expediente entero.
func TestUpdateEmploymentStart_RechazaFuturoDisparatado(t *testing.T) {
	const empID, userID = uint(53), uint(11)
	target := models.Employment{UserID: userID, Status: models.EmploymentActive, StartedAt: time.Now()}
	target.ID = empID
	s, repo := newStartService(target)

	err := s.UpdateEmploymentStart(userID, empID, time.Now().AddDate(200, 0, 0))
	if err == nil || !strings.Contains(err.Error(), "lejos en el futuro") {
		t.Fatalf("se esperaba rechazo por fecha disparatada; llegó %v", err)
	}
	if repo.updates != nil {
		t.Fatal("no debió persistirse nada con una fecha rechazada")
	}
}

// En un empleo terminado la corrección sigue permitida (muchas salen al cerrar
// el expediente), pero no puede invertir el período.
func TestUpdateEmploymentStart_RechazaIngresoPosteriorALaBaja(t *testing.T) {
	const empID, userID = uint(52), uint(9)
	ended := time.Now().Add(-30 * 24 * time.Hour)
	target := models.Employment{UserID: userID, Status: models.EmploymentEnded, StartedAt: ended.Add(-365 * 24 * time.Hour), EndedAt: &ended}
	target.ID = empID
	s, repo := newStartService(target)

	err := s.UpdateEmploymentStart(userID, empID, ended.Add(24*time.Hour))
	if err == nil || !strings.Contains(err.Error(), "posterior a la baja") {
		t.Fatalf("se esperaba rechazo por ingreso posterior a la baja; llegó %v", err)
	}
	if repo.updates != nil {
		t.Fatal("no debió persistirse nada con una fecha rechazada")
	}

	// La misma corrección dentro del período sí entra.
	if err := s.UpdateEmploymentStart(userID, empID, ended.Add(-400*24*time.Hour)); err != nil {
		t.Fatalf("corregir el ingreso de un empleo terminado debió funcionar: %v", err)
	}
}

func TestUpdateEmploymentStart_EmpleoAjeno(t *testing.T) {
	target := models.Employment{UserID: 10, Status: models.EmploymentActive, StartedAt: time.Now()}
	target.ID = 60
	s, repo := newStartService(target)

	err := s.UpdateEmploymentStart(10, 61, time.Now().Add(-24*time.Hour))
	if err == nil || err.Error() != "Membresía no encontrada" {
		t.Fatalf("se esperaba 'Membresía no encontrada'; llegó %v", err)
	}
	if repo.updates != nil {
		t.Fatal("no debió tocarse un empleo que no es del usuario")
	}
}
