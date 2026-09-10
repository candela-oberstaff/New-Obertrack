package handlers

import "testing"

// El ETag es lo que decide si Obersuite se ahorra el padrón entero. Estos casos
// fijan las dos mitades: que el mismo contenido dé el mismo identificador, y que
// la comparación entienda lo que un cliente manda de verdad.
func TestWeakETag(t *testing.T) {
	a := weakETag([]byte(`[{"id":1,"name":"Acme"}]`))
	b := weakETag([]byte(`[{"id":1,"name":"Acme"}]`))
	if a != b {
		t.Errorf("el mismo cuerpo debe dar el mismo ETag: %s != %s", a, b)
	}

	// Un cambio mínimo tiene que cambiarlo, o devolveríamos 304 con datos viejos.
	c := weakETag([]byte(`[{"id":1,"name":"Acme S.A"}]`))
	if a == c {
		t.Error("un cuerpo distinto debe dar un ETag distinto")
	}

	// Formato: débil y entrecomillado, como exige la especificación.
	if len(a) < 4 || a[:3] != `W/"` || a[len(a)-1] != '"' {
		t.Errorf("formato inesperado: %s", a)
	}
}

func TestMatchesETag(t *testing.T) {
	actual := `W/"abc123"`

	casos := []struct {
		nombre        string
		ifNoneMatch   string
		quieroAcierto bool
	}{
		{"sin cabecera es la primera visita", "", false},
		{"el mismo ETag", `W/"abc123"`, true},
		{"un ETag viejo", `W/"deadbeef"`, false},
		// Un cliente puede guardar varias versiones y mandarlas todas.
		{"lista separada por comas", `W/"otro", W/"abc123"`, true},
		{"lista sin coincidencia", `W/"uno", W/"dos"`, false},
		// El comodín lo manda la especificación y significa "cualquiera que tengas".
		{"comodín", "*", true},
		// Con espacios alrededor, que es como llegan de verdad.
		{"con espacios", `   W/"abc123"   `, true},
	}
	for _, tc := range casos {
		if got := matchesETag(tc.ifNoneMatch, actual); got != tc.quieroAcierto {
			t.Errorf("%s: matchesETag(%q) = %v, se esperaba %v",
				tc.nombre, tc.ifNoneMatch, got, tc.quieroAcierto)
		}
	}
}
