package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Insignias: se otorgan al aprobar, no se duplican y los méritos dependen de
// cómo se hizo el programa.

type fakeBadgeRepo struct {
	repository.BadgeRepository
	awarded []models.UserBadge
}

func (f *fakeBadgeRepo) Award(b *models.UserBadge) (bool, error) {
	for _, a := range f.awarded {
		if a.UserID == b.UserID && a.SourceKey == b.SourceKey {
			return false, nil
		}
	}
	b.ID = uint(len(f.awarded) + 1)
	f.awarded = append(f.awarded, *b)
	return true, nil
}

func (f *fakeBadgeRepo) ListByUser(userID uint) ([]models.UserBadge, error) {
	var out []models.UserBadge
	for _, a := range f.awarded {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func kinds(badges []models.UserBadge) []string {
	out := make([]string, 0, len(badges))
	for _, b := range badges {
		out = append(out, b.SourceKey)
	}
	return out
}

func TestEarnedMerits(t *testing.T) {
	passed := func(attempts int, best float64) models.InductionInviteBlock {
		return models.InductionInviteBlock{Status: models.InductionPassed, Attempts: attempts, BestScore: best}
	}
	cases := []struct {
		name   string
		blocks []models.InductionInviteBlock
		want   []string
	}{
		{"a la primera e impecable", []models.InductionInviteBlock{passed(1, 100), passed(1, 100)}, []string{MeritFirstTry, MeritPerfect}},
		{"solo a la primera", []models.InductionInviteBlock{passed(1, 80), passed(1, 100)}, []string{MeritFirstTry}},
		{"solo impecable", []models.InductionInviteBlock{passed(2, 100), passed(1, 100)}, []string{MeritPerfect}},
		{"ninguno", []models.InductionInviteBlock{passed(2, 70)}, nil},
		{"sin bloques", nil, nil},
		{"con un bloque sin aprobar no hay merito", []models.InductionInviteBlock{passed(1, 100), {Status: models.InductionPending}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := earnedMerits(tc.blocks)
			if len(got) != len(tc.want) {
				t.Fatalf("earnedMerits = %v, esperaba %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("earnedMerits = %v, esperaba %v", got, tc.want)
				}
			}
		})
	}
}

func TestSubmit_AprobarUnBloqueOtorgaSuInsignia(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	badges := &fakeBadgeRepo{}
	svc.badgeRepo = badges
	pendingInvite(repo, 3)
	repo.blocks[11] = &models.InductionBlock{ID: 11, Name: "Bienvenida", SurveyID: 7, BadgeTitle: "Bienvenido a bordo", BadgeIcon: "Rocket", BadgeColor: "sky"}

	res, err := svc.Submit("tok", 11, []SubmittedAnswer{{QuestionID: 71, Value: "a"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if len(res.BadgesEarned) != 1 {
		t.Fatalf("se esperaba 1 insignia, got %v", kinds(res.BadgesEarned))
	}
	b := res.BadgesEarned[0]
	// El nombre propio de la insignia manda sobre el del bloque.
	if b.Kind != models.BadgeKindBlock || b.SourceKey != "block:11" || b.Title != "Bienvenido a bordo" {
		t.Fatalf("insignia mal formada: %+v", b)
	}
	// El aspecto sale de la definición viva del bloque.
	if b.Icon != "Rocket" || b.Color != "sky" {
		t.Fatalf("debe usar el icono y color del bloque: %+v", b)
	}
	if b.Score != 100 || b.ProgramName != "Por defecto" {
		t.Fatalf("debe llevar puntaje y programa: %+v", b)
	}
}

// Un bloque sin definición viva (borrado) igual da insignia, con el aspecto
// de respaldo.
func TestSubmit_BloqueBorradoUsaAspectoDeRespaldo(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	svc.badgeRepo = &fakeBadgeRepo{}
	pendingInvite(repo, 3)

	res, err := svc.Submit("tok", 11, []SubmittedAnswer{{QuestionID: 71, Value: "a"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if len(res.BadgesEarned) != 1 || res.BadgesEarned[0].Icon != DefaultBlockBadgeIcon {
		t.Fatalf("esperaba la insignia de respaldo: %v", res.BadgesEarned)
	}
}

func TestSubmit_CompletarElProgramaOtorgaProgramaYMeritos(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	badges := &fakeBadgeRepo{}
	svc.badgeRepo = badges
	pendingInvite(repo, 3)
	programID := uint(1)
	repo.invite.ProgramID = &programID
	repo.inviteBlocks[0].Status = models.InductionPassed
	repo.inviteBlocks[0].Attempts = 1
	repo.inviteBlocks[0].BestScore = 100

	res, err := svc.Submit("tok", 12, []SubmittedAnswer{{QuestionID: 81, Value: "b"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := kinds(res.BadgesEarned)
	want := []string{"block:12", "program:1", "merit:first_try:1", "merit:perfect:1"}
	if len(got) != len(want) {
		t.Fatalf("insignias = %v, esperaba %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("insignias = %v, esperaba %v", got, want)
		}
	}
	// La del programa promedia los bloques.
	if res.BadgesEarned[1].Kind != models.BadgeKindProgram || res.BadgesEarned[1].Score != 100 {
		t.Fatalf("insignia de programa mal formada: %+v", res.BadgesEarned[1])
	}
}

// Fallar antes quita el mérito "a la primera" pero no la insignia del programa.
func TestSubmit_ConFallosNoHayMeritoALaPrimera(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	svc.badgeRepo = &fakeBadgeRepo{}
	pendingInvite(repo, 3)
	programID := uint(1)
	repo.invite.ProgramID = &programID
	repo.inviteBlocks[0].Status = models.InductionPassed
	repo.inviteBlocks[0].Attempts = 2
	repo.inviteBlocks[0].BestScore = 100

	res, err := svc.Submit("tok", 12, []SubmittedAnswer{{QuestionID: 81, Value: "b"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := kinds(res.BadgesEarned)
	if len(got) != 3 || got[1] != "program:1" || got[2] != "merit:perfect:1" {
		t.Fatalf("insignias = %v, esperaba bloque, programa e impecable", got)
	}
}

// Reaprobar tras un reinicio no duplica: la insignia ya ganada no vuelve a
// salir en el resultado.
func TestSubmit_NoDuplicaInsigniasYaGanadas(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	badges := &fakeBadgeRepo{awarded: []models.UserBadge{{ID: 1, UserID: 5, SourceKey: "block:11"}}}
	svc.badgeRepo = badges
	pendingInvite(repo, 3)

	res, err := svc.Submit("tok", 11, []SubmittedAnswer{{QuestionID: 71, Value: "a"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if len(res.BadgesEarned) != 0 {
		t.Fatalf("no debía otorgarse nada nuevo: %v", kinds(res.BadgesEarned))
	}
	if len(badges.awarded) != 1 {
		t.Fatalf("la tabla no debe crecer: %d", len(badges.awarded))
	}
}

// Sin repositorio de insignias (pruebas viejas, arranque parcial) la
// aprobación sigue funcionando: las insignias son best-effort.
func TestSubmit_SinRepositorioDeInsigniasIgualAprueba(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)
	res, err := svc.Submit("tok", 11, []SubmittedAnswer{{QuestionID: 71, Value: "a"}})
	if err != nil || !res.Passed {
		t.Fatalf("debe aprobar sin insignias: %+v %v", res, err)
	}
	if len(res.BadgesEarned) != 0 {
		t.Fatalf("sin repositorio no hay insignias: %v", res.BadgesEarned)
	}
}

func TestListBadges_IncluyeLoGanadoYLoPendiente(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	svc.badgeRepo = &fakeBadgeRepo{awarded: []models.UserBadge{
		{ID: 1, UserID: 5, Kind: models.BadgeKindBlock, SourceKey: "block:11", Title: "Bienvenida"},
		{ID: 2, UserID: 9, Kind: models.BadgeKindBlock, SourceKey: "block:11", Title: "De otra persona"},
	}}
	pendingInvite(repo, 3)
	programID := uint(1)
	repo.invite.ProgramID = &programID
	repo.inviteBlocks[0].Status = models.InductionPassed
	repo.blocks[12] = &models.InductionBlock{ID: 12, Name: "Seguridad", SurveyID: 8, BadgeIcon: "Shield", BadgeColor: "indigo"}

	overview, err := svc.ListBadges(5)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(overview.Earned) != 1 || overview.Earned[0].Title != "Bienvenida" {
		t.Fatalf("ganadas = %+v", overview.Earned)
	}
	// Pendientes: el bloque 2 (con su aspecto) y el programa.
	if len(overview.Pending) != 2 {
		t.Fatalf("pendientes = %+v", overview.Pending)
	}
	if overview.Pending[0].Title != "Seguridad" || overview.Pending[0].Icon != "Shield" || overview.Pending[0].Color != "indigo" {
		t.Fatalf("pendiente de bloque mal formado: %+v", overview.Pending[0])
	}
	if overview.Pending[1].Kind != models.BadgeKindProgram || overview.Pending[1].Title != "Por defecto" {
		t.Fatalf("pendiente de programa mal formado: %+v", overview.Pending[1])
	}
}

// Una inducción ya resuelta no tiene pendientes.
func TestListBadges_SinPendientesSiYaSeResolvio(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	svc.badgeRepo = &fakeBadgeRepo{}
	pendingInvite(repo, 3)
	repo.invite.Status = models.InductionPassed

	overview, err := svc.ListBadges(5)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(overview.Pending) != 0 || len(overview.Earned) != 0 {
		t.Fatalf("esperaba vacío: %+v", overview)
	}
}

func TestNormalizeBadgeAspect(t *testing.T) {
	if normalizeBadgeIcon("Rocket", "Award") != "Rocket" || normalizeBadgeIcon("<script>", "Award") != "Award" {
		t.Fatal("el icono debe validarse contra el set cerrado")
	}
	if normalizeBadgeColor("gold", "orchid") != "gold" || normalizeBadgeColor("#ff0000", "orchid") != "orchid" {
		t.Fatal("el color debe validarse contra la paleta")
	}
}
