package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratePreviewFiles no es una prueba: es un generador que vuelca las
// plantillas de correo a HTML estáticos para abrirlos en el navegador sin
// levantar el backend. Correr con: go test -run TestGeneratePreviewFiles
func TestGeneratePreviewFiles(t *testing.T) {
	out := filepath.Join("..", "..", "..", "email-previews")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	index := `<!doctype html><meta charset="utf-8"><title>Correos</title>
<body style="font-family:system-ui;padding:40px;background:#f5f2fb"><h1>Vista previa de correos</h1><ul>`
	for _, p := range previews {
		f := p.Slug + ".html"
		if err := os.WriteFile(filepath.Join(out, f), []byte(p.Build()), 0o644); err != nil {
			t.Fatal(err)
		}
		index += `<li><a href="` + f + `">` + p.Title + `</a></li>`
	}
	index += `</ul></body>`
	_ = os.WriteFile(filepath.Join(out, "index.html"), []byte(index), 0o644)
	abs, _ := filepath.Abs(out)
	t.Logf("Plantillas generadas en: %s", abs)
}
