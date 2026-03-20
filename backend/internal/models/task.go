package models

import (
	"time"

	"gorm.io/gorm"
)

type Board struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Color       string         `gorm:"size:20;default:'#3b82f6'" json:"color"`
	CreatedBy   uint           `gorm:"not null;index" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Creator User    `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Members []User  `gorm:"many2many:board_members" json:"members,omitempty"`
	Phases  []Phase `gorm:"many2many:board_phases" json:"phases,omitempty"`
}

func (Board) TableName() string {
	return "boards"
}

type Phase struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Name   string `gorm:"size:100;not null" json:"name"`
	Status string `gorm:"size:50" json:"status,omitempty"`
	Color  string `gorm:"size:20;default:'#6b7280'" json:"color"`
	Order  int    `gorm:"default:0" json:"order"`
}

func (Phase) TableName() string {
	return "phases"
}

type BoardPhase struct {
	BoardID uint `gorm:"primaryKey" json:"board_id"`
	PhaseID uint `gorm:"primaryKey" json:"phase_id"`
}

func (BoardPhase) TableName() string {
	return "board_phases"
}

type BoardMember struct {
	BoardID uint `gorm:"primaryKey" json:"board_id"`
	UserID  uint `gorm:"primaryKey" json:"user_id"`
}

func (BoardMember) TableName() string {
	return "board_members"
}

type Task struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"size:255;not null" json:"title"`
	Description string         `gorm:"type:text" json:"description"`
	Status      TaskStatus     `gorm:"type:varchar(20);not null;default:'por_hacer';index:idx_status_board" json:"status"`
	Priority    TaskPriority   `gorm:"type:varchar(20);not null;default:'medium'" json:"priority"`
	StartDate   *time.Time     `gorm:"type:date" json:"start_date,omitempty"`
	EndDate     *time.Time     `gorm:"type:date" json:"end_date,omitempty"`
	Completed   bool           `gorm:"default:false" json:"completed"`
	CreatedBy   uint           `gorm:"not null;index" json:"created_by"`
	BoardID     uint           `gorm:"index:idx_status_board" json:"board_id"`
	Creator     User           `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Board       Board          `gorm:"foreignKey:BoardID" json:"board,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Assignees   []User           `gorm:"many2many:task_users" json:"assignees,omitempty"`
	Comments    []Comment        `json:"comments,omitempty"`
	Attachments []TaskAttachment `json:"attachments,omitempty"`
}

func (Task) TableName() string {
	return "tasks"
}

type TaskUser struct {
	TaskID uint `gorm:"primaryKey" json:"task_id"`
	UserID uint `gorm:"primaryKey" json:"user_id"`
}

func (TaskUser) TableName() string {
	return "task_users"
}

type Comment struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	TaskID    uint           `gorm:"not null;index" json:"task_id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Comment) TableName() string {
	return "comments"
}

type TaskAttachment struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	TaskID     uint           `gorm:"not null;index" json:"task_id"`
	FileName   string         `gorm:"size:255;not null" json:"file_name"`
	FileURL    string         `gorm:"size:500;not null" json:"file_url"`
	FileSize   int64          `json:"file_size"`
	MimeType   string         `gorm:"size:100" json:"mime_type"`
	UploadedBy uint           `gorm:"not null" json:"uploaded_by"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (TaskAttachment) TableName() string {
	return "task_attachments"
}
