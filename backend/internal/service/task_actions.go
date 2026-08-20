package service

import (
	"errors"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/utils"
)

func (s *taskService) AddComment(id uint, tenantID uint, userID uint, content string, isSuperadmin bool) (*models.Comment, error) {
	if _, err := s.authorizeTaskByID(id, tenantID, isSuperadmin); err != nil {
		return nil, err
	}

	comment := &models.Comment{
		TaskID:  id,
		UserID:  userID,
		Content: utils.SanitizeHTML(content),
	}

	if err := s.repo.AddComment(comment); err != nil {
		return nil, err
	}

	return s.repo.GetComment(comment.ID)
}

func (s *taskService) UpdateComment(commentID uint, userID uint, content string, isSuperadmin bool) (*models.Comment, error) {
	comment, err := s.repo.GetComment(commentID)
	if err != nil {
		return nil, errors.New("comentario no encontrado")
	}
	if !isSuperadmin && comment.UserID != userID {
		return nil, errors.New("no tienes permiso para editar este comentario")
	}
	if err := s.repo.UpdateComment(commentID, utils.SanitizeHTML(content)); err != nil {
		return nil, err
	}
	return s.repo.GetComment(commentID)
}

func (s *taskService) DeleteComment(commentID uint, userID uint, isSuperadmin bool) error {
	comment, err := s.repo.GetComment(commentID)
	if err != nil {
		return errors.New("comentario no encontrado")
	}
	if !isSuperadmin && comment.UserID != userID {
		return errors.New("no tienes permiso para eliminar este comentario")
	}
	return s.repo.DeleteComment(commentID)
}

func (s *taskService) AddAttachment(taskID uint, tenantID uint, fileName, fileURL string, fileSize int64, mimeType string, uploadedBy uint, isSuperadmin bool) (*models.TaskAttachment, error) {
	if _, err := s.authorizeTaskByID(taskID, tenantID, isSuperadmin); err != nil {
		return nil, err
	}

	attachment := &models.TaskAttachment{
		TaskID:     taskID,
		FileName:   fileName,
		FileURL:    fileURL,
		FileSize:   fileSize,
		MimeType:   mimeType,
		UploadedBy: uploadedBy,
	}

	if err := s.repo.AddAttachment(attachment); err != nil {
		return nil, err
	}
	return attachment, nil
}

func (s *taskService) DeleteAttachment(attachmentID uint, tenantID uint, isSuperadmin bool) error {
	attachment, err := s.repo.GetAttachmentByID(attachmentID)
	if err != nil {
		return err
	}
	if _, err := s.authorizeTaskByID(attachment.TaskID, tenantID, isSuperadmin); err != nil {
		return err
	}
	return s.repo.DeleteAttachment(attachment)
}
