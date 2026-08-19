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

	QuestionTypeText   QuestionType = "text"
	QuestionTypeRating QuestionType = "rating"
	QuestionTypeChoice QuestionType = "choice"

	// Tipo de encuesta. Una encuesta normal solo recolecta opiniones; una de
	// inducción se CALIFICA (respuesta correcta + ponderación) y decide el
	// acceso del profesional recién contratado.
	SurveyKindFeedback  = "feedback"
	SurveyKindInduction = "induction"
)

type Survey struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Title         string         `json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	Status        SurveyStatus   `gorm:"type:varchar(20);default:'draft'" json:"status"`
	CreatedBy     uint           `json:"created_by"`
	SendByEmail   bool           `gorm:"default:false" json:"send_by_email"`
	SendByInApp   bool           `gorm:"default:true" json:"send_by_inapp"`
	RecipientList string         `gorm:"type:text" json:"recipient_list"` // JSON array of user IDs

	// Kind distingue una encuesta de opinión de un cuestionario de inducción
	// calificable. Por defecto 'feedback': las encuestas existentes no cambian.
	Kind string `gorm:"size:20;not null;default:'feedback';index" json:"kind"`
	// PassingScore es el mínimo aprobatorio en porcentaje. Solo aplica a las de
	// tipo induction.
	PassingScore int `gorm:"not null;default:70" json:"passing_score"`

	Questions []SurveyQuestion `gorm:"foreignKey:SurveyID;constraint:OnDelete:CASCADE;" json:"questions"`
	Responses []SurveyResponse `gorm:"foreignKey:SurveyID;constraint:OnDelete:CASCADE;" json:"responses"`
}

type SurveyQuestion struct {
	ID         uint         `gorm:"primaryKey" json:"id"`
	SurveyID   uint         `json:"survey_id"`
	Text       string       `gorm:"type:text" json:"text"`
	Type       QuestionType `gorm:"type:varchar(20)" json:"type"`
	Options    string       `gorm:"type:text" json:"options"` // JSON array of strings for 'choice' type
	IsRequired bool         `gorm:"default:true" json:"is_required"`
	OrderIndex int          `json:"order_index"`

	// --- Calificación (solo cuestionarios de inducción) ---
	// CorrectAnswer es la respuesta esperada. Vacío = la pregunta no puntúa.
	//
	// Se serializa para que el editor (superadmin) pueda cargarla y releerla,
	// pero NUNCA debe llegar a quien responde: la landing pública arma su propio
	// DTO sin este campo, y GetSurvey lo borra para los no-superadmin. El
	// puntaje se calcula siempre en el servidor.
	CorrectAnswer string `gorm:"type:text" json:"correct_answer,omitempty"`
	// Weight es la ponderación de la pregunta dentro del cuestionario.
	Weight int `gorm:"not null;default:1" json:"weight"`
}

// IsScorable indica si la pregunta participa en el puntaje.
func (q *SurveyQuestion) IsScorable() bool {
	return q != nil && q.Weight > 0 && q.CorrectAnswer != ""
}

type SurveyResponse struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	SurveyID    uint       `json:"survey_id"`
	UserID      uint       `json:"user_id"`
	User        User       `gorm:"foreignKey:UserID" json:"user"`
	CompletedAt *time.Time `json:"completed_at"`

	Answers []SurveyAnswer `gorm:"foreignKey:ResponseID;constraint:OnDelete:CASCADE;" json:"answers"`
}

type SurveyAnswer struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ResponseID  uint           `json:"response_id"`
	QuestionID  uint           `json:"question_id"`
	Question    SurveyQuestion `gorm:"foreignKey:QuestionID" json:"question"`
	TextValue   string         `gorm:"type:text" json:"text_value"`
	NumberValue int            `json:"number_value"`
}
