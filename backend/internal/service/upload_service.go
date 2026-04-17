package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type UploadService interface {
	ValidateFile(file interface{}) (string, error)
	GenerateFilename(userID uint, originalName string, contentType string) (string, string, int64, error)
	GetUploadPath() string
	GetAllowedMimeTypes() map[string]string
}

type UploadResponse struct {
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	Path      string `json:"path"`
}

type uploadService struct {
	uploadPath  string
	maxFileSize int64
}

func NewUploadService(uploadPath string) UploadService {
	if uploadPath == "" {
		uploadPath = "./uploads"
	}
	os.MkdirAll(uploadPath, 0755)
	return &uploadService{
		uploadPath:  uploadPath,
		maxFileSize: 50 << 20, // 50MB
	}
}

func (s *uploadService) GetAllowedMimeTypes() map[string]string {
	return map[string]string{
		"application/pdf":                                                                                             ".pdf",
		"application/msword":                                                                                        ".doc",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":                                        ".docx",
		"application/vnd.ms-excel":                                                                                   ".xls",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":                                            ".xlsx",
		"image/jpeg":                                                                                                ".jpg",
		"image/png":                                                                                                 ".png",
		"image/gif":                                                                                                 ".gif",
		"image/webp":                                                                                                ".webp",
		"audio/mpeg":                                                                                                ".mp3",
		"audio/wav":                                                                                                 ".wav",
		"audio/ogg":                                                                                                 ".ogg",
		"audio/webm":                                                                                                ".webm",
	}
}

func (s *uploadService) ValidateFile(file interface{}) (string, error) {
	// Type assertion to get file header
	fileHeader, ok := file.(interface {
		GetHeader(string) string
		Size() int64
		Filename() string
	})
	if !ok {
		return "", fmt.Errorf("invalid file type")
	}

	contentType := fileHeader.GetHeader("Content-Type")
	ext, allowed := s.GetAllowedMimeTypes()[contentType]
	if !allowed {
		return "", fmt.Errorf("file type not allowed")
	}

	if fileHeader.Size() > s.maxFileSize {
		return "", fmt.Errorf("file too large (max 50MB)")
	}

	return ext, nil
}

func (s *uploadService) GenerateFilename(userID uint, originalName string, contentType string) (string, string, int64, error) {
	ext, allowed := s.GetAllowedMimeTypes()[contentType]
	if !allowed {
		return "", "", 0, fmt.Errorf("file type not allowed")
	}

	sanitized := s.sanitizeFilename(originalName)
	filename := fmt.Sprintf("%d_%d_%s%s", userID, time.Now().UnixNano(), sanitized, ext)
	filepath := filepath.Join(s.uploadPath, filename)

	return filename, filepath, 0, nil
}

func (s *uploadService) GetUploadPath() string {
	return s.uploadPath
}

func (s *uploadService) sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	if len(name) > 50 {
		name = name[:50]
	}
	return name
}
