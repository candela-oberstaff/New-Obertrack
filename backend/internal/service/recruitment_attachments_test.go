package service

import (
	"encoding/base64"
	"errors"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// El adjunto de una nota de Reclutamiento. Lo que se fija: que el tipo y el
// tamaño se rechacen con 400 ANTES de crear la nota (un 400 no deja nada a
// medias), que el archivo quede colgado de la entrada como cualquier otro del
// expediente, que el reintento complete una nota que quedó sin archivo, y que
// borrar la nota se lleve el hilo.

type fakeAttachUpload struct{ dir string }

func (f *fakeAttachUpload) GetUploadPath() string { return f.dir }
func (f *fakeAttachUpload) GetAllowedMimeTypes() map[string]string {
	return map[string]string{"application/pdf": ".pdf", "image/png": ".png"}
}

// Lo que la interfaz exige y aquí no se usa: el binario ya viene decodificado.
func (f *fakeAttachUpload) ValidateFile(_ *multipart.FileHeader) (string, error) { return "", nil }
func (f *fakeAttachUpload) GenerateFilename(_ uint, _ string, _ string) (string, string, int64, error) {
	return "", "", 0, nil
}

type fakeAttachThreads struct {
	CompanyThreadService
	attachments map[uint][]models.CompanyEventAttachment
	addErr      error
	deleted     []uint
}

func (f *fakeAttachThreads) AddAttachment(companyID, eventID, byUserID uint, commentID *uint, fileName, storedName string, size int64, mimeType string) (*models.CompanyEventAttachment, error) {
	if f.addErr != nil {
		return nil, f.addErr
	}
	if f.attachments == nil {
		f.attachments = map[uint][]models.CompanyEventAttachment{}
	}
	a := models.CompanyEventAttachment{EventID: eventID, CompanyID: companyID, ByUserID: byUserID, FileName: fileName, StoredName: storedName, FileSize: size, MimeType: mimeType}
	f.attachments[eventID] = append(f.attachments[eventID], a)
	return &a, nil
}

func (f *fakeAttachThreads) ThreadSize(_ uint, eventID uint) (int64, int64, error) {
	return 0, int64(len(f.attachments[eventID])), nil
}

func (f *fakeAttachThreads) LoadThreads(_ uint, ids []uint) (map[uint]EventThread, error) {
	out := map[uint]EventThread{}
	for _, id := range ids {
		out[id] = EventThread{Attachments: f.attachments[id]}
	}
	return out, nil
}

func (f *fakeAttachThreads) DeleteThreadForEvent(_ uint, eventID uint) error {
	f.deleted = append(f.deleted, eventID)
	delete(f.attachments, eventID)
	return nil
}

func newRecruitAttachSvc(t *testing.T) (*adminService, *fakeRecruitRepo, *fakeAttachThreads, string) {
	t.Helper()
	svc, repo := newRecruitSvc()
	dir := t.TempDir()
	threads := &fakeAttachThreads{}
	svc.SetRecruitmentAttachmentDeps(&fakeAttachUpload{dir: dir}, threads)
	return svc, repo, threads, dir
}

func pdf(bytes int) *RecruitmentAttachment {
	return &RecruitmentAttachment{
		FileName:      "oferta.pdf",
		MimeType:      "application/pdf",
		ContentBase64: base64.StdEncoding.EncodeToString(make([]byte, bytes)),
	}
}

func TestAdjunto_QuedaColgadoDeLaNotaYEnDisco(t *testing.T) {
	svc, repo, threads, dir := newRecruitAttachSvc(t)
	in := notaBase()
	in.Attachment = pdf(1234)

	res, err := svc.AddRecruitmentNote(36, in)

	if err != nil || res.Status != "created" {
		t.Fatalf("res=%v err=%v", res, err)
	}
	as := threads.attachments[res.ID]
	if len(as) != 1 {
		t.Fatalf("la nota debía tener UN adjunto, got %d", len(as))
	}
	a := as[0]
	if a.FileName != "oferta.pdf" || a.MimeType != "application/pdf" || a.FileSize != 1234 {
		t.Errorf("adjunto mal registrado: %+v", a)
	}
	// El nombre en disco lo pone el servidor, nunca el file_name entrante.
	if !strings.HasPrefix(a.StoredName, "recruit_") || !strings.HasSuffix(a.StoredName, ".pdf") {
		t.Errorf("nombre en disco %q", a.StoredName)
	}
	if st, err := os.Stat(filepath.Join(dir, a.StoredName)); err != nil || st.Size() != 1234 {
		t.Errorf("el archivo no está en disco o no pesa lo que debe: %v", err)
	}
	// Y el autor del archivo es la misma cuenta de servicio que firma la nota.
	if a.ByUserID != repo.created[0].ByUserID {
		t.Errorf("autor del adjunto %d, de la nota %d", a.ByUserID, repo.created[0].ByUserID)
	}
}

// Los 400 del adjunto, con el texto que verá el reclutador. Y lo importante: se
// rechazan ANTES de escribir la nota.
func TestAdjunto_Los400NoDejanNotaAMedias(t *testing.T) {
	casos := []struct {
		nombre string
		adj    *RecruitmentAttachment
		frase  string
	}{
		{"pesa más de 10 MB", pdf(10<<20 + 1), "10 MB"},
		{"tipo no permitido", &RecruitmentAttachment{FileName: "x.exe", MimeType: "application/x-msdownload", ContentBase64: base64.StdEncoding.EncodeToString([]byte("x"))}, "no está permitido"},
		{"base64 roto", &RecruitmentAttachment{FileName: "x.pdf", MimeType: "application/pdf", ContentBase64: "%%%no-es-base64%%%"}, "base64"},
		{"vacío", &RecruitmentAttachment{FileName: "x.pdf", MimeType: "application/pdf", ContentBase64: ""}, "vacío"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			svc, repo, threads, _ := newRecruitAttachSvc(t)
			in := notaBase()
			in.Attachment = c.adj

			_, err := svc.AddRecruitmentNote(36, in)

			if !errors.Is(err, apperrors.ErrInvalidInput) {
				t.Fatalf("debía ser 400, got %v", err)
			}
			if !strings.Contains(err.Error(), c.frase) {
				t.Errorf("mensaje %q debía mencionar %q", err.Error(), c.frase)
			}
			if len(repo.created) != 0 || len(threads.attachments) != 0 {
				t.Error("un 400 no escribe la nota ni el archivo")
			}
		})
	}
}

// Justo 10 MB entra: el límite es "hasta", no "menos de". Obersuite deja pasar
// hasta 10 en su formulario y un byte de diferencia rebotaría archivos suyos.
func TestAdjunto_DiezMegasExactosEntran(t *testing.T) {
	svc, _, threads, _ := newRecruitAttachSvc(t)
	in := notaBase()
	in.Attachment = pdf(10 << 20)

	res, err := svc.AddRecruitmentNote(36, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(threads.attachments[res.ID]) != 1 {
		t.Fatal("10 MB exactos debían aceptarse")
	}
}

// Si la nota se creó y el archivo falló, Obersuite ve un 500 y reintenta con el
// mismo external_id. Ese reintento tiene que dejar la nota ENTERA: responder
// already_exists sin más habría dejado una nota sin el archivo que el reclutador
// cree que mandó, para siempre.
func TestAdjunto_ElReintentoCompletaUnaNotaQueQuedoSinArchivo(t *testing.T) {
	svc, repo, threads, _ := newRecruitAttachSvc(t)
	in := notaBase()
	in.Attachment = pdf(100)

	threads.addErr = errors.New("disco lleno")
	if _, err := svc.AddRecruitmentNote(36, in); err == nil {
		t.Fatal("con el archivo fallando tiene que ser error (500), no un 200 a medias")
	}
	if len(repo.created) != 1 {
		t.Fatalf("la nota sí quedó creada: %d", len(repo.created))
	}

	threads.addErr = nil
	res, err := svc.AddRecruitmentNote(36, in)
	if err != nil || res.Status != "already_exists" {
		t.Fatalf("el reintento debía ser already_exists: %v %v", res, err)
	}
	if len(threads.attachments[res.ID]) != 1 {
		t.Fatal("el reintento debía completar el adjunto que faltaba")
	}
	if len(repo.created) != 1 {
		t.Fatal("y no crear una segunda nota")
	}
}

func TestAdjunto_ElReintentoNoDuplicaElArchivo(t *testing.T) {
	svc, _, threads, _ := newRecruitAttachSvc(t)
	in := notaBase()
	in.Attachment = pdf(100)

	res, _ := svc.AddRecruitmentNote(36, in)
	svc.AddRecruitmentNote(36, in)

	if len(threads.attachments[res.ID]) != 1 {
		t.Fatalf("dos POST iguales = un archivo, got %d", len(threads.attachments[res.ID]))
	}
}

// Un archivo escrito cuya fila no se pudo guardar es basura en disco: se
// recoge en el acto.
func TestAdjunto_SiLaFilaFallaElArchivoNoQuedaHuerfano(t *testing.T) {
	svc, _, threads, dir := newRecruitAttachSvc(t)
	threads.addErr = errors.New("no se pudo")
	in := notaBase()
	in.Attachment = pdf(100)

	svc.AddRecruitmentNote(36, in)

	entradas, _ := os.ReadDir(dir)
	if len(entradas) != 0 {
		t.Fatalf("quedó un archivo huérfano en disco: %v", entradas)
	}
}

func TestAdjunto_BorrarLaNotaSeLlevaElHilo(t *testing.T) {
	svc, repo, threads, _ := newRecruitAttachSvc(t)
	in := notaBase()
	in.Attachment = pdf(100)
	res, _ := svc.AddRecruitmentNote(36, in)
	repo.deleteRows = 1

	if err := svc.DeleteRecruitmentNote(36, "obersuite-note-abc"); err != nil {
		t.Fatal(err)
	}
	if len(threads.deleted) != 1 || threads.deleted[0] != res.ID {
		t.Fatalf("el hilo de la nota debía retirarse: %v", threads.deleted)
	}
}

func TestAdjunto_DescargaValidadaContraLaEmpresa(t *testing.T) {
	svc, _, _, _ := newRecruitAttachSvc(t)
	in := notaBase()
	in.Attachment = pdf(100)
	svc.AddRecruitmentNote(36, in)

	a, err := svc.RecruitmentAttachmentForDownload(36, "obersuite-note-abc")
	if err != nil || a == nil || a.FileName != "oferta.pdf" {
		t.Fatalf("debía devolver el adjunto: %v %v", a, err)
	}
	if _, err := svc.RecruitmentAttachmentForDownload(36, "otra-nota"); !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("una nota que no existe es 404, got %v", err)
	}
}

func TestAdjunto_SinArchivoLaNotaSigueIgualQueAntes(t *testing.T) {
	svc, _, threads, _ := newRecruitAttachSvc(t)

	res, err := svc.AddRecruitmentNote(36, notaBase())

	if err != nil || res.Status != "created" {
		t.Fatalf("%v %v", res, err)
	}
	if len(threads.attachments) != 0 {
		t.Fatal("sin attachment no se cuelga nada")
	}
	if _, err := svc.RecruitmentAttachmentForDownload(36, "obersuite-note-abc"); !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("sin adjunto la descarga es 404, got %v", err)
	}
}
