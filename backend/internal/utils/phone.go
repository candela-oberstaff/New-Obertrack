package utils

import "strings"

// NormalizePhoneDigits reduce un teléfono a sus dígitos para poder compararlo.
//
// Los números llegan de dos sitios que no se parecen: los de WhatsApp vienen de
// WAHA ya limpios ("34600000000"), y los que teclea una persona en la ficha de
// una empresa vienen como le sale ("+34 600 00 00 00", "0034-600-000-000").
// Cruzarlos tal cual no encuentra nunca nada.
//
// Solo se quita el prefijo internacional "00" (la forma europea de marcar el
// "+"). NO se adivina el prefijo de país cuando falta: un "600000000" suelto
// podría ser de media docena de países, y equivocarse aquí significa abrir la
// conversación de otra persona.
func NormalizePhoneDigits(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	// "0034600..." y "+34600..." son el mismo número escrito de dos maneras.
	if len(digits) > 2 && strings.HasPrefix(digits, "00") {
		digits = digits[2:]
	}
	return digits
}
