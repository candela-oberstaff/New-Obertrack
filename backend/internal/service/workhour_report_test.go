package service

import "testing"

// htmlToPlainText convierte el HTML del editor tiptap (campo Activities de
// WorkHour) a texto plano para los reportes PDF/Excel. Estos casos documentan
// el comportamiento real de la función.
func TestHtmlToPlainText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "texto plano sin HTML queda igual con trim",
			in:   "  horas de soporte al cliente  ",
			want: "horas de soporte al cliente",
		},
		{
			name: "parrafos separados por salto de linea",
			in:   "<p>hola</p><p>mundo</p>",
			want: "hola\nmundo",
		},
		{
			name: "lista con viñetas en lineas separadas",
			in:   "<ul><li>uno</li><li>dos</li></ul>",
			want: "- uno\n- dos",
		},
		{
			name: "br simple genera salto de linea",
			in:   "<p>hola<br>mundo</p>",
			want: "hola\nmundo",
		},
		{
			name: "br autocerrado genera salto de linea",
			in:   "<p>hola<br/>mundo</p>",
			want: "hola\nmundo",
		},
		{
			name: "entidad amp decodificada",
			in:   "<p>ventas &amp; soporte</p>",
			want: "ventas & soporte",
		},
		{
			name: "entidad lt decodificada",
			in:   "<p>a &lt; b</p>",
			want: "a < b",
		},
		{
			name: "html de tiptap con atributos data-path-to-node",
			in:   `<p data-path-to-node="1"><b data-path-to-node="1.0">Negrita</b> normal</p>`,
			want: "Negrita normal",
		},
		{
			name: "divs anidados con estilos tailwind quedan en lineas separadas",
			in:   `<div style="--tw-border-spacing-y: 0;">llamadas<div>PS</div></div>`,
			want: "llamadas\nPS",
		},
		{
			name: "acentos UTF-8 intactos",
			in:   "<p>Reunión de capacitación</p>",
			want: "Reunión de capacitación",
		},
		{
			name: "espacios multiples colapsados dentro de HTML",
			in:   "<p>hola     mundo   cruel</p>",
			want: "hola mundo cruel",
		},
		{
			name: "espacios multiples sin HTML tambien se colapsan",
			in:   "hola     mundo",
			want: "hola mundo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := htmlToPlainText(tc.in)
			if got != tc.want {
				t.Errorf("htmlToPlainText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
