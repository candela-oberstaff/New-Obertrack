package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// El usuario de sistema "Obertrack" es de tipo superadmin —lo necesita para
// publicar avisos automáticos— así que aparece en cualquier consulta de
// superadmins. Ofrecerlo como destino de un traspaso es ofrecer asignarle una
// conversación a un bot que no la va a atender.
func TestElBotDeSistemaNoEsUnDestinoDeTraspaso(t *testing.T) {
	bot := models.User{Email: models.SystemBotEmail, Name: models.SystemBotName, IsActive: true}
	if !isAssignableAgentExcluded(bot) {
		t.Error("el usuario de sistema debería quedar fuera del selector de traspaso")
	}
}

func TestLasPersonasSiSonDestinoDeTraspaso(t *testing.T) {
	cases := []models.User{
		{Email: "carlos.osvell@gmail.com", Name: "Osvell Chacon", IsActive: true},
		{Email: "osvell@gmail.com", Name: "Osvell CS", IsActive: true},
		{Email: "", Name: "Sin correo", IsActive: true},
	}
	for _, u := range cases {
		if isAssignableAgentExcluded(u) {
			t.Errorf("%q debería poder recibir un traspaso", u.Name)
		}
	}
}
