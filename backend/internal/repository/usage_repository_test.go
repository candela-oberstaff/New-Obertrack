package repository

import (
	"strings"
	"testing"
	"time"
)

// La comparación con el período anterior es la que pinta las flechas de caída.
// Si se activara antes de tener medido ese período, el "-40%" que saldría sería
// el hueco de datos y no una caída: el panel acusaría de abandono a empresas que
// nunca dejaron de entrar.
func TestComparable(t *testing.T) {
	day := 24 * time.Hour
	ago := func(d int) *time.Time {
		t := time.Now().Add(-time.Duration(d) * day)
		return &t
	}

	cases := []struct {
		name  string
		since *time.Time
		days  int
		want  bool
	}{
		{"sin nada medido nunca se compara", nil, 30, false},
		{"medido justo hoy, con 30 días pedidos", ago(0), 30, false},
		{"medido a mitad del período anterior", ago(40), 30, false},
		{"medido justo al inicio del período anterior", ago(59), 30, true},
		{"medido mucho antes", ago(200), 30, true},
		{"períodos cortos también se comparan", ago(13), 7, true},
	}
	for _, tc := range cases {
		if got := comparable(tc.since, tc.days); got != tc.want {
			t.Errorf("%s: comparable() = %v, se esperaba %v", tc.name, got, tc.want)
		}
	}
}

// Las ventanas se interpolan en el SQL, así que un error de un día aquí no lo
// atrapa el compilador: se convierte en un porcentaje ligeramente equivocado que
// nadie va a cuestionar.
func TestUsageWindows(t *testing.T) {
	cur, prev, both := usageWindows(30)

	// 30 días son los 29 anteriores MÁS hoy, no 31.
	if !strings.Contains(cur, "CURRENT_DATE - 29") {
		t.Errorf("ventana actual mal calculada: %s", cur)
	}
	// El período anterior termina justo donde empieza el actual, sin solaparse
	// ni dejar un día sin contar en medio.
	if !strings.Contains(prev, "CURRENT_DATE - 59") || !strings.Contains(prev, "< CURRENT_DATE - 29") {
		t.Errorf("ventana anterior mal calculada: %s", prev)
	}
	if !strings.Contains(both, "CURRENT_DATE - 59") {
		t.Errorf("ventana conjunta mal calculada: %s", both)
	}

	// days=1 es "solo hoy": el día anterior es ayer, y el cálculo no puede
	// desbordar a un número negativo.
	cur1, prev1, _ := usageWindows(1)
	if !strings.Contains(cur1, "CURRENT_DATE - 0") {
		t.Errorf("un solo día mal calculado: %s", cur1)
	}
	if !strings.Contains(prev1, "CURRENT_DATE - 1") {
		t.Errorf("el día anterior mal calculado: %s", prev1)
	}
}

func TestPct(t *testing.T) {
	// Dividir entre cero es el caso normal aquí: una empresa sin gente todavía.
	if got := pct(3, 0); got != 0 {
		t.Errorf("pct(3, 0) = %v, se esperaba 0", got)
	}
	if got := pct(1, 4); got != 25 {
		t.Errorf("pct(1, 4) = %v, se esperaba 25", got)
	}
}
