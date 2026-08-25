package service

import (
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// A qué empresa pertenece un tablero lo dice su propia columna, no quién lo creó.
//
// Deducirlo del creador dejaba fuera a la empresa de cualquier tablero creado por un
// superadmin desde soporte: el creador no pertenece a ninguna empresa, así que el
// dueño real del tablero no podía ni crear una tarea en él. Es el mismo error que ya
// se arregló una vez a nivel de TAREA (authorizeTaskTenant) y que había quedado vivo
// a nivel de tablero.
func TestAlcance_ElTableroLoDefineSuEmpresaYNoSuCreador(t *testing.T) {
	// Tablero de la empresa 2, creado por un superadmin (que no tiene empleador).
	tablero := &models.Board{
		ID: 17, TenantID: 2, CreatedBy: 1,
		Creator: models.User{ID: 1, IsSuperadmin: true},
	}
	s := &taskService{boardRepo: &dmBoardRepo{board: tablero}}

	if err := s.authorizeBoardTenant(17, 2, false); err != nil {
		t.Fatalf("la empresa dueña del tablero tiene que poder usarlo: %v", err)
	}
	// Y otra empresa sigue sin entrar.
	err := s.authorizeBoardTenant(17, 99, false)
	if err == nil || !strings.Contains(err.Error(), "permiso") {
		t.Fatalf("otra empresa no debe alcanzar el tablero, got %v", err)
	}
}

// Los tableros anteriores a que existiera la columna pueden traer tenant_id vacío: en
// esos se sigue resolviendo por el creador, como hasta ahora.
func TestAlcance_TableroViejoSinEmpresaSeResuelvePorElCreador(t *testing.T) {
	empleadorID := uint(2)
	casos := []struct {
		nombre  string
		tablero *models.Board
		tenant  uint
		vale    bool
	}{
		{
			"lo creó la propia empresa",
			&models.Board{ID: 1, CreatedBy: 2, Creator: models.User{ID: 2}},
			2, true,
		},
		{
			"lo creó un profesional de la empresa",
			&models.Board{ID: 1, CreatedBy: 7, Creator: models.User{ID: 7, EmpleadorID: &empleadorID}},
			2, true,
		},
		{
			"lo creó alguien de otra empresa",
			&models.Board{ID: 1, CreatedBy: 8, Creator: models.User{ID: 8}},
			2, false,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := &taskService{boardRepo: &dmBoardRepo{board: c.tablero}}
			err := s.authorizeBoardTenant(1, c.tenant, false)
			if c.vale && err != nil {
				t.Fatalf("debería dejar pasar: %v", err)
			}
			if !c.vale && err == nil {
				t.Fatal("no debería dejar pasar")
			}
		})
	}
}
