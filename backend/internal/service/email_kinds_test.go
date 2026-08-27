package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Todo correo tiene que poder apagarse.
//
// El interruptor vive en Configuración → Correos y se consulta por CLAVE. Una clave que
// no esté en el catálogo no aparece en la pantalla, no se puede guardar —SetEnabled
// rechaza las desconocidas— y `Enabled` la da por ENCENDIDA, porque "sin fila" significa
// "todavía nadie lo apagó". Las tres cosas juntas hacen que un correo enviado con una
// clave inventada sea imposible de apagar, y así estuvo el de las automatizaciones:
// `SendEmailKind("workflow", ...)` escrito a mano.
//
// Esta prueba lee el código fuente en vez de las constantes porque el fallo era
// justamente una cadena escrita a mano: una prueba que enumere constantes no lo ve.

var llamadaConLiteral = regexp.MustCompile(`SendEmailKind[A-Za-z]*\(\s*"([^"]+)"`)

func TestCorreos_NingunEnvioUsaUnaClaveFueraDelCatalogo(t *testing.T) {
	conocidas := map[string]bool{}
	for _, tipo := range emailCatalog {
		conocidas[tipo.Key] = true
	}

	raiz := ".." // internal/
	err := filepath.Walk(raiz, func(ruta string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(ruta, ".go") {
			return err
		}
		if strings.HasSuffix(ruta, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(ruta)
		if rerr != nil {
			return rerr
		}
		for _, m := range llamadaConLiteral.FindAllStringSubmatch(string(src), -1) {
			clave := m[1]
			if !conocidas[clave] {
				t.Errorf(
					"%s envía con la clave %q, que no está en emailCatalog: ese correo no "+
						"aparece en Configuración → Correos y no hay forma de apagarlo. "+
						"Añádelo al catálogo y usa la constante.",
					ruta, clave,
				)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo el código: %v", err)
	}
}

// Y al revés: cada constante declarada tiene que estar en el catálogo. Una constante
// suelta se usa igual en un envío y produce el mismo agujero, sólo que sin la pista de
// la cadena escrita a mano.
func TestCorreos_ElCatalogoCubreTodasLasClavesDeclaradas(t *testing.T) {
	declaradas := []string{
		EmailKindInactivityAlert, EmailKindWorkHourReport,
		EmailKindSupportTicket, EmailKindPasswordReset, EmailKindAccountSetup,
		EmailKindAccessCredentials, EmailKindInductionInvite, EmailKindTestimonialRequest,
		EmailKindIncidentBroadcast, EmailKindSurveyInvite, EmailKindTicketReply,
		EmailKindManualComposer, EmailKindCampaign, EmailKindWorkflow,
	}
	enCatalogo := map[string]bool{}
	for _, tipo := range emailCatalog {
		enCatalogo[tipo.Key] = true
	}
	for _, k := range declaradas {
		if !enCatalogo[k] {
			t.Errorf("la clave %q está declarada pero no en el catálogo: nadie podría apagar ese correo", k)
		}
	}
}
