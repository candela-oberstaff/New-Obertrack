package service

import (
	"errors"
	"image"
	"image/color"
	_ "image/gif" // decodificar adjuntos GIF
	"image/jpeg"
	_ "image/png" // decodificar capturas PNG
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Copias reducidas de las imágenes subidas, para quien no necesita el original:
// la app móvil pide una captura de 1920 px en PNG (cientos de KB) a ~960 px en
// JPEG (decenas de KB). Se generan la primera vez y quedan en disco junto al
// original, en thumbs/; el nombre del original es único y nunca cambia, así que
// la copia tampoco caduca.

// ThumbWidths son los anchos que se sirven. Una lista cerrada evita que alguien
// llene el disco pidiendo un ancho distinto en cada petición.
var ThumbWidths = []int{320, 640, 960, 1280}

// ErrNotThumbnailable: el archivo no es una imagen que se pueda reducir (PDF,
// SVG, WebP...). Quien llama sirve el original.
var ErrNotThumbnailable = errors.New("no se puede reducir")

const thumbQuality = 82

// ThumbWidthFor devuelve el ancho de la lista más chico que cubre lo pedido, o
// 0 si no se pidió ninguno válido.
func ThumbWidthFor(requested int) int {
	if requested <= 0 {
		return 0
	}
	for _, w := range ThumbWidths {
		if w >= requested {
			return w
		}
	}
	return ThumbWidths[len(ThumbWidths)-1]
}

// ThumbnailPath devuelve la ruta de la copia de [filename] a [width] px de
// ancho, creándola si hace falta. Si el original ya es más angosto, devuelve el
// original: agrandar no aporta nada.
func ThumbnailPath(uploadPath, filename string, width int) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
		return "", ErrNotThumbnailable
	}
	original := filepath.Join(uploadPath, filename)
	dir := filepath.Join(uploadPath, "thumbs")
	thumb := filepath.Join(dir, strings.TrimSuffix(filename, filepath.Ext(filename))+"_w"+strconv.Itoa(width)+".jpg")
	if _, err := os.Stat(thumb); err == nil {
		return thumb, nil
	}

	f, err := os.Open(original)
	if err != nil {
		return "", err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return "", ErrNotThumbnailable
	}
	if cfg.Width <= width {
		return original, nil
	}
	// Una imagen enorme (o maliciosa) no se decodifica entera en memoria.
	if cfg.Width*cfg.Height > 40_000_000 {
		return "", ErrNotThumbnailable
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return "", ErrNotThumbnailable
	}

	height := cfg.Height * width / cfg.Width
	if height < 1 {
		height = 1
	}
	dst := downscale(src, width, height)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Se escribe aparte y se renombra: dos peticiones a la vez no dejan un
	// archivo a medias para la tercera.
	tmp, err := os.CreateTemp(dir, ".thumb-*")
	if err != nil {
		return "", err
	}
	if err := jpeg.Encode(tmp, dst, &jpeg.Options{Quality: thumbQuality}); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	if err := os.Rename(tmp.Name(), thumb); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return thumb, nil
}

// downscale reduce por promedio de área (cada píxel de destino es la media de
// los que cubre en el original): para achicar capturas con texto se ve mejor
// que el vecino más cercano y no necesita librerías externas. Lo transparente
// se aplana sobre blanco, porque JPEG no tiene alfa.
func downscale(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0 := b.Min.Y + y*sh/h
		y1 := b.Min.Y + (y+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0 := b.Min.X + x*sw/w
			x1 := b.Min.X + (x+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					// Premultiplicado: sumar el blanco que falta por la transparencia.
					white := 0xffff - uint64(pa)
					r += uint64(pr) + white
					g += uint64(pg) + white
					bl += uint64(pb) + white
					n++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r / n >> 8),
				G: uint8(g / n >> 8),
				B: uint8(bl / n >> 8),
				A: 0xff,
			})
		}
	}
	return dst
}
