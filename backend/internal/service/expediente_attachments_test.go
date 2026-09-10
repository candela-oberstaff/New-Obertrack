package service

import (
	"errors"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// fakeAttachRepo cubre lo justo para AddDocument / UpdateNote / DeleteNote:
// buscar un empleo, buscar una nota, y anotar qué se hizo con los adjuntos.
type fakeAttachRepo struct {
	repository.EmploymentRepository
	notes    map[uint]*models.EmploymentNote
	created  []*models.EmploymentDocument
	docsUpd  []map[string]interface{}
	detached []uint
}

func (f *fakeAttachRepo) GetByID(id uint) (*models.Employment, error) {
	return &models.Employment{ID: id}, nil
}

func (f *fakeAttachRepo) GetNote(id uint) (*models.EmploymentNote, error) {
	n, ok := f.notes[id]
	if !ok {
		return nil, errors.New("nota no encontrada")
	}
	return n, nil
}

func (f *fakeAttachRepo) CreateDocument(doc *models.EmploymentDocument) error {
	f.created = append(f.created, doc)
	return nil
}

func (f *fakeAttachRepo) UpdateNote(note *models.EmploymentNote, updates map[string]interface{}) error {
	return nil
}

func (f *fakeAttachRepo) UpdateDocumentsOfNote(noteID uint, updates map[string]interface{}) error {
	f.docsUpd = append(f.docsUpd, updates)
	return nil
}

func (f *fakeAttachRepo) DetachDocumentsOfNote(noteID uint) error {
	f.detached = append(f.detached, noteID)
	return nil
}

func (f *fakeAttachRepo) DeleteNote(id uint) error { return nil }

func newAttachSvc(notes map[uint]*models.EmploymentNote) (*employmentService, *fakeAttachRepo) {
	repo := &fakeAttachRepo{notes: notes}
	return &employmentService{repo: repo}, repo
}

// Un adjunto de nota NO decide quién lo ve: lo hereda de su nota. Si pudiera
// divergir, una evaluación interna podría llevar colgado el informe en que se
// apoya visible para el profesional — filtrar justo lo que se quiso reservar.
func TestAddDocument_ElAdjuntoHeredaLaVisibilidadDeSuNota(t *testing.T) {
	notaPrivada := &models.EmploymentNote{ID: 7, EmploymentID: 1, Visibility: models.ExpedientePrivate}
	svc, repo := newAttachSvc(map[uint]*models.EmploymentNote{7: notaPrivada})

	noteID := uint(7)
	// Se pide "shared" a propósito: la nota manda igual.
	_, err := svc.AddDocument(1, 99, &noteID, "Informe", "a.pdf", "/api/uploads/a.pdf", 10, "application/pdf", models.ExpedienteShared, nil)
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("se esperaba 1 documento creado, hay %d", len(repo.created))
	}
	if got := repo.created[0].Visibility; got != models.ExpedientePrivate {
		t.Errorf("el adjunto quedó como %q; debía heredar %q de su nota", got, models.ExpedientePrivate)
	}
	if repo.created[0].NoteID == nil || *repo.created[0].NoteID != 7 {
		t.Error("el adjunto debería quedar colgado de la nota 7")
	}
}

// Sin esta comprobación, un id de nota ajeno colgaría el archivo del expediente
// de otra persona: se guardaría con este employment_id pero aparecería bajo una
// nota que no es suya.
func TestAddDocument_RechazaUnaNotaDeOtroExpediente(t *testing.T) {
	ajena := &models.EmploymentNote{ID: 7, EmploymentID: 2, Visibility: models.ExpedientePrivate}
	svc, repo := newAttachSvc(map[uint]*models.EmploymentNote{7: ajena})

	noteID := uint(7)
	_, err := svc.AddDocument(1, 99, &noteID, "", "a.pdf", "/api/uploads/a.pdf", 10, "application/pdf", "", nil)
	if err == nil {
		t.Fatal("adjuntar a la nota de otro expediente debería fallar")
	}
	if len(repo.created) != 0 {
		t.Error("no debería haberse creado ningún documento")
	}
}

// Un documento suelto del expediente (contrato, certificado) sigue funcionando
// como siempre: sin nota y con la visibilidad que se le pida.
func TestAddDocument_SinNotaConservaSuPropiaVisibilidad(t *testing.T) {
	svc, repo := newAttachSvc(map[uint]*models.EmploymentNote{})

	_, err := svc.AddDocument(1, 99, nil, "Contrato", "c.pdf", "/api/uploads/c.pdf", 10, "application/pdf", models.ExpedienteShared, nil)
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if repo.created[0].NoteID != nil {
		t.Error("un documento del expediente no debe colgar de ninguna nota")
	}
	if repo.created[0].Visibility != models.ExpedienteShared {
		t.Error("sin nota, la visibilidad pedida es la que vale")
	}
}

// Dejar de compartir una evaluación tiene que llevarse consigo sus pruebas: si
// no, el texto desaparece del expediente del profesional y el informe firmado
// se queda ahí solo.
func TestUpdateNote_LaVisibilidadArrastraALosAdjuntos(t *testing.T) {
	nota := &models.EmploymentNote{ID: 7, EmploymentID: 1, Visibility: models.ExpedienteShared, Kind: models.NoteKindNote}
	svc, repo := newAttachSvc(map[uint]*models.EmploymentNote{7: nota})

	if _, err := svc.UpdateNote(7, models.NoteKindNote, nil, "texto", models.ExpedientePrivate); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if len(repo.docsUpd) != 1 {
		t.Fatalf("se esperaba una propagación a los adjuntos, hubo %d", len(repo.docsUpd))
	}
	if repo.docsUpd[0]["visibility"] != models.ExpedientePrivate {
		t.Errorf("los adjuntos quedaron en %v", repo.docsUpd[0]["visibility"])
	}
}

// Borrar la nota NO destruye los archivos: se sueltan y quedan como documentos
// del expediente. Un archivo que alguien subió a mano no debe desaparecer como
// efecto lateral de borrar el texto que lo acompañaba.
func TestDeleteNote_SueltaLosAdjuntosEnVezDeBorrarlos(t *testing.T) {
	nota := &models.EmploymentNote{ID: 7, EmploymentID: 1, Visibility: models.ExpedientePrivate}
	svc, repo := newAttachSvc(map[uint]*models.EmploymentNote{7: nota})

	if err := svc.DeleteNote(7); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if len(repo.detached) != 1 || repo.detached[0] != 7 {
		t.Errorf("se esperaba soltar los adjuntos de la nota 7, hubo %v", repo.detached)
	}
}
