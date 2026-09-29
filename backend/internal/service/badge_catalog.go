package service

import (
	"fmt"

	"github.com/obertrack/backend/internal/models"
)

// Set cerrado de iconos y colores para las insignias. Cerrado a propósito: el
// frontend tiene exactamente estos iconos cargados, y una paleta fija evita
// que cada insignia se vea distinta a las demás. Los nombres son los de
// lucide-react.
var badgeIcons = map[string]bool{
	"Award": true, "Medal": true, "Trophy": true, "Star": true,
	"Shield": true, "ShieldCheck": true, "GraduationCap": true, "BookOpen": true,
	"Lightbulb": true, "Rocket": true, "Target": true, "Flag": true,
	"Heart": true, "Zap": true, "Crown": true, "Sparkles": true,
}

var badgeColors = map[string]bool{
	"orchid": true, "indigo": true, "emerald": true, "amber": true,
	"rose": true, "sky": true, "slate": true, "gold": true,
}

const (
	DefaultBlockBadgeIcon    = "Award"
	DefaultBlockBadgeColor   = "orchid"
	DefaultProgramBadgeIcon  = "Trophy"
	DefaultProgramBadgeColor = "gold"
)

// normalizeBadgeIcon devuelve el icono si es válido, o el de respaldo.
func normalizeBadgeIcon(icon, fallback string) string {
	if badgeIcons[icon] {
		return icon
	}
	return fallback
}

func normalizeBadgeColor(color, fallback string) string {
	if badgeColors[color] {
		return color
	}
	return fallback
}

// Méritos automáticos. Se ganan al completar un programa según CÓMO se hizo,
// sin configuración: son las insignias que le dan juego a la inducción.
const (
	MeritFirstTry = "first_try" // Ningún intento fallido en todo el programa
	MeritPerfect  = "perfect"   // 100% en todos los bloques
)

type meritDef struct {
	Title string
	Icon  string
	Color string
	// describe arma la descripción con el nombre del programa.
	describe func(program string) string
}

var meritCatalog = map[string]meritDef{
	MeritFirstTry: {
		Title: "A la primera",
		Icon:  "Zap",
		Color: "amber",
		describe: func(program string) string {
			return fmt.Sprintf("Completaste «%s» sin fallar ningún intento.", program)
		},
	},
	MeritPerfect: {
		Title: "Impecable",
		Icon:  "Crown",
		Color: "gold",
		describe: func(program string) string {
			return fmt.Sprintf("Obtuviste 100%% en todos los bloques de «%s».", program)
		},
	},
}

// earnedMerits decide qué méritos corresponden a un programa recién
// completado, mirando los bloques de la invitación ya con el último intento
// aplicado.
func earnedMerits(blocks []models.InductionInviteBlock) []string {
	if len(blocks) == 0 {
		return nil
	}
	firstTry, perfect := true, true
	for _, b := range blocks {
		if b.Status != models.InductionPassed {
			return nil
		}
		if b.Attempts != 1 {
			firstTry = false
		}
		if b.BestScore < 100 {
			perfect = false
		}
	}
	var out []string
	if firstTry {
		out = append(out, MeritFirstTry)
	}
	if perfect {
		out = append(out, MeritPerfect)
	}
	return out
}

func blockBadgeKey(blockID uint) string   { return fmt.Sprintf("block:%d", blockID) }
func programBadgeKey(programID uint) string { return fmt.Sprintf("program:%d", programID) }
func meritBadgeKey(merit string, programID uint) string {
	return fmt.Sprintf("merit:%s:%d", merit, programID)
}
