package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// Los procesos de reclutamiento de una empresa, leídos de Obersuite y servidos
// a nuestra ficha. Obertrack hace de proxy: el token hacia Obersuite vive en
// el servidor y el navegador nunca lo ve. Solo lectura, en pantalla y aquí: no
// hay ninguna ruta de escritura hacia sus procesos.
type TenantSubscriptionsHandler struct {
	obersuite service.ObersuiteClient
}

func NewTenantSubscriptionsHandler(client service.ObersuiteClient) *TenantSubscriptionsHandler {
	return &TenantSubscriptionsHandler{obersuite: client}
}

// List es GET /admin/tenants/:id/subscriptions.
//
// Los fallos de Obersuite NO son 500 nuestros: se contestan como 200 con
// `available: false` y un motivo, para que la ficha se abra igual y el bloque
// diga por qué está vacío. Un espejo caído no puede tumbar la pantalla que lo
// enseña.
func (h *TenantSubscriptionsHandler) List(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Empresa inválida"})
		return
	}
	if !h.obersuite.Configured() {
		c.JSON(http.StatusOK, gin.H{"available": false, "reason": "not_configured"})
		return
	}
	raw, err := h.obersuite.CompanySubscriptions(uint(id))
	if err != nil {
		if service.IsObersuiteNotFound(err) {
			// Obersuite no conoce la empresa: lista vacía, no error. Ellos
			// mismos contestan 200 vacío cuando no hay procesos; el 404 es que
			// el id no les suena, y para la ficha es lo mismo.
			c.JSON(http.StatusOK, gin.H{"available": true, "company_id": id, "subscriptions": []any{}, "stages": []any{}, "counts": gin.H{"active": 0, "archived": 0}})
			return
		}
		log.Printf("[Obersuite] procesos de la empresa %d no disponibles: %v", id, err)
		c.JSON(http.StatusOK, gin.H{"available": false, "reason": "unreachable"})
		return
	}
	// El JSON de Obersuite va tal cual, con `available` delante. Se envuelve a
	// mano para no deserializar y volver a serializar una forma que es suya.
	c.Data(http.StatusOK, "application/json; charset=utf-8", append(append([]byte(`{"available":true,"data":`), raw...), '}'))
}

// Attachment es GET /admin/tenants/:id/subscriptions/:sid/attachments/:aid.
// Reenvía los bytes de Obersuite con su nombre y tipo. El id de empresa va en
// la ruta por coherencia con el resto de la ficha; la autorización es la del
// grupo (superadmin / Customer Success).
func (h *TenantSubscriptionsHandler) Attachment(c *gin.Context) {
	if !h.obersuite.Configured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "La conexión con Obersuite no está configurada"})
		return
	}
	// Preferimos la URL que Obersuite publica en el adjunto (?url=): sus ids de
	// ruta no son el external_id de la suscripción, y construirla nosotros era
	// adivinar. Si no viene, se cae al camino por partes.
	var (
		body        io.ReadCloser
		ctype       string
		disposition string
		err         error
	)
	if raw := strings.TrimSpace(c.Query("url")); raw != "" {
		body, ctype, disposition, err = h.obersuite.AttachmentByURL(raw)
	} else {
		body, ctype, disposition, err = h.obersuite.SubscriptionAttachment(c.Param("sid"), c.Param("aid"))
	}
	if err != nil {
		if service.IsObersuiteNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "El archivo ya no está en Obersuite"})
			return
		}
		// Una URL que no es de Obersuite es un dato malo de quien llama, no un
		// fallo del otro sistema: 400, no 502.
		if errors.Is(err, service.ErrNotAnObersuiteURL) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ese enlace no es un adjunto de Obersuite"})
			return
		}
		log.Printf("[Obersuite] adjunto %s/%s no disponible: %v", c.Param("sid"), c.Param("aid"), err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "No se pudo traer el archivo de Obersuite. Inténtalo en un momento."})
		return
	}
	defer body.Close()
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	if disposition != "" {
		c.Header("Content-Disposition", disposition)
	} else {
		c.Header("Content-Disposition", "attachment")
	}
	c.Header("Content-Type", ctype)
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, body); err != nil {
		log.Printf("[Obersuite] adjunto %s/%s cortado a mitad: %v", c.Param("sid"), c.Param("aid"), err)
	}
}
