package service

import (
	"errors"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type TrashItem struct {
	Type      string     `json:"type"`
	TypeLabel string     `json:"type_label"`
	ID        uint       `json:"id"`
	Title     string     `json:"title"`
	Subtitle  string     `json:"subtitle"`
	DeletedAt *time.Time `json:"deleted_at"`
}

type TrashTypeInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type trashType struct {
	label string
	model interface{}
	list  func(db *gorm.DB) ([]TrashItem, error)
}

type TrashService interface {
	Types() []TrashTypeInfo
	List(types []string) ([]TrashItem, error)
	Restore(typeKey string, id uint) error
	Purge(typeKey string, id uint) error
}

type trashService struct {
	db       *gorm.DB
	registry map[string]trashType
	order    []string
}

func deletedAtPtr(d gorm.DeletedAt) *time.Time {
	if d.Valid {
		t := d.Time
		return &t
	}
	return nil
}

func joinParts(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, " · ")
}

func NewTrashService(db *gorm.DB) TrashService {
	s := &trashService{db: db, registry: map[string]trashType{}}
	reg := func(key, label string, model interface{}, list func(db *gorm.DB) ([]TrashItem, error)) {
		s.registry[key] = trashType{label: label, model: model, list: list}
		s.order = append(s.order, key)
	}

	reg("users", "Usuarios", &models.User{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.User
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "users", TypeLabel: "Usuarios", ID: r.ID, Title: r.Name, Subtitle: joinParts(r.Email, string(r.UserType)), DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("tickets", "Tickets", &models.Ticket{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.Ticket
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "tickets", TypeLabel: "Tickets", ID: r.ID, Title: r.Title, Subtitle: joinParts(string(r.Stage), r.CompanyName), DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("contacts", "Contactos", &models.Contact{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.Contact
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "contacts", TypeLabel: "Contactos", ID: r.ID, Title: r.Name, Subtitle: joinParts(r.Phone, r.CompanyName), DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("incidents", "Incidentes", &models.Incident{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.Incident
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "incidents", TypeLabel: "Incidentes", ID: r.ID, Title: r.Title, Subtitle: joinParts(r.Kind, r.Country), DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("surveys", "Encuestas", &models.Survey{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.Survey
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "surveys", TypeLabel: "Encuestas", ID: r.ID, Title: r.Title, Subtitle: string(r.Status), DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("tutorials", "Novedades", &models.Tutorial{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.Tutorial
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "tutorials", TypeLabel: "Novedades", ID: r.ID, Title: r.Title, Subtitle: r.Category, DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("boards", "Tableros", &models.Board{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.Board
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "boards", TypeLabel: "Tableros", ID: r.ID, Title: r.Name, Subtitle: r.Description, DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("tasks", "Tareas", &models.Task{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.Task
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "tasks", TypeLabel: "Tareas", ID: r.ID, Title: r.Title, Subtitle: string(r.Status), DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	reg("emergency_templates", "Plantillas de emergencia", &models.EmergencyTemplate{}, func(db *gorm.DB) ([]TrashItem, error) {
		var rows []models.EmergencyTemplate
		if err := db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]TrashItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, TrashItem{Type: "emergency_templates", TypeLabel: "Plantillas de emergencia", ID: r.ID, Title: r.Title, Subtitle: r.Subject, DeletedAt: deletedAtPtr(r.DeletedAt)})
		}
		return out, nil
	})

	return s
}

func (s *trashService) Types() []TrashTypeInfo {
	out := make([]TrashTypeInfo, 0, len(s.order))
	for _, k := range s.order {
		out = append(out, TrashTypeInfo{Key: k, Label: s.registry[k].label})
	}
	return out
}

func (s *trashService) List(types []string) ([]TrashItem, error) {
	want := map[string]bool{}
	for _, t := range types {
		if t = strings.TrimSpace(t); t != "" {
			want[t] = true
		}
	}
	out := []TrashItem{}
	for _, key := range s.order {
		if len(want) > 0 && !want[key] {
			continue
		}
		items, err := s.registry[key].list(s.db)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

func (s *trashService) Restore(typeKey string, id uint) error {
	t, ok := s.registry[typeKey]
	if !ok {
		return errors.New("tipo de papelera inválido: " + typeKey)
	}
	return s.db.Unscoped().Model(t.model).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (s *trashService) Purge(typeKey string, id uint) error {
	t, ok := s.registry[typeKey]
	if !ok {
		return errors.New("tipo de papelera inválido: " + typeKey)
	}
	return s.db.Unscoped().Delete(t.model, id).Error
}
