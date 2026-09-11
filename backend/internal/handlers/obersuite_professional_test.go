package handlers

import (
	"reflect"
	"strings"
	"testing"
)

// Los contactos de emergencia entran por /hire como lista y salen por la ficha
// como lista, pero entre medias viven en UN texto separado por comas (la
// convención de nuestra pantalla de perfil). Lo que se fija aquí es que el viaje
// de ida y vuelta devuelva exactamente lo que se mandó: si no, el reclutador los
// rellena obligado y a la ficha llegan cambiados.
func TestContactosDeEmergencia_IdaYVueltaExacta(t *testing.T) {
	enviados := []string{"+58 412 1234567 mamá", "0414 5556677 Juan (hermano)"}

	// Así los junta /hire (onboarding_service.go).
	guardado := strings.Join(enviados, ", ")

	if got := splitEmergencyContacts(guardado); !reflect.DeepEqual(got, enviados) {
		t.Fatalf("la ficha devolvería %q en vez de lo que mandaron %q", got, enviados)
	}
}

func TestContactosDeEmergencia_LoQueEscribioNuestraPantalla(t *testing.T) {
	casos := []struct {
		nombre string
		raw    string
		quiero []string
	}{
		{"vacío devuelve lista vacía, no nula", "", []string{}},
		{"solo espacios y comas", " , ,", []string{}},
		{"sin espacio tras la coma", "111,222", []string{"111", "222"}},
		{"espacios de más alrededor", "  111  ,  222 ", []string{"111", "222"}},
		{"uno solo sin coma", "0414 5556677", []string{"0414 5556677"}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := splitEmergencyContacts(c.raw); !reflect.DeepEqual(got, c.quiero) {
				t.Fatalf("%q -> %q, quería %q", c.raw, got, c.quiero)
			}
		})
	}
}

// La limitación conocida, dejada por escrito para que nadie la descubra en
// producción: una coma DENTRO de un contacto se lee como separador. Está
// documentado en /hire; si algún día se cambia el formato de guardado, esta
// prueba es la que hay que invertir.
func TestContactosDeEmergencia_UnaComaDentroSepara(t *testing.T) {
	got := splitEmergencyContacts("Juan, hermano 0414 5556677")
	if len(got) != 2 {
		t.Fatalf("hoy una coma dentro separa en dos: %q", got)
	}
}
