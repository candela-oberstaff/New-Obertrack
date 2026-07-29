package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Frecuencias soportadas. Es un conjunto cerrado a propósito: la RRULE la
// construye nuestro formulario, y aceptar cualquier regla del estándar
// obligaría a implementar BYDAY, BYSETPOS y compañía para poder calcular las
// ocurrencias nosotros mismos.
const (
	FreqDaily   = "DAILY"
	FreqWeekly  = "WEEKLY"
	FreqMonthly = "MONTHLY"
)

// maxOccurrenceSteps acota la búsqueda de la próxima ocurrencia. Con una serie
// diaria cubre unos cinco años y medio, de sobra para una reunión de trabajo;
// sin el tope, una regla corrupta colgaría la petición en un bucle infinito.
const maxOccurrenceSteps = 2000

// Recurrence es una RRULE en la forma acotada que maneja Obertrack.
type Recurrence struct {
	Freq     string
	Interval int
	// Until es el último instante en el que puede empezar una ocurrencia
	// (inclusive). Nil junto a Count 0 = la serie no termina.
	Until *time.Time
	// Count es el número total de ocurrencias, incluida la primera. 0 = sin tope.
	Count int
}

// ParseRecurrence lee la RRULE. Devuelve nil sin error para la cadena vacía, que
// es una sesión única y no un dato inválido.
func ParseRecurrence(rule string) (*Recurrence, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil, nil
	}
	body := strings.TrimPrefix(strings.ToUpper(rule), "RRULE:")

	rec := &Recurrence{Interval: 1}
	for _, part := range strings.Split(body, ";") {
		key, value, found := strings.Cut(part, "=")
		if !found {
			return nil, fmt.Errorf("%w: la repetición tiene un tramo ilegible (%q)", ErrMeetingValidation, part)
		}
		switch key {
		case "FREQ":
			rec.Freq = value
		case "INTERVAL":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("%w: intervalo de repetición inválido (%q)", ErrMeetingValidation, value)
			}
			rec.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("%w: número de repeticiones inválido (%q)", ErrMeetingValidation, value)
			}
			rec.Count = n
		case "UNTIL":
			// Formato UTC básico del estándar: 20260930T235959Z.
			until, err := time.Parse("20060102T150405Z", value)
			if err != nil {
				return nil, fmt.Errorf("%w: fecha de fin de repetición inválida (%q)", ErrMeetingValidation, value)
			}
			rec.Until = &until
		default:
			return nil, fmt.Errorf("%w: la repetición usa una opción no soportada (%s)", ErrMeetingValidation, key)
		}
	}

	switch rec.Freq {
	case FreqDaily, FreqWeekly, FreqMonthly:
	default:
		return nil, fmt.Errorf("%w: frecuencia de repetición no soportada (%q)", ErrMeetingValidation, rec.Freq)
	}
	if rec.Count > 0 && rec.Until != nil {
		return nil, fmt.Errorf("%w: la repetición no puede terminar por fecha y por número a la vez", ErrMeetingValidation)
	}
	return rec, nil
}

// String vuelve a serializar la regla ya normalizada. Se guarda esto —y no lo
// que mandó el cliente— para que en la base de datos no convivan variantes del
// mismo significado.
func (r *Recurrence) String() string {
	if r == nil {
		return ""
	}
	parts := []string{"FREQ=" + r.Freq}
	if r.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(r.Interval))
	}
	if r.Count > 0 {
		parts = append(parts, "COUNT="+strconv.Itoa(r.Count))
	}
	if r.Until != nil {
		parts = append(parts, "UNTIL="+r.Until.UTC().Format("20060102T150405Z"))
	}
	return "RRULE:" + strings.Join(parts, ";")
}

// occurrenceAt devuelve el inicio de la ocurrencia número n (0 = la primera).
//
// El salto se calcula sobre la hora LOCAL de la serie y no sumando 24h: una
// reunión de las 15:00 debe seguir siendo a las 15:00 después del cambio de
// horario de verano, y sumar duraciones fijas la desplazaría una hora. time.Date
// normaliza el desbordamiento, así que "31 de enero + 1 mes" cae en marzo igual
// que hace Google.
func occurrenceAt(seriesStart time.Time, freq string, interval, n int, loc *time.Location) time.Time {
	local := seriesStart.In(loc)
	y, mo, d := local.Date()
	h, mi, s := local.Clock()

	switch freq {
	case FreqDaily:
		d += interval * n
	case FreqWeekly:
		d += 7 * interval * n
	case FreqMonthly:
		mo += time.Month(interval * n)
	}
	return time.Date(y, mo, d, h, mi, s, 0, loc)
}

// NextOccurrence devuelve el inicio de la próxima ocurrencia que todavía no ha
// terminado en el instante `after`. Una reunión ya empezada pero en curso cuenta
// como próxima: es a la que la gente se va a unir.
//
// El segundo valor es false cuando la serie ya se agotó (por COUNT o UNTIL).
func (r *Recurrence) NextOccurrence(
	seriesStart time.Time, duration time.Duration, loc *time.Location, after time.Time,
) (time.Time, bool) {
	if r == nil {
		if seriesStart.Add(duration).After(after) {
			return seriesStart, true
		}
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.UTC
	}

	limit := maxOccurrenceSteps
	if r.Count > 0 && r.Count < limit {
		limit = r.Count
	}
	for n := 0; n < limit; n++ {
		start := occurrenceAt(seriesStart, r.Freq, r.Interval, n, loc)
		if r.Until != nil && start.After(*r.Until) {
			return time.Time{}, false
		}
		if start.Add(duration).After(after) {
			return start, true
		}
	}
	return time.Time{}, false
}

// SeriesEnd devuelve el instante en que termina la ÚLTIMA ocurrencia, o nil si
// la serie no acaba nunca. Se persiste para poder filtrar en SQL qué series
// siguen vivas sin recorrer sus ocurrencias en Go.
func (r *Recurrence) SeriesEnd(seriesStart time.Time, duration time.Duration, loc *time.Location) *time.Time {
	if r == nil {
		end := seriesStart.Add(duration)
		return &end
	}
	if loc == nil {
		loc = time.UTC
	}

	if r.Count > 0 {
		end := occurrenceAt(seriesStart, r.Freq, r.Interval, r.Count-1, loc).Add(duration)
		return &end
	}
	if r.Until != nil {
		// UNTIL acota el INICIO de la última ocurrencia; el fin real es esa
		// ocurrencia más su duración, y usar UNTIL a secas dejaría fuera de
		// "próximas" una reunión que todavía está en curso.
		end := r.Until.Add(duration)
		return &end
	}
	return nil // serie infinita
}
