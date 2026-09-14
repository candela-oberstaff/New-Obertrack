package service

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// Notas de Reclutamiento escritas DESDE Obersuite: la primera escritura de la
// integración, que hasta hoy era un espejo de solo lectura.
//
// Se guardan en el mismo expediente que las notas de Customer Success (tabla
// company_events) para que haya UNA fuente de verdad: Obersuite las lee de
// vuelta por /timeline como cualquier otra categoría y apaga su tabla propia.
//
// Lo que las distingue del resto del expediente:
//   - el autor técnico es la cuenta de servicio "Obersuite" y la persona real
//     va en author_name (no es un usuario de aquí);
//   - llevan external_id, que es lo que las hace idempotentes y borrables desde
//     Obersuite;
//   - en Obertrack son de solo lectura. Quien la escribió la borra, desde donde
//     la escribió.

// RecruitmentNoteInput es lo que manda Obersuite. Los nombres siguen su
// formulario (vía en español) porque el contrato se escribió en sus términos.
type RecruitmentNoteInput struct {
	ExternalID string
	Via        string
	Content    string
	PersonIDs  []uint
	AuthorName string
	CreatedAt  *time.Time
	Attachment *RecruitmentAttachment
}

// RecruitmentAttachment es el archivo tal como viaja en el POST: binario en
// base64, misma forma que el CV de /hire.
type RecruitmentAttachment struct {
	FileName      string
	MimeType      string
	ContentBase64 string
}

// maxRecruitmentAttachmentBytes es el tope del adjunto de una nota. 10 MB y no
// los 8 del CV porque el formulario de Obersuite ya dejaba pasar hasta 10 y
// cortar más abajo rebotaría archivos que su pantalla acepta.
const maxRecruitmentAttachmentBytes = 10 << 20

// recruitmentAttachmentPrefix es cómo empiezan en disco los adjuntos que
// llegan por aquí. Se elige el nombre en el servidor: el file_name entrante es
// para enseñar, nunca para la ruta.
const recruitmentAttachmentPrefix = "recruit_"

// SetRecruitmentAttachmentDeps inyecta lo que hace falta para guardar el archivo
// de una nota: dónde escribirlo y con qué servicio colgarlo de la entrada. Van
// por setter y no por el constructor porque el servicio de administración tiene
// ya trece dependencias y solo esta función usa estas dos.
func (s *adminService) SetRecruitmentAttachmentDeps(upload UploadService, threads CompanyThreadService) {
	s.uploadSvc = upload
	s.threadSvc = threads
}

// decodedRecruitmentAttachment es el adjunto ya validado y decodificado, listo
// para escribir. Se valida ANTES de crear la nota: un 400 no puede dejar una
// nota a medias, sin el archivo que el reclutador cree que mandó.
type decodedRecruitmentAttachment struct {
	fileName string
	mime     string
	ext      string
	data     []byte
}

func (s *adminService) decodeRecruitmentAttachment(a *RecruitmentAttachment) (*decodedRecruitmentAttachment, error) {
	if a == nil {
		return nil, nil
	}
	if s.uploadSvc == nil || s.threadSvc == nil {
		return nil, fmt.Errorf("el guardado de adjuntos no está configurado en este servidor")
	}
	raw := strings.TrimSpace(a.ContentBase64)
	if raw == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "attachment.content_base64 está vacío")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, hireFail(apperrors.ErrInvalidInput, "attachment.content_base64 no es base64 válido")
	}
	if len(data) == 0 {
		return nil, hireFail(apperrors.ErrInvalidInput, "attachment: el archivo está vacío")
	}
	if len(data) > maxRecruitmentAttachmentBytes {
		// "supera", no "pesa X MB": con 10 MB y un byte, redondeado decía "pesa
		// 10.0 MB y el máximo es 10 MB", que parece una contradicción.
		return nil, hireFail(apperrors.ErrInvalidInput,
			fmt.Sprintf("attachment: el archivo supera el máximo de 10 MB (%d bytes de %d)", len(data), maxRecruitmentAttachmentBytes))
	}
	mime := normalizeContentType(a.MimeType)
	ext, ok := s.uploadSvc.GetAllowedMimeTypes()[mime]
	if !ok {
		return nil, hireFail(apperrors.ErrInvalidInput,
			fmt.Sprintf("attachment.mime_type %q no está permitido: admite PDF, Word, Excel, JPEG, PNG, GIF y WEBP", a.MimeType))
	}
	name := strings.TrimSpace(a.FileName)
	if name == "" {
		name = "adjunto" + ext
	}
	return &decodedRecruitmentAttachment{fileName: name, mime: mime, ext: ext, data: data}, nil
}

// storeRecruitmentAttachment escribe el archivo y lo cuelga de la nota. Va
// DESPUÉS de crear la nota porque necesita su id, y por eso un fallo aquí no
// puede ser 400 —la nota ya existe— sino error: Obersuite lo reintenta con el
// mismo external_id, la nota responde already_exists y el adjunto se vuelve a
// intentar (ver AddRecruitmentNote).
func (s *adminService) storeRecruitmentAttachment(companyID, eventID, byUserID uint, d *decodedRecruitmentAttachment) error {
	storedName := fmt.Sprintf("%s%d_%d%s", recruitmentAttachmentPrefix, eventID, time.Now().UnixNano(), d.ext)
	path := filepath.Join(s.uploadSvc.GetUploadPath(), storedName)
	if err := os.WriteFile(path, d.data, 0o644); err != nil {
		return fmt.Errorf("no se pudo escribir el adjunto: %w", err)
	}
	if _, err := s.threadSvc.AddAttachment(companyID, eventID, byUserID, nil, d.fileName, storedName, int64(len(d.data)), d.mime); err != nil {
		// El archivo sin fila es basura en disco: se recoge aquí mismo.
		if rerr := os.Remove(path); rerr != nil {
			log.Printf("[Obersuite] adjunto huérfano %q: %v", path, rerr)
		}
		return fmt.Errorf("no se pudo registrar el adjunto: %w", err)
	}
	return nil
}

// RecruitmentNoteResult distingue el alta del reintento: con el mismo
// external_id no se crea otra nota ni se modifica la que hay.
type RecruitmentNoteResult struct {
	ID     uint   `json:"id"`
	Status string `json:"status"` // "created" | "already_exists"
}

// recruitmentVias traduce la vía del formulario de Obersuite a nuestro canal de
// contacto con la empresa. "general" es "sin vía": una nota suelta.
//
// Lista cerrada a propósito: Obersuite se comprometió a avisar antes de añadir
// una, porque lo desconocido se rechaza con 400 y no se adivina.
var recruitmentVias = map[string]string{
	"reunion":  models.CompanyContactMeeting,
	"whatsapp": models.CompanyContactWhatsApp,
	"llamada":  models.CompanyContactCall,
	"correo":   models.CompanyContactEmail,
	"general":  "",
}

// AddRecruitmentNote guarda la nota. Idempotente por external_id dentro de la
// empresa.
//
// Los destinatarios (person_ids) no existen como estructura en nuestro
// expediente —el filtro por persona es por quién ACTUÓ, no sobre quién trata—,
// así que en esta versión se validan (tienen que ser de la empresa) y sus
// nombres van al texto como prefijo "Para: …". Está acordado con Obersuite y
// escrito en el contrato; si algún día hace falta filtrar por destinatario, es
// una tabla propia, no un apaño aquí.
func (s *adminService) AddRecruitmentNote(companyID uint, in RecruitmentNoteInput) (*RecruitmentNoteResult, error) {
	if err := s.assertEmployer(companyID); err != nil {
		return nil, hireFail(apperrors.ErrNotFound, "la empresa no existe o no es una empresa")
	}

	externalID := strings.TrimSpace(in.ExternalID)
	if externalID == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id es obligatorio")
	}
	if len(externalID) > 120 {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id no puede superar los 120 caracteres")
	}

	via := strings.ToLower(strings.TrimSpace(in.Via))
	channel, ok := recruitmentVias[via]
	if !ok {
		return nil, hireFail(apperrors.ErrInvalidInput,
			fmt.Sprintf("via %q no es válida: admite reunion, whatsapp, llamada, correo o general", in.Via))
	}

	content, err := validateNoteText(in.Content)
	if err != nil {
		return nil, hireFail(apperrors.ErrInvalidInput, "content: "+strings.ToLower(err.Error()))
	}

	author := strings.TrimSpace(in.AuthorName)
	if author == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "author_name es obligatorio")
	}
	if r := []rune(author); len(r) > 255 {
		author = string(r[:255])
	}

	// El adjunto se valida ANTES de escribir nada: si el tipo o el tamaño no
	// valen es 400, y un 400 no deja una nota sin su archivo.
	adjunto, err := s.decodeRecruitmentAttachment(in.Attachment)
	if err != nil {
		return nil, err
	}

	// El reintento ANTES de escribir nada: el segundo POST con el mismo id
	// devuelve lo que ya hay, sin tocarlo. Con una excepción: si la nota quedó
	// sin su adjunto (se creó y el archivo falló), el reintento lo completa. Es
	// lo que hace que "500 → reintentar" deje la nota entera y no a medias.
	existing, err := s.repo.FindCompanyEventByExternalID(companyID, externalID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if adjunto != nil {
			if _, nAdj, cerr := s.threadSvc.ThreadSize(companyID, existing.ID); cerr == nil && nAdj == 0 {
				if aerr := s.storeRecruitmentAttachment(companyID, existing.ID, existing.ByUserID, adjunto); aerr != nil {
					return nil, aerr
				}
			}
		}
		return &RecruitmentNoteResult{ID: existing.ID, Status: "already_exists"}, nil
	}

	prefix, err := s.recipientsPrefix(companyID, in.PersonIDs)
	if err != nil {
		return nil, err
	}
	detail := prefix + content
	// El prefijo puede llevar el texto por encima del máximo. Se acota aquí y
	// no en validateNoteText para que el 400 hable del content que mandaron,
	// no de una longitud que no controlan.
	if r := []rune(detail); len(r) > models.MaxCompanyNoteLength {
		detail = string(r[:models.MaxCompanyNoteLength])
	}

	account, err := s.userRepo.GetByEmail(models.ObersuiteServiceEmail)
	if err != nil || account == nil {
		return nil, fmt.Errorf("no existe la cuenta de servicio de Obersuite: %w", err)
	}

	event := &models.CompanyEvent{
		CompanyID:  companyID,
		Type:       models.CompanyEventRecruitment,
		Detail:     detail,
		ByUserID:   account.ID,
		Channel:    channel,
		ExternalID: externalID,
		AuthorName: author,
	}
	if in.CreatedAt != nil && !in.CreatedAt.IsZero() {
		event.CreatedAt = *in.CreatedAt
	}
	if err := s.repo.CreateCompanyEvent(event); err != nil {
		// Dos POST simultáneos con el mismo id: uno gana el índice y el otro
		// choca. Para quien lo mandó es el mismo "ya existe".
		if isUniqueViolation(err) {
			if again, ferr := s.repo.FindCompanyEventByExternalID(companyID, externalID); ferr == nil && again != nil {
				return &RecruitmentNoteResult{ID: again.ID, Status: "already_exists"}, nil
			}
		}
		return nil, err
	}
	if adjunto != nil {
		if err := s.storeRecruitmentAttachment(companyID, event.ID, account.ID, adjunto); err != nil {
			return nil, err
		}
	}
	return &RecruitmentNoteResult{ID: event.ID, Status: "created"}, nil
}

// RecruitmentAttachmentForDownload devuelve el adjunto de la nota que Obersuite
// conoce por ese id, validado contra la empresa. 404 si la nota no existe o no
// tiene adjunto.
func (s *adminService) RecruitmentAttachmentForDownload(companyID uint, externalID string) (*models.CompanyEventAttachment, error) {
	ev, err := s.repo.FindCompanyEventByExternalID(companyID, strings.TrimSpace(externalID))
	if err != nil {
		return nil, err
	}
	if ev == nil || ev.Type != models.CompanyEventRecruitment {
		return nil, hireFail(apperrors.ErrNotFound, "la nota no existe o no es de esta empresa")
	}
	if s.threadSvc == nil {
		return nil, hireFail(apperrors.ErrNotFound, "la nota no tiene adjunto")
	}
	threads, err := s.threadSvc.LoadThreads(companyID, []uint{ev.ID})
	if err != nil {
		return nil, err
	}
	t := threads[ev.ID]
	if len(t.Attachments) == 0 {
		return nil, hireFail(apperrors.ErrNotFound, "la nota no tiene adjunto")
	}
	return &t.Attachments[0], nil
}

// recipientsPrefix valida que cada destinatario sea de la empresa y arma el
// "Para: Ana Pérez, Luis Gómez — ". Vacío si la nota es para toda la empresa.
//
// Un id que no es de esa empresa es 400 con el id, no un silencio: escribir
// "Para: <alguien de otro cliente>" en el expediente de este sería exactamente
// el enlace-a-la-ficha-equivocada que la integración evita en todas partes.
func (s *adminService) recipientsPrefix(companyID uint, ids []uint) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	names := make([]string, 0, len(ids))
	seen := map[uint]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		row, err := s.userRepo.GetObersuiteProfessional(companyID, id)
		if err != nil {
			return "", err
		}
		if row == nil {
			return "", hireFail(apperrors.ErrInvalidInput,
				fmt.Sprintf("person_id %d no es un profesional de esta empresa", id))
		}
		names = append(names, row.Name)
	}
	if len(names) == 0 {
		return "", nil
	}
	return "Para: " + strings.Join(names, ", ") + " — ", nil
}

// DeleteRecruitmentNote borra EN FIRME la nota que Obersuite conoce por ese id.
// Solo alcanza las de tipo recruitment: una nota de Customer Success no se
// puede borrar desde fuera ni adivinando el id.
func (s *adminService) DeleteRecruitmentNote(companyID uint, externalID string) error {
	if err := s.assertEmployer(companyID); err != nil {
		return hireFail(apperrors.ErrNotFound, "la empresa no existe o no es una empresa")
	}
	ext := strings.TrimSpace(externalID)
	// El hilo (el adjunto) se retira ANTES que la nota: si se borrara la nota y
	// fallara el hilo, quedaría un archivo colgado de una entrada que ya no
	// existe.
	if s.threadSvc != nil {
		if ev, ferr := s.repo.FindCompanyEventByExternalID(companyID, ext); ferr == nil && ev != nil && ev.Type == models.CompanyEventRecruitment {
			if terr := s.threadSvc.DeleteThreadForEvent(companyID, ev.ID); terr != nil {
				return terr
			}
		}
	}
	rows, err := s.repo.DeleteCompanyEventByExternalID(companyID, ext)
	if err != nil {
		return err
	}
	if rows == 0 {
		return hireFail(apperrors.ErrNotFound, "la nota no existe o no es de esta empresa")
	}
	return nil
}

// isUniqueViolation reconoce el choque contra un índice único, por texto y sin
// atarse al driver. Suficiente aquí: solo decide si volver a buscar la fila.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
