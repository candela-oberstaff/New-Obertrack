package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// SecretSealer cifra secretos de terceros (hoy: los tokens de OAuth de Google)
// antes de guardarlos en la BD. Un refresh token en claro equivale a la llave
// del calendario del usuario: quien lea la tabla podría escribir en su agenda
// aunque nunca haya tocado Obertrack.
//
// AES-256-GCM: cifrado autenticado, así que un ciphertext manipulado falla al
// abrir en vez de devolver basura. El nonce (12B, aleatorio por operación) va
// prefijado al ciphertext, por lo que cifrar dos veces el mismo token produce
// valores distintos y no se puede correlacionar quién comparte credencial.
type SecretSealer struct {
	gcm cipher.AEAD
}

// ErrSealerNotConfigured lo devuelven Seal/Open sobre un sealer nil, que es lo
// que queda cuando la integración está apagada. Se trata como error normal para
// que un módulo mal configurado no tumbe el proceso en caliente.
var ErrSealerNotConfigured = errors.New("cifrado de secretos no configurado")

// NewSecretSealer construye el sealer a partir de una clave de 32 bytes en
// base64 (openssl rand -base64 32). Rechaza cualquier otro tamaño: AES-128 o
// una clave truncada pasarían silenciosamente y degradarían el cifrado.
func NewSecretSealer(base64Key string) (*SecretSealer, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("la clave de cifrado no es base64 válido: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("la clave de cifrado debe ser de 32 bytes (AES-256), se recibieron %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretSealer{gcm: gcm}, nil
}

// Seal cifra un secreto y lo devuelve como base64 de nonce||ciphertext.
func (s *SecretSealer) Seal(plaintext string) (string, error) {
	if s == nil {
		return "", ErrSealerNotConfigured
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open descifra lo que produjo Seal. Falla si el dato fue manipulado o si se
// cifró con otra clave (p.ej. tras rotar GOOGLE_TOKEN_ENC_KEY sin re-vincular).
func (s *SecretSealer) Open(ciphertext string) (string, error) {
	if s == nil {
		return "", ErrSealerNotConfigured
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("secreto almacenado ilegible: %w", err)
	}
	nonceSize := s.gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("secreto almacenado truncado")
	}
	nonce, body := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := s.gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("no se pudo descifrar el secreto (¿cambió la clave?): %w", err)
	}
	return string(plaintext), nil
}
