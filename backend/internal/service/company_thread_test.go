package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// fakeThreadRepo implementa solo lo que toca el servicio. Embebe la interfaz
// para que cualquier método no previsto reviente en el test.
type fakeThreadRepo struct {
	repository.CompanyThreadRepository

	// belongs decide si el evento es de la empresa; se consulta con la pareja
	// (eventID, companyID) para poder comprobar que el servicio pregunta bien.
	belongs    map[[2]uint]bool
	belongsErr error

	comments    []*models.CompanyEventComment
	attachments []*models.CompanyEventAttachment

	attachmentCount int64

	updatedContent string
	updateRows     int64
	deleteRows     int64
}

func (f *fakeThreadRepo) EventBelongsTo(eventID, companyID uint) (bool, error) {
	if f.belongsErr != nil {
		return false, f.belongsErr
	}
	return f.belongs[[2]uint{eventID, companyID}], nil
}

func (f *fakeThreadRepo) CreateComment(c *models.CompanyEventComment) error {
	f.comments = append(f.comments, c)
	return nil
}

func (f *fakeThreadRepo) UpdateComment(_, _ uint, content string, _ time.Time) (int64, error) {
	f.updatedContent = content
	return f.updateRows, nil
}

func (f *fakeThreadRepo) DeleteComment(uint, uint) (int64, error) { return f.deleteRows, nil }

func (f *fakeThreadRepo) CreateAttachment(a *models.CompanyEventAttachment) error {
	f.attachments = append(f.attachments, a)
	return nil
}

func (f *fakeThreadRepo) CountAttachmentsForEvent(uint) (int64, error) {
	return f.attachmentCount, nil
}

func newThreadSvc(belongs ...[2]uint) (*companyThreadService, *fakeThreadRepo) {
	m := map[[2]uint]bool{}
	for _, b := range belongs {
		m[b] = true
	}
	repo := &fakeThreadRepo{belongs: m}
	return &companyThreadService{repo: repo}, repo
}

// --- Aislamiento entre empresas ---------------------------------------------

// Lo más importante de todo el servicio: mandar el id de una entrada de OTRA
// empresa no puede dejar escribir en ella.
func TestAddComment_RechazaEventoDeOtraEmpresa(t *testing.T) {
	svc, repo := newThreadSvc([2]uint{50, 7}) // el evento 50 es de la empresa 7

	if _, err := svc.AddComment(9 /*otra empresa*/, 50, 1, "hola"); !errors.Is(err, ErrEventNotInCompany) {
		t.Fatalf("se esperaba ErrEventNotInCompany, got %v", err)
	}
	if len(repo.comments) != 0 {
		t.Fatal("no debería escribirse nada")
	}
}

func TestAddAttachment_RechazaEventoDeOtraEmpresa(t *testing.T) {
	svc, repo := newThreadSvc([2]uint{50, 7})

	_, err := svc.AddAttachment(9, 50, 1, nil, "captura.png", "1_2_captura.png", 100, "image/png")
	if !errors.Is(err, ErrEventNotInCompany) {
		t.Fatalf("se esperaba ErrEventNotInCompany, got %v", err)
	}
	if len(repo.attachments) != 0 {
		t.Fatal("no debería escribirse nada")
	}
}

// --- Comentarios -------------------------------------------------------------

func TestAddComment_GuardaFirmadoYSaneado(t *testing.T) {
	svc, repo := newThreadSvc([2]uint{50, 7})

	c, err := svc.AddComment(7, 50, 42, "  Reproducido en staging  ")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.Content != "Reproducido en staging" {
		t.Fatalf("content: got %q", c.Content)
	}
	if c.EventID != 50 || c.CompanyID != 7 || c.ByUserID != 42 {
		t.Fatalf("firma: %+v", c)
	}
	if len(repo.comments) != 1 {
		t.Fatalf("escrituras: %d", len(repo.comments))
	}
}

func TestAddComment_RechazaVacio(t *testing.T) {
	svc, repo := newThreadSvc([2]uint{50, 7})

	for _, text := range []string{"", "   ", "\n\t "} {
		if _, err := svc.AddComment(7, 50, 42, text); err == nil {
			t.Fatalf("se esperaba error con %q", text)
		}
	}
	if len(repo.comments) != 0 {
		t.Fatal("no debería escribirse nada")
	}
}

func TestAddComment_RechazaDemasiadoLargo(t *testing.T) {
	svc, repo := newThreadSvc([2]uint{50, 7})

	// El límite es en runas: contar bytes recortaría comentarios válidos con
	// acentos.
	justo := strings.Repeat("á", models.MaxCompanyCommentLength)
	if _, err := svc.AddComment(7, 50, 42, justo); err != nil {
		t.Fatalf("el comentario en el límite debería entrar: %v", err)
	}
	if _, err := svc.AddComment(7, 50, 42, justo+"á"); err == nil {
		t.Fatal("se esperaba error al pasarse")
	}
	if len(repo.comments) != 1 {
		t.Fatalf("solo debería guardarse el válido, got %d", len(repo.comments))
	}
}

func TestUpdateComment_ErrorSiNoExiste(t *testing.T) {
	svc, repo := newThreadSvc()
	repo.updateRows = 0

	if err := svc.UpdateComment(7, 123, "texto"); err == nil {
		t.Fatal("se esperaba error")
	}
}

func TestUpdateComment_ValidaIgualQueAlCrear(t *testing.T) {
	svc, repo := newThreadSvc()
	repo.updateRows = 1

	if err := svc.UpdateComment(7, 123, "   "); err == nil {
		t.Fatal("un comentario corregido tampoco puede quedar vacío")
	}
	if repo.updatedContent != "" {
		t.Fatal("no debería haber llegado a escribir")
	}
}

// --- Adjuntos ----------------------------------------------------------------

// El nombre en disco se reduce a su base: si se guardara con directorios,
// servirlo después sacaría archivos de fuera de la carpeta de subidas.
func TestAddAttachment_NeutralizaRutasEnElNombreGuardado(t *testing.T) {
	svc, repo := newThreadSvc([2]uint{50, 7})

	a, err := svc.AddAttachment(7, 50, 42, nil, "informe.pdf", "../../etc/passwd", 10, "application/pdf")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if a.StoredName != "passwd" {
		t.Fatalf("stored_name: got %q, se esperaba solo la base", a.StoredName)
	}
	if len(repo.attachments) != 1 {
		t.Fatalf("escrituras: %d", len(repo.attachments))
	}
}

func TestAddAttachment_RechazaSinArchivo(t *testing.T) {
	svc, _ := newThreadSvc([2]uint{50, 7})

	for _, stored := range []string{"", "   ", "/", "."} {
		if _, err := svc.AddAttachment(7, 50, 42, nil, "x.png", stored, 1, "image/png"); err == nil {
			t.Fatalf("se esperaba error con stored_name %q", stored)
		}
	}
}

// Sin nombre visible se cae al del disco: es feo, pero es mejor que una fila
// con el nombre en blanco que nadie sabe qué es.
func TestAddAttachment_SinNombreVisibleUsaElDelDisco(t *testing.T) {
	svc, _ := newThreadSvc([2]uint{50, 7})

	a, err := svc.AddAttachment(7, 50, 42, nil, "   ", "1_2_captura.png", 10, "image/png")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if a.FileName != "1_2_captura.png" {
		t.Fatalf("file_name: got %q", a.FileName)
	}
}

func TestAddAttachment_RespetaElTopePorEntrada(t *testing.T) {
	svc, repo := newThreadSvc([2]uint{50, 7})
	repo.attachmentCount = models.MaxAttachmentsPerEntry

	if _, err := svc.AddAttachment(7, 50, 42, nil, "x.png", "x.png", 1, "image/png"); err == nil {
		t.Fatal("se esperaba error al superar el tope")
	}
	if len(repo.attachments) != 0 {
		t.Fatal("no debería escribirse nada")
	}
}

// --- Reparto de hilos --------------------------------------------------------

type listThreadRepo struct {
	repository.CompanyThreadRepository
	comments    []models.CompanyEventComment
	attachments []models.CompanyEventAttachment
}

func (f *listThreadRepo) ListComments(uint, []uint) ([]models.CompanyEventComment, error) {
	return f.comments, nil
}

func (f *listThreadRepo) ListAttachments(uint, []uint) ([]models.CompanyEventAttachment, error) {
	return f.attachments, nil
}

// Un archivo subido dentro de un comentario tiene que viajar DENTRO de ese
// comentario, no suelto en la entrada: si no, la captura aparecería separada
// del texto que la explica.
func TestLoadThreads_ReparteAdjuntosEntreComentarioYEntrada(t *testing.T) {
	comentario := uint(1000)
	repo := &listThreadRepo{
		comments: []models.CompanyEventComment{
			{ID: comentario, EventID: 50, Content: "mira esto"},
		},
		attachments: []models.CompanyEventAttachment{
			{ID: 1, EventID: 50, CommentID: &comentario, FileName: "en-comentario.png"},
			{ID: 2, EventID: 50, FileName: "en-la-nota.pdf"},
		},
	}
	svc := &companyThreadService{repo: repo}

	threads, err := svc.LoadThreads(7, []uint{50})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	th, ok := threads[50]
	if !ok {
		t.Fatal("falta el hilo de la entrada 50")
	}
	if len(th.Attachments) != 1 || th.Attachments[0].FileName != "en-la-nota.pdf" {
		t.Fatalf("sueltos: %+v", th.Attachments)
	}
	if len(th.Comments) != 1 || len(th.Comments[0].Attachments) != 1 ||
		th.Comments[0].Attachments[0].FileName != "en-comentario.png" {
		t.Fatalf("comentario: %+v", th.Comments)
	}
}

// Las entradas sin nada no aparecen en el mapa: mandar hilos vacíos por cada
// jornada de la página engordaría la respuesta sin decir nada.
func TestLoadThreads_OmiteEntradasSinHilo(t *testing.T) {
	svc := &companyThreadService{repo: &listThreadRepo{}}

	threads, err := svc.LoadThreads(7, []uint{50, 51})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("se esperaba vacío, got %+v", threads)
	}
}

func TestLoadThreads_SinEntradasNoConsultaNada(t *testing.T) {
	svc, _ := newThreadSvc()

	threads, err := svc.LoadThreads(7, nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("se esperaba vacío, got %+v", threads)
	}
}
