package service

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// Empresas escritas DESDE Obersuite: crear, editar la información básica,
// asignar el analista y suspender/reactivar. Es lo que nuestra ficha hace con
// una empresa, expuesto por el bridge.
//
// Lo que se decidió y por qué, para no volver a discutirlo:
//
//   - Crear NO envía ningún correo ni credenciales. La cuenta nace activa con
//     una contraseña inservible, y el acceso se entrega después, a mano, desde
//     "Enviar acceso por correo" en Obertrack. Es la misma regla que el equipo
//     fijó para el import: las credenciales no salen solas.
//   - responsible_email es el correo con el que ENTRA el cliente. Se fija al
//     crear y no se edita por aquí: cambiar el login de un cliente desde otro
//     sistema es la clase de cosa que tiene que hacer una persona mirando la
//     ficha. En Obertrack sí se puede.
//   - Suspender expulsa en el acto a la empresa y a todos sus profesionales
//     (revoca sesiones) y no avisa a nadie por correo. Es lo mismo que hace el
//     botón de nuestra ficha; solo cambia quién lo pulsa.
//   - Borrar no se expone. Suspender cubre el caso; borrar en firme se queda
//     en la Papelera de Obertrack.
//
// La idempotencia de crear va por obersuite_id, la misma columna que usa /hire
// para los profesionales: la empresa ES un usuario, y el índice único parcial
// ya existe. Sin migración.

type ObersuiteCompanyCreate struct {
	ExternalID        string
	Name              string
	ResponsibleName   string
	ResponsibleEmail  string
	PhoneNumber       string
	Industry          string
	Country           string
	State             string
	City              string
	Address           string
	CustomerSuccessID *uint
}

type ObersuiteCompanyCreateResult struct {
	ID     uint   `json:"id"`
	Status string `json:"status"` // "created" | "already_exists"
}

// ObersuiteCompanyPatch es el PUT parcial: lo que no viaja (nil) no se toca.
type ObersuiteCompanyPatch struct {
	Name            *string
	ResponsibleName *string
	PhoneNumber     *string
	Industry        *string
	Country         *string
	State           *string
	City            *string
	Address         *string
}

// ObersuiteAnalyst es un analista que se puede asignar.
type ObersuiteAnalyst struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func industryError(v string) error {
	return hireFail(apperrors.ErrInvalidInput,
		fmt.Sprintf("industry %q no está en la lista: consulta GET /industries y manda uno de esos valores tal cual", v))
}

// CreateCompanyFromObersuite crea la cuenta de la empresa. Idempotente por
// external_id. Un correo ya ocupado es 409 con quién lo ocupa.
func (s *adminService) CreateCompanyFromObersuite(in ObersuiteCompanyCreate) (*ObersuiteCompanyCreateResult, error) {
	externalID := strings.TrimSpace(in.ExternalID)
	if externalID == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id es obligatorio: es lo que evita duplicar la empresa al reintentar")
	}
	if len(externalID) > 120 {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id no puede superar los 120 caracteres")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "name es obligatorio")
	}
	responsible := strings.TrimSpace(in.ResponsibleName)
	if responsible == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "responsible_name es obligatorio: es quien contrata")
	}
	email := strings.ToLower(strings.TrimSpace(in.ResponsibleEmail))
	if email == "" || !strings.Contains(email, "@") {
		return nil, hireFail(apperrors.ErrInvalidInput, "responsible_email es obligatorio y tiene que tener forma de correo: es con el que la empresa entrará en Obertrack")
	}
	industry := strings.TrimSpace(in.Industry)
	if !models.IsValidIndustry(industry) {
		return nil, industryError(industry)
	}
	if in.CustomerSuccessID != nil && *in.CustomerSuccessID != 0 {
		if err := s.assertAnalyst(*in.CustomerSuccessID); err != nil {
			return nil, err
		}
	}

	// El reintento ANTES de escribir nada.
	if existing, err := s.userRepo.GetByObersuiteID(externalID); err == nil && existing != nil {
		if existing.UserType != models.UserTypeEmployer {
			return nil, hireFail(apperrors.ErrConflict,
				fmt.Sprintf("No se puede crear: el external_id %s ya identifica a una persona, no a una empresa", externalID))
		}
		return &ObersuiteCompanyCreateResult{ID: existing.ID, Status: "already_exists"}, nil
	}

	// Contraseña inservible: nadie entra con ella. El acceso se entrega desde
	// Obertrack cuando toque.
	hashed, err := bcrypt.GenerateFromPassword([]byte(generateRandomPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Name:        responsible,
		Email:       email,
		Password:    string(hashed),
		UserType:    models.UserTypeEmployer,
		CompanyName: name,
		PhoneNumber: strings.TrimSpace(in.PhoneNumber),
		Industry:    industry,
		Country:     strings.TrimSpace(in.Country),
		State:       strings.TrimSpace(in.State),
		City:        strings.TrimSpace(in.City),
		Address:     strings.TrimSpace(in.Address),
		IsActive:    true,
		ObersuiteID: externalID,
	}
	if in.CustomerSuccessID != nil && *in.CustomerSuccessID != 0 {
		id := *in.CustomerSuccessID
		user.AssignedCSID = &id
	}
	if err := s.userRepo.Create(user); err != nil {
		if isDuplicateEmailErr(err) {
			return nil, hireFail(apperrors.ErrEmailTaken, "No se puede crear: "+describeEmailConflict(s.userRepo, email).Error())
		}
		if isUniqueViolation(err) {
			// Dos POST a la vez con el mismo external_id: uno gana el índice.
			if again, ferr := s.userRepo.GetByObersuiteID(externalID); ferr == nil && again != nil {
				return &ObersuiteCompanyCreateResult{ID: again.ID, Status: "already_exists"}, nil
			}
		}
		return nil, err
	}
	return &ObersuiteCompanyCreateResult{ID: user.ID, Status: "created"}, nil
}

// UpdateCompanyFromObersuite aplica un PUT parcial y devuelve qué campos
// cambiaron de verdad (un valor igual al que había no cuenta).
func (s *adminService) UpdateCompanyFromObersuite(companyID uint, p ObersuiteCompanyPatch) ([]string, error) {
	company, err := s.userRepo.GetByID(companyID)
	if err != nil || company == nil || company.UserType != models.UserTypeEmployer {
		return nil, hireFail(apperrors.ErrNotFound, "la empresa no existe o no es una empresa")
	}

	updates := map[string]interface{}{}
	changed := []string{}
	set := func(field, current string, incoming *string, required bool) error {
		if incoming == nil {
			return nil
		}
		v := strings.TrimSpace(*incoming)
		if required && v == "" {
			return hireFail(apperrors.ErrInvalidInput, field+" no puede quedar vacío")
		}
		if v != current {
			updates[field] = v
			changed = append(changed, field)
		}
		return nil
	}
	// El campo JSON y la columna no siempre se llaman igual: se registra el
	// nombre que Obersuite mandó, que es el que va a reconocer en la respuesta.
	if p.Name != nil {
		v := strings.TrimSpace(*p.Name)
		if v == "" {
			return nil, hireFail(apperrors.ErrInvalidInput, "name no puede quedar vacío")
		}
		if v != company.CompanyName {
			updates["company_name"] = v
			changed = append(changed, "name")
		}
	}
	if p.ResponsibleName != nil {
		v := strings.TrimSpace(*p.ResponsibleName)
		if v == "" {
			return nil, hireFail(apperrors.ErrInvalidInput, "responsible_name no puede quedar vacío")
		}
		if v != company.Name {
			updates["name"] = v
			changed = append(changed, "responsible_name")
		}
	}
	if p.Industry != nil {
		v := strings.TrimSpace(*p.Industry)
		if !models.IsValidIndustry(v) {
			return nil, industryError(v)
		}
		if v != company.Industry {
			updates["industry"] = v
			changed = append(changed, "industry")
		}
	}
	if err := set("phone_number", company.PhoneNumber, p.PhoneNumber, false); err != nil {
		return nil, err
	}
	if err := set("country", company.Country, p.Country, false); err != nil {
		return nil, err
	}
	if err := set("state", company.State, p.State, false); err != nil {
		return nil, err
	}
	if err := set("city", company.City, p.City, false); err != nil {
		return nil, err
	}
	if err := set("address", company.Address, p.Address, false); err != nil {
		return nil, err
	}

	if len(updates) == 0 {
		return []string{}, nil
	}
	if err := s.userRepo.Update(company, updates); err != nil {
		return nil, err
	}
	return changed, nil
}

// ListAssignableAnalysts devuelve a quién se puede asignar como analista: los
// mismos que ofrece el desplegable de nuestra ficha (Customer Success y
// superadmins activos), sin las cuentas de sistema.
func (s *adminService) ListAssignableAnalysts() ([]ObersuiteAnalyst, error) {
	users, err := s.userRepo.ListActiveByTypes([]models.UserType{models.UserTypeCustomerSuccess, models.UserTypeSuperadmin})
	if err != nil {
		return nil, err
	}
	out := make([]ObersuiteAnalyst, 0, len(users))
	for _, u := range users {
		if u.IsSystem {
			continue
		}
		out = append(out, ObersuiteAnalyst{ID: u.ID, Name: u.Name, Email: u.Email})
	}
	return out, nil
}

// assertAnalyst comprueba que el id sea alguien asignable. Nuestra pantalla no
// lo comprueba (el desplegable ya acota), pero por el bridge llega un número.
func (s *adminService) assertAnalyst(id uint) error {
	u, err := s.userRepo.GetByID(id)
	if err != nil || u == nil {
		return hireFail(apperrors.ErrInvalidInput, fmt.Sprintf("customer_success_id %d no existe", id))
	}
	if u.IsSystem || !u.IsActive || (u.UserType != models.UserTypeCustomerSuccess && u.UserType != models.UserTypeSuperadmin) {
		return hireFail(apperrors.ErrInvalidInput,
			fmt.Sprintf("customer_success_id %d no es un analista asignable: consulta GET /analysts", id))
	}
	return nil
}

// SetCompanyAnalystFromObersuite asigna (o quita, con nil/0) el analista.
func (s *adminService) SetCompanyAnalystFromObersuite(companyID uint, analystID *uint) error {
	if err := s.assertEmployer(companyID); err != nil {
		return hireFail(apperrors.ErrNotFound, "la empresa no existe o no es una empresa")
	}
	var id uint
	if analystID != nil {
		id = *analystID
	}
	if id != 0 {
		if err := s.assertAnalyst(id); err != nil {
			return err
		}
	}
	return s.repo.AssignCSToTenant(companyID, id)
}

// SetCompanyStatusFromObersuite suspende o reactiva. Idempotente: suspender lo
// suspendido no expulsa dos veces ni deja dos hitos. El hito del Expediente lo
// firma la cuenta de servicio, así que se lee "· Obersuite", y lleva el motivo.
func (s *adminService) SetCompanyStatusFromObersuite(companyID uint, active bool, reason string) error {
	company, err := s.userRepo.GetByID(companyID)
	if err != nil || company == nil || company.UserType != models.UserTypeEmployer {
		return hireFail(apperrors.ErrNotFound, "la empresa no existe o no es una empresa")
	}
	if company.IsActive == active {
		return nil
	}
	account, err := s.userRepo.GetByEmail(models.ObersuiteServiceEmail)
	if err != nil || account == nil {
		return fmt.Errorf("no existe la cuenta de servicio de Obersuite: %w", err)
	}
	_, err = s.setTenantStatus(companyID, active, account.ID, strings.TrimSpace(reason))
	return err
}
