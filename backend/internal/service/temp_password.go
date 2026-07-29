package service

import (
	"crypto/rand"
	"math/big"
)

// tempPasswordAlphabet excluye caracteres ambiguos (0/O, 1/l/I) para que la
// contraseña temporal sea fácil de dictar/transcribir.
const tempPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// GenerateTempPassword genera una contraseña temporal alfanumérica de n
// caracteres usando crypto/rand (sin caracteres ambiguos).
//
// Vive en el servicio porque la usan tanto el alta masiva por importación como
// el reenvío de accesos, y ambas deben producir el mismo tipo de clave.
func GenerateTempPassword(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(tempPasswordAlphabet)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = tempPasswordAlphabet[idx.Int64()]
	}
	return string(b), nil
}
