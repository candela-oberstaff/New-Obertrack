package service

import (
	"regexp"
	"strings"

	"github.com/jung-kurt/gofpdf"

	"github.com/obertrack/backend/internal/models"
)

// El campo "Texto libre" admite variables que se rellenan con los datos del
// certificado: así el párrafo ("por haber completado y aprobado la
// {programa}...") lo escribe la app y el diseño puede ir limpio.
var certificateVariable = regexp.MustCompile(`\{(nombre|programa|fecha|codigo)\}`)

func variableValue(name string, data certificateData) string {
	switch name {
	case "nombre":
		return data.Name
	case "programa":
		return data.Program
	case "fecha":
		return data.Date
	case "codigo":
		return data.Code
	}
	return ""
}

// paragraphSegment es un trozo de texto con un mismo estilo: el texto normal o
// el valor de una variable (que va resaltado).
type paragraphSegment struct {
	text     string
	variable bool
}

// paragraphWord es lo que no se puede partir entre líneas. Puede mezclar
// estilos: en "{programa}," el valor va resaltado y la coma no.
type paragraphWord []paragraphSegment

// paragraphLines devuelve, por cada línea escrita a mano (\n), sus palabras.
func paragraphLines(text string, data certificateData) [][]paragraphWord {
	var out [][]paragraphWord
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		// Segmentos de la línea, con las variables ya sustituidas.
		var segs []paragraphSegment
		last := 0
		for _, m := range certificateVariable.FindAllStringSubmatchIndex(line, -1) {
			if m[0] > last {
				segs = append(segs, paragraphSegment{text: line[last:m[0]]})
			}
			segs = append(segs, paragraphSegment{text: variableValue(line[m[2]:m[3]], data), variable: true})
			last = m[1]
		}
		if last < len(line) {
			segs = append(segs, paragraphSegment{text: line[last:]})
		}

		// Palabras: se corta en los espacios; lo que va pegado sigue junto.
		var words []paragraphWord
		var cur paragraphWord
		flush := func() {
			if len(cur) > 0 {
				words = append(words, cur)
				cur = nil
			}
		}
		for _, sg := range segs {
			parts := strings.Split(sg.text, " ")
			for i, p := range parts {
				if i > 0 {
					flush()
				}
				if p != "" {
					cur = append(cur, paragraphSegment{text: p, variable: sg.variable})
				}
			}
		}
		flush()
		out = append(out, words)
	}
	return out
}

// paragraphStyle es la tipografía con la que se mide y se escribe cada trozo.
type paragraphStyle struct {
	pdf       *gofpdf.Fpdf
	font      string
	size      float64
	bold      bool
	color     [3]int
	highlight [3]int
	tr        func(string) string
}

func (st paragraphStyle) apply(variable bool) {
	style := ""
	// Las variables van en negrita (semibold en Poppins) además de su color.
	if st.bold || variable {
		style = "B"
	}
	st.pdf.SetFont(st.font, style, st.size)
	c := st.color
	if variable {
		c = st.highlight
	}
	st.pdf.SetTextColor(c[0], c[1], c[2])
}

func (st paragraphStyle) text(s string) string {
	if utf8Fonts[st.font] {
		return s
	}
	return st.tr(s)
}

func (st paragraphStyle) wordWidth(w paragraphWord) float64 {
	total := 0.0
	for _, sg := range w {
		st.apply(sg.variable)
		total += st.pdf.GetStringWidth(st.text(sg.text))
	}
	return total
}

// paragraphLine es una línea ya partida: sus palabras y su ancho total.
type paragraphLine struct {
	words  []paragraphWord
	widths []float64
	width  float64
}

// layoutParagraph parte el texto en líneas que caben en Wrap (% de la página;
// 0 = sin límite), respetando los saltos escritos a mano. Mide con la
// tipografía real de cada trozo, así que coincide con lo que se escribe.
func layoutParagraph(st paragraphStyle, f models.CertificateField, data certificateData, pageW float64) []paragraphLine {
	maxW := pageW * f.Wrap / 100
	st.apply(false)
	space := st.pdf.GetStringWidth(" ")

	var lines []paragraphLine
	for _, words := range paragraphLines(f.Text, data) {
		cur := paragraphLine{}
		for _, w := range words {
			ww := st.wordWidth(w)
			extra := ww
			if len(cur.words) > 0 {
				extra += space
			}
			if maxW > 0 && len(cur.words) > 0 && cur.width+extra > maxW {
				lines = append(lines, cur)
				cur = paragraphLine{}
				extra = ww
			}
			cur.words = append(cur.words, w)
			cur.widths = append(cur.widths, ww)
			cur.width += extra
		}
		lines = append(lines, cur)
	}
	// Quita las líneas vacías del principio y del final (texto en blanco).
	for len(lines) > 0 && len(lines[0].words) == 0 {
		lines = lines[1:]
	}
	for len(lines) > 0 && len(lines[len(lines)-1].words) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// drawParagraph escribe un "Texto libre": sustituye las variables, parte las
// líneas al ancho del campo (Wrap, % de la página; 0 = una sola línea) y
// centra el bloque en (X, Y), alineando cada línea como diga el campo.
func drawParagraph(pdf *gofpdf.Fpdf, f models.CertificateField, data certificateData, font string, pageW, pageH float64, tr func(string) string) {
	color, ok := hexToRGB(f.Color)
	if !ok {
		color = [3]int{15, 23, 42}
	}
	highlight, ok := hexToRGB(f.Highlight)
	if !ok {
		highlight = color
	}
	st := paragraphStyle{pdf: pdf, font: font, size: f.Size, bold: f.Bold, color: color, highlight: highlight, tr: tr}

	lines := layoutParagraph(st, f, data, pageW)
	if len(lines) == 0 {
		return
	}
	st.apply(false) // el espacio, medido igual que en layoutParagraph
	space := pdf.GetStringWidth(" ")

	// Sin el margen interno de las celdas: los trozos de una misma palabra
	// ("Inducción" + ",") van pegados, uno detrás de otro.
	prevMargin := pdf.GetCellMargin()
	pdf.SetCellMargin(0)
	defer pdf.SetCellMargin(prevMargin)

	// Alto de línea en mm (puntos × 0.3528) con un poco de aire.
	lineH := f.Size * 0.3528 * 1.25
	anchorX := pageW * f.X / 100
	top := pageH*f.Y/100 - lineH*float64(len(lines))/2
	for i, ln := range lines {
		x := anchorX
		switch f.Align {
		case "C":
			x -= ln.width / 2
		case "R":
			x -= ln.width
		}
		y := top + lineH*float64(i)
		for j, w := range ln.words {
			if j > 0 {
				x += space
			}
			for _, sg := range w {
				st.apply(sg.variable)
				txt := st.text(sg.text)
				sw := pdf.GetStringWidth(txt)
				pdf.SetXY(x, y)
				pdf.CellFormat(sw, lineH, txt, "", 0, "L", false, 0, "")
				x += sw
			}
		}
	}
}
