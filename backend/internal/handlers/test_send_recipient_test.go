package handlers

import "testing"

// La prueba de correo puede ir a otra dirección, pero solo a UNA y solo si es
// una dirección plausible. Estos casos fijan lo que acepta el filtro que decide
// si el destinatario indicado es válido.
func TestValidEmail_FiltraLoQuePuedeLlegarComoDestinoDePrueba(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"corporativo normal", "marketing@empresa.com", true},
		{"con subdominio", "qa@mail.empresa.co.uk", true},
		{"con etiqueta +", "carlos+pruebas@empresa.com", true},
		{"sin arroba", "empresa.com", false},
		{"arroba al principio", "@empresa.com", false},
		{"sin punto en el dominio", "alguien@localhost", false},
		{"vacío", "", false},
		// Un espacio delata que se pegaron dos direcciones o que se coló texto:
		// aquí solo cabe un destinatario.
		{"dos direcciones separadas por espacio", "a@b.com c@d.com", false},
		{"con tabulador", "a@b.com\tc", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validEmail(tc.in); got != tc.want {
				t.Fatalf("validEmail(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
