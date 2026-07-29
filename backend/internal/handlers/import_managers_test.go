package handlers

import (
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// Cobertura de la columna reporta_a en la previsualización del import. Lo que se
// valida acá es lo que el backend NO puede saber después: los managers que
// todavía no existen porque vienen en el mismo archivo.

// mismaEmpresa trata todas las filas como de una sola empresa (el caso de la
// plantilla de empresa); los tests que necesitan varias lo sobreescriben.
func testResolver(existing map[string]*models.User, companyKey func(map[string]string) string) managerResolver {
	if companyKey == nil {
		companyKey = func(map[string]string) string { return "unica" }
	}
	return managerResolver{
		findUser: func(email string) *models.User {
			return existing[strings.ToLower(email)]
		},
		companyKey: companyKey,
		existingUsable: func(_ map[string]string, u *models.User) (bool, string) {
			if !u.IsManager {
				return false, "no está marcado como manager"
			}
			return true, ""
		},
	}
}

func row(n int, email, esManager, reportaA string) profSheetRow {
	return profSheetRow{row: n, data: map[string]string{
		"nombre": email, "email": email, "es_manager": esManager, "reporta_a": reportaA,
	}}
}

// El manager está más abajo en el archivo: la fila debe validar igual, porque
// la asignación ocurre en una segunda pasada.
func TestResolveProfManagers_ManagerLaterInFile(t *testing.T) {
	rows := []profSheetRow{
		row(2, "pedro@x.com", "No", "maria@x.com"),
		row(3, "maria@x.com", "Sí", ""),
	}
	errs, _ := resolveProfManagers(rows, testResolver(nil, nil))

	if errs[0] != "" {
		t.Fatalf("el manager de una fila posterior debe aceptarse, got: %q", errs[0])
	}
}

// Si alguien recibe reportes pero no venía marcado, se lo marca y se avisa.
func TestResolveProfManagers_AutoMarksManager(t *testing.T) {
	rows := []profSheetRow{
		row(2, "pedro@x.com", "No", "maria@x.com"),
		row(3, "maria@x.com", "", ""),
	}
	errs, warns := resolveProfManagers(rows, testResolver(nil, nil))

	if errs[0] != "" {
		t.Fatalf("no debe ser error, got: %q", errs[0])
	}
	if !managerFlag(rows[1].data["es_manager"]) {
		t.Fatalf("María debe quedar marcada como manager, got: %q", rows[1].data["es_manager"])
	}
	if warns[1] == "" {
		t.Fatal("el auto-marcado debe avisarse en la fila de la manager")
	}
	if !strings.Contains(warns[1], "2") {
		t.Fatalf("el aviso debe señalar qué fila le reporta, got: %q", warns[1])
	}
}

// Un círculo dentro del archivo no puede detectarlo el guard del servicio
// (ninguno de los dos existe todavía), así que se frena en la previsualización.
func TestResolveProfManagers_CycleInFile(t *testing.T) {
	rows := []profSheetRow{
		row(2, "a@x.com", "Sí", "b@x.com"),
		row(3, "b@x.com", "Sí", "a@x.com"),
	}
	errs, _ := resolveProfManagers(rows, testResolver(nil, nil))

	if errs[0] == "" || errs[1] == "" {
		t.Fatalf("ambas filas del círculo deben marcarse, got: %+v", errs)
	}
	if !strings.Contains(errs[0], "círculo") {
		t.Fatalf("expected mensaje de círculo, got: %q", errs[0])
	}
}

// Una cadena de tres niveles dentro del archivo es válida.
func TestResolveProfManagers_ThreeLevelChainOK(t *testing.T) {
	rows := []profSheetRow{
		row(2, "analista@x.com", "No", "gerente@x.com"),
		row(3, "gerente@x.com", "Sí", "director@x.com"),
		row(4, "director@x.com", "Sí", ""),
	}
	errs, _ := resolveProfManagers(rows, testResolver(nil, nil))

	for i, e := range errs {
		if e != "" {
			t.Fatalf("la cadena de tres niveles debe aceptarse; fila %d: %q", i, e)
		}
	}
}

// Quien entra en un círculo sin ser parte de él no debe marcarse: su fila está
// bien, el problema es de las otras dos.
func TestResolveProfManagers_RowFeedingCycleNotFlagged(t *testing.T) {
	rows := []profSheetRow{
		row(2, "ajeno@x.com", "No", "a@x.com"),
		row(3, "a@x.com", "Sí", "b@x.com"),
		row(4, "b@x.com", "Sí", "a@x.com"),
	}
	errs, _ := resolveProfManagers(rows, testResolver(nil, nil))

	if errs[0] != "" {
		t.Fatalf("la fila que solo alimenta el círculo no debe marcarse, got: %q", errs[0])
	}
	if errs[1] == "" || errs[2] == "" {
		t.Fatalf("las dos filas del círculo sí deben marcarse, got: %+v", errs)
	}
}

func TestResolveProfManagers_SelfReference(t *testing.T) {
	rows := []profSheetRow{row(2, "solo@x.com", "Sí", "solo@x.com")}
	errs, _ := resolveProfManagers(rows, testResolver(nil, nil))

	if errs[0] == "" {
		t.Fatal("nadie puede reportarse a sí mismo")
	}
}

// El manager no está en el archivo ni existe: hay que decirlo antes de importar.
func TestResolveProfManagers_UnknownManager(t *testing.T) {
	rows := []profSheetRow{row(2, "pedro@x.com", "No", "fantasma@x.com")}
	errs, _ := resolveProfManagers(rows, testResolver(nil, nil))

	if !strings.Contains(errs[0], "no existe") {
		t.Fatalf("expected 'no existe', got: %q", errs[0])
	}
}

// Existe en el sistema pero no es manager: no se lo auto-marca (tocaría un
// usuario que no forma parte del archivo), se pide corregirlo.
func TestResolveProfManagers_ExistingNotManager(t *testing.T) {
	existing := map[string]*models.User{
		"jefe@x.com": {ID: 7, Email: "jefe@x.com", IsManager: false, IsActive: true},
	}
	rows := []profSheetRow{row(2, "pedro@x.com", "No", "jefe@x.com")}
	errs, _ := resolveProfManagers(rows, testResolver(existing, nil))

	if errs[0] == "" {
		t.Fatal("un usuario existente sin rol de manager debe rechazarse")
	}
}

// El manager está en el archivo pero pertenece a otra empresa (solo puede pasar
// en la plantilla de superadmin, donde la empresa viene por columna).
func TestResolveProfManagers_ManagerInOtherCompany(t *testing.T) {
	rows := []profSheetRow{
		{row: 2, data: map[string]string{"email": "pedro@x.com", "empresa": "Alfa", "es_manager": "No", "reporta_a": "maria@y.com"}},
		{row: 3, data: map[string]string{"email": "maria@y.com", "empresa": "Beta", "es_manager": "Sí", "reporta_a": ""}},
	}
	byCompany := func(d map[string]string) string { return strings.ToLower(d["empresa"]) }
	errs, _ := resolveProfManagers(rows, testResolver(nil, byCompany))

	if !strings.Contains(errs[0], "otra empresa") {
		t.Fatalf("expected 'otra empresa', got: %q", errs[0])
	}
}
