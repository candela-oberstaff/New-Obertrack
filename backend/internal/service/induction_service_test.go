package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// q construye una pregunta calificable.
func q(id uint, correct string, weight int) models.SurveyQuestion {
	return models.SurveyQuestion{ID: id, CorrectAnswer: correct, Weight: weight}
}

func TestScoreAnswers(t *testing.T) {
	tests := []struct {
		name      string
		questions []models.SurveyQuestion
		answers   []SubmittedAnswer
		want      float64
	}{
		{
			name:      "todas correctas",
			questions: []models.SurveyQuestion{q(1, "a", 1), q(2, "b", 1)},
			answers:   []SubmittedAnswer{{QuestionID: 1, Value: "a"}, {QuestionID: 2, Value: "b"}},
			want:      100,
		},
		{
			name:      "todas incorrectas",
			questions: []models.SurveyQuestion{q(1, "a", 1), q(2, "b", 1)},
			answers:   []SubmittedAnswer{{QuestionID: 1, Value: "x"}, {QuestionID: 2, Value: "y"}},
			want:      0,
		},
		{
			name:      "la ponderacion pesa mas que el conteo",
			questions: []models.SurveyQuestion{q(1, "a", 3), q(2, "b", 1)},
			answers:   []SubmittedAnswer{{QuestionID: 1, Value: "a"}, {QuestionID: 2, Value: "mal"}},
			want:      75,
		},
		{
			name:      "normaliza mayusculas y espacios",
			questions: []models.SurveyQuestion{q(1, "Caracas", 1)},
			answers:   []SubmittedAnswer{{QuestionID: 1, Value: "  caracas "}},
			want:      100,
		},
		{
			name:      "las preguntas sin respuesta correcta no puntuan",
			questions: []models.SurveyQuestion{q(1, "a", 1), {ID: 2, CorrectAnswer: "", Weight: 5}},
			answers:   []SubmittedAnswer{{QuestionID: 1, Value: "a"}},
			want:      100,
		},
		{
			name:      "las preguntas con peso cero no puntuan",
			questions: []models.SurveyQuestion{q(1, "a", 1), q(2, "b", 0)},
			answers:   []SubmittedAnswer{{QuestionID: 1, Value: "a"}},
			want:      100,
		},
		{
			name:      "pregunta sin responder cuenta como incorrecta",
			questions: []models.SurveyQuestion{q(1, "a", 1), q(2, "b", 1)},
			answers:   []SubmittedAnswer{{QuestionID: 1, Value: "a"}},
			want:      50,
		},
		{
			// Un cuestionario mal configurado no debe dejar a todo profesional
			// nuevo fuera de la plataforma: se aprueba y se registra el aviso.
			name:      "sin preguntas calificables aprueba en vez de bloquear",
			questions: []models.SurveyQuestion{{ID: 1, CorrectAnswer: "", Weight: 0}},
			answers:   []SubmittedAnswer{},
			want:      100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scoreAnswers(tc.questions, tc.answers)
			if got != tc.want {
				t.Errorf("scoreAnswers() = %v, esperaba %v", got, tc.want)
			}
		})
	}
}

func TestInviteAttemptsLeft(t *testing.T) {
	inv := &models.InductionInvite{MaxAttempts: 3, Attempts: 1}
	if got := inv.AttemptsLeft(); got != 2 {
		t.Errorf("AttemptsLeft() = %d, esperaba 2", got)
	}
	// Nunca negativo, aunque los intentos superen el tope.
	inv.Attempts = 5
	if got := inv.AttemptsLeft(); got != 0 {
		t.Errorf("AttemptsLeft() = %d, esperaba 0", got)
	}
}

func TestConfigReady(t *testing.T) {
	surveyID := uint(7)
	cases := []struct {
		name string
		cfg  *models.InductionConfig
		want bool
	}{
		{"apagada", &models.InductionConfig{IsActive: false, SurveyID: &surveyID}, false},
		{"encendida sin cuestionario", &models.InductionConfig{IsActive: true}, false},
		{"encendida con cuestionario", &models.InductionConfig{IsActive: true, SurveyID: &surveyID}, true},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.Ready(); got != tc.want {
				t.Errorf("Ready() = %v, esperaba %v", got, tc.want)
			}
		})
	}
}
