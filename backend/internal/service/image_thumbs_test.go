package service

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Mitad izquierda roja opaca, mitad derecha transparente.
			if x < w/2 {
				img.Set(x, y, color.NRGBA{R: 255, A: 255})
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestThumbWidthForRedondeaALaLista(t *testing.T) {
	cases := map[int]int{0: 0, -5: 0, 100: 320, 700: 960, 960: 960, 5000: 1280}
	for in, want := range cases {
		if got := ThumbWidthFor(in); got != want {
			t.Errorf("ThumbWidthFor(%d) = %d, esperaba %d", in, got, want)
		}
	}
}

func TestThumbnailReduceYReusaLaCopia(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "captura.png"), 2000, 1000)

	thumb, err := ThumbnailPath(dir, "captura.png", 960)
	if err != nil {
		t.Fatalf("ThumbnailPath() = %v", err)
	}
	if filepath.Dir(thumb) != filepath.Join(dir, "thumbs") {
		t.Fatalf("la copia quedó en %s", thumb)
	}
	f, _ := os.Open(thumb)
	img, err := jpeg.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("la copia no es un JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 960 || b.Dy() != 480 {
		t.Fatalf("tamaño = %v, esperaba 960x480", b.Size())
	}
	// Lo transparente se aplana sobre blanco.
	r, g, b, _ := img.At(900, 240).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("lo transparente salió %d,%d,%d; esperaba blanco", r>>8, g>>8, b>>8)
	}
	r, g, _, _ = img.At(100, 240).RGBA()
	if r>>8 < 200 || g>>8 > 60 {
		t.Fatalf("lo rojo salió %d,%d", r>>8, g>>8)
	}

	// La segunda vez no se vuelve a generar.
	info, _ := os.Stat(thumb)
	again, err := ThumbnailPath(dir, "captura.png", 960)
	if err != nil || again != thumb {
		t.Fatalf("segunda vez = %s, %v", again, err)
	}
	info2, _ := os.Stat(again)
	if !info2.ModTime().Equal(info.ModTime()) {
		t.Fatal("la copia se regeneró")
	}
}

func TestThumbnailNoAgrandaNiTocaLoQueNoEsImagen(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "chica.png"), 300, 200)
	got, err := ThumbnailPath(dir, "chica.png", 960)
	if err != nil || got != filepath.Join(dir, "chica.png") {
		t.Fatalf("una imagen angosta debe servirse tal cual: %s, %v", got, err)
	}

	_ = os.WriteFile(filepath.Join(dir, "doc.pdf"), []byte("%PDF-1.4"), 0o644)
	if _, err := ThumbnailPath(dir, "doc.pdf", 960); err != ErrNotThumbnailable {
		t.Fatalf("un PDF = %v, esperaba ErrNotThumbnailable", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "rota.png"), []byte("no es png"), 0o644)
	if _, err := ThumbnailPath(dir, "rota.png", 960); err != ErrNotThumbnailable {
		t.Fatalf("un PNG roto = %v, esperaba ErrNotThumbnailable", err)
	}
}
