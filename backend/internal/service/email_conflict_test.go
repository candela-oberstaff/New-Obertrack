package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type fakeEmailLookup struct {
	user *models.User
	err  error
}

func (f fakeEmailLookup) FindAnyByEmail(string) (*models.User, error) { return f.user, f.err }

// El reconocimiento del error va por texto, así que estos casos son la única
// red que hay: si deja de reconocerlo, vuelve el SQLSTATE crudo a la pantalla;
// si reconoce de más, manda a buscar a la Papelera algo que no está ahí.
func TestIsDuplicateEmailErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			// El error literal que devolvió Postgres en el panel de Usuarios.
			name: "violación real del índice de correo",
			err:  errors.New(`ERROR: duplicate key value violates unique constraint "idx_users_email" (SQLSTATE 23505)`),
			want: true,
		},
		{
			// Otro índice único de la misma tabla NO puede contestar "ese correo
			// ya existe": el operador iría a buscar a la Papelera en vano.
			name: "violación única de otro índice",
			err:  errors.New(`ERROR: duplicate key value violates unique constraint "idx_channel_name_type_tenant" (SQLSTATE 23505)`),
			want: false,
		},
		{
			name: "error de validación cualquiera",
			err:  errors.New("Manager inválido: manager no encontrado"),
			want: false,
		},
		{
			// Un fallo de conexión mencionando un correo no es un duplicado.
			name: "error no único que menciona email",
			err:  errors.New("could not send email: connection refused"),
			want: false,
		},
		{name: "sin error", err: nil, want: false},
	}
	for _, tc := range cases {
		if got := isDuplicateEmailErr(tc.err); got != tc.want {
			t.Errorf("%s: isDuplicateEmailErr = %v, se esperaba %v", tc.name, got, tc.want)
		}
	}
}

// Cada estado de la cuenta que ocupa el correo tiene una salida distinta, y el
// mensaje existe para nombrarla. Un texto que solo diga "ya existe" deja al
// operador donde estaba.
func TestDescribeEmailConflict(t *testing.T) {
	borrada := &models.User{
		Name: "Lidia González", Email: "lidia@empresa.es", IsActive: true,
		DeletedAt: gorm.DeletedAt{Time: time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	inactiva := &models.User{
		Name: "Lidia González", Email: "lidia@empresa.es", IsActive: false,
		CompanyName: "Accés Vertical",
	}
	activa := &models.User{
		Name: "Lidia González", Email: "lidia@empresa.es", IsActive: true,
		CompanyName: "Accés Vertical",
	}

	cases := []struct {
		name    string
		lookup  emailLookup
		mustSay []string
	}{
		{"en la papelera", fakeEmailLookup{user: borrada}, []string{"Papelera", "12/03/2026", "Lidia González"}},
		{"desactivada", fakeEmailLookup{user: inactiva}, []string{"desactivada", "Accés Vertical", "Reactívala"}},
		{"activa en otra empresa", fakeEmailLookup{user: activa}, []string{"activa", "Accés Vertical"}},
		// Si la búsqueda falla se cae a la frase genérica, que sigue siendo mejor
		// que el error de la base de datos.
		{"búsqueda fallida", fakeEmailLookup{err: errors.New("db caída")}, []string{"Ya existe un usuario con ese correo"}},
		{"sin repositorio", nil, []string{"Ya existe un usuario con ese correo"}},
	}

	for _, tc := range cases {
		err := describeEmailConflict(tc.lookup, "lidia@empresa.es")

		// Los handlers deciden el 409 con errors.Is: sin esto el conflicto
		// volvería a salir como un 500.
		if !errors.Is(err, apperrors.ErrEmailTaken) {
			t.Errorf("%s: el error debería envolver ErrEmailTaken", tc.name)
		}
		// Y el centinela no puede asomar en el texto que lee la persona.
		if strings.Contains(err.Error(), "email already registered") {
			t.Errorf("%s: el texto técnico del centinela se coló en el mensaje: %s", tc.name, err.Error())
		}
		for _, frag := range tc.mustSay {
			if !strings.Contains(err.Error(), frag) {
				t.Errorf("%s: el mensaje debería mencionar %q, y dice: %s", tc.name, frag, err.Error())
			}
		}
	}
}
