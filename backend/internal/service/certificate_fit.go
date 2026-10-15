package service

import (
	"strings"

	"github.com/jung-kurt/gofpdf"

	"github.com/obertrack/backend/internal/models"
)

// Los campos de una sola línea (nombre, programa, fecha, código) no tienen
// ancho propio: un programa con nombre largo se salía del diseño. Ahora se
// ajustan al espacio que queda entre el ancla y los márgenes de la página.
const (
	// singleLineMargin es la fracción del ancho de la página que queda libre a
	// cada lado: los diseños suelen llevar un marco y el texto no debe pisarlo.
	singleLineMargin = 0.10
	// singleLineMinScale es lo más que se achica la letra antes de partir el
	// texto en varias líneas: por debajo deja de parecer un título.
	singleLineMinScale = 0.7
)

// singleLineMaxWidth es el ancho disponible para el campo según su ancla y su
// alineación (centrado: el doble del lado más corto). 0 = no hay espacio útil.
func singleLineMaxWidth(f models.CertificateField, pageW float64) float64 {
	margin := pageW * singleLineMargin
	x := pageW * f.X / 100
	left, right := x-margin, pageW-margin-x
	var w float64
	switch f.Align {
	case "C":
		w = 2 * min(left, right)
	case "R":
		w = left
	default:
		w = right
	}
	return max(w, 0)
}

// fitSingleLine deja la fuente puesta y devuelve el tamaño y las líneas con que
// cabe txt en maxW: primero achica la letra (hasta singleLineMinScale) y, si
// aun así no cabe, parte el texto por palabras a ese tamaño mínimo.
func fitSingleLine(pdf *gofpdf.Fpdf, font, style string, size float64, txt string, maxW float64) (float64, []string) {
	pdf.SetFont(font, style, size)
	w := pdf.GetStringWidth(txt)
	if maxW <= 0 || w <= maxW {
		return size, []string{txt}
	}
	// El ancho crece en proporción al tamaño; el 0.99 cubre el redondeo.
	if scaled := size * maxW / w * 0.99; scaled >= size*singleLineMinScale {
		pdf.SetFont(font, style, scaled)
		return scaled, []string{txt}
	}
	size *= singleLineMinScale
	pdf.SetFont(font, style, size)
	space := pdf.GetStringWidth(" ")
	var lines []string
	cur, curW := "", 0.0
	for _, word := range strings.Fields(txt) {
		ww := pdf.GetStringWidth(word)
		if cur != "" && curW+space+ww > maxW {
			lines = append(lines, cur)
			cur, curW = "", 0
		}
		if cur == "" {
			cur, curW = word, ww
		} else {
			cur, curW = cur+" "+word, curW+space+ww
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return size, lines
}
