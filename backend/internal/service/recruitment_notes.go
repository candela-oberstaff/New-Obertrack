package service

import (
	"fmt"
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

	// El reintento ANTES de escribir nada: el segundo POST con el mismo id
	// devuelve lo que ya hay, sin tocarlo.
	existing, err := s.repo.FindCompanyEventByExternalID(companyID, externalID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
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
	return &RecruitmentNoteResult{ID: event.ID, Status: "created"}, nil
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
	rows, err := s.repo.DeleteCompanyEventByExternalID(companyID, strings.TrimSpace(externalID))
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
