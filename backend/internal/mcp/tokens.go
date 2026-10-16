// Package mcp es el servidor MCP (Model Context Protocol) de solo lectura de
// Obertrack: cada empleado conecta su asistente de IA (ChatGPT, Claude, Grok,
// Gemini CLI) y ve exactamente lo que ve en la web con su rol.
//
// Réplica de la integración de Obersuite (src/lib/mcp): Obertrack es su propio
// Authorization Server OAuth 2.1 (PKCE S256, registro dinámico, refresh con
// rotación) y los tools consultan la base en transacciones READ ONLY.
// Diseño completo en docs/features/mcp-asistentes-ia.md.
package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
)

// Prefijos de los tokens. Permiten reconocer de un vistazo qué es cada cadena
// (y a los escáneres de secretos, detectarlas si alguien las pega en un repo).
const (
	PrefixAccess   = "obs_at_"
	PrefixRefresh  = "obs_rt_"
	PrefixPersonal = "obs_pt_"
	prefixClient   = "obc_"
	prefixSecret   = "obs_cs_"
)

// Vidas útiles.
const (
	AccessTTL     = time.Hour
	RefreshTTL    = 30 * 24 * time.Hour
	ConnectionTTL = 90 * 24 * time.Hour
	PersonalTTL   = 90 * 24 * time.Hour
	CodeTTL       = 5 * time.Minute
	ConsentTTL    = 10 * time.Minute
)

// randomString devuelve n bytes aleatorios en base64url sin relleno: 32 bytes
// dan 43 caracteres.
func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand no falla en la práctica; si lo hace, no se emite nada.
		panic("mcp: crypto/rand: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// NewToken genera un token con prefijo + 43 caracteres aleatorios.
func NewToken(prefix string) string { return prefix + randomString(32) }

// HashToken es lo único que se guarda de cualquier token o código.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// tokenKind deduce el tipo por el prefijo; "" si no es un token del MCP.
func tokenKind(token string) string {
	switch {
	case strings.HasPrefix(token, PrefixAccess):
		return "access"
	case strings.HasPrefix(token, PrefixRefresh):
		return "refresh"
	case strings.HasPrefix(token, PrefixPersonal):
		return "personal"
	}
	return ""
}

// VerifyPKCE comprueba code_verifier contra code_challenge con S256 (RFC 7636).
// Es el único método admitido: "plain" no protege nada.
func VerifyPKCE(verifier, challenge string) bool {
	// RFC 7636 §4.1: 43 a 128 caracteres.
	if len(verifier) < 43 || len(verifier) > 128 || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}
