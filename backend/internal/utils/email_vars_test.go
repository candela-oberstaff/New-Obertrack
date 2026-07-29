package utils

import "testing"

func TestRenderVariablesHTML(t *testing.T) {
	data := EmailRecipient{
		Name:        "María González",
		Email:       "maria@acme.com",
		JobTitle:    "Diseñadora UX",
		CompanyName: "Acme Corp",
		City:        "Caracas",
	}.VariableData()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"clave simple", "Hola {{primer_nombre}}", "Hola María"},
		{"nombre completo", "{{nombre}}", "María González"},
		{"con espacios", "{{ empresa }}", "Acme Corp"},
		{"varias en una línea", "{{primer_nombre}} de {{empresa}}", "María de Acme Corp"},
		{"dentro de atributo", `<a href="mailto:{{email}}">esc</a>`, `<a href="mailto:maria@acme.com">esc</a>`},
		{"campo vacío usa el fallback del catálogo", "Saludos desde {{pais}}.", "Saludos desde ."},
		{"campo vacío con default en línea", "Zona: {{pais|LATAM}}", "Zona: LATAM"},
		{"default en línea ignorado si hay dato", "{{ciudad|tu ciudad}}", "Caracas"},
		{"clave desconocida no deja el token crudo", "x{{no_existe}}y", "xy"},
		{"clave desconocida con default", "{{no_existe|respaldo}}", "respaldo"},
		{"texto sin variables intacto", "<p>Hola a todos</p>", "<p>Hola a todos</p>"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RenderVariablesHTML(tc.in, data); got != tc.want {
				t.Errorf("RenderVariablesHTML(%q) = %q, se esperaba %q", tc.in, got, tc.want)
			}
		})
	}
}

// El fallback del catálogo debe usarse cuando el destinatario no tiene el dato
// y la plantilla no declaró uno propio: "Hola ," se lee peor que "Hola colega,".
func TestRenderVariablesFallbackDelCatalogo(t *testing.T) {
	data := EmailRecipient{Email: "sin-nombre@acme.com"}.VariableData()

	if got := RenderVariablesHTML("Hola {{primer_nombre}},", data); got != "Hola colega," {
		t.Errorf("se esperaba el fallback del catálogo, se obtuvo %q", got)
	}
	if got := RenderVariablesHTML("Hola {{primer_nombre|equipo}},", data); got != "Hola equipo," {
		t.Errorf("el default en línea debe ganarle al del catálogo, se obtuvo %q", got)
	}
}

// Los valores vienen de datos de usuario: en el cuerpo HTML se escapan para no
// romper el marcado, pero en el asunto (texto plano) deben viajar literales.
func TestRenderVariablesEscapaSoloEnHTML(t *testing.T) {
	data := EmailRecipient{Name: `Ana <script>alert(1)</script> & Co`}.VariableData()

	gotHTML := RenderVariablesHTML("<p>{{nombre}}</p>", data)
	want := "<p>Ana &lt;script&gt;alert(1)&lt;/script&gt; &amp; Co</p>"
	if gotHTML != want {
		t.Errorf("el cuerpo HTML debe escapar el valor:\n  obtenido: %q\n  esperado: %q", gotHTML, want)
	}

	gotText := RenderVariablesText("Novedades de {{nombre}}", data)
	if gotText != `Novedades de Ana <script>alert(1)</script> & Co` {
		t.Errorf("el asunto no debe escaparse, se obtuvo %q", gotText)
	}
}

func TestHasEmailVariables(t *testing.T) {
	cases := map[string]bool{
		"Hola {{nombre}}":    true,
		"{{empresa|Acme}}":   true,
		"Sin variables":      false,
		"Llaves { } sueltas": false,
		"{{ }} vacío":        false,
		"{{123 espacios}}":   false,
	}
	for in, want := range cases {
		if got := HasEmailVariables(in); got != want {
			t.Errorf("HasEmailVariables(%q) = %v, se esperaba %v", in, got, want)
		}
	}
}

// El catálogo es la fuente de verdad compartida con el frontend: cada entrada
// debe ser resoluble y única.
func TestCatalogoConsistente(t *testing.T) {
	data := EmailRecipient{Name: "Ana Pérez"}.VariableData()
	vistas := map[string]bool{}

	for _, v := range EmailVariables() {
		if v.Key == "" || v.Label == "" || v.Group == "" {
			t.Errorf("variable incompleta: %+v", v)
		}
		if vistas[v.Key] {
			t.Errorf("clave duplicada en el catálogo: %q", v.Key)
		}
		vistas[v.Key] = true

		if _, ok := data[v.Key]; !ok {
			t.Errorf("la variable %q está en el catálogo pero VariableData no la resuelve", v.Key)
		}
	}

	for key := range data {
		if !vistas[key] {
			t.Errorf("VariableData resuelve %q pero no está publicada en el catálogo", key)
		}
	}
}
