package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Notas y filtros del expediente de empresa. Mismo patrón de fakes que
// manager_flow_test.go: se embebe la interfaz real y se sobrescribe solo lo
// que toca el camino bajo prueba.

type fakeNotesAdminRepo struct {
	repository.AdminRepository

	created []*models.CompanyEvent

	deleteCompanyID uint
	deleteNoteID    uint
	deleteRows      int64
	deleteErr       error

	updatedDetail   string
	updatedEditedAt time.Time
	updateRows      int64

	pinnedValue bool
	pinnedNote  uint
	pinRows     int64

	countRows []repository.TenantActivityCount

	// Argumentos con los que el servicio llamó a la consulta del expediente.
	gotCategory string
	gotUserID   uint
	gotOffset   int
	gotLimit    int
}

func (f *fakeNotesAdminRepo) CreateCompanyEvent(event *models.CompanyEvent) error {
	f.created = append(f.created, event)
	return nil
}

func (f *fakeNotesAdminRepo) DeleteCompanyNote(companyID, noteID uint) (int64, error) {
	f.deleteCompanyID, f.deleteNoteID = companyID, noteID
	return f.deleteRows, f.deleteErr
}

func (f *fakeNotesAdminRepo) UpdateCompanyNote(companyID, noteID uint, detail string, editedAt time.Time) (int64, error) {
	f.deleteCompanyID, f.deleteNoteID = companyID, noteID
	f.updatedDetail, f.updatedEditedAt = detail, editedAt
	return f.updateRows, nil
}

func (f *fakeNotesAdminRepo) SetCompanyNotePinned(companyID, noteID uint, pinned bool) (int64, error) {
	f.deleteCompanyID, f.pinnedNote, f.pinnedValue = companyID, noteID, pinned
	return f.pinRows, nil
}

func (f *fakeNotesAdminRepo) GetTenantActivityCounts(_ uint, _ uint) ([]repository.TenantActivityCount, error) {
	return f.countRows, nil
}

func (f *fakeNotesAdminRepo) GetTenantActivities(_ uint, category string, userID uint, offset, limit int) ([]repository.TenantActivity, int64, error) {
	f.gotCategory, f.gotUserID, f.gotOffset, f.gotLimit = category, userID, offset, limit
	return []repository.TenantActivity{}, 0, nil
}

type fakeNotesUserRepo struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (f *fakeNotesUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func newNotesSvc(users ...*models.User) (*adminService, *fakeNotesAdminRepo) {
	byID := map[uint]*models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	repo := &fakeNotesAdminRepo{}
	return &adminService{repo: repo, userRepo: &fakeNotesUserRepo{users: byID}}, repo
}

func company(id uint) *models.User {
	return &models.User{ID: id, UserType: models.UserTypeEmployer, IsActive: true}
}

// --- AddTenantNote ----------------------------------------------------------

func TestAddTenantNote_GuardaLaNotaFirmada(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	event, err := svc.AddTenantNote(7 /*companyID*/, 42 /*byUserID*/, "  Llamada con el responsable  ")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	// Se guarda sin los espacios de los bordes: el expediente se lee, no se parsea.
	if event.Detail != "Llamada con el responsable" {
		t.Fatalf("detail: got %q", event.Detail)
	}
	if event.Type != models.CompanyEventNote {
		t.Fatalf("type: want %q, got %q", models.CompanyEventNote, event.Type)
	}
	if event.CompanyID != 7 || event.ByUserID != 42 {
		t.Fatalf("firma: company=%d by=%d", event.CompanyID, event.ByUserID)
	}
	if len(repo.created) != 1 {
		t.Fatalf("se esperaba una escritura, got %d", len(repo.created))
	}
}

func TestAddTenantNote_RechazaNotaVacia(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	for _, text := range []string{"", "   ", "\n\t "} {
		if _, err := svc.AddTenantNote(7, 42, text); err == nil {
			t.Fatalf("se esperaba error con %q", text)
		}
	}
	if len(repo.created) != 0 {
		t.Fatal("no debería escribirse nada")
	}
}

func TestAddTenantNote_RechazaNotaDemasiadoLarga(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	// El límite es en runas, no en bytes: con acentos, contar bytes recortaría
	// notas válidas.
	justo := strings.Repeat("á", models.MaxCompanyNoteLength)
	if _, err := svc.AddTenantNote(7, 42, justo); err != nil {
		t.Fatalf("la nota en el límite debería entrar: %v", err)
	}
	if _, err := svc.AddTenantNote(7, 42, justo+"á"); err == nil {
		t.Fatal("se esperaba error al pasarse del límite")
	}
	if len(repo.created) != 1 {
		t.Fatalf("solo debería guardarse la válida, got %d", len(repo.created))
	}
}

func TestAddTenantNote_RechazaUsuarioQueNoEsEmpresa(t *testing.T) {
	pro := &models.User{ID: 7, UserType: models.UserTypeProfessional}
	svc, repo := newNotesSvc(pro)

	if _, err := svc.AddTenantNote(7, 42, "hola"); err == nil {
		t.Fatal("se esperaba error sobre un no-empleador")
	}
	if len(repo.created) != 0 {
		t.Fatal("no debería escribirse nada")
	}
}

func TestAddTenantNote_RechazaEmpresaInexistente(t *testing.T) {
	svc, _ := newNotesSvc()

	if _, err := svc.AddTenantNote(404, 42, "hola"); err == nil {
		t.Fatal("se esperaba error con una empresa inexistente")
	}
}

// --- DeleteTenantNote -------------------------------------------------------

func TestDeleteTenantNote_AcotaElBorradoALaEmpresa(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.deleteRows = 1

	if err := svc.DeleteTenantNote(7, 55); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if repo.deleteCompanyID != 7 || repo.deleteNoteID != 55 {
		t.Fatalf("el borrado debe ir acotado: company=%d note=%d", repo.deleteCompanyID, repo.deleteNoteID)
	}
}

// Sin filas borradas la nota no existe, o no era una nota (una suspensión, por
// ejemplo): en ambos casos es un 404, no un borrado silencioso.
func TestDeleteTenantNote_ErrorSiNoBorroNada(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.deleteRows = 0

	if err := svc.DeleteTenantNote(7, 55); err == nil {
		t.Fatal("se esperaba error cuando no se borra ninguna fila")
	}
}

// --- UpdateTenantNote -------------------------------------------------------

func TestUpdateTenantNote_CorrigeYMarcaEditada(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.updateRows = 1

	if err := svc.UpdateTenantNote(7, 55, "  Texto corregido  "); err != nil {
		t.Fatalf("update: %v", err)
	}
	if repo.updatedDetail != "Texto corregido" {
		t.Fatalf("detail: got %q", repo.updatedDetail)
	}
	if repo.updatedEditedAt.IsZero() {
		t.Fatal("editar debe dejar constancia con una fecha de edición")
	}
	if repo.deleteCompanyID != 7 || repo.deleteNoteID != 55 {
		t.Fatalf("la edición debe ir acotada: company=%d note=%d", repo.deleteCompanyID, repo.deleteNoteID)
	}
}

// Editar aplica las mismas reglas que crear: corregir no es una puerta trasera
// para dejar una nota vacía o kilométrica.
func TestUpdateTenantNote_ValidaIgualQueAlCrear(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.updateRows = 1

	if err := svc.UpdateTenantNote(7, 55, "   "); err == nil {
		t.Fatal("se esperaba error con una nota vacía")
	}
	largo := strings.Repeat("á", models.MaxCompanyNoteLength+1)
	if err := svc.UpdateTenantNote(7, 55, largo); err == nil {
		t.Fatal("se esperaba error al pasarse del límite")
	}
	if repo.updatedDetail != "" {
		t.Fatalf("no debería haberse escrito nada, got %q", repo.updatedDetail)
	}
}

func TestUpdateTenantNote_ErrorSiNoExiste(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.updateRows = 0

	if err := svc.UpdateTenantNote(7, 55, "hola"); err == nil {
		t.Fatal("se esperaba error cuando la nota no existe")
	}
}

// --- SetTenantNotePinned ----------------------------------------------------

func TestSetTenantNotePinned_FijaYDesfija(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.pinRows = 1

	if err := svc.SetTenantNotePinned(7, 55, true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if !repo.pinnedValue || repo.pinnedNote != 55 || repo.deleteCompanyID != 7 {
		t.Fatalf("fijar: pinned=%v note=%d company=%d", repo.pinnedValue, repo.pinnedNote, repo.deleteCompanyID)
	}

	if err := svc.SetTenantNotePinned(7, 55, false); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if repo.pinnedValue {
		t.Fatal("desfijar debería pasar pinned=false")
	}
}

func TestSetTenantNotePinned_ErrorSiNoExiste(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.pinRows = 0

	if err := svc.SetTenantNotePinned(7, 55, true); err == nil {
		t.Fatal("se esperaba error cuando la nota no existe")
	}
}

// --- GetTenantActivityCounts ------------------------------------------------

func TestGetTenantActivityCounts_AgregaElTotalBajoLaClaveVacia(t *testing.T) {
	svc, repo := newNotesSvc(company(7))
	repo.countRows = []repository.TenantActivityCount{
		{Category: repository.TenantActivityWork, Count: 9},
		{Category: repository.TenantActivityNote, Count: 1},
		{Category: repository.TenantActivityStaff, Count: 40},
	}

	counts, err := svc.GetTenantActivityCounts(7, 0)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts[repository.TenantActivityWork] != 9 || counts[repository.TenantActivityNote] != 1 {
		t.Fatalf("contadores por categoría: %v", counts)
	}
	// El chip "Todo" lee su número de la misma estructura que los demás.
	if counts[""] != 50 {
		t.Fatalf("total: want 50, got %d", counts[""])
	}
}

func TestGetTenantActivityCounts_SinMovimientosDaCero(t *testing.T) {
	svc, _ := newNotesSvc(company(7))

	counts, err := svc.GetTenantActivityCounts(7, 0)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts[""] != 0 {
		t.Fatalf("total: want 0, got %d", counts[""])
	}
}

// --- GetTenantActivities (saneado de filtros y paginación) ------------------

func TestGetTenantActivities_IgnoraCategoriaDesconocida(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	// Una categoría inventada devuelve el expediente completo, no una página
	// vacía sin explicación.
	if _, _, err := svc.GetTenantActivities(7, "inventada", 0, 0, 20); err != nil {
		t.Fatalf("get: %v", err)
	}
	if repo.gotCategory != "" {
		t.Fatalf("category: want vacía, got %q", repo.gotCategory)
	}
}

// El filtro por persona viaja por id, no por nombre: dos empleados homónimos
// no deben mezclar sus movimientos.
func TestGetTenantActivities_FiltraPorPersona(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	if _, _, err := svc.GetTenantActivities(7, "", 33, 0, 20); err != nil {
		t.Fatalf("get: %v", err)
	}
	if repo.gotUserID != 33 {
		t.Fatalf("user_id: want 33, got %d", repo.gotUserID)
	}

	// Sin persona seleccionada pasa 0 = todas.
	if _, _, err := svc.GetTenantActivities(7, "", 0, 0, 20); err != nil {
		t.Fatalf("get: %v", err)
	}
	if repo.gotUserID != 0 {
		t.Fatalf("user_id sin filtro: want 0, got %d", repo.gotUserID)
	}
}

// Persona y categoría son independientes: se combinan sin pisarse.
func TestGetTenantActivities_CombinaPersonaYCategoria(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	if _, _, err := svc.GetTenantActivities(7, repository.TenantActivityWork, 33, 0, 20); err != nil {
		t.Fatalf("get: %v", err)
	}
	if repo.gotCategory != repository.TenantActivityWork || repo.gotUserID != 33 {
		t.Fatalf("combinación: category=%q user=%d", repo.gotCategory, repo.gotUserID)
	}
}

func TestGetTenantActivities_RespetaCategoriaValida(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	if _, _, err := svc.GetTenantActivities(7, "  management  ", 0, 0, 20); err != nil {
		t.Fatalf("get: %v", err)
	}
	if repo.gotCategory != repository.TenantActivityManagement {
		t.Fatalf("category: want %q, got %q", repository.TenantActivityManagement, repo.gotCategory)
	}
}

func TestGetTenantActivities_AcotaLaPaginacion(t *testing.T) {
	svc, repo := newNotesSvc(company(7))

	cases := []struct {
		offset, limit         int
		wantOffset, wantLimit int
	}{
		{0, 0, 0, 20},    // sin límite -> por defecto
		{0, 5000, 0, 20}, // límite absurdo -> por defecto (no se vacía la BD)
		{-10, 50, 0, 50}, // offset negativo -> primera página
		{40, 50, 40, 50}, // valores razonables intactos
		{0, 100, 0, 100}, // el máximo admitido pasa tal cual
	}
	for _, tc := range cases {
		if _, _, err := svc.GetTenantActivities(7, "", 0, tc.offset, tc.limit); err != nil {
			t.Fatalf("get: %v", err)
		}
		if repo.gotOffset != tc.wantOffset || repo.gotLimit != tc.wantLimit {
			t.Fatalf("offset/limit %d/%d -> got %d/%d, want %d/%d",
				tc.offset, tc.limit, repo.gotOffset, repo.gotLimit, tc.wantOffset, tc.wantLimit)
		}
	}
}
