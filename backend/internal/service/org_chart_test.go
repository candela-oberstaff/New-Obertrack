package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// El organigrama lo consultan tres públicos y cada uno tiene que ver una cosa
// distinta: la empresa entera (superadmin y empleador) o solo su rama (el
// supervisor). Lo que se protege aquí es ese recorte.

type fakeOrgEmploymentRepo struct {
	*fakeTreeEmploymentRepo
	org []repository.OrgNode
}

func (f *fakeOrgEmploymentRepo) ListCompanyOrg(_ uint) ([]repository.OrgNode, error) {
	return f.org, nil
}

// Empresa 20: Ana (7, supervisora) → María (8, manager) → nieto (9); y Ajeno
// (77), que cuelga de otra rama y no es de nadie del árbol de Ana.
func orgSetup() (*fakeUserRepo, *fakeOrgEmploymentRepo) {
	userRepo, tree := supervisorSetup()
	sup, mgr, kid, other := supervisorID, subManagerID, grandChildID, uint(77)
	return userRepo, &fakeOrgEmploymentRepo{
		fakeTreeEmploymentRepo: tree,
		org: []repository.OrgNode{
			{UserID: sup, Name: "Ana", ManagerID: uintPtr(1)}, // Ana le reporta al dueño (1)
			{UserID: mgr, Name: "María", ManagerID: &sup},
			{UserID: kid, Name: "Nieto", ManagerID: &mgr},
			{UserID: other, Name: "Ajeno", ManagerID: nil},
		},
	}
}

func orgNames(nodes []repository.OrgNode) []string {
	out := make([]string, len(nodes))
	for i := range nodes {
		out[i] = nodes[i].Name
	}
	return out
}

func TestOrgChart_SuperadminVeLaEmpresaEntera(t *testing.T) {
	userRepo, empRepo := orgSetup()
	s := &employmentService{repo: empRepo, userRepo: userRepo}

	nodes, err := s.OrgChart(supCompanyID, 999, "superadmin", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Las 4 personas. La cabeza de empresa la cubre TestOrgChart_LaEmpresaEsLaRaiz;
	// aquí el fake no tiene usuario empleador, así que el árbol va sin ella.
	if len(nodes) != 4 {
		t.Fatalf("el superadmin ve la empresa entera, got %v", orgNames(nodes))
	}
}

// La cuenta de empresa encabeza el organigrama y todo el que no tenía manager
// pasa a colgar de ella, en vez de quedar flotando suelto al lado del árbol.
func TestOrgChart_LaEmpresaEsLaRaiz(t *testing.T) {
	userRepo, empRepo := orgSetup()
	// La empresa (tenant) es un usuario empleador.
	userRepo.getByID[supCompanyID] = &models.User{
		ID: supCompanyID, UserType: models.UserTypeEmployer,
		CompanyName: "Acme S.A", IsActive: true,
	}
	s := &employmentService{repo: empRepo, userRepo: userRepo}

	nodes, err := s.OrgChart(supCompanyID, 999, "superadmin", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) == 0 || !nodes[0].IsCompany || nodes[0].Name != "Acme S.A" {
		t.Fatalf("la cuenta de empresa debe encabezar el árbol, got %v", orgNames(nodes))
	}
	if nodes[0].ManagerID != nil {
		t.Fatal("la empresa no cuelga de nadie")
	}
	for _, n := range nodes[1:] {
		if n.ManagerID == nil {
			t.Fatalf("%s se quedó sin colgar de la empresa", n.Name)
		}
	}
	// "Ajeno" no tenía manager: ahora reporta a la empresa.
	for _, n := range nodes {
		if n.Name == "Ajeno" && *n.ManagerID != supCompanyID {
			t.Fatalf("quien no tenía manager debe colgar de la empresa, got %v", *n.ManagerID)
		}
	}
}

// Si la empresa no se puede resolver, el árbol se devuelve sin cabeza en vez de
// fallar: es preferible un organigrama incompleto a ninguno.
func TestOrgChart_SinCuentaDeEmpresaNoRompe(t *testing.T) {
	userRepo, empRepo := orgSetup() // sin usuario empleador para supCompanyID
	s := &employmentService{repo: empRepo, userRepo: userRepo}

	nodes, err := s.OrgChart(supCompanyID, 999, "superadmin", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range nodes {
		if n.IsCompany {
			t.Fatal("no debe inventarse una cuenta de empresa")
		}
	}
}

func TestOrgChart_EmpleadorVeLaEmpresaEntera(t *testing.T) {
	userRepo, empRepo := orgSetup()
	s := &employmentService{repo: empRepo, userRepo: userRepo}

	nodes, err := s.OrgChart(supCompanyID, 1, string(models.UserTypeEmployer), false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("la empresa ve su organigrama completo, got %v", orgNames(nodes))
	}
	// Sin usuario empleador resoluble en el fake no se agrega raíz; eso lo cubre
	// TestOrgChart_LaEmpresaEsLaRaiz.
}

// El supervisor ve su rama y queda como raíz de su propia vista: por encima de
// él no se asoma nadie, aunque en los datos le reporte al dueño.
func TestOrgChart_SupervisorSoloVeSuRamaYEsLaRaiz(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := orgSetup()
	s := &employmentService{repo: empRepo, userRepo: userRepo}

	nodes, err := s.OrgChart(supCompanyID, supervisorID, "profesional", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("debe ver su rama (él, su manager y el nieto), got %v", orgNames(nodes))
	}
	for _, n := range nodes {
		if n.Name == "Ajeno" {
			t.Fatal("otra rama del organigrama no debe aparecer")
		}
		if n.UserID == supervisorID && n.ManagerID != nil {
			t.Fatal("el supervisor debe quedar como raíz de su propia vista")
		}
	}
}

// Sin el flag el rol no existe, así que tampoco hay rama que enseñarle.
func TestOrgChart_SupervisorSinFlagNoVeNada(t *testing.T) {
	userRepo, empRepo := orgSetup()
	s := &employmentService{repo: empRepo, userRepo: userRepo}

	if _, err := s.OrgChart(supCompanyID, supervisorID, "profesional", false, true); err == nil {
		t.Fatal("sin SUPERVISOR_SCOPE no hay organigrama para un supervisor")
	}
}

// ---------------------------------------------------------------------------
// Arrastrar en el organigrama de UNA empresa
// ---------------------------------------------------------------------------

// El caso que destapó el organigrama: un profesional con empleo en dos empresas.
// El superadmin edita el árbol de la 20, pero su empresa ACTIVA es la 99. El
// cambio tiene que caer en la 20 y no arrastrar a la persona en la otra.
func TestAssignToManager_RespetaLaEmpresaQueSeEdita(t *testing.T) {
	const otherCompany = uint(99)
	prof := &models.User{ID: 4, UserType: models.UserTypeProfessional, IsActive: true, EmpleadorID: uintPtr(otherCompany)}
	mgr := &models.User{ID: 8, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(supCompanyID)}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{4: prof, 8: mgr}}
	empRepo := &fakeEmploymentRepo{activeByKey: map[[2]uint]*models.Employment{
		{4, supCompanyID}: {ID: 300, UserID: 4, CompanyID: supCompanyID, Status: models.EmploymentActive},
		{4, otherCompany}: {ID: 301, UserID: 4, CompanyID: otherCompany, Status: models.EmploymentActive},
		{8, supCompanyID}: {ID: 302, UserID: 8, CompanyID: supCompanyID, Status: models.EmploymentActive},
	}}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := s.AssignToManager(4, 8, 1, 0, supCompanyID, "superadmin", false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(empRepo.updatedEmps) != 1 {
		t.Fatalf("debe tocarse exactamente un empleo, got %v", empRepo.updatedEmps)
	}
	if mid, _ := empRepo.updatedEmps[0]["manager_id"].(*uint); mid == nil || *mid != 8 {
		t.Fatalf("el empleo de la empresa editada debe apuntar al nuevo manager, got %v", empRepo.updatedEmps[0])
	}
	// users.manager_id refleja la empresa ACTIVA (la 99), que no es la que se
	// está editando: tocarlo movería a la persona en la otra empresa.
	if len(userRepo.saved) != 0 {
		t.Fatalf("no debe escribirse el puntero global al editar otra empresa, got %v", userRepo.saved)
	}
}

// Cuando la empresa que se edita SÍ es la activa, el puntero global acompaña.
func TestAssignToManager_EnLaEmpresaActivaSincronizaElPuntero(t *testing.T) {
	prof := &models.User{ID: 4, UserType: models.UserTypeProfessional, IsActive: true, EmpleadorID: uintPtr(supCompanyID)}
	mgr := &models.User{ID: 8, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(supCompanyID)}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{4: prof, 8: mgr}}
	empRepo := &fakeEmploymentRepo{activeByKey: map[[2]uint]*models.Employment{
		{4, supCompanyID}: {ID: 300, UserID: 4, CompanyID: supCompanyID, Status: models.EmploymentActive},
		{8, supCompanyID}: {ID: 302, UserID: 8, CompanyID: supCompanyID, Status: models.EmploymentActive},
	}}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := s.AssignToManager(4, 8, 1, 0, supCompanyID, "superadmin", false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userRepo.saved) != 1 || prof.ManagerID == nil || *prof.ManagerID != 8 {
		t.Fatalf("en la empresa activa el puntero global debe seguir al empleo, got %v", prof.ManagerID)
	}
}

// Asignar manager REEMPLAZA: el vínculo anterior se limpia antes de escribir el
// nuevo. Sin esto el manager saliente seguía aprobándole las horas a alguien que
// ya no era suyo, y solo se notaba por el contador de managers extra.
func TestAssignToManager_ReemplazaElVinculoAnterior(t *testing.T) {
	prof := &models.User{ID: 4, UserType: models.UserTypeProfessional, IsActive: true, EmpleadorID: uintPtr(supCompanyID)}
	mgr := &models.User{ID: 8, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(supCompanyID)}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{4: prof, 8: mgr}}
	empRepo := &fakeEmploymentRepo{activeByKey: map[[2]uint]*models.Employment{
		{4, supCompanyID}: {ID: 300, UserID: 4, CompanyID: supCompanyID, Status: models.EmploymentActive},
		{8, supCompanyID}: {ID: 302, UserID: 8, CompanyID: supCompanyID, Status: models.EmploymentActive},
	}}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := s.AssignToManager(4, 8, 1, 0, supCompanyID, "superadmin", false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(empRepo.clearedManagers) != 1 || empRepo.clearedManagers[0] != 300 {
		t.Fatalf("debe limpiarse el vínculo anterior del empleo, got %v", empRepo.clearedManagers)
	}
	if len(empRepo.primaryManager) != 1 || empRepo.primaryManager[0] != [2]uint{300, 8} {
		t.Fatalf("y quedar el nuevo como principal, got %v", empRepo.primaryManager)
	}
}

// Un empleador no puede apuntar a otra empresa aunque lo mande en el cuerpo:
// solo el superadmin elige empresa.
func TestAssignToManager_ElEmpleadorNoPuedeApuntarAOtraEmpresa(t *testing.T) {
	const otherCompany = uint(99)
	prof := &models.User{ID: 4, UserType: models.UserTypeProfessional, IsActive: true, EmpleadorID: uintPtr(supCompanyID)}
	mgr := &models.User{ID: 8, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(supCompanyID)}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{4: prof, 8: mgr}}
	empRepo := &fakeEmploymentRepo{activeByKey: map[[2]uint]*models.Employment{
		{4, supCompanyID}: {ID: 300, UserID: 4, CompanyID: supCompanyID, Status: models.EmploymentActive},
		{8, supCompanyID}: {ID: 302, UserID: 8, CompanyID: supCompanyID, Status: models.EmploymentActive},
	}}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	// Manda company_id=99, pero su tenant es supCompanyID: se ignora el cuerpo.
	if _, err := s.AssignToManager(4, 8, 1, supCompanyID, otherCompany,
		string(models.UserTypeEmployer), false, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(empRepo.updatedEmps) != 1 {
		t.Fatalf("debe tocarse solo el empleo de SU empresa, got %v", empRepo.updatedEmps)
	}
}

// Un profesional cualquiera no tiene organigrama que mirar.
func TestOrgChart_ProfesionalSinAcceso(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := orgSetup()
	s := &employmentService{repo: empRepo, userRepo: userRepo}

	if _, err := s.OrgChart(supCompanyID, grandChildID, "profesional", false, false); err == nil {
		t.Fatal("un profesional sin gente a cargo no accede al organigrama")
	}
}
