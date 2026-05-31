package models

import (
	"time"

	"gorm.io/gorm"
)

type SurveyStatus string
type QuestionType string

const (
	SurveyStatusDraft  SurveyStatus = "draft"
	SurveyStatusActive SurveyStatus = "active"
	SurveyStatusClosed SurveyStatus = "closed"

	QuestionTypeText     QuestionType = "text"
	QuestionTypeRating   QuestionType = "rating"
	QuestionTypeChoice   QuestionType = "choice"
)

type Survey struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Title          string         `json:"title"`
	Description    string         `gorm:"type:text" json:"description"`
	Status         SurveyStatus   `gorm:"type:varchar(20);default:'draft'" json:"status"`
	CreatedBy      uint           `json:"created_by"`
	SendByEmail    bool           `gorm:"default:false" json:"send_by_email"`
	SendByInApp    bool           `gorm:"default:true" json:"send_by_inapp"`
	RecipientList  string         `gorm:"type:text" json:"recipient_list"` // JSON array of user IDs
	
	Questions      []SurveyQuestion `gorm:"foreignKey:SurveyID;constraint:OnDelete:CASCADE;" json:"questions"`
	Responses      []SurveyResponse `gorm:"foreignKey:SurveyID;constraint:OnDelete:CASCADE;" json:"responses"`
}

type SurveyQuestion struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	SurveyID    uint         `json:"survey_id"`
	Text        string       `gorm:"type:text" json:"text"`
	Type        QuestionType `gorm:"type:varchar(20)" json:"type"`
	Options     string       `gorm:"type:text" json:"options"` // JSON array of strings for 'choice' type
	IsRequired  bool         `gorm:"default:true" json:"is_required"`
	OrderIndex  int          `json:"order_index"`
}

type SurveyResponse struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	SurveyID    uint           `json:"survey_id"`
	UserID      uint           `json:"user_id"`
	User        User           `gorm:"foreignKey:UserID" json:"user"`
	CompletedAt *time.Time     `json:"completed_at"`
	
	Answers     []SurveyAnswer `gorm:"foreignKey:ResponseID;constraint:OnDelete:CASCADE;" json:"answers"`
}

type SurveyAnswer struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ResponseID   uint           `json:"response_id"`
	QuestionID   uint           `json:"question_id"`
	Question     SurveyQuestion `gorm:"foreignKey:QuestionID" json:"question"`
	TextValue    string         `gorm:"type:text" json:"text_value"`
	NumberValue  int            `json:"number_value"`
}
