package utils

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("no se pudo generar la clave: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestSecretSealerRoundTrip(t *testing.T) {
	sealer, err := NewSecretSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSecretSealer: %v", err)
	}

	secret := "1//0gRefreshTokenDeGoogle-con_simbolos.raros"
	sealed, err := sealer.Seal(secret)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if strings.Contains(sealed, secret) {
		t.Fatal("el ciphertext contiene el secreto en claro")
	}

	opened, err := sealer.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opened != secret {
		t.Fatalf("round-trip alterado: %q != %q", opened, secret)
	}
}

// Cifrar dos veces el mismo token debe dar salidas distintas (nonce aleatorio),
// para que no se pueda deducir de la BD qué usuarios comparten credencial.
func TestSecretSealerNonceIsRandom(t *testing.T) {
	sealer, err := NewSecretSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSecretSealer: %v", err)
	}

	a, err := sealer.Seal("mismo-token")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	b, err := sealer.Seal("mismo-token")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if a == b {
		t.Fatal("dos cifrados del mismo texto son idénticos: el nonce no es aleatorio")
	}
}

// GCM es cifrado autenticado: un ciphertext manipulado debe fallar al abrir en
// vez de devolver datos corruptos que luego mandaríamos a Google.
func TestSecretSealerRejectsTampering(t *testing.T) {
	sealer, err := NewSecretSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSecretSealer: %v", err)
	}

	sealed, err := sealer.Seal("token-original")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := sealer.Open(tampered); err == nil {
		t.Fatal("Open aceptó un ciphertext manipulado")
	}
}

// Un secreto cifrado con otra clave no debe abrirse: es el escenario de rotar
// GOOGLE_TOKEN_ENC_KEY, donde queremos un error claro y no un token corrupto.
func TestSecretSealerWrongKeyFails(t *testing.T) {
	a, err := NewSecretSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSecretSealer: %v", err)
	}
	b, err := NewSecretSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSecretSealer: %v", err)
	}

	sealed, err := a.Seal("token")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := b.Open(sealed); err == nil {
		t.Fatal("Open aceptó un secreto cifrado con otra clave")
	}
}

func TestNewSecretSealerRejectsBadKeys(t *testing.T) {
	cases := map[string]string{
		"no base64":     "no-es-base64-!!",
		"clave corta":   base64.StdEncoding.EncodeToString([]byte("solo-16-bytes-xx")),
		"clave vacía":   "",
		"clave de 31 B": base64.StdEncoding.EncodeToString(make([]byte, 31)),
	}
	for name, key := range cases {
		if _, err := NewSecretSealer(key); err == nil {
			t.Errorf("%s: NewSecretSealer aceptó una clave inválida", name)
		}
	}
}

// Un sealer nil es lo que queda cuando la integración está apagada: debe dar
// error normal, no pánico.
func TestNilSealerReturnsError(t *testing.T) {
	var sealer *SecretSealer
	if _, err := sealer.Seal("x"); err != ErrSealerNotConfigured {
		t.Errorf("Seal sobre sealer nil: se esperaba ErrSealerNotConfigured, se obtuvo %v", err)
	}
	if _, err := sealer.Open("x"); err != ErrSealerNotConfigured {
		t.Errorf("Open sobre sealer nil: se esperaba ErrSealerNotConfigured, se obtuvo %v", err)
	}
}
