package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// La encuesta sin sesión se apoya en dos cosas y ninguna se ve a simple vista:
// que el enlace no se pueda fabricar ni estirar, y que la puerta esté abierta
// SOLO para las cuentas empresa. Si alguna de las dos se rompiera en un cambio
// futuro, se rompería en silencio —el enlace seguiría abriendo la encuesta—,
// así que van fijadas aquí.

type repoDeEnlace struct {
	repository.SurveyRepository
	survey *models.Survey
}

func (r *repoDeEnlace) GetSurveyByID(id uint) (*models.Survey, error) {
	if r.survey == nil || r.survey.ID != id {
		return nil, fmt.Errorf("no existe")
	}
	return r.survey, nil
}

func (r *repoDeEnlace) GetResponseByUser(_, _ uint) (*models.SurveyResponse, error) {
	return nil, nil
}

type repoDeUsuarios struct {
	repository.UserRepository
	user *models.User
}

func (r *repoDeUsuarios) GetByID(id uint) (*models.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, fmt.Errorf("no existe")
	}
	return r.user, nil
}

// encuestaActiva arma una encuesta enviada a la empresa con ID 7.
func encuestaActiva() *models.Survey {
	return &models.Survey{
		ID:            3,
		Status:        models.SurveyStatusActive,
		RecipientList: "[7]",
		Questions: []models.SurveyQuestion{
			{ID: 11, Text: "¿Qué tal el servicio?", Type: models.QuestionTypeRating, OrderIndex: 0},
			{ID: 12, Text: "Cuéntanos por qué", Type: models.QuestionTypeText, OrderIndex: 1},
		},
	}
}

func handlerDeEnlace(survey *models.Survey, user *models.User) *SurveyHandler {
	return &SurveyHandler{
		repo:     &repoDeEnlace{survey: survey},
		userRepo: &repoDeUsuarios{user: user},
	}
}

func TestTokenDeEnlace_IdaYVuelta(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	token := newSurveyLinkToken(3, 7, time.Now().Add(time.Hour))
	surveyID, userID, err := parseSurveyLinkToken(token)
	if err != nil {
		t.Fatalf("un token recién creado no debería fallar: %v", err)
	}
	if surveyID != 3 || userID != 7 {
		t.Fatalf("el token devolvió encuesta %d y usuario %d, se esperaba 3 y 7", surveyID, userID)
	}
}

// Cambiar el usuario del enlace es el ataque obvio: respondería la encuesta en
// nombre de otra empresa.
func TestTokenDeEnlace_NoSeDejaReescribir(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	token := newSurveyLinkToken(3, 7, time.Now().Add(time.Hour))
	partes := strings.Split(token, ".")
	partes[1] = "8"

	if _, _, err := parseSurveyLinkToken(strings.Join(partes, ".")); err != errSurveyLinkInvalid {
		t.Fatalf("se esperaba enlace inválido, se obtuvo %v", err)
	}
}

// Estirar la fecha a mano tiene que seguir siendo "inválido" y no "vencido":
// un mensaje distinto le confirmaría a quien lo intenta que el resto del token
// estaba bien.
func TestTokenDeEnlace_LaFechaEstiradaEsInvalidaNoVencida(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	token := newSurveyLinkToken(3, 7, time.Now().Add(-time.Hour))
	partes := strings.Split(token, ".")
	partes[2] = fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix())

	if _, _, err := parseSurveyLinkToken(strings.Join(partes, ".")); err != errSurveyLinkInvalid {
		t.Fatalf("se esperaba enlace inválido, se obtuvo %v", err)
	}
}

func TestTokenDeEnlace_Vencido(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	token := newSurveyLinkToken(3, 7, time.Now().Add(-time.Minute))
	if _, _, err := parseSurveyLinkToken(token); err != errSurveyLinkExpired {
		t.Fatalf("se esperaba enlace vencido, se obtuvo %v", err)
	}
}

// El corazón de la función: responder sin sesión es SOLO para empresas.
func TestEnlaceSinSesion_SoloParaEmpresas(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	casos := []struct {
		rol     models.UserType
		abre    bool
		nombre  string
		esperar error
	}{
		{rol: models.UserTypeEmployer, abre: true, nombre: "empresa"},
		{rol: models.UserTypeProfessional, nombre: "profesional", esperar: errSurveyLinkNotCompany},
		{rol: models.UserTypeSuperadmin, nombre: "superadmin", esperar: errSurveyLinkNotCompany},
		{rol: models.UserTypeCustomerSuccess, nombre: "customer success", esperar: errSurveyLinkNotCompany},
	}

	for _, caso := range casos {
		h := handlerDeEnlace(encuestaActiva(), &models.User{ID: 7, Name: "Cliente", UserType: caso.rol})
		_, _, err := h.resolveSurveyLink(newSurveyLinkToken(3, 7, time.Now().Add(time.Hour)))

		if caso.abre && err != nil {
			t.Errorf("%s: el enlace debería abrir, falló con %v", caso.nombre, err)
		}
		if !caso.abre && err != caso.esperar {
			t.Errorf("%s: se esperaba %v, se obtuvo %v", caso.nombre, caso.esperar, err)
		}
	}
}

// Un token de una empresa que no está en la lista de destinatarios no vale,
// aunque la firma sea buena.
func TestEnlaceSinSesion_SoloDestinatarios(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	h := handlerDeEnlace(encuestaActiva(), &models.User{ID: 9, UserType: models.UserTypeEmployer})
	if _, _, err := h.resolveSurveyLink(newSurveyLinkToken(3, 9, time.Now().Add(time.Hour))); err != errSurveyLinkInvalid {
		t.Fatalf("se esperaba enlace inválido, se obtuvo %v", err)
	}
}

// Una encuesta cerrada deja de responderse aunque el enlace siga vigente.
func TestEnlaceSinSesion_EncuestaCerrada(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	survey := encuestaActiva()
	survey.Status = models.SurveyStatusClosed
	h := handlerDeEnlace(survey, &models.User{ID: 7, UserType: models.UserTypeEmployer})

	if _, _, err := h.resolveSurveyLink(newSurveyLinkToken(3, 7, time.Now().Add(time.Hour))); err != errSurveyLinkClosed {
		t.Fatalf("se esperaba encuesta cerrada, se obtuvo %v", err)
	}
}

func TestBotonesDelCorreo_SalenDeLaPrimeraValoracion(t *testing.T) {
	t.Setenv("JWT_SECRET", "una-llave-larga-para-la-prueba")

	survey := encuestaActiva()
	pregunta, opciones := surveyQuickOptions(survey, "https://api.obertrack.com", "TOKEN")

	if pregunta != "¿Qué tal el servicio?" {
		t.Errorf("el correo preguntó %q", pregunta)
	}
	if len(opciones) != 5 {
		t.Fatalf("se esperaban 5 botones, hay %d", len(opciones))
	}
	if opciones[3].Label != "4" {
		t.Errorf("el cuarto botón dice %q", opciones[3].Label)
	}
	for _, fragmento := range []string{"/api/surveys/3/quick-response", "t=TOKEN", "q_id=11", "score=4"} {
		if !strings.Contains(opciones[3].Href, fragmento) {
			t.Errorf("el enlace del botón no lleva %q: %s", fragmento, opciones[3].Href)
		}
	}
}

// Sin pregunta de valoración no hay nada que pulsar: el correo tiene que salir
// solo con el enlace en vez de con botones que no registrarían nada.
func TestBotonesDelCorreo_SinValoracionNoHayBotones(t *testing.T) {
	survey := &models.Survey{
		ID:        3,
		Questions: []models.SurveyQuestion{{ID: 12, Type: models.QuestionTypeText}},
	}
	if _, opciones := surveyQuickOptions(survey, "https://api.obertrack.com", "TOKEN"); len(opciones) != 0 {
		t.Fatalf("se esperaban 0 botones, hay %d", len(opciones))
	}
}

// Una escala que se sale del 1-10 no se puede contestar de un clic: el endpoint
// la rechazaría y el correo prometería algo que no ocurre.
func TestBotonesDelCorreo_EscalaFueraDeRango(t *testing.T) {
	survey := &models.Survey{
		ID: 3,
		Questions: []models.SurveyQuestion{
			{ID: 12, Type: models.QuestionTypeLinearScale, Options: `{"min":0,"max":10}`},
		},
	}
	if _, opciones := surveyQuickOptions(survey, "https://api.obertrack.com", "TOKEN"); len(opciones) != 0 {
		t.Fatalf("se esperaban 0 botones para una escala 0-10, hay %d", len(opciones))
	}
}

// --- Una persona, una participación ---

// repoDeParticipacion recuerda qué se le pidió: crear una participación nueva o
// reemplazar la que ya había.
type repoDeParticipacion struct {
	repository.SurveyRepository
	existente  *models.SurveyResponse
	errAlCrear error
	// apareceTrasFallar simula la carrera: la participación no existía al
	// consultar, pero sí cuando el insert choca con el índice único.
	apareceTrasFallar *models.SurveyResponse
	consultas         int
	creadas           int
	reemplazadas      uint
}

func (r *repoDeParticipacion) GetResponseByUser(_, _ uint) (*models.SurveyResponse, error) {
	r.consultas++
	if r.existente != nil {
		return r.existente, nil
	}
	if r.consultas > 1 {
		return r.apareceTrasFallar, nil
	}
	return nil, nil
}

func (r *repoDeParticipacion) CreateResponse(_ *models.SurveyResponse) error {
	r.creadas++
	return r.errAlCrear
}

func (r *repoDeParticipacion) ReplaceResponseAnswers(responseID uint, _ []models.SurveyAnswer, _ time.Time) error {
	r.reemplazadas = responseID
	return nil
}

// Responder por segunda vez tiene que ACTUALIZAR. Antes creaba otra fila: quien
// abría la encuesta en dos pestañas contaba como dos personas y el panel decía
// "Respuestas: 2" con un solo participante.
func TestUnaSolaParticipacion_ElSegundoEnvioReemplaza(t *testing.T) {
	repo := &repoDeParticipacion{existente: &models.SurveyResponse{ID: 8}}
	h := &SurveyHandler{repo: repo}

	if err := h.saveOneResponsePerUser(3, 7, []models.SurveyAnswer{{QuestionID: 11, NumberValue: 4}}); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if repo.creadas != 0 {
		t.Errorf("creó %d participaciones nuevas, no debía crear ninguna", repo.creadas)
	}
	if repo.reemplazadas != 8 {
		t.Errorf("reemplazó la participación %d, se esperaba la 8", repo.reemplazadas)
	}
}

func TestUnaSolaParticipacion_LaPrimeraVezSeCrea(t *testing.T) {
	repo := &repoDeParticipacion{}
	h := &SurveyHandler{repo: repo}

	if err := h.saveOneResponsePerUser(3, 7, []models.SurveyAnswer{{QuestionID: 11, NumberValue: 4}}); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if repo.creadas != 1 {
		t.Errorf("creó %d participaciones, se esperaba 1", repo.creadas)
	}
	if repo.reemplazadas != 0 {
		t.Errorf("no había nada que reemplazar y reemplazó la %d", repo.reemplazadas)
	}
}

// Dos pestañas enviando a la vez: el índice único frena al segundo insert. Quien
// respondió no tiene la culpa ni forma de saberlo, así que su envío se guarda
// encima en lugar de devolverle un error.
func TestUnaSolaParticipacion_DosPestanasALaVez(t *testing.T) {
	repo := &repoDeParticipacion{
		errAlCrear:        fmt.Errorf("duplicate key value violates unique constraint"),
		apareceTrasFallar: &models.SurveyResponse{ID: 9},
	}
	h := &SurveyHandler{repo: repo}

	if err := h.saveOneResponsePerUser(3, 7, []models.SurveyAnswer{{QuestionID: 11, NumberValue: 4}}); err != nil {
		t.Fatalf("la carrera no debería llegar a quien responde: %v", err)
	}
	if repo.reemplazadas != 9 {
		t.Errorf("reemplazó la participación %d, se esperaba la 9 (la que ganó la carrera)", repo.reemplazadas)
	}
}
