package service

import (
	"encoding/json"
	"testing"
)

// La etiqueta es el único hilo que ata un evento del webhook a su campaña: las
// campañas salen por la API transaccional, así que Brevo no manda `campaign_id`
// y sin esta ida y vuelta las aperturas y clics se quedaban sin dueño (el panel
// mostraba 0.0% en todo). Si el formato de la etiqueta cambia de un lado y no
// del otro, la atribución vuelve a romperse en silencio.
func TestCampaignTag_IdaYVuelta(t *testing.T) {
	for _, id := range []uint{1, 15, 4096} {
		if got := CampaignIDFromTags([]string{CampaignTag(id)}); got != id {
			t.Errorf("ida y vuelta de la campaña %d devolvió %d", id, got)
		}
	}
}

func TestCampaignIDFromTags_IgnoraLoQueNoEsNuestro(t *testing.T) {
	cases := []struct {
		name string
		tags []string
		want uint
	}{
		{"sin etiquetas", nil, 0},
		{"etiqueta ajena puesta a mano en Brevo", []string{"newsletter", "verano"}, 0},
		{"prefijo parecido pero sin número", []string{"obertrack-campaign-"}, 0},
		{"prefijo con basura detrás", []string{"obertrack-campaign-abc"}, 0},
		{"convive con etiquetas ajenas", []string{"newsletter", "obertrack-campaign-7"}, 7},
		{"con espacios alrededor", []string{"  obertrack-campaign-7  "}, 7},
		// Un correo suelto del sistema (aviso, recordatorio) no lleva etiqueta:
		// su evento se guarda igual, pero sin campaña.
		{"correo transaccional del sistema", []string{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CampaignIDFromTags(tc.tags); got != tc.want {
				t.Errorf("CampaignIDFromTags(%v) = %d, want %d", tc.tags, got, tc.want)
			}
		})
	}
}

// El webhook de Brevo no manda las etiquetas de una sola forma: según el evento
// llegan como arreglo, como cadena con JSON dentro, o en el campo `tag` con una
// sola. Aceptar las tres es lo que impide perder eventos por la forma en que
// vinieron.
func TestParseBrevoTags_AceptaLasTresFormasDeBrevo(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		single string
		want   uint
	}{
		{"arreglo", `["obertrack-campaign-15"]`, "", 15},
		{"arreglo con varias", `["newsletter","obertrack-campaign-15"]`, "", 15},
		{"cadena con JSON dentro", `"[\"obertrack-campaign-15\"]"`, "", 15},
		{"cadena suelta", `"obertrack-campaign-15"`, "", 15},
		{"campo tag", ``, "obertrack-campaign-15", 15},
		{"nada", ``, "", 0},
		{"arreglo vacío", `[]`, "", 0},
		{"null", `null`, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage
			if tc.raw != "" {
				raw = json.RawMessage(tc.raw)
			}
			got := CampaignIDFromTags(ParseBrevoTags(raw, tc.single))
			if got != tc.want {
				t.Errorf("raw=%s tag=%q → %d, want %d", tc.raw, tc.single, got, tc.want)
			}
		})
	}
}
