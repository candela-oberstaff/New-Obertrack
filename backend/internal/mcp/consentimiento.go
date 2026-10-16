package mcp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Anti-CSRF de la página de consentimiento.
//
// La página va con Referrer-Policy: no-referrer, y con eso Chrome manda
// Origin: null en el POST del formulario: validar Origin rechazaría a usuarios
// legítimos. En su lugar el formulario lleva un campo oculto firmado con HMAC,
// atado al usuario, al cliente, a la redirect_uri y al code_challenge, y con
// 10 minutos de vida. Un atacante no puede fabricarlo sin la clave, y uno
// robado no sirve para otro usuario ni para otro flujo.

// consentKey separa el dominio de la clave: del JWT_SECRET se deriva una clave
// que solo sirve para esto, como deriveStateSecret en el OAuth de Google.
func consentKey(secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("mcp-consent-csrf-v1"))
	return mac.Sum(nil)
}

func consentMAC(secret string, userID uint, clientID, redirectURI, challenge string, exp int64) string {
	mac := hmac.New(sha256.New, consentKey(secret))
	// El separador \x00 impide que dos combinaciones distintas den el mismo
	// mensaje ("a"+"bc" frente a "ab"+"c").
	fmt.Fprintf(mac, "%d\x00%s\x00%s\x00%s\x00%d", userID, clientID, redirectURI, challenge, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

// NewConsentToken firma el campo csrf del formulario.
func NewConsentToken(secret string, userID uint, clientID, redirectURI, challenge string, now time.Time) string {
	exp := now.Add(ConsentTTL).Unix()
	return strconv.FormatInt(exp, 10) + "." + consentMAC(secret, userID, clientID, redirectURI, challenge, exp)
}

// VerifyConsentToken valida el campo csrf. La firma se compara en tiempo
// constante y antes que la fecha, para no dar pistas por el tiempo de
// respuesta.
func VerifyConsentToken(secret, token string, userID uint, clientID, redirectURI, challenge string, now time.Time) bool {
	expStr, sig, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	want := consentMAC(secret, userID, clientID, redirectURI, challenge, exp)
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return false
	}
	return now.Unix() <= exp
}
