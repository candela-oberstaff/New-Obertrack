package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Empresas escritas desde Obersuite. Lo que se fija es el contrato: qué es
// 400/409 y con qué texto, que crear no manda correo ni credenciales, que el
// PUT es parcial y dice qué cambió, que solo se asigna a un analista de verdad,
// y que suspender expulsa igual que nuestro botón.

type fakeCompanyUserRepo struct {
	repository.UserRepository
	byID        map[uint]*models.User
	byObersuite map[string]*models.User
	byEmail     map[string]*models.User
	created     *models.User
	updates     map[string]interface{}
	revokedFor  []uint
	createErr   error
	analistas   []models.User
}

func (f *fakeCompanyUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeCompanyUserRepo) GetByObersuiteID(id string) (*models.User, error) {
	if u, ok := f.byObersuite[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeCompanyUserRepo) GetByEmail(email string) (*models.User, error) {
	if email == models.ObersuiteServiceEmail {
		return &models.User{ID: 999, Email: email, Name: models.ObersuiteServiceName, IsSystem: true}, nil
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeCompanyUserRepo) FindAnyByEmail(email string) (*models.User, error) {
	return f.GetByEmail(email)
}
func (f *fakeCompanyUserRepo) Create(u *models.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	if _, taken := f.byEmail[u.Email]; taken {
		return errors.New(`ERROR: duplicate key value violates unique constraint "idx_users_email" (SQLSTATE 23505)`)
	}
	u.ID = 512
	f.created = u
	if f.byObersuite == nil {
		f.byObersuite = map[string]*models.User{}
	}
	f.byObersuite[u.ObersuiteID] = u
	return nil
}
func (f *fakeCompanyUserRepo) Update(_ *models.User, updates map[string]interface{}) error {
	f.updates = updates
	return nil
}
func (f *fakeCompanyUserRepo) RevokeSessionsByEmployer(id uint) (int64, error) {
	f.revokedFor = append(f.revokedFor, id)
	return 3, nil
}
func (f *fakeCompanyUserRepo) ListActiveByTypes(_ []models.UserType) ([]models.User, error) {
	return f.analistas, nil
}

type fakeCompanyAdminRepo struct {
	fakeNotesAdminRepo
	asignado map[uint]uint
}

func (f *fakeCompanyAdminRepo) AssignCSToTenant(tenantID, csID uint) error {
	if f.asignado == nil {
		f.asignado = map[uint]uint{}
	}
	f.asignado[tenantID] = csID
	return nil
}

func newCompanySvc() (*adminService, *fakeCompanyUserRepo, *fakeCompanyAdminRepo) {
	analista := &models.User{ID: 225, Name: "Yeilyn Crespo", Email: "yeilyn@oberstaff.com", UserType: models.UserTypeCustomerSuccess, IsActive: true}
	profesional := &models.User{ID: 300, Name: "Ana", UserType: models.UserTypeProfessional, IsActive: true}
	empresa := &models.User{ID: 36, Name: "Hermann", Email: "hermann@360bim.es", CompanyName: "360BIM", UserType: models.UserTypeEmployer, IsActive: true, Industry: "Construcción", City: "Sevilla"}
	users := &fakeCompanyUserRepo{
		byID:      map[uint]*models.User{225: analista, 300: profesional, 36: empresa},
		byEmail:   map[string]*models.User{"hermann@360bim.es": empresa},
		analistas: []models.User{*analista, {ID: 1, Name: "Bot", IsSystem: true, UserType: models.UserTypeSuperadmin, IsActive: true}},
	}
	repo := &fakeCompanyAdminRepo{}
	return &adminService{repo: repo, userRepo: users}, users, repo
}

func empresaBase() ObersuiteCompanyCreate {
	return ObersuiteCompanyCreate{
		ExternalID: "obersuite-company-abc", Name: "Acme S.A.",
		ResponsibleName: "Laura Méndez", ResponsibleEmail: "Laura@Acme.com",
		Industry: "Tecnología / Software", Country: "España",
	}
}

func TestEmpresa_SeCreaActivaSinCorreoNiCredenciales(t *testing.T) {
	svc, users, _ := newCompanySvc()

	res, err := svc.CreateCompanyFromObersuite(empresaBase())

	if err != nil || res.Status != "created" || res.ID != 512 {
		t.Fatalf("%v %v", res, err)
	}
	u := users.created
	if u.UserType != models.UserTypeEmployer || !u.IsActive {
		t.Errorf("tipo/estado: %s/%v", u.UserType, u.IsActive)
	}
	if u.CompanyName != "Acme S.A." || u.Name != "Laura Méndez" || u.Email != "laura@acme.com" {
		t.Errorf("campos: %q %q %q", u.CompanyName, u.Name, u.Email)
	}
	if u.ObersuiteID != "obersuite-company-abc" {
		t.Errorf("obersuite_id %q", u.ObersuiteID)
	}
	// Contraseña puesta, pero inservible: nadie la conoce.
	if u.Password == "" || len(u.Password) < 20 {
		t.Error("la cuenta tiene que nacer con una contraseña que nadie conozca")
	}
}

func TestEmpresa_ElMismoIdNoDuplica(t *testing.T) {
	svc, _, _ := newCompanySvc()
	primera, _ := svc.CreateCompanyFromObersuite(empresaBase())

	otra := empresaBase()
	otra.Name = "Otro nombre que NO debe pisar"
	segunda, err := svc.CreateCompanyFromObersuite(otra)

	if err != nil || segunda.Status != "already_exists" || segunda.ID != primera.ID {
		t.Fatalf("%v %v", segunda, err)
	}
}

// Un correo ya ocupado es 409 y empieza por el veredicto, con quién lo ocupa.
func TestEmpresa_CorreoOcupadoEs409ConVeredicto(t *testing.T) {
	svc, _, _ := newCompanySvc()
	in := empresaBase()
	in.ResponsibleEmail = "hermann@360bim.es"

	_, err := svc.CreateCompanyFromObersuite(in)

	if !errors.Is(err, apperrors.ErrEmailTaken) {
		t.Fatalf("debía ser 409, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "No se puede crear:") || !strings.Contains(err.Error(), "Hermann") {
		t.Errorf("mensaje: %s", err.Error())
	}
}

func TestEmpresa_Los400DelContrato(t *testing.T) {
	casos := []struct {
		nombre string
		muta   func(*ObersuiteCompanyCreate)
		frase  string
	}{
		{"sin external_id", func(i *ObersuiteCompanyCreate) { i.ExternalID = "" }, "external_id"},
		{"sin name", func(i *ObersuiteCompanyCreate) { i.Name = " " }, "name"},
		{"sin responsable", func(i *ObersuiteCompanyCreate) { i.ResponsibleName = "" }, "responsible_name"},
		{"correo sin forma", func(i *ObersuiteCompanyCreate) { i.ResponsibleEmail = "laura" }, "responsible_email"},
		{"rubro fuera de lista", func(i *ObersuiteCompanyCreate) { i.Industry = "Software" }, "GET /industries"},
		{"analista que no existe", func(i *ObersuiteCompanyCreate) { id := uint(9999); i.CustomerSuccessID = &id }, "customer_success_id"},
		{"analista que es profesional", func(i *ObersuiteCompanyCreate) { id := uint(300); i.CustomerSuccessID = &id }, "GET /analysts"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			svc, users, _ := newCompanySvc()
			in := empresaBase()
			c.muta(&in)

			_, err := svc.CreateCompanyFromObersuite(in)

			if !errors.Is(err, apperrors.ErrInvalidInput) {
				t.Fatalf("debía ser 400, got %v", err)
			}
			if !strings.Contains(err.Error(), c.frase) {
				t.Errorf("mensaje %q debía mencionar %q", err.Error(), c.frase)
			}
			if users.created != nil {
				t.Error("un 400 no escribe nada")
			}
		})
	}
}

func TestEmpresa_ElRubroVacioVale(t *testing.T) {
	svc, _, _ := newCompanySvc()
	in := empresaBase()
	in.Industry = ""
	if _, err := svc.CreateCompanyFromObersuite(in); err != nil {
		t.Fatalf("el rubro es opcional: %v", err)
	}
}

// PUT parcial: lo que no viaja no se toca, y se devuelve lo que CAMBIÓ de
// verdad, con el nombre que Obersuite mandó.
func TestEmpresa_ElPutEsParcialYDiceQueCambio(t *testing.T) {
	svc, users, _ := newCompanySvc()
	ciudad, mismaIndustria, nombre := "Madrid", "Construcción", "360BIM Servicios"

	changed, err := svc.UpdateCompanyFromObersuite(36, ObersuiteCompanyPatch{City: &ciudad, Industry: &mismaIndustria, Name: &nombre})

	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(changed, ",") != "name,city" {
		t.Fatalf("updated_fields = %v", changed)
	}
	if users.updates["company_name"] != "360BIM Servicios" || users.updates["city"] != "Madrid" {
		t.Errorf("updates: %v", users.updates)
	}
	if _, tocada := users.updates["industry"]; tocada {
		t.Error("un valor igual al que había no se escribe")
	}
	if _, tocada := users.updates["country"]; tocada {
		t.Error("lo que no viajó no se toca")
	}
}

func TestEmpresa_ElPutSinCambiosNoEscribe(t *testing.T) {
	svc, users, _ := newCompanySvc()
	changed, err := svc.UpdateCompanyFromObersuite(36, ObersuiteCompanyPatch{})
	if err != nil || len(changed) != 0 || users.updates != nil {
		t.Fatalf("changed=%v updates=%v err=%v", changed, users.updates, err)
	}
}

func TestEmpresa_ElPutRechazaNombreVacioYRubroFueraDeLista(t *testing.T) {
	svc, _, _ := newCompanySvc()
	vacio, malo := "  ", "Software"
	if _, err := svc.UpdateCompanyFromObersuite(36, ObersuiteCompanyPatch{Name: &vacio}); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Fatalf("name vacío debía ser 400: %v", err)
	}
	if _, err := svc.UpdateCompanyFromObersuite(36, ObersuiteCompanyPatch{Industry: &malo}); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Fatalf("rubro fuera de lista debía ser 400: %v", err)
	}
}

func TestEmpresa_LosAnalistasNoIncluyenCuentasDeSistema(t *testing.T) {
	svc, _, _ := newCompanySvc()
	list, err := svc.ListAssignableAnalysts()
	if err != nil || len(list) != 1 || list[0].ID != 225 {
		t.Fatalf("%v %v", list, err)
	}
}

func TestEmpresa_AsignarYQuitarAnalista(t *testing.T) {
	svc, _, repo := newCompanySvc()
	id := uint(225)
	if err := svc.SetCompanyAnalystFromObersuite(36, &id); err != nil || repo.asignado[36] != 225 {
		t.Fatalf("asignar: %v %v", err, repo.asignado)
	}
	if err := svc.SetCompanyAnalystFromObersuite(36, nil); err != nil || repo.asignado[36] != 0 {
		t.Fatalf("quitar: %v %v", err, repo.asignado)
	}
	mal := uint(300)
	if err := svc.SetCompanyAnalystFromObersuite(36, &mal); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Fatalf("un profesional no es analista: %v", err)
	}
}

// Suspender hace lo mismo que nuestro botón: apaga, expulsa a la plantilla y
// deja hito con el motivo firmado por la cuenta de servicio. Y es idempotente.
func TestEmpresa_SuspenderExpulsaYDejaHitoConMotivo(t *testing.T) {
	svc, users, repo := newCompanySvc()

	if err := svc.SetCompanyStatusFromObersuite(36, false, "Impago"); err != nil {
		t.Fatal(err)
	}
	if users.updates["is_active"] != false {
		t.Error("no se apagó")
	}
	if len(users.revokedFor) != 1 || users.revokedFor[0] != 36 {
		t.Error("no se expulsó a la plantilla")
	}
	if len(repo.created) != 1 || repo.created[0].Type != models.CompanyEventSuspended || repo.created[0].Detail != "Impago" || repo.created[0].ByUserID != 999 {
		t.Errorf("hito: %+v", repo.created)
	}

	// Ya suspendida: no-op.
	users.byID[36].IsActive = false
	users.revokedFor = nil
	if err := svc.SetCompanyStatusFromObersuite(36, false, "otra vez"); err != nil {
		t.Fatal(err)
	}
	if len(users.revokedFor) != 0 || len(repo.created) != 1 {
		t.Error("suspender lo suspendido no expulsa dos veces ni deja dos hitos")
	}
}
