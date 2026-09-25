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

func TestInviteBlockAttemptsLeft(t *testing.T) {
	block := &models.InductionInviteBlock{Attempts: 1}
	if got := block.AttemptsLeft(3); got != 2 {
		t.Errorf("AttemptsLeft() = %d, esperaba 2", got)
	}
	// Nunca negativo, aunque los intentos superen el tope.
	block.Attempts = 5
	if got := block.AttemptsLeft(3); got != 0 {
		t.Errorf("AttemptsLeft() = %d, esperaba 0", got)
	}
}

// El mínimo del bloque manda sobre el del programa, y solo si lo tiene.
func TestBlockEffectivePassingScore(t *testing.T) {
	own := 85
	if got := (&models.InductionBlock{PassingScore: &own}).EffectivePassingScore(70); got != 85 {
		t.Errorf("con mínimo propio: %d, esperaba 85", got)
	}
	if got := (&models.InductionBlock{}).EffectivePassingScore(70); got != 70 {
		t.Errorf("sin mínimo propio: %d, esperaba 70", got)
	}
}

func TestProgramUsable(t *testing.T) {
	block := models.InductionBlock{ID: 1, SurveyID: 7}
	cases := []struct {
		name    string
		program *models.InductionProgram
		want    bool
	}{
		{"apagado", &models.InductionProgram{IsActive: false, Blocks: []models.InductionBlock{block}}, false},
		{"encendido sin bloques", &models.InductionProgram{IsActive: true}, false},
		{"encendido con bloques", &models.InductionProgram{IsActive: true, Blocks: []models.InductionBlock{block}}, true},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.program.Usable(); got != tc.want {
				t.Errorf("Usable() = %v, esperaba %v", got, tc.want)
			}
		})
	}
}

// El bloque actual es el primer pendiente en orden; si no queda ninguno, nil.
func TestCurrentBlock(t *testing.T) {
	blocks := []models.InductionInviteBlock{
		{BlockID: 1, OrderIndex: 0, Status: models.InductionPassed},
		{BlockID: 2, OrderIndex: 1, Status: models.InductionPending},
		{BlockID: 3, OrderIndex: 2, Status: models.InductionPending},
	}
	if cur := currentBlock(blocks); cur == nil || cur.BlockID != 2 {
		t.Fatalf("esperaba el bloque 2, got %+v", cur)
	}
	blocks[1].Status = models.InductionPassed
	blocks[2].Status = models.InductionPassed
	if cur := currentBlock(blocks); cur != nil {
		t.Fatalf("sin pendientes debe ser nil, got %+v", cur)
	}
	if got := countCompleted(blocks); got != 3 {
		t.Fatalf("countCompleted = %d, esperaba 3", got)
	}
}
