package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Cliente HACIA Obersuite. Es la primera vez que Obertrack llama a Obersuite y
// no al revés, y por eso se escribe con dos reglas que el resto del bridge no
// necesitaba:
//
//   - Tiempo de espera corto (8 s). Esto se llama al abrir la ficha de una
//     empresa; si Obersuite tarda, la ficha no puede tardar con ella.
//   - Nunca tumba a quien llama. Sin configurar, caído, o contestando raro, el
//     cliente devuelve un error que el handler traduce a "no disponible" y la
//     ficha se abre igual con el bloque vacío. Un espejo que no responde no es
//     un error de Obertrack.
//
// Configuración: OBERSUITE_API_URL (p. ej. https://obersuite.oberstaff.com) y
// OBERSUITE_API_TOKEN (el que ellos nos entregan; va en X-Service-Token).

var ErrObersuiteNotConfigured = errors.New("la conexión con Obersuite no está configurada (OBERSUITE_API_URL / OBERSUITE_API_TOKEN)")

// ObersuiteClient es lo que el resto del código usa: una interfaz, para poder
// probar los handlers sin red.
type ObersuiteClient interface {
	// CompanySubscriptions devuelve el JSON de Obersuite tal cual (los procesos
	// de reclutamiento de una empresa). Se reexporta sin reinterpretar: la
	// forma es suya y la pantalla la pinta directamente.
	CompanySubscriptions(companyID uint) (json.RawMessage, error)
	// SubscriptionAttachment abre el archivo de un proceso. Devuelve el cuerpo
	// (que el llamador tiene que cerrar), el Content-Type y el
	// Content-Disposition de Obersuite.
	SubscriptionAttachment(subscriptionID, attachmentID string) (io.ReadCloser, string, string, error)
	// AttachmentByURL abre el archivo por la URL que Obersuite publica en cada
	// adjunto. Es el camino bueno: el id que lleva esa URL NO es el
	// external_id de la suscripción (external_id es
	// "obersuite-subscription-<uuid>" y la URL usa el "<uuid>" a secas), así
	// que construirla nosotros era adivinar su formato, y su servidor
	// contestaba 400. Se valida que la URL sea de SU dominio y de la ruta de
	// integración: es un proxy autenticado, no puede convertirse en uno abierto.
	AttachmentByURL(rawURL string) (io.ReadCloser, string, string, error)
	// Configured dice si hay URL y token: sin ellos la pantalla no ofrece el
	// bloque en vez de ofrecerlo vacío con un error.
	Configured() bool
}

type obersuiteClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewObersuiteClient() ObersuiteClient {
	return &obersuiteClient{
		baseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("OBERSUITE_API_URL")), "/"),
		token:   strings.TrimSpace(os.Getenv("OBERSUITE_API_TOKEN")),
		http:    &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *obersuiteClient) Configured() bool { return c.baseURL != "" && c.token != "" }

func (c *obersuiteClient) get(path string) (*http.Response, error) {
	if !c.Configured() {
		return nil, ErrObersuiteNotConfigured
	}
	// La barra final se quita AQUÍ y no solo al construir: una URL con barra
	// producía "//api/…", y algunos proxies tratan eso como otra ruta.
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(c.baseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Service-Token", c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("obersuite no responde: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, errors.New("obersuite rechaza el token (401): revisa OBERSUITE_API_TOKEN")
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, errObersuiteNotFound
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("obersuite contestó %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

var errObersuiteNotFound = errors.New("no encontrado en obersuite")

// ErrNotAnObersuiteURL es un dato malo de quien llama, no un fallo de
// Obersuite: el handler lo contesta como 400 y no como 502.
var ErrNotAnObersuiteURL = errors.New("esa URL no es un adjunto de Obersuite")

// IsObersuiteNotFound dice si el error es un 404 de Obersuite.
func IsObersuiteNotFound(err error) bool { return errors.Is(err, errObersuiteNotFound) }

func (c *obersuiteClient) CompanySubscriptions(companyID uint) (json.RawMessage, error) {
	resp, err := c.get(fmt.Sprintf("/api/integrations/obertrack/companies/%d/subscriptions", companyID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Tope de 4 MB: un historial largo cabe de sobra, y un cuerpo enorme por
	// error no se carga entero en memoria.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, errors.New("obersuite contestó algo que no es JSON")
	}
	return json.RawMessage(body), nil
}

func (c *obersuiteClient) SubscriptionAttachment(subscriptionID, attachmentID string) (io.ReadCloser, string, string, error) {
	// Respaldo para cuando el adjunto no trae `url`: el prefijo del external_id
	// se recorta porque su ruta usa el uuid a secas.
	subscriptionID = strings.TrimPrefix(subscriptionID, "obersuite-subscription-")
	resp, err := c.get("/api/integrations/obertrack/subscriptions/" + url.PathEscape(subscriptionID) + "/attachments/" + url.PathEscape(attachmentID))
	if err != nil {
		return nil, "", "", err
	}
	return resp.Body, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Disposition"), nil
}

// obersuiteIntegrationPath es lo único que AttachmentByURL deja pedir. Acotar a
// esta ruta —y no solo al host— evita que el proxy sirva cualquier cosa del
// servidor de Obersuite con nuestro token.
const obersuiteIntegrationPath = "/api/integrations/obertrack/"

func (c *obersuiteClient) AttachmentByURL(rawURL string) (io.ReadCloser, string, string, error) {
	if !c.Configured() {
		return nil, "", "", ErrObersuiteNotConfigured
	}
	rawURL = strings.TrimSpace(rawURL)
	base := strings.TrimRight(c.baseURL, "/")
	if !strings.HasPrefix(rawURL, base+obersuiteIntegrationPath) || strings.Contains(rawURL, "..") {
		return nil, "", "", ErrNotAnObersuiteURL
	}
	resp, err := c.get(strings.TrimPrefix(rawURL, base))
	if err != nil {
		return nil, "", "", err
	}
	return resp.Body, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Disposition"), nil
}

// ObersuiteNotFoundForTests expone el 404 a las pruebas de otros paquetes sin
// exportar el centinela: nadie fuera de aquí debería fabricarlo.
func ObersuiteNotFoundForTests() error { return errObersuiteNotFound }
