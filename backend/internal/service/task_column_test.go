package service

import (
	"errors"
	"testing"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// Mover una tarea a un estado que ninguna columna pinta la hacía desaparecer
// del tablero sin estar borrada. El servidor tiene que impedirlo, no solo la UI.
func TestValidateColumn(t *testing.T) {
	board := &models.Board{ID: 1, Phases: []models.Phase{
		{ID: 1, Name: "Por hacer", Status: "por_hacer"},
		{ID: 2, Name: "En revisión"}, // id derivado del nombre: "en_revisión"
		{ID: 3, Name: "Entregado", Status: "entregado"},
	}}
	s := &taskService{boardRepo: &wfBoardRepo{board: board}}

	for _, ok := range []string{"por_hacer", "en_revisión", "entregado", "finalizado"} {
		if err := s.validateColumn(1, ok); err != nil {
			t.Errorf("%q debería aceptarse: %v", ok, err)
		}
	}

	err := s.validateColumn(1, "en_proceso")
	if err == nil {
		t.Fatal("una columna que no existe debería rechazarse")
	}
	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("debería ser ErrInvalidInput (400), fue %v", err)
	}
	if err.Error() != `La columna "en_proceso" no existe en este tablero` {
		t.Errorf("mensaje inesperado: %q", err.Error())
	}
}

// Los tableros antiguos no tienen fases y usan los estados de siempre.
func TestValidateColumn_TableroSinFases(t *testing.T) {
	s := &taskService{boardRepo: &wfBoardRepo{board: &models.Board{ID: 1}}}
	if err := s.validateColumn(1, "en_proceso"); err != nil {
		t.Errorf("sin fases no se valida: %v", err)
	}
}
