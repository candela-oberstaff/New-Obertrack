// Generador de archivos de prueba para la importación masiva (columna reporta_a).
// Se corre a mano: go run ./cmd/gentestdata <carpeta-destino>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

var companyHeaders = []string{
	"nombre_responsable *", "email *", "nombre_empresa *",
	"industria", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "direccion",
}

var profHeaders = []string{
	"nombre *", "email *", "empresa *",
	"cargo", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "es_manager", "reporta_a",
}

var employerProfHeaders = []string{
	"nombre *", "email *",
	"cargo", "telefono", "pais", "estado_provincia", "ciudad", "ubicacion", "es_manager", "reporta_a",
}

func sheet(f *excelize.File, name string, headers []string, rows [][]string, headerStyle int) {
	if _, err := f.NewSheet(name); err != nil {
		panic(err)
	}
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetCellValue(name, fmt.Sprintf("%s1", col), h)
		_ = f.SetCellStyle(name, fmt.Sprintf("%s1", col), fmt.Sprintf("%s1", col), headerStyle)
		_ = f.SetColWidth(name, col, col, 26)
	}
	for r, row := range rows {
		for i, v := range row {
			col, _ := excelize.ColumnNumberToName(i + 1)
			_ = f.SetCellValue(name, fmt.Sprintf("%s%d", col, r+2), v)
		}
	}
	_ = f.SetRowHeight(name, 1, 22)
}

func newBook() (*excelize.File, int) {
	f := excelize.NewFile()
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"6D28D9"}, Pattern: 1},
	})
	return f, style
}

func save(f *excelize.File, path string) {
	// La hoja por defecto sobra: se borra recién al final, porque un libro no
	// puede quedarse sin ninguna hoja.
	if idx, err := f.GetSheetIndex("Sheet1"); err == nil && idx >= 0 {
		_ = f.DeleteSheet("Sheet1")
	}
	f.SetActiveSheet(0)
	if err := f.SaveAs(path); err != nil {
		panic(err)
	}
	fmt.Println("✔", path)
}

// ---------------------------------------------------------------------------
// 1) Caso feliz: dos empresas y un organigrama de tres niveles.
// ---------------------------------------------------------------------------

func happyPath(dir string) {
	f, hs := newBook()

	empresas := [][]string{
		{"Juan Peralta", "juan.peralta@nordika.test", "Nordika Logística S.A.",
			"Logística", "+58 412 300 1100", "Venezuela", "Distrito Capital", "Caracas", "Los Ruices", "Av. Francisco de Miranda, Torre Alfa, piso 6"},
		{"Sofía Marín", "sofia.marin@velastudio.test", "Vela Studio C.A.",
			"Diseño y publicidad", "+58 414 220 7788", "Venezuela", "Zulia", "Maracaibo", "Bella Vista", "Calle 67 con Av. 3E, Edif. Vela"},
	}

	// Nordika: dirección → dos gerencias → equipos.
	// Vela Studio: una manager con tres personas a cargo.
	// Andrés Belmonte llega con es_manager "No" a propósito: dos personas le
	// reportan, así que el sistema debe marcarlo solo y avisarlo.
	profesionales := [][]string{
		{"Ricardo Osorio", "ricardo.osorio@nordika.test", "Nordika Logística S.A.",
			"Director de Operaciones", "+58 412 300 1101", "Venezuela", "Distrito Capital", "Caracas", "Los Ruices", "Sí", ""},

		{"Lucía Ferrer", "lucia.ferrer@nordika.test", "Nordika Logística S.A.",
			"Gerente de Almacén", "+58 412 300 1102", "Venezuela", "Carabobo", "Valencia", "San Diego", "Sí", "ricardo.osorio@nordika.test"},
		{"Martín Salas", "martin.salas@nordika.test", "Nordika Logística S.A.",
			"Gerente de Tecnología", "+58 412 300 1103", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "Sí", "ricardo.osorio@nordika.test"},

		{"Daniela Rangel", "daniela.rangel@nordika.test", "Nordika Logística S.A.",
			"Coordinadora de Inventario", "+58 424 511 2200", "Venezuela", "Carabobo", "Valencia", "San Diego", "No", "lucia.ferrer@nordika.test"},
		{"Héctor Quintero", "hector.quintero@nordika.test", "Nordika Logística S.A.",
			"Supervisor de Despacho", "+58 424 511 2201", "Venezuela", "Carabobo", "Valencia", "Naguanagua", "No", "lucia.ferrer@nordika.test"},

		{"Andrés Belmonte", "andres.belmonte@nordika.test", "Nordika Logística S.A.",
			"Líder de Desarrollo", "+58 426 700 3310", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "No", "martin.salas@nordika.test"},
		{"Valentina Ochoa", "valentina.ochoa@nordika.test", "Nordika Logística S.A.",
			"Desarrolladora Backend", "+58 426 700 3311", "Venezuela", "Distrito Capital", "Caracas", "El Rosal", "No", "andres.belmonte@nordika.test"},
		{"Emilio Castro", "emilio.castro@nordika.test", "Nordika Logística S.A.",
			"Desarrollador Frontend", "+58 426 700 3312", "Venezuela", "Miranda", "Los Teques", "Centro", "No", "andres.belmonte@nordika.test"},
		{"Paula Mendoza", "paula.mendoza@nordika.test", "Nordika Logística S.A.",
			"Analista de Soporte", "+58 426 700 3313", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "No", "martin.salas@nordika.test"},

		{"Camila Duarte", "camila.duarte@velastudio.test", "Vela Studio C.A.",
			"Directora Creativa", "+58 414 220 7790", "Venezuela", "Zulia", "Maracaibo", "Bella Vista", "Sí", ""},
		{"Tomás Iriarte", "tomas.iriarte@velastudio.test", "Vela Studio C.A.",
			"Diseñador Senior", "+58 414 220 7791", "Venezuela", "Zulia", "Maracaibo", "La Lago", "No", "camila.duarte@velastudio.test"},
		{"Renata Sifontes", "renata.sifontes@velastudio.test", "Vela Studio C.A.",
			"Community Manager", "+58 414 220 7792", "Venezuela", "Lara", "Barquisimeto", "Este", "No", "camila.duarte@velastudio.test"},
		{"Gabriel Antúnez", "gabriel.antunez@velastudio.test", "Vela Studio C.A.",
			"Editor de Video", "+58 414 220 7793", "Venezuela", "Zulia", "Maracaibo", "Bella Vista", "No", "camila.duarte@velastudio.test"},
	}

	sheet(f, "Empresas", companyHeaders, empresas, hs)
	sheet(f, "Profesionales", profHeaders, profesionales, hs)
	save(f, filepath.Join(dir, "prueba_import_ok.xlsx"))
}

// ---------------------------------------------------------------------------
// 2) Casos borde: cada fila rompe (o pone a prueba) una validación distinta.
// ---------------------------------------------------------------------------

func edgeCases(dir string) {
	f, hs := newBook()

	empresas := [][]string{
		{"Irene Vargas", "irene.vargas@casoborde.test", "Caso Borde S.A.",
			"Pruebas", "+58 412 111 0001", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "Av. Principal 1"},
		{"Óscar Pineda", "oscar.pineda@otraempresa.test", "Otra Empresa C.A.",
			"Pruebas", "+58 412 111 0002", "Venezuela", "Miranda", "Guarenas", "Centro", "Calle 2"},
	}

	profesionales := [][]string{
		// OK — raíz sana de la que cuelga el resto.
		{"Base Correcta", "base@casoborde.test", "Caso Borde S.A.",
			"Directora", "+58 412 111 1000", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "Sí", ""},

		// AVISO — recibe reportes sin venir marcada: debe auto-marcarse.
		{"Auto Marcada", "automarcada@casoborde.test", "Caso Borde S.A.",
			"Coordinadora", "+58 412 111 1001", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", "base@casoborde.test"},
		{"Subordinado Uno", "sub1@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1002", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", "automarcada@casoborde.test"},

		// ERROR — círculo: cada uno reporta al otro.
		{"Ciclo Ana", "ciclo.ana@casoborde.test", "Caso Borde S.A.",
			"Gerente", "+58 412 111 1003", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "Sí", "ciclo.bruno@casoborde.test"},
		{"Ciclo Bruno", "ciclo.bruno@casoborde.test", "Caso Borde S.A.",
			"Gerente", "+58 412 111 1004", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "Sí", "ciclo.ana@casoborde.test"},

		// OK — alimenta el círculo pero no forma parte: NO debe marcarse.
		{"Ajeno Al Ciclo", "ajeno@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1005", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", "ciclo.ana@casoborde.test"},

		// ERROR — se reporta a sí mismo.
		{"Auto Referencia", "autoref@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1006", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", "autoref@casoborde.test"},

		// ERROR — el manager no está en el archivo ni en el sistema.
		{"Manager Fantasma", "fantasma@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1007", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", "noexiste@casoborde.test"},

		// ERROR — el manager está en el archivo pero es de la otra empresa.
		{"Cruce Empresas", "cruce@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1008", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", "jefe.otra@otraempresa.test"},
		{"Jefe Otra Empresa", "jefe.otra@otraempresa.test", "Otra Empresa C.A.",
			"Gerente", "+58 412 111 1009", "Venezuela", "Miranda", "Guarenas", "Centro", "Sí", ""},

		// AVISO — su manager viene con error, así que quedará sin asignar.
		{"Huérfano", "huerfano@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1010", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", "email-invalido@@casoborde.test"},
		// ERROR — email inválido (es el manager del de arriba).
		{"Email Roto", "email-invalido@@casoborde.test", "Caso Borde S.A.",
			"Gerente", "+58 412 111 1011", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "Sí", ""},

		// ERROR — faltan campos obligatorios (sin nombre).
		{"", "sinnombre@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1012", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", ""},

		// ERROR — email repetido dentro del archivo.
		{"Repetido", "base@casoborde.test", "Caso Borde S.A.",
			"Analista", "+58 412 111 1013", "Venezuela", "Distrito Capital", "Caracas", "Altamira", "No", ""},
	}

	sheet(f, "Empresas", companyHeaders, empresas, hs)
	sheet(f, "Profesionales", profHeaders, profesionales, hs)
	save(f, filepath.Join(dir, "prueba_import_casos_borde.xlsx"))
}

// ---------------------------------------------------------------------------
// 3) Plantilla de empresa: una sola hoja, sin columna de empresa.
// ---------------------------------------------------------------------------

func employerFile(dir string) {
	f, hs := newBook()

	profesionales := [][]string{
		{"Marisol Aguirre", "marisol.aguirre@miempresa.test",
			"Gerente de Proyectos", "+58 412 900 5500", "Venezuela", "Distrito Capital", "Caracas", "La Castellana", "Sí", ""},
		{"Iván Rosales", "ivan.rosales@miempresa.test",
			"Líder de Equipo", "+58 412 900 5501", "Venezuela", "Distrito Capital", "Caracas", "La Castellana", "No", "marisol.aguirre@miempresa.test"},
		{"Noelia Bracho", "noelia.bracho@miempresa.test",
			"Analista Funcional", "+58 412 900 5502", "Venezuela", "Aragua", "Maracay", "Centro", "No", "ivan.rosales@miempresa.test"},
		{"Simón Alcalá", "simon.alcala@miempresa.test",
			"QA", "+58 412 900 5503", "Venezuela", "Aragua", "Maracay", "Centro", "No", "ivan.rosales@miempresa.test"},
		{"Fabiana Trejo", "fabiana.trejo@miempresa.test",
			"Diseñadora UX", "+58 412 900 5504", "Venezuela", "Distrito Capital", "Caracas", "Chacao", "No", "marisol.aguirre@miempresa.test"},
		{"Rubén Casanova", "ruben.casanova@miempresa.test",
			"Soporte Técnico", "+58 412 900 5505", "Venezuela", "Miranda", "Los Teques", "Centro", "No", "marisol.aguirre@miempresa.test"},
	}

	sheet(f, "Profesionales", employerProfHeaders, profesionales, hs)
	save(f, filepath.Join(dir, "prueba_import_empresa.xlsx"))
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	happyPath(dir)
	edgeCases(dir)
	employerFile(dir)
}
