package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

// ErrEventNotInCompany se devuelve cuando el evento no existe o es de otra
// empresa. Se responden igual a propósito: distinguirlos diría a quien prueba
// ids ajenos cuáles existen.
var ErrEventNotInCompany = errors.New("La entrada del expediente no existe en esta empresa")

// CompanyThreadService gestiona el hilo del expediente de empresa: comentarios
// y archivos colgados de cada entrada.
type CompanyThreadService interface {
	AddComment(companyID, eventID, byUserID uint, content string) (*models.CompanyEventComment, error)
	UpdateComment(companyID, commentID uint, content string) error
	DeleteComment(companyID, commentID uint) error

	// AddAttachment registra un archivo YA SUBIDO por /api/uploads. El binario
	// no pasa por aquí: este servicio solo guarda a qué entrada pertenece.
	AddAttachment(companyID, eventID, byUserID uint, commentID *uint, fileName, storedName string, size int64, mimeType string) (*models.CompanyEventAttachment, error)
	DeleteAttachment(companyID, attachmentID uint) error
	// AttachmentForDownload devuelve el adjunto listo para servir, ya validado
	// contra la empresa.
	AttachmentForDownload(companyID, attachmentID uint) (*models.CompanyEventAttachment, error)

	// LoadThreads reparte comentarios y adjuntos entre las entradas indicadas,
	// en dos consultas en total y no dos por entrada.
	LoadThreads(companyID uint, eventIDs []uint) (map[uint]EventThread, error)
	// ThreadSize dice qué se llevaría por delante borrar una entrada.
	ThreadSize(companyID, eventID uint) (comments int64, attachments int64, err error)
	DeleteThreadForEvent(companyID, eventID uint) error
}

// EventThread es lo que cuelga de una entrada del expediente.
type EventThread struct {
	Comments    []models.CompanyEventComment    `json:"comments"`
	Attachments []models.CompanyEventAttachment `json:"attachments"`
}

type companyThreadService struct {
	repo repository.CompanyThreadRepository
}

func NewCompanyThreadService(repo repository.CompanyThreadRepository) CompanyThreadService {
	return &companyThreadService{repo: repo}
}

// assertEvent es la comprobación que va delante de toda escritura: el evento
// tiene que existir Y ser de esta empresa.
func (s *companyThreadService) assertEvent(eventID, companyID uint) error {
	ok, err := s.repo.EventBelongsTo(eventID, companyID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrEventNotInCompany
	}
	return nil
}

func validateCommentText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("El comentario no puede estar vacío")
	}
	if len([]rune(text)) > models.MaxCompanyCommentLength {
		return "", fmt.Errorf("El comentario no puede superar los %d caracteres", models.MaxCompanyCommentLength)
	}
	// Se guarda como texto plano saneado, no como HTML: el expediente es un
	// registro de auditoría y no hay ninguna razón para que una nota pueda
	// inyectar marcado en la ficha de la empresa. Las imágenes van como
	// adjuntos, que es donde se pueden comprobar tipo y tamaño.
	return utils.SanitizeHTML(text), nil
}

func (s *companyThreadService) AddComment(companyID, eventID, byUserID uint, content string) (*models.CompanyEventComment, error) {
	if err := s.assertEvent(eventID, companyID); err != nil {
		return nil, err
	}
	content, err := validateCommentText(content)
	if err != nil {
		return nil, err
	}
	c := &models.CompanyEventComment{
		EventID:   eventID,
		CompanyID: companyID,
		ByUserID:  byUserID,
		Content:   content,
	}
	if err := s.repo.CreateComment(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *companyThreadService) UpdateComment(companyID, commentID uint, content string) error {
	content, err := validateCommentText(content)
	if err != nil {
		return err
	}
	rows, err := s.repo.UpdateComment(commentID, companyID, content, time.Now())
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("Comentario no encontrado")
	}
	return nil
}

func (s *companyThreadService) DeleteComment(companyID, commentID uint) error {
	rows, err := s.repo.DeleteComment(commentID, companyID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("Comentario no encontrado")
	}
	return nil
}

func (s *companyThreadService) AddAttachment(companyID, eventID, byUserID uint, commentID *uint, fileName, storedName string, size int64, mimeType string) (*models.CompanyEventAttachment, error) {
	if err := s.assertEvent(eventID, companyID); err != nil {
		return nil, err
	}

	// El nombre en disco se reduce a su base: si llegara con directorios, servir
	// por él más tarde sacaría archivos de fuera de la carpeta de subidas.
	storedName = filepath.Base(strings.TrimSpace(storedName))
	if storedName == "" || storedName == "." || storedName == string(filepath.Separator) {
		return nil, errors.New("Falta el archivo subido")
	}

	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = storedName
	}
	if len([]rune(fileName)) > 255 {
		fileName = string([]rune(fileName)[:255])
	}

	n, err := s.repo.CountAttachmentsForEvent(eventID)
	if err != nil {
		return nil, err
	}
	if n >= models.MaxAttachmentsPerEntry {
		return nil, fmt.Errorf("Una entrada no puede tener más de %d archivos", models.MaxAttachmentsPerEntry)
	}

	a := &models.CompanyEventAttachment{
		EventID:    eventID,
		CompanyID:  companyID,
		CommentID:  commentID,
		ByUserID:   byUserID,
		FileName:   utils.SanitizeHTML(fileName),
		StoredName: storedName,
		FileSize:   size,
		MimeType:   strings.TrimSpace(mimeType),
	}
	if err := s.repo.CreateAttachment(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *companyThreadService) DeleteAttachment(companyID, attachmentID uint) error {
	rows, err := s.repo.DeleteAttachment(attachmentID, companyID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("Archivo no encontrado")
	}
	return nil
}

func (s *companyThreadService) AttachmentForDownload(companyID, attachmentID uint) (*models.CompanyEventAttachment, error) {
	a, err := s.repo.GetAttachment(attachmentID, companyID)
	if err != nil {
		return nil, errors.New("Archivo no encontrado")
	}
	return a, nil
}

func (s *companyThreadService) LoadThreads(companyID uint, eventIDs []uint) (map[uint]EventThread, error) {
	threads := map[uint]EventThread{}
	if len(eventIDs) == 0 {
		return threads, nil
	}

	comments, err := s.repo.ListComments(companyID, eventIDs)
	if err != nil {
		return nil, err
	}
	attachments, err := s.repo.ListAttachments(companyID, eventIDs)
	if err != nil {
		return nil, err
	}

	// Los adjuntos que cuelgan de un comentario viajan dentro de ese comentario;
	// los demás van sueltos en la entrada. Así el frontend pinta cada archivo
	// donde se subió sin tener que cruzar nada.
	byComment := map[uint][]models.CompanyEventAttachment{}
	loose := map[uint][]models.CompanyEventAttachment{}
	for _, a := range attachments {
		if a.CommentID != nil {
			byComment[*a.CommentID] = append(byComment[*a.CommentID], a)
			continue
		}
		loose[a.EventID] = append(loose[a.EventID], a)
	}

	byEvent := map[uint][]models.CompanyEventComment{}
	for _, c := range comments {
		c.Attachments = byComment[c.ID]
		byEvent[c.EventID] = append(byEvent[c.EventID], c)
	}

	for _, id := range eventIDs {
		cs, as := byEvent[id], loose[id]
		if len(cs) == 0 && len(as) == 0 {
			continue
		}
		if cs == nil {
			cs = []models.CompanyEventComment{}
		}
		if as == nil {
			as = []models.CompanyEventAttachment{}
		}
		threads[id] = EventThread{Comments: cs, Attachments: as}
	}
	return threads, nil
}

func (s *companyThreadService) ThreadSize(companyID, eventID uint) (int64, int64, error) {
	return s.repo.CountThread(eventID, companyID)
}

func (s *companyThreadService) DeleteThreadForEvent(companyID, eventID uint) error {
	return s.repo.DeleteThreadForEvent(eventID, companyID)
}
