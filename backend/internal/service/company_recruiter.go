package service

import (
	"strings"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// El reclutador de Obersuite que lleva una empresa: la segunda escritura de la
// integración, y la primera que toca a la empresa en vez de a su expediente.
//
// Es el espejo de assigned_cs_id. Nuestro analista lo asignamos aquí y
// Obersuite lo lee; su reclutador lo asignan allí y nosotros lo leemos. Ninguno
// de los dos escribe el del otro: cada sistema es dueño de su persona.

// RecruiterInput es lo que manda Obersuite en el PUT.
type RecruiterInput struct {
	ExternalID string
	Name       string
	Email      string
}

// SetTenantRecruiter asigna o reemplaza al reclutador de la empresa. Es un PUT:
// el mismo cuerpo dos veces deja el mismo estado, y mandar otra persona la
// sustituye. Para quitarlo está ClearTenantRecruiter, no un PUT vacío.
func (s *adminService) SetTenantRecruiter(companyID uint, in RecruiterInput) (*models.CompanyRecruiter, error) {
	if err := s.assertEmployer(companyID); err != nil {
		return nil, hireFail(apperrors.ErrNotFound, "la empresa no existe o no es una empresa")
	}
	externalID := strings.TrimSpace(in.ExternalID)
	if externalID == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id es obligatorio: es lo que identifica al reclutador en Obersuite")
	}
	if len(externalID) > 120 {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id no puede superar los 120 caracteres")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "name es obligatorio: es lo que se enseña en la ficha de la empresa")
	}
	if r := []rune(name); len(r) > 255 {
		name = string(r[:255])
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email != "" && !strings.Contains(email, "@") {
		return nil, hireFail(apperrors.ErrInvalidInput, "email no tiene forma de correo")
	}

	cr := &models.CompanyRecruiter{CompanyID: companyID, ExternalID: externalID, Name: name, Email: email}
	if err := s.repo.SetCompanyRecruiter(cr); err != nil {
		return nil, err
	}
	// Se relee para devolver assigned_at real: si era la misma persona con el
	// nombre corregido, la fecha es la original, no ahora.
	return s.repo.GetCompanyRecruiter(companyID)
}

// ClearTenantRecruiter quita al reclutador. 404 si no había: el segundo DELETE
// no es un error de nadie, pero tampoco se finge que borró algo.
func (s *adminService) ClearTenantRecruiter(companyID uint) error {
	if err := s.assertEmployer(companyID); err != nil {
		return hireFail(apperrors.ErrNotFound, "la empresa no existe o no es una empresa")
	}
	rows, err := s.repo.ClearCompanyRecruiter(companyID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return hireFail(apperrors.ErrNotFound, "la empresa no tiene reclutador asignado")
	}
	return nil
}

// GetTenantRecruiter devuelve el reclutador, o nil si no hay.
func (s *adminService) GetTenantRecruiter(companyID uint) (*models.CompanyRecruiter, error) {
	return s.repo.GetCompanyRecruiter(companyID)
}
