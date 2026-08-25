package service

import (
	"encoding/json"
	"fmt"
	"time"
)

// Árbol de condiciones. Deliberadamente pequeño: dos combinadores y cuatro
// operadores cubren todas las reglas del catálogo de la v1, y cada operador que se
// añade hay que saber representarlo también en el constructor visual de la Fase 4.
//
//	{"all": [
//	   {"field": "estado",            "op": "eq", "value": "en_proceso"},
//	   {"field": "tiene_responsable", "op": "eq", "value": false}
//	]}
//
// Un árbol vacío ({}) significa "sin condiciones": la regla aplica siempre que su
// disparador y su ámbito coincidan.
type conditionNode struct {
	All []conditionNode `json:"all,omitempty"`
	Any []conditionNode `json:"any,omitempty"`

	Field string `json:"field,omitempty"`
	Op    string `json:"op,omitempty"`
	Value any    `json:"value,omitempty"`
}

const (
	condOpEq  = "eq"
	condOpNeq = "neq"
	condOpIn  = "in"
	condOpNin = "nin"
)

// evalConditions decide si una regla aplica a un evento. Devuelve además el motivo
// cuando NO aplica, para que la ejecución quede 'skipped' con una explicación y no
// como un silencio indistinguible de una regla rota.
//
// Fail-closed ante un árbol ilegible: una condición que no se entiende NO se
// interpreta como "sin condiciones". Al revés, una regla mal guardada empezaría a
// avisar a gente que su autor quiso excluir.
func evalConditions(raw string, fields map[string]any) (bool, string) {
	if raw == "" || raw == "{}" || raw == "null" {
		return true, ""
	}
	var root conditionNode
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return false, fmt.Sprintf("las condiciones de la regla no se pudieron interpretar: %v", err)
	}
	if root.isEmpty() {
		return true, ""
	}
	if root.eval(fields) {
		return true, ""
	}
	return false, "las condiciones de la regla no se cumplen para este cambio"
}

func (n conditionNode) isEmpty() bool {
	return len(n.All) == 0 && len(n.Any) == 0 && n.Field == ""
}

func (n conditionNode) eval(fields map[string]any) bool {
	if len(n.All) > 0 {
		for _, child := range n.All {
			if !child.eval(fields) {
				return false
			}
		}
		// Un "all" vacío no llega aquí (isEmpty lo filtra antes), así que esto es
		// siempre el resultado de haber comprobado al menos una condición.
		return true
	}
	if len(n.Any) > 0 {
		for _, child := range n.Any {
			if child.eval(fields) {
				return true
			}
		}
		return false
	}
	if n.Field == "" {
		return false
	}

	actual, known := fields[n.Field]
	if !known {
		// Un campo que el motor no expone nunca se da por bueno: si una regla
		// pregunta por algo que no existe, lo seguro es no ejecutarla.
		return false
	}

	switch n.Op {
	case condOpEq, "":
		return sameValue(actual, n.Value)
	case condOpNeq:
		return !sameValue(actual, n.Value)
	case condOpIn:
		return containsValue(n.Value, actual)
	case condOpNin:
		return !containsValue(n.Value, actual)
	default:
		return false
	}
}

// sameValue compara sin sorpresas de tipo. El JSON del árbol trae números como
// float64 y el contexto también, pero un id que viaje como cadena ("3") debe seguir
// coincidiendo con el número 3: quien escribe la regla no tiene por qué saberlo.
func sameValue(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	if ab, ok := a.(bool); ok {
		bb, ok2 := toBool(b)
		return ok2 && ab == bb
	}
	if af, ok := toFloat(a); ok {
		if bf, ok2 := toFloat(b); ok2 {
			return af == bf
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func containsValue(list any, actual any) bool {
	items, ok := list.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if sameValue(actual, item) {
			return true
		}
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint64:
		return float64(t), true
	}
	return 0, false
}

func toBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		if t == "true" {
			return true, true
		}
		if t == "false" {
			return false, true
		}
	}
	return false, false
}

// isBeforeToday compara por día de calendario, no por instante: una tarea que vence
// hoy no está vencida aunque sean las 23:59.
func isBeforeToday(year, month, day int) bool {
	if year == 0 || month == 0 || day == 0 {
		return false
	}
	due := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return due.Before(today)
}
