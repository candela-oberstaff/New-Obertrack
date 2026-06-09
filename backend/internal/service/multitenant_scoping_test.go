package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// These tests pin down the superadmin company-scoping rules that protect tenant
// isolation across tasks, boards, work hours and chat. Each fake repo embeds the
// real interface (so it satisfies it) and overrides only the method under test,
// capturing the filters the service builds.

// ---------------------------------------------------------------------------
// Tasks
// ---------------------------------------------------------------------------

type fakeTaskRepo struct {
	repository.TaskRepository
	findFilters map[string]interface{}
	findCalled  bool
	countTenant uint
	countCalled bool
	countRows   []repository.BoardStatusCount
}

func (f *fakeTaskRepo) FindAll(filters map[string]interface{}, _, _ int) ([]models.Task, int64, error) {
	f.findFilters = filters
	f.findCalled = true
	return nil, 0, nil
}

func (f *fakeTaskRepo) CountByBoardAndStatus(tenantID uint) ([]repository.BoardStatusCount, error) {
	f.countTenant = tenantID
	f.countCalled = true
	return f.countRows, nil
}

func uintFilter(t *testing.T, filters map[string]interface{}, key string) uint {
	t.Helper()
	v, ok := filters[key].(uint)
	if !ok {
		t.Fatalf("expected uint filter %q, got %v", key, filters[key])
	}
	return v
}

func TestTaskGetAll_SuperadminWithoutCompany_ReturnsEmptyWithoutQuery(t *testing.T) {
	repo := &fakeTaskRepo{}
	s := &taskService{repo: repo}

	tasks, total, err := s.GetAll(1, "superadmin", false, true, 0, 0, "", "", "", "", "", "", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 || total != 0 {
		t.Fatalf("superadmin without company should see nothing, got %d tasks", len(tasks))
	}
	if repo.findCalled {
		t.Fatal("repo must NOT be queried when superadmin has no company selected (tenant leak guard)")
	}
}

func TestTaskGetAll_SuperadminWithCompany_FiltersByThatTenant(t *testing.T) {
	repo := &fakeTaskRepo{}
	s := &taskService{repo: repo}

	if _, _, err := s.GetAll(1, "superadmin", false, true, 0, 5, "", "", "", "", "", "", 0, 50); err != nil {
		t.Fatal(err)
	}
	if !repo.findCalled {
		t.Fatal("repo.FindAll should have been called")
	}
	if got := uintFilter(t, repo.findFilters, "tenant_id"); got != 5 {
		t.Fatalf("tenant_id filter: want 5, got %d", got)
	}
}

func TestTaskGetAll_NonSuperadmin_FiltersByOwnTenant(t *testing.T) {
	repo := &fakeTaskRepo{}
	s := &taskService{repo: repo}

	if _, _, err := s.GetAll(1, "empleador", false, false, 3, 0, "", "", "", "", "", "", 0, 50); err != nil {
		t.Fatal(err)
	}
	if got := uintFilter(t, repo.findFilters, "tenant_id"); got != 3 {
		t.Fatalf("non-superadmin tenant_id: want 3, got %d", got)
	}
}

func TestBoardStatusCounts_SuperadminWithoutCompany_EmptyWithoutQuery(t *testing.T) {
	repo := &fakeTaskRepo{}
	s := &taskService{repo: repo}

	res, err := s.GetBoardStatusCounts(true, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatalf("want empty map, got %v", res)
	}
	if repo.countCalled {
		t.Fatal("aggregation must not run when superadmin has no company")
	}
}

func TestBoardStatusCounts_GroupsByBoardAndStatus(t *testing.T) {
	repo := &fakeTaskRepo{countRows: []repository.BoardStatusCount{
		{BoardID: 1, Status: "por_hacer", Count: 2},
		{BoardID: 1, Status: "finalizado", Count: 3},
		{BoardID: 2, Status: "por_hacer", Count: 1},
	}}
	s := &taskService{repo: repo}

	res, err := s.GetBoardStatusCounts(false, 7, 0)
	if err != nil {
		t.Fatal(err)
	}
	if repo.countTenant != 7 {
		t.Fatalf("scope tenant: want 7, got %d", repo.countTenant)
	}
	if res[1]["por_hacer"] != 2 || res[1]["finalizado"] != 3 || res[2]["por_hacer"] != 1 {
		t.Fatalf("grouping wrong: %+v", res)
	}
}

// ---------------------------------------------------------------------------
// Boards
// ---------------------------------------------------------------------------

type fakeBoardRepo struct {
	repository.BoardRepository
	filters map[string]interface{}
	called  bool
}

func (f *fakeBoardRepo) FindAll(filters map[string]interface{}) ([]models.Board, error) {
	f.filters = filters
	f.called = true
	return nil, nil
}

func TestBoardGetAll_SuperadminWithoutCompany_EmptyWithoutQuery(t *testing.T) {
	repo := &fakeBoardRepo{}
	s := &boardService{repo: repo}

	res, err := s.GetAll(1, "superadmin", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatal("superadmin without company must see no boards")
	}
	if repo.called {
		t.Fatal("repo must not be queried (would leak boards across tenants)")
	}
}

func TestBoardGetAll_SuperadminWithCompany_NoPerUserFilter(t *testing.T) {
	repo := &fakeBoardRepo{}
	s := &boardService{repo: repo}

	if _, err := s.GetAll(1, "superadmin", true, 5); err != nil {
		t.Fatal(err)
	}
	if got := uintFilter(t, repo.filters, "tenant_id"); got != 5 {
		t.Fatalf("tenant_id: want 5, got %d", got)
	}
	if _, ok := repo.filters["user_id"]; ok {
		t.Fatal("superadmin scoped to a company must NOT be restricted to boards they belong to")
	}
}

func TestBoardGetAll_Professional_RestrictedToOwnBoards(t *testing.T) {
	repo := &fakeBoardRepo{}
	s := &boardService{repo: repo}

	if _, err := s.GetAll(42, "profesional", false, 5); err != nil {
		t.Fatal(err)
	}
	if got := uintFilter(t, repo.filters, "tenant_id"); got != 5 {
		t.Fatalf("tenant_id: want 5, got %d", got)
	}
	if got := uintFilter(t, repo.filters, "user_id"); got != 42 {
		t.Fatalf("professional must be limited to their boards (user_id 42), got %d", got)
	}
}

func TestBoardGetAll_Employer_SeesWholeTenant(t *testing.T) {
	repo := &fakeBoardRepo{}
	s := &boardService{repo: repo}

	if _, err := s.GetAll(10, "empleador", false, 5); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.filters["user_id"]; ok {
		t.Fatal("employer should see all boards in their tenant (no user_id filter)")
	}
}

// ---------------------------------------------------------------------------
// Work hours
// ---------------------------------------------------------------------------

type fakeWHRepo struct {
	repository.WorkHourRepository
	findFilters    map[string]interface{}
	findCalled     bool
	summaryFilters map[string]interface{}
	summaryCalled  bool
}

func (f *fakeWHRepo) FindAll(filters map[string]interface{}, _, _ int) ([]models.WorkHour, int64, error) {
	f.findFilters = filters
	f.findCalled = true
	return nil, 0, nil
}

func (f *fakeWHRepo) GetSummary(filters map[string]interface{}) (map[string]float64, error) {
	f.summaryFilters = filters
	f.summaryCalled = true
	return map[string]float64{}, nil
}

func TestWorkHoursGetAll_SuperadminWithoutCompany_EmptyWithoutQuery(t *testing.T) {
	repo := &fakeWHRepo{}
	s := &workHourService{repo: repo}

	res, total, err := s.GetAll(1, "superadmin", true, 0, 0, "", "", "", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 || total != 0 {
		t.Fatal("superadmin without company must see no work hours")
	}
	if repo.findCalled {
		t.Fatal("repo must not be queried (tenant leak guard)")
	}
}

func TestWorkHoursGetAll_SuperadminWithCompany_FiltersByTenant(t *testing.T) {
	repo := &fakeWHRepo{}
	s := &workHourService{repo: repo}

	if _, _, err := s.GetAll(1, "superadmin", true, 0, 5, "", "", "", 0, 50); err != nil {
		t.Fatal(err)
	}
	if got := uintFilter(t, repo.findFilters, "tenant_id"); got != 5 {
		t.Fatalf("tenant_id: want 5, got %d", got)
	}
}

func TestWorkHoursSummary_SuperadminWithoutCompany_ZeroWithoutQuery(t *testing.T) {
	repo := &fakeWHRepo{}
	s := &workHourService{repo: repo}

	sum, err := s.GetSummary(1, "superadmin", true, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if sum["total_hours"] != 0 {
		t.Fatalf("want zero summary, got %v", sum)
	}
	if repo.summaryCalled {
		t.Fatal("summary must not be computed without a company scope")
	}
}

func TestWorkHoursPending_SuperadminWithCompany_FiltersByTenant(t *testing.T) {
	repo := &fakeWHRepo{}
	s := &workHourService{repo: repo}

	if _, err := s.GetPending(0, 1, "superadmin", true, false, 5, ""); err != nil {
		t.Fatal(err)
	}
	if !repo.findCalled {
		t.Fatal("pending should query with a company scope")
	}
	if got := uintFilter(t, repo.findFilters, "tenant_id"); got != 5 {
		t.Fatalf("tenant_id: want 5, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Chat (channels + DM user picker)
// ---------------------------------------------------------------------------

type fakeChannelRepo struct {
	repository.ChannelRepository
	byCompanyArg    uint
	byCompanyCalled bool
	byUserArg       uint
	byUserCalled    bool
	activeTenant    uint
	activeSuper     bool
	activeCalled    bool
}

func (f *fakeChannelRepo) GetChannelsByCompany(companyID uint) ([]models.Channel, error) {
	f.byCompanyArg = companyID
	f.byCompanyCalled = true
	return nil, nil
}

func (f *fakeChannelRepo) GetChannelsByUser(userID uint) ([]models.Channel, error) {
	f.byUserArg = userID
	f.byUserCalled = true
	return nil, nil
}

func (f *fakeChannelRepo) GetActiveUsers(tenantID uint, isSuperadmin bool) ([]models.User, error) {
	f.activeTenant = tenantID
	f.activeSuper = isSuperadmin
	f.activeCalled = true
	return nil, nil
}

func TestChannels_SuperadminWithoutCompany_EmptyWithoutQuery(t *testing.T) {
	repo := &fakeChannelRepo{}
	s := &channelService{repo: repo}

	res, err := s.GetChannels(1, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatal("superadmin without company must see no channels")
	}
	if repo.byCompanyCalled || repo.byUserCalled {
		t.Fatal("no channel query should run without a company scope")
	}
}

func TestChannels_SuperadminWithCompany_QueriesThatTenant(t *testing.T) {
	repo := &fakeChannelRepo{}
	s := &channelService{repo: repo}

	if _, err := s.GetChannels(1, true, 5); err != nil {
		t.Fatal(err)
	}
	if !repo.byCompanyCalled || repo.byCompanyArg != 5 {
		t.Fatalf("expected GetChannelsByCompany(5), called=%v arg=%d", repo.byCompanyCalled, repo.byCompanyArg)
	}
}

func TestChannels_NonSuperadmin_ScopedToOwnMembership(t *testing.T) {
	repo := &fakeChannelRepo{}
	s := &channelService{repo: repo}

	if _, err := s.GetChannels(42, false, 0); err != nil {
		t.Fatal(err)
	}
	if !repo.byUserCalled || repo.byUserArg != 42 {
		t.Fatalf("expected GetChannelsByUser(42), called=%v arg=%d", repo.byUserCalled, repo.byUserArg)
	}
}

func TestChannelUsers_SuperadminWithoutCompany_Empty(t *testing.T) {
	repo := &fakeChannelRepo{}
	s := &channelService{repo: repo}

	res, err := s.GetAllUsers(0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatal("superadmin without company must get no users for DMs")
	}
	if repo.activeCalled {
		t.Fatal("user query should not run without a company scope")
	}
}

func TestChannelUsers_SuperadminWithCompany_ScopedToTenant(t *testing.T) {
	repo := &fakeChannelRepo{}
	s := &channelService{repo: repo}

	if _, err := s.GetAllUsers(0, true, 5); err != nil {
		t.Fatal(err)
	}
	if !repo.activeCalled || repo.activeTenant != 5 || repo.activeSuper {
		t.Fatalf("expected GetActiveUsers(5,false), tenant=%d super=%v", repo.activeTenant, repo.activeSuper)
	}
}

func TestChannelUsers_NonSuperadmin_UsesOwnTenant(t *testing.T) {
	repo := &fakeChannelRepo{}
	s := &channelService{repo: repo}

	if _, err := s.GetAllUsers(3, false, 0); err != nil {
		t.Fatal(err)
	}
	if !repo.activeCalled || repo.activeTenant != 3 {
		t.Fatalf("expected GetActiveUsers(3,...), tenant=%d", repo.activeTenant)
	}
}
