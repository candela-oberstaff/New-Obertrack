package repository

import (
	"reflect"
	"testing"
)

// El expediente de la empresa se escanea en una fila plana (tenantActivityRow) y
// luego se copia campo a campo a TenantActivity. Esa indirección tiene un modo de
// fallo silencioso: si se añade un campo a TenantActivity —y su columna al SQL—
// pero no a la fila plana, GORM escanea sin quejarse y el valor llega siempre en
// cero. No hay error, no hay log; el dato simplemente no aparece en la interfaz.
//
// Pasó de verdad: ref_id se añadió al SELECT y a TenantActivity pero no aquí, y
// el enlace "Ver testimonio" del expediente nunca se pintaba porque el id llegaba
// como 0.
//
// Esta prueba fija el espejo. Si falla, falta el campo en tenantActivityRow —y
// casi seguro también su columna en tenantEventsCTE y su línea en la copia.
func TestTenantActivityRow_EspejaTenantActivity(t *testing.T) {
	row := reflect.TypeOf(tenantActivityRow{})
	campos := make(map[string]reflect.Type, row.NumField())
	for i := 0; i < row.NumField(); i++ {
		f := row.Field(i)
		campos[f.Name] = f.Type
	}

	activity := reflect.TypeOf(TenantActivity{})
	for i := 0; i < activity.NumField(); i++ {
		f := activity.Field(i)
		got, ok := campos[f.Name]
		if !ok {
			t.Errorf("TenantActivity.%s no existe en tenantActivityRow: "+
				"llegaría siempre en cero, sin error", f.Name)
			continue
		}
		if got != f.Type {
			t.Errorf("TenantActivity.%s es %s pero en tenantActivityRow es %s", f.Name, f.Type, got)
		}
	}
}
