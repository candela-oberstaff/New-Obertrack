package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// Bloqueo del acceso: solo el ingreso (Obersuite o casilla marcada) deja a la
// persona sin entrar; una capacitación a quien ya trabaja no toca el acceso.

func TestInvite_CapacitacionSinBloqueoNoTocaElAcceso(t *testing.T) {
	pro := professional(5)
	pro.OnboardingStatus = models.OnboardingNotRequired
	svc, repo, userRepo := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Repaso")

	if err := svc.Invite(5, 0, false); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created == nil || repo.created.GatesAccess {
		t.Fatalf("la invitación debe quedar sin bloqueo: %+v", repo.created)
	}
	if userRepo.updates[5] != nil {
		t.Fatalf("el acceso no debe tocarse: %v", userRepo.updates[5])
	}
}

// Quien ya aprobó su ingreso puede recibir una capacitación (sin bloqueo),
// que antes se rechazaba de plano.
func TestInvite_QuienYaAproboPuedeRecibirCapacitacion(t *testing.T) {
	pro := professional(5)
	pro.OnboardingStatus = models.OnboardingPassed
	svc, repo, _ := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Repaso")

	if err := svc.Invite(5, 0, false); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created == nil || repo.created.GatesAccess {
		t.Fatalf("debe emitirse sin bloqueo: %+v", repo.created)
	}
	// Pero bloquearle el acceso sí sigue rechazado.
	repo.created = nil
	if err := svc.Invite(5, 0, true); err == nil {
		t.Fatal("bloquear a quien ya aprobó su ingreso debe rechazarse")
	}
}

// Quien nunca tuvo acceso sigue sin tenerlo aunque Soporte no marque la
// casilla: no hay capacitación "abierta" para alguien que no puede entrar.
func TestInvite_SinAccesoPrevioSeFuerzaElBloqueo(t *testing.T) {
	pro := professional(5)
	pro.OnboardingStatus = models.OnboardingBlocked
	svc, repo, userRepo := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Ingreso")

	if err := svc.Invite(5, 0, false); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created == nil || !repo.created.GatesAccess {
		t.Fatalf("debe forzarse el bloqueo: %+v", repo.created)
	}
	if userRepo.updates[5]["onboarding_status"] != models.OnboardingPending {
		t.Fatalf("debe quedar pendiente de ingreso: %v", userRepo.updates[5])
	}
}

func TestInviteIfEnabled_ElIngresoDesdeObersuiteBloquea(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")

	if _, err := svc.InviteIfEnabled(professional(5)); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if !repo.created.GatesAccess || userRepo.updates[5]["onboarding_status"] != models.OnboardingPending {
		t.Fatalf("el ingreso siempre bloquea: %+v %v", repo.created, userRepo.updates[5])
	}
}

func TestSubmit_CompletarCapacitacionNoHabilitaNiNotificaAcceso(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	tickets := &fakeInductionTicketSvc{}
	svc.ticketSvc = tickets
	svc.badgeRepo = &fakeBadgeRepo{}
	pendingInvite(repo, 3)
	repo.invite.GatesAccess = false
	programID := uint(1)
	repo.invite.ProgramID = &programID
	repo.inviteBlocks[0].Status = models.InductionPassed
	repo.inviteBlocks[0].Attempts = 1
	repo.inviteBlocks[0].BestScore = 100

	res, err := svc.Submit("tok", 12, []SubmittedAnswer{{QuestionID: 81, Value: "b"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !res.Completed || res.Status != models.InductionPassed {
		t.Fatalf("debe completar: %+v", res)
	}
	if userRepo.updates[5] != nil {
		t.Fatalf("una capacitación no toca el acceso: %v", userRepo.updates[5])
	}
	if len(tickets.closed) != 0 {
		t.Fatal("no hay ticket de incorporación que cerrar en una capacitación")
	}
	// Las insignias sí se ganan igual.
	if len(res.BadgesEarned) == 0 {
		t.Fatal("la capacitación debe dar insignias")
	}
}

func TestSubmit_AgotarIntentosEnCapacitacionNoBloqueaElAcceso(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	tickets := &fakeInductionTicketSvc{}
	svc.ticketSvc = tickets
	pendingInvite(repo, 1)
	repo.invite.GatesAccess = false

	res, err := svc.Submit("tok", 11, []SubmittedAnswer{{QuestionID: 71, Value: "mal"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if res.Status != models.InductionBlocked || repo.invite.Status != models.InductionBlocked {
		t.Fatalf("la capacitación sí queda bloqueada: %+v", res)
	}
	if userRepo.updates[5] != nil {
		t.Fatalf("el acceso no debe bloquearse: %v", userRepo.updates[5])
	}
	// Soporte se entera igual, pero sabiendo que el acceso no cambió.
	if len(tickets.alerts) != 1 || tickets.alerts[0].GatesAccess {
		t.Fatalf("la alerta debe salir marcada como capacitación: %+v", tickets.alerts)
	}
}

func TestReset_ConservaElModoSinBloqueo(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	repo.invite = &models.InductionInvite{ID: 1, UserID: 5, Token: "viejo", Status: models.InductionBlocked, MaxAttempts: 3, GatesAccess: false}

	if err := svc.Reset(5); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if userRepo.updates[5] != nil {
		t.Fatalf("reiniciar una capacitación no bloquea el acceso: %v", userRepo.updates[5])
	}
	if repo.inviteUpdate["status"] != models.InductionPending || repo.resetInvite != 1 {
		t.Fatalf("el reinicio debe reponer la capacitación: %v", repo.inviteUpdate)
	}
}

func TestMyInduction_DevuelveLaPendienteConEnlace(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)
	repo.invite.GatesAccess = false
	repo.inviteBlocks[0].Status = models.InductionPassed

	view, err := svc.MyInduction(5)
	if err != nil || view == nil {
		t.Fatalf("esperaba la capacitación pendiente: %+v %v", view, err)
	}
	if view.Token != "tok" || view.TotalBlocks != 2 || view.CompletedBlocks != 1 || view.GatesAccess {
		t.Fatalf("vista mal formada: %+v", view)
	}

	repo.invite.Status = models.InductionPassed
	if view, _ := svc.MyInduction(5); view != nil {
		t.Fatalf("una capacitación terminada no está pendiente: %+v", view)
	}
}
