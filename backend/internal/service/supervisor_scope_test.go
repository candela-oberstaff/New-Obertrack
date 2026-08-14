package service

import (
	"errors"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Fase 2 del rol supervisor: el alcance deja de ser "mis reportes directos" y
// pasa a ser "mi árbol". Lo que estos tests protegen, por orden de importancia:
//
//  1. Con SUPERVISOR_SCOPE apagado NADA cambia, ni siquiera para un supervisor.
//     Es la garantía de que el flag es el único interruptor.
//  2. Con el flag encendido, el supervisor alcanza a los nietos (la gente de sus
//     managers) y sigue sin poder tocar sus propias jornadas.
//  3. Un árbol vacío filtra a cero, no a "toda la empresa".
//
// Mismo patrón de fakes que manager_flow_test.go: se embebe la interfaz real y
// solo se implementa lo que toca el camino bajo prueba.

const (
	supCompanyID = uint(20)
	supervisorID = uint(7)
	subManagerID = uint(8) // manager que le reporta al supervisor
	grandChildID = uint(9) // profesional del manager: el "nieto" del supervisor
)

// fakeTreeEmploymentRepo resuelve el árbol desde un mapa inyectado en vez de
// recorrer employment_managers.
type fakeTreeEmploymentRepo struct {
	repository.EmploymentRepository

	// descendants[(rootID, companyID)] = subárbol completo (a cualquier nivel)
	descendants map[[2]uint][]uint
	// directManagers[(userID, companyID)] = jefes DIRECTOS, que es hasta donde
	// llega un manager sin el flag de supervisor.
	directManagers map[[2]uint][]uint
	// employments[(userID, companyID)] = membresía activa. La necesitan los
	// caminos de escritura (asignar/reasignar), que validan al manager destino.
	employments map[[2]uint]*models.Employment
	descErr     error

	// captura
	descCalls  int
	lastDepth  int
	isDescHits int
}

func (f *fakeTreeEmploymentRepo) DescendantIDs(rootID, companyID uint, maxDepth int) ([]uint, error) {
	f.descCalls++
	f.lastDepth = maxDepth
	if f.descErr != nil {
		return nil, f.descErr
	}
	return f.descendants[[2]uint{rootID, companyID}], nil
}

func (f *fakeTreeEmploymentRepo) IsManagerOf(userID, companyID, managerID uint) (bool, error) {
	for _, m := range f.directManagers[[2]uint{userID, companyID}] {
		if m == managerID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeTreeEmploymentRepo) IsDescendantOf(rootID, userID, companyID uint, maxDepth int) (bool, error) {
	f.isDescHits++
	if f.descErr != nil {
		return false, f.descErr
	}
	if rootID == userID {
		return false, nil
	}
	for _, id := range f.descendants[[2]uint{rootID, companyID}] {
		if id == userID {
			return true, nil
		}
	}
	return false, nil
}

// --- Escrituras: no-ops que solo dejan pasar los caminos de asignación ---

func (f *fakeTreeEmploymentRepo) GetActive(userID, companyID uint) (*models.Employment, error) {
	if e, ok := f.employments[[2]uint{userID, companyID}]; ok {
		return e, nil
	}
	return nil, errors.New("no active employment")
}

func (f *fakeTreeEmploymentRepo) Update(*models.Employment, map[string]interface{}) error { return nil }
func (f *fakeTreeEmploymentRepo) SetPrimaryManager(_, _ uint) error                       { return nil }
func (f *fakeTreeEmploymentRepo) ClearManagers(_ uint) error                              { return nil }
func (f *fakeTreeEmploymentRepo) ReassignManagerLinks(_, _ *uint, _ uint) error           { return nil }

func (f *fakeTreeEmploymentRepo) ReassignManager(_ uint, _ *uint, _ uint) (int64, error) {
	return 0, nil
}

// fakeFilterWHRepo captura los filtros con los que el service consulta.
type fakeFilterWHRepo struct {
	repository.WorkHourRepository
	lastFilters map[string]interface{}
}

func (f *fakeFilterWHRepo) FindAll(filters map[string]interface{}, _, _ int) ([]models.WorkHour, int64, error) {
	f.lastFilters = filters
	return []models.WorkHour{}, 0, nil
}

func (f *fakeFilterWHRepo) GetSummary(filters map[string]interface{}) (map[string]float64, error) {
	f.lastFilters = filters
	return map[string]float64{}, nil
}

// supervisorSetup arma el trío supervisor → manager → nieto con el árbol ya
// resuelto, más un manager normal (sin el flag de supervisor) para contrastar.
func supervisorSetup() (*fakeUserRepo, *fakeTreeEmploymentRepo) {
	users := map[uint]*models.User{
		supervisorID: {
			ID: supervisorID, UserType: models.UserTypeProfessional,
			IsManager: true, IsSupervisor: true, IsActive: true,
			EmpleadorID: uintPtr(supCompanyID),
		},
		subManagerID: {
			ID: subManagerID, UserType: models.UserTypeProfessional,
			IsManager: true, IsActive: true, EmpleadorID: uintPtr(supCompanyID),
		},
		grandChildID: {
			ID: grandChildID, UserType: models.UserTypeProfessional,
			IsActive: true, EmpleadorID: uintPtr(supCompanyID),
		},
	}
	empRepo := &fakeTreeEmploymentRepo{
		descendants: map[[2]uint][]uint{
			{supervisorID, supCompanyID}: {subManagerID, grandChildID},
			{subManagerID, supCompanyID}: {grandChildID},
		},
		directManagers: map[[2]uint][]uint{
			{subManagerID, supCompanyID}: {supervisorID},
			{grandChildID, supCompanyID}: {subManagerID},
		},
		employments: map[[2]uint]*models.Employment{
			{supervisorID, supCompanyID}: {ID: 100, UserID: supervisorID, CompanyID: supCompanyID, Status: models.EmploymentActive},
			{subManagerID, supCompanyID}: {ID: 101, UserID: subManagerID, CompanyID: supCompanyID, Status: models.EmploymentActive, ManagerID: uintPtr(supervisorID)},
			{grandChildID, supCompanyID}: {ID: 102, UserID: grandChildID, CompanyID: supCompanyID, Status: models.EmploymentActive, ManagerID: uintPtr(subManagerID)},
		},
	}
	return &fakeUserRepo{getByID: users}, empRepo
}

// ---------------------------------------------------------------------------
// 1. El flag apagado no cambia nada
// ---------------------------------------------------------------------------

// Con SUPERVISOR_SCOPE apagado, un supervisor filtra por el camino de manager de
// siempre: ni se consulta el árbol.
func TestSupervisorScope_FlagOffMantieneElCaminoDeManager(t *testing.T) {
	SetMultiManagerReads(true)
	defer SetMultiManagerReads(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeFilterWHRepo{}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo}

	if _, err := s.GetPending(supCompanyID, supervisorID, "profesional", false, true, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := whRepo.lastFilters["user_ids"]; ok {
		t.Fatal("con el flag apagado no debe usarse el filtro de árbol")
	}
	if whRepo.lastFilters["manager_links_id"] != supervisorID {
		t.Fatalf("debe caer al filtro de manager de siempre, got %v", whRepo.lastFilters)
	}
	if empRepo.descCalls != 0 {
		t.Fatal("con el flag apagado no debe consultarse el árbol")
	}
}

// Y tampoco le deja aprobar a un nieto: sin el flag, el supervisor solo llega a
// donde llegaría cualquier manager.
func TestSupervisorScope_FlagOffNoApruebaAlNieto(t *testing.T) {
	SetMultiManagerReads(true)
	defer SetMultiManagerReads(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: grandChildID, TenantID: supCompanyID},
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	_, skipped, err := s.Approve([]uint{1}, supervisorID, "profesional", false, true, supCompanyID)
	if err == nil {
		t.Fatal("sin el flag, el supervisor no alcanza al nieto")
	}
	if skipped != 1 {
		t.Fatalf("la jornada del nieto debe omitirse, got skipped=%d", skipped)
	}
}

// ---------------------------------------------------------------------------
// 2. El flag encendido abre el árbol
// ---------------------------------------------------------------------------

// La bandeja de pendientes de un supervisor es su árbol SIN él mismo: la
// separación de funciones no cambia por subir de nivel.
func TestSupervisorScope_PendientesUsanElArbolSinElMismo(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeFilterWHRepo{}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo}

	if _, err := s.GetPending(supCompanyID, supervisorID, "profesional", false, true, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, ok := whRepo.lastFilters["user_ids"].([]uint)
	if !ok {
		t.Fatalf("debe filtrarse por el árbol, got %v", whRepo.lastFilters)
	}
	if len(ids) != 2 || !contieneID(ids, subManagerID) || !contieneID(ids, grandChildID) {
		t.Fatalf("el árbol debe traer al manager y al nieto, got %v", ids)
	}
	if contieneID(ids, supervisorID) {
		t.Fatal("la bandeja de pendientes no debe incluir al propio supervisor")
	}
	if empRepo.lastDepth != maxSupervisorDepth {
		t.Fatalf("debe recorrerse con el tope de profundidad, got %d", empRepo.lastDepth)
	}
}

// El listado y el resumen sí lo incluyen a él: es la vista de "mi equipo y yo",
// igual que la de un manager.
func TestSupervisorScope_ListadoIncluyeAlSupervisor(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeFilterWHRepo{}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo}

	if _, _, err := s.GetAll(supervisorID, "profesional", false, true, supCompanyID, 0, "", "", "", 0, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, ok := whRepo.lastFilters["user_ids"].([]uint)
	if !ok {
		t.Fatalf("debe filtrarse por el árbol, got %v", whRepo.lastFilters)
	}
	if !contieneID(ids, supervisorID) {
		t.Fatalf("el listado debe incluir las jornadas del propio supervisor, got %v", ids)
	}
}

// Un manager que NO es supervisor sigue por el camino de siempre aunque el flag
// esté encendido.
func TestSupervisorScope_ManagerNormalNoCambia(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)
	SetMultiManagerReads(true)
	defer SetMultiManagerReads(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeFilterWHRepo{}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo}

	if _, err := s.GetPending(supCompanyID, subManagerID, "profesional", false, true, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := whRepo.lastFilters["user_ids"]; ok {
		t.Fatal("un manager sin el flag de supervisor no debe usar el alcance de árbol")
	}
	if whRepo.lastFilters["manager_links_id"] != subManagerID {
		t.Fatalf("debe caer al filtro de manager, got %v", whRepo.lastFilters)
	}
}

// Un supervisor sin nadie debajo tiene que ver CERO, no toda la empresa. El
// filtro distingue "árbol vacío" de "sin filtro"; si se colara, el supervisor
// recién creado vería las jornadas del tenant entero.
func TestSupervisorScope_ArbolVacioNoAbreElTenant(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	empRepo.descendants = map[[2]uint][]uint{} // todavía no tiene a nadie
	whRepo := &fakeFilterWHRepo{}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo}

	if _, err := s.GetPending(supCompanyID, supervisorID, "profesional", false, true, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, ok := whRepo.lastFilters["user_ids"].([]uint)
	if !ok {
		t.Fatalf("debe seguir aplicándose el filtro de árbol, got %v", whRepo.lastFilters)
	}
	if len(ids) != 0 {
		t.Fatalf("un árbol vacío debe filtrar a cero, got %v", ids)
	}
}

// El caso que da sentido al rol: aprobar las horas de alguien que cuelga de uno
// de sus managers, no de él directamente.
func TestSupervisorScope_ApruebaAlNieto(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: grandChildID, TenantID: supCompanyID},
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	approved, skipped, err := s.Approve([]uint{1}, supervisorID, "profesional", false, true, supCompanyID)
	if err != nil {
		t.Fatalf("el supervisor debe poder aprobar a un nieto, got: %v", err)
	}
	if approved != 1 || skipped != 0 {
		t.Fatalf("esperaba 1 aprobada y 0 omitidas, got %d/%d", approved, skipped)
	}
}

// Subir de nivel no exime de la separación de funciones: sus propias jornadas se
// omiten igual que las de un manager.
func TestSupervisorScope_NoApruebaLasPropias(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: grandChildID, TenantID: supCompanyID}, // del árbol: entra
		{ID: 2, UserID: supervisorID, TenantID: supCompanyID}, // propia: se omite
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	approved, skipped, err := s.Approve([]uint{1, 2}, supervisorID, "profesional", false, true, supCompanyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved != 1 || skipped != 1 {
		t.Fatalf("esperaba 1 aprobada y 1 omitida (la propia), got %d/%d", approved, skipped)
	}
	if len(whRepo.approvedIDs) != 1 || whRepo.approvedIDs[0] != 1 {
		t.Fatalf("solo debe persistirse la jornada del árbol, got %v", whRepo.approvedIDs)
	}
}

// Alguien de otra rama del organigrama no está en el árbol y queda fuera.
func TestSupervisorScope_NoAlcanzaFueraDeSuArbol(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	const ajenoID = uint(77)
	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: ajenoID, TenantID: supCompanyID},
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	if _, skipped, err := s.Approve([]uint{1}, supervisorID, "profesional", false, true, supCompanyID); err == nil {
		t.Fatal("un usuario fuera del árbol no debe ser aprobable")
	} else if skipped != 1 {
		t.Fatalf("debe omitirse, got skipped=%d", skipped)
	}
}

// Un fallo al resolver el árbol no puede degradar a "sin filtro": se propaga.
func TestSupervisorScope_ErrorDeArbolNoDegrada(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	empRepo.descErr = errors.New("db caída")
	whRepo := &fakeFilterWHRepo{}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo}

	if _, err := s.GetPending(supCompanyID, supervisorID, "profesional", false, true, 0, ""); err == nil {
		t.Fatal("si no se puede resolver el árbol, la consulta debe fallar en vez de devolver de más")
	}
}

// ---------------------------------------------------------------------------
// 3. Idempotencia: manager y supervisor compitiendo por lo mismo
// ---------------------------------------------------------------------------

// El caso que crea el rol: el manager aprueba, y el supervisor —con la lista en
// pantalla sin refrescar— vuelve a darle a "Aprobar todos". Gana el primero: no
// se reescribe el aprobador ni le llega al profesional un segundo aviso.
func TestSupervisorScope_AprobarLoYaAprobadoNoHaceNada(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: grandChildID, TenantID: supCompanyID, Approved: true},
	}}
	notif := &fakeNotifSvc{}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: notif}

	approved, skipped, err := s.Approve([]uint{1}, supervisorID, "profesional", false, true, supCompanyID)
	if err != nil {
		t.Fatalf("re-aprobar no es un error: el estado pedido ya se cumple, got %v", err)
	}
	if approved != 0 || skipped != 1 {
		t.Fatalf("esperaba 0 aprobadas y 1 omitida, got %d/%d", approved, skipped)
	}
	if whRepo.approveMultiHit {
		t.Fatal("no debe reescribirse el aprobador de una jornada ya aprobada")
	}
}

// En un lote mixto entran solo las pendientes; las ya aprobadas se omiten sin
// tumbar el resto.
func TestSupervisorScope_LoteMixtoSoloApruebaLoPendiente(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: grandChildID, TenantID: supCompanyID, Approved: true},
		{ID: 2, UserID: subManagerID, TenantID: supCompanyID},
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	approved, skipped, err := s.Approve([]uint{1, 2}, supervisorID, "profesional", false, true, supCompanyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved != 1 || skipped != 1 {
		t.Fatalf("esperaba 1 aprobada y 1 omitida, got %d/%d", approved, skipped)
	}
	if len(whRepo.approvedIDs) != 1 || whRepo.approvedIDs[0] != 2 {
		t.Fatalf("solo debe persistirse la pendiente, got %v", whRepo.approvedIDs)
	}
}

// La falta de permiso SIGUE siendo un error: la idempotencia no puede acabar
// convirtiendo un 403 en un 200 silencioso.
func TestSupervisorScope_SinPermisoSigueSiendoError(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: 77, TenantID: supCompanyID}, // fuera del árbol
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	if _, _, err := s.Approve([]uint{1}, supervisorID, "profesional", false, true, supCompanyID); err == nil {
		t.Fatal("sin permiso y sin nada ya aprobado, debe seguir siendo error")
	}
}

// Repetir el MISMO rechazo no hace nada...
func TestSupervisorScope_RechazoRepetidoNoHaceNada(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeRejectWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: grandChildID, TenantID: supCompanyID, Rejected: true, RejectionReason: "Faltan actividades"},
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	rejected, skipped, err := s.Reject([]uint{1}, supervisorID, "profesional", false, true, supCompanyID, "Faltan actividades")
	if err != nil {
		t.Fatalf("repetir el mismo rechazo no es un error, got %v", err)
	}
	if rejected != 0 || skipped != 1 {
		t.Fatalf("esperaba 0 rechazadas y 1 omitida, got %d/%d", rejected, skipped)
	}
	if whRepo.rejectHit {
		t.Fatal("no debe reescribirse un rechazo idéntico")
	}
}

// ...pero corregir el motivo SÍ se aplica: es una acción deliberada, y lo que
// lee el profesional cambia.
func TestSupervisorScope_RechazoConOtroMotivoSiSeAplica(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	whRepo := &fakeRejectWHRepo{byID: []models.WorkHour{
		{ID: 1, UserID: grandChildID, TenantID: supCompanyID, Rejected: true, RejectionReason: "Faltan actividades"},
	}}
	s := &workHourService{repo: whRepo, userRepo: userRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}}

	rejected, _, err := s.Reject([]uint{1}, supervisorID, "profesional", false, true, supCompanyID, "El horario no cuadra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rejected != 1 || !whRepo.rejectHit {
		t.Fatalf("corregir el motivo debe aplicarse, got rejected=%d hit=%v", rejected, whRepo.rejectHit)
	}
}

// fakeRejectWHRepo es el espejo de fakeApproveWHRepo para el camino de rechazo.
type fakeRejectWHRepo struct {
	repository.WorkHourRepository
	byID        []models.WorkHour
	rejectHit   bool
	rejectedIDs []uint
}

func (f *fakeRejectWHRepo) FindManyByIDs(ids []uint) ([]models.WorkHour, error) {
	return f.byID, nil
}
func (f *fakeRejectWHRepo) FindManyByIDsAndTenant(ids []uint, _ uint) ([]models.WorkHour, error) {
	return f.byID, nil
}
func (f *fakeRejectWHRepo) RejectMultiple(ids []uint, _ uint, _ time.Time, _ string) error {
	f.rejectHit = true
	f.rejectedIDs = ids
	return nil
}
func (f *fakeRejectWHRepo) RejectMultipleAndTenant(ids []uint, _ uint, _ time.Time, _ string, _ uint) error {
	f.rejectHit = true
	f.rejectedIDs = ids
	return nil
}

// ---------------------------------------------------------------------------
// 4. Mi equipo
// ---------------------------------------------------------------------------

func TestSupervisorScope_MiEquipoDevuelveElArbol(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := supervisorSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	team, err := s.GetMyTeam(supervisorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(team) != 2 {
		t.Fatalf("el equipo de un supervisor es su árbol (manager + nieto), got %d", len(team))
	}
	ids := make([]uint, len(team))
	for i := range team {
		ids[i] = team[i].ID
	}
	if !contieneID(ids, subManagerID) || !contieneID(ids, grandChildID) {
		t.Fatalf("el equipo debe traer al manager y al nieto, got %v", ids)
	}
}

// fakeUserRepo (manager_flow_test.go) no implementa GetByIDs porque ningún
// camino de allí materializa un conjunto de usuarios; el árbol del supervisor
// sí.
func (f *fakeUserRepo) GetByIDs(ids []uint) ([]models.User, error) {
	out := []models.User{}
	for _, id := range ids {
		if u, ok := f.getByID[id]; ok {
			out = append(out, *u)
		}
	}
	return out, nil
}

func contieneID(ids []uint, id uint) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
