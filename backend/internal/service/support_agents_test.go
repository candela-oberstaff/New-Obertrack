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
	// Se reconoce por la bandera is_system, que es la fuente de verdad...
	porBandera := models.User{IsSystem: true, Name: models.SystemBotName, IsActive: true}
	if !isAssignableAgentExcluded(porBandera) {
		t.Error("con is_system marcado debería quedar fuera del selector de traspaso")
	}
	// ...y también por el correo, para el caso de un usuario cargado sin esa
	// columna o construido a mano: un falso negativo lo devolvería a la lista.
	porCorreo := models.User{Email: models.SystemBotEmail, Name: models.SystemBotName, IsActive: true}
	if !isAssignableAgentExcluded(porCorreo) {
		t.Error("por su correo también debería quedar fuera del selector de traspaso")
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
