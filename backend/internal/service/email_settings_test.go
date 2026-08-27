package service

import (
	"errors"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// fakeSettingRepo simula la tabla de interruptores. failList reproduce un fallo
// de base, que es el caso donde antes se ignoraba el apagado en silencio.
type fakeSettingRepo struct {
	repository.EmailSettingRepository
	rows     []models.EmailSetting
	failList bool
	upserted []models.EmailSetting
}

func (f *fakeSettingRepo) List() ([]models.EmailSetting, error) {
	if f.failList {
		return nil, errors.New("la base no responde")
	}
	return f.rows, nil
}

func (f *fakeSettingRepo) Upsert(s *models.EmailSetting) error {
	f.upserted = append(f.upserted, *s)
	f.rows = append(f.rows, *s)
	return nil
}

func TestEnabled_SinFilaGuardadaEstaEncendido(t *testing.T) {
	// La ausencia de fila significa "nunca se tocó": por defecto todo sale.
	s := NewEmailSettingsService(&fakeSettingRepo{}, nil)
	if !s.Enabled(EmailKindInactivityAlert) {
		t.Fatal("sin fila guardada el correo debe estar encendido")
	}
}

func TestEnabled_RespetaElApagado(t *testing.T) {
	repo := &fakeSettingRepo{rows: []models.EmailSetting{{Key: EmailKindSurveyInvite, Enabled: false}}}
	s := NewEmailSettingsService(repo, nil)
	if s.Enabled(EmailKindSurveyInvite) {
		t.Fatal("un correo apagado no debe poder salir")
	}
	// Apagar uno no apaga los demás.
	if !s.Enabled(EmailKindInactivityAlert) {
		t.Fatal("apagar un tipo no debe afectar a los otros")
	}
}

// Este es el caso que dejaba salir correos apagados sin dejar rastro: si no se
// puede leer la tabla, antes se respondía "encendido" para TODO.
func TestEnabled_AnteFalloDeBaseSoloPasanLosEsenciales(t *testing.T) {
	s := NewEmailSettingsService(&fakeSettingRepo{failList: true}, nil)

	// Esenciales: sin ellos alguien queda fuera de la plataforma, así que ante
	// la duda se envían.
	for _, kind := range []string{EmailKindPasswordReset, EmailKindAccountSetup} {
		if !s.Enabled(kind) {
			t.Errorf("%s es esencial: ante un fallo de base debe poder salir", kind)
		}
	}

	// El resto se frena: si alguien lo apagó a propósito, un error de base no
	// puede ser la razón por la que vuelve a salir.
	for _, kind := range []string{EmailKindInactivityAlert, EmailKindCampaign, EmailKindManualComposer} {
		if s.Enabled(kind) {
			t.Errorf("%s no es esencial: ante un fallo de base no debe salir", kind)
		}
	}
}

func TestSetEnabled_GuardaYSurteEfectoDeInmediato(t *testing.T) {
	repo := &fakeSettingRepo{}
	s := NewEmailSettingsService(repo, nil)

	// Se consulta antes para dejar el caché cargado: apagar tiene que invalidarlo.
	if !s.Enabled(EmailKindInactivityAlert) {
		t.Fatal("debía arrancar encendido")
	}
	if err := s.SetEnabled(EmailKindInactivityAlert, false, 7); err != nil {
		t.Fatalf("SetEnabled falló: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].Enabled {
		t.Fatalf("debía guardarse enabled=false; llegó %+v", repo.upserted)
	}
	if s.Enabled(EmailKindInactivityAlert) {
		t.Fatal("tras apagarlo no debe seguir saliendo (caché sin invalidar)")
	}
}

func TestSetEnabled_RechazaTipoDesconocido(t *testing.T) {
	repo := &fakeSettingRepo{}
	s := NewEmailSettingsService(repo, nil)
	if err := s.SetEnabled("inventado", false, 1); err == nil {
		t.Fatal("un tipo fuera del catálogo debe rechazarse")
	}
	if len(repo.upserted) != 0 {
		t.Fatal("no debió guardarse nada")
	}
}

// El catálogo es el contrato con el panel: si un tipo del código no está ahí,
// no aparece en Configuración y nadie puede apagarlo.
func TestCatalogo_CubreTodosLosTiposEnUso(t *testing.T) {
	enUso := []string{
		EmailKindInactivityAlert, EmailKindWorkHourReport,
		EmailKindSupportTicket, EmailKindPasswordReset, EmailKindAccountSetup,
		EmailKindAccessCredentials, EmailKindInductionInvite, EmailKindIncidentBroadcast,
		EmailKindTestimonialRequest,
		EmailKindSurveyInvite, EmailKindTicketReply, EmailKindManualComposer,
		EmailKindCampaign,
	}
	for _, k := range enUso {
		if !isKnownEmailKind(k) {
			t.Errorf("%q se usa en el código pero no está en el catálogo", k)
		}
	}
}

// La puerta de Brevo: un tipo apagado devuelve el centinela, no nil. Devolver
// nil hacía que una campaña se reportara "enviada" sin haber salido.
func TestBrevoGate_TipoApagadoDevuelveElCentinela(t *testing.T) {
	settings := NewEmailSettingsService(
		&fakeSettingRepo{rows: []models.EmailSetting{{Key: EmailKindCampaign, Enabled: false}}}, nil)
	brevo := NewBrevoService()
	brevo.SetKindGate(settings.Enabled)

	err := brevo.SendEmailKind(EmailKindCampaign, "a@x.com", "A", "Asunto", "<p>hola</p>")
	if !errors.Is(err, ErrEmailKindDisabled) {
		t.Fatalf("se esperaba ErrEmailKindDisabled; llegó %v", err)
	}

	// Y la variante con etiquetas, que es la que usan las campañas.
	err = brevo.SendEmailKindTagged(EmailKindCampaign, "a@x.com", "A", "Asunto", "<p>hola</p>", []string{"t"})
	if !errors.Is(err, ErrEmailKindDisabled) {
		t.Fatalf("la variante con etiquetas también debe frenarse; llegó %v", err)
	}
}
