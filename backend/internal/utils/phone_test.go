package utils

import "testing"

func TestNormalizePhoneDigits(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ya limpio (como lo da WAHA)", "34600000000", "34600000000"},
		{"con + y espacios", "+34 600 00 00 00", "34600000000"},
		{"con guiones", "+34-600-000-000", "34600000000"},
		{"prefijo internacional 00", "0034600000000", "34600000000"},
		{"con paréntesis y prefijo local", "+54 (11) 4444-5555", "541144445555"},
		{"vacío", "", ""},
		{"solo símbolos", "+ - ()", ""},
		// Un "00" que no es prefijo internacional sino parte del número no debe
		// perderse: solo se recorta cuando queda algo detrás.
		{"solo ceros", "00", "00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizePhoneDigits(tc.in); got != tc.want {
				t.Fatalf("NormalizePhoneDigits(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// No se inventa el prefijo de país: un número local se queda como está, para
// que el cruce no acabe abriendo la conversación de otra persona.
func TestNormalizePhoneDigits_NoAdivinaPrefijoDePais(t *testing.T) {
	if got := NormalizePhoneDigits("600 000 000"); got != "600000000" {
		t.Fatalf("got %q", got)
	}
}
