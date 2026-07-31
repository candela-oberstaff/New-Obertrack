package service

import "testing"

// Brevo rechaza la petición ENTERA con "missing_parameter - name is missing in
// to" cuando el nombre del destinatario va vacío. Como hay destinatarios sin
// nombre por toda la base (contactos recién creados, importaciones a medias,
// direcciones escritas a mano), el nombre se rellena aquí antes de salir.
func TestRecipient_RellenaElNombreQueBrevoExige(t *testing.T) {
	cases := []struct {
		name      string
		email     string
		inName    string
		wantName  string
		wantEmail string
	}{
		{"con nombre se respeta", "ana@empresa.com", "Ana Ruiz", "Ana Ruiz", "ana@empresa.com"},
		{"sin nombre usa la parte local", "marketing@empresa.com", "", "marketing", "marketing@empresa.com"},
		{"solo espacios cuenta como vacío", "qa@empresa.com", "   ", "qa", "qa@empresa.com"},
		// Una dirección rota no debería impedir el intento: que responda Brevo.
		{"sin arroba se usa entera", "roto", "", "roto", "roto"},
		{"arroba al principio no deja parte local vacía", "@empresa.com", "", "@empresa.com", "@empresa.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := recipient(tc.email, tc.inName)
			if got.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tc.wantName)
			}
			if got.Email != tc.wantEmail {
				t.Errorf("Email = %q, want %q", got.Email, tc.wantEmail)
			}
			// Lo que nunca puede pasar, sea cual sea la entrada.
			if got.Name == "" {
				t.Error("el nombre nunca puede quedar vacío: Brevo devuelve 400")
			}
		})
	}
}
