package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// El reclutador de Obersuite en la ficha de la empresa. Lo que se fija: el
// contrato del PUT (qué es 400, con qué texto), que reemplazar es reemplazar y
// corregir es corregir, y que el analista viaja al padrón como contacto o como
// nulo, nunca como un objeto vacío.

type fakeRecruiterRepo struct {
	fakeNotesAdminRepo
	byCompany map[uint]*models.CompanyRecruiter
	setErr    error
}

func (f *fakeRecruiterRepo) GetCompanyRecruiter(companyID uint) (*models.CompanyRecruiter, error) {
	return f.byCompany[companyID], nil
}

func (f *fakeRecruiterRepo) SetCompanyRecruiter(cr *models.CompanyRecruiter) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.byCompany == nil {
		f.byCompany = map[uint]*models.CompanyRecruiter{}
	}
	// Misma regla que el SQL: misma persona conserva assigned_at.
	now := time.Now()
	if prev, ok := f.byCompany[cr.CompanyID]; ok && prev.ExternalID == cr.ExternalID {
		cr.AssignedAt = prev.AssignedAt
	} else {
		cr.AssignedAt = now
	}
	cr.UpdatedAt = now
	f.byCompany[cr.CompanyID] = cr
	return nil
}

func (f *fakeRecruiterRepo) ClearCompanyRecruiter(companyID uint) (int64, error) {
	if _, ok := f.byCompany[companyID]; !ok {
		return 0, nil
	}
	delete(f.byCompany, companyID)
	return 1, nil
}

func newRecruiterSvc() (*adminService, *fakeRecruiterRepo) {
	repo := &fakeRecruiterRepo{}
	users := &fakeNotesUserRepo{users: map[uint]*models.User{36: company(36)}}
	return &adminService{repo: repo, userRepo: users}, repo
}

func lorena() RecruiterInput {
	return RecruiterInput{ExternalID: "obersuite-user-42", Name: "Lorena Moujalli", Email: "Lorena@Oberstaff.com"}
}

func TestReclutador_SeAsignaYSeLee(t *testing.T) {
	svc, _ := newRecruiterSvc()

	cr, err := svc.SetTenantRecruiter(36, lorena())

	if err != nil || cr == nil {
		t.Fatalf("%v %v", cr, err)
	}
	if cr.ExternalID != "obersuite-user-42" || cr.Name != "Lorena Moujalli" {
		t.Errorf("guardado mal: %+v", cr)
	}
	// El correo se normaliza a minúsculas: es una llave para ellos, no un texto.
	if cr.Email != "lorena@oberstaff.com" {
		t.Errorf("email %q", cr.Email)
	}
	got, _ := svc.GetTenantRecruiter(36)
	if got == nil || got.ExternalID != cr.ExternalID {
		t.Fatal("la lectura no devuelve lo asignado")
	}
}

// Corregir el nombre de la MISMA persona no es reasignar: assigned_at se queda.
// Poner a OTRA persona sí lo es.
func TestReclutador_CorregirNoEsReasignar(t *testing.T) {
	svc, _ := newRecruiterSvc()
	primero, _ := svc.SetTenantRecruiter(36, lorena())
	time.Sleep(2 * time.Millisecond)

	corregida := lorena()
	corregida.Name = "Lorena Moujalli Pérez"
	segundo, _ := svc.SetTenantRecruiter(36, corregida)
	if !segundo.AssignedAt.Equal(primero.AssignedAt) {
		t.Fatal("corregir el nombre movió assigned_at: parecería una reasignación")
	}
	if segundo.Name != "Lorena Moujalli Pérez" {
		t.Fatal("y el nombre sí tiene que cambiar")
	}

	otra := RecruiterInput{ExternalID: "obersuite-user-99", Name: "Otra Persona"}
	tercero, _ := svc.SetTenantRecruiter(36, otra)
	if tercero.AssignedAt.Equal(primero.AssignedAt) {
		t.Fatal("otra persona ES una reasignación: assigned_at tiene que moverse")
	}
}

func TestReclutador_Los400DelContrato(t *testing.T) {
	casos := []struct {
		nombre string
		muta   func(*RecruiterInput)
		frase  string
	}{
		{"sin external_id", func(i *RecruiterInput) { i.ExternalID = "  " }, "external_id"},
		{"sin name", func(i *RecruiterInput) { i.Name = "" }, "name"},
		{"email sin forma de correo", func(i *RecruiterInput) { i.Email = "lorena" }, "email"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			svc, repo := newRecruiterSvc()
			in := lorena()
			c.muta(&in)

			_, err := svc.SetTenantRecruiter(36, in)

			if !errors.Is(err, apperrors.ErrInvalidInput) {
				t.Fatalf("debía ser 400, got %v", err)
			}
			if !strings.Contains(err.Error(), c.frase) {
				t.Errorf("mensaje %q debía mencionar %q", err.Error(), c.frase)
			}
			if len(repo.byCompany) != 0 {
				t.Error("un 400 no escribe nada")
			}
		})
	}
}

// El correo es opcional: un reclutador sin correo es un reclutador igual.
func TestReclutador_ElCorreoEsOpcional(t *testing.T) {
	svc, _ := newRecruiterSvc()
	in := lorena()
	in.Email = ""
	if _, err := svc.SetTenantRecruiter(36, in); err != nil {
		t.Fatalf("sin correo debía aceptarse: %v", err)
	}
}

func TestReclutador_QuitarYElSegundoEs404(t *testing.T) {
	svc, _ := newRecruiterSvc()
	svc.SetTenantRecruiter(36, lorena())

	if err := svc.ClearTenantRecruiter(36); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetTenantRecruiter(36); got != nil {
		t.Fatal("tras quitarlo no debía quedar nadie")
	}
	if err := svc.ClearTenantRecruiter(36); !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("el segundo DELETE es 404, got %v", err)
	}
}

func TestReclutador_EmpresaInexistenteEs404(t *testing.T) {
	svc, _ := newRecruiterSvc()
	if _, err := svc.SetTenantRecruiter(77, lorena()); !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

// El analista en el padrón: contacto o NULO. Nunca {"id":"0","name":""}, que
// parecería un analista sin nombre y no "no tiene".
func TestPadron_ElAnalistaEsContactoONulo(t *testing.T) {
	if c := customerSuccessContact(0, "", ""); c != nil {
		t.Fatalf("sin analista tiene que ser nulo, got %+v", c)
	}
	c := customerSuccessContact(12, "Yeilyn Crespo", "yeilyn@oberstaff.com")
	if c == nil || c.ID != "12" || c.Name != "Yeilyn Crespo" || c.Email != "yeilyn@oberstaff.com" {
		t.Fatalf("contacto mal armado: %+v", c)
	}
}
