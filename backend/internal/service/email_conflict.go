package service

import (
	"fmt"
	"strings"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// emailLookup es lo único que describeEmailConflict necesita del repositorio de
// usuarios. Se declara aquí, en vez de pedir la interfaz entera, para poder
// probar el mensaje sin implementar las treinta y pico operaciones que no usa.
type emailLookup interface {
	FindAnyByEmail(email string) (*models.User, error)
}

// emailTakenError es un choque de correo con un mensaje que dice DÓNDE está la
// cuenta que lo ocupa.
//
// Unwrap devuelve apperrors.ErrEmailTaken para que los handlers respondan 409
// con errors.Is, mientras Error() sigue siendo solo la frase para la persona:
// envolver con fmt.Errorf("%w: ...") habría pegado el texto técnico del
// centinela delante del mensaje, que es justo lo que se quiere quitar.
type emailTakenError struct{ msg string }

func (e *emailTakenError) Error() string { return e.msg }
func (e *emailTakenError) Unwrap() error { return apperrors.ErrEmailTaken }

// isDuplicateEmailErr reconoce la violación del índice único de correo.
//
// Se mira por texto y no con un error tipado de pgx porque el driver entra como
// dependencia indirecta; a cambio se exige que el error sea 23505 (unique_violation)
// Y que mencione el correo. Sin la segunda condición, cualquier otro índice único
// de la tabla acabaría contestando "ese correo ya existe", que es peor que no
// traducir nada: manda a buscar a la Papelera algo que no está ahí.
func isDuplicateEmailErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	isUnique := strings.Contains(msg, "23505") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint")
	return isUnique && strings.Contains(msg, "email")
}

// describeEmailConflict convierte el choque del índice en una instrucción.
//
// "Ya existe un usuario con ese correo" deja al operador en el mismo sitio: la
// cuenta no aparece en el listado —está borrada, o inactiva en otra empresa— y
// no hay forma de adivinar dónde mirar. Cada caso tiene una salida distinta y
// una sola de ellas es crear otra cuenta, así que el mensaje la nombra.
//
// El repositorio puede ser nil o fallar; entonces se devuelve la frase genérica,
// que sigue siendo mejor que el SQLSTATE crudo.
func describeEmailConflict(userRepo emailLookup, email string) error {
	generic := &emailTakenError{msg: "Ya existe un usuario con ese correo."}
	if userRepo == nil {
		return generic
	}
	existing, err := userRepo.FindAnyByEmail(email)
	if err != nil || existing == nil {
		return generic
	}

	who := strings.TrimSpace(existing.Name)
	if who == "" {
		who = existing.Email
	}

	switch {
	case existing.DeletedAt.Valid:
		return &emailTakenError{msg: fmt.Sprintf(
			"El correo %s pertenece a %s, una cuenta que está en la Papelera (se borró el %s). "+
				"Restáurala desde Papelera para conservar su expediente y sus horas, o bórrala en firme para liberar el correo.",
			existing.Email, who, existing.DeletedAt.Time.Format("02/01/2006"))}

	case !existing.IsActive:
		return &emailTakenError{msg: fmt.Sprintf(
			"El correo %s pertenece a %s, una cuenta desactivada%s. "+
				"Reactívala desde su ficha en vez de crear una nueva: así conserva su historial.",
			existing.Email, who, companySuffix(existing))}

	default:
		return &emailTakenError{msg: fmt.Sprintf(
			"El correo %s ya lo usa %s, una cuenta activa%s. Si es la misma persona, edita esa ficha en vez de crear otra.",
			existing.Email, who, companySuffix(existing))}
	}
}

// companySuffix añade " en <Empresa>" cuando se sabe de cuál es. Sin esto, un
// correo repetido entre dos clientes se lee como un error del formulario en vez
// de como lo que es: la misma persona dada de alta dos veces.
func companySuffix(u *models.User) string {
	name := strings.TrimSpace(u.CompanyName)
	if name == "" && u.Empleador != nil {
		name = strings.TrimSpace(u.Empleador.CompanyName)
		if name == "" {
			name = strings.TrimSpace(u.Empleador.Name)
		}
	}
	if name == "" {
		return ""
	}
	return " de " + name
}
