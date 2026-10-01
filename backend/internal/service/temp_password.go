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
//
// Siempre lleva al menos una letra y un dígito: es lo que exige
// ValidatePasswordStrength, y sin forzarlo ~1 de cada 6 claves de 12
// caracteres salía sin ningún número.
func GenerateTempPassword(n int) (string, error) {
	if n < 2 {
		n = 2
	}
	b := make([]byte, n)
	for i := range b {
		c, err := randomFrom(tempPasswordAlphabet)
		if err != nil {
			return "", err
		}
		b[i] = c
	}

	hasLetter, hasDigit := false, false
	for _, c := range b {
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else {
			hasLetter = true
		}
	}
	// Las posiciones que se reemplazan también son aleatorias, para no dejar
	// un patrón fijo (p. ej. "siempre termina en número").
	pos, err := randomPerm(n)
	if err != nil {
		return "", err
	}
	if !hasDigit {
		c, err := randomFrom(tempPasswordDigits)
		if err != nil {
			return "", err
		}
		b[pos[0]] = c
	}
	if !hasLetter {
		c, err := randomFrom(tempPasswordLetters)
		if err != nil {
			return "", err
		}
		b[pos[1]] = c
	}
	return string(b), nil
}

const (
	tempPasswordDigits  = "23456789"
	tempPasswordLetters = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

func randomFrom(alphabet string) (byte, error) {
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
	if err != nil {
		return 0, err
	}
	return alphabet[idx.Int64()], nil
}

// randomPerm es una permutación aleatoria de 0..n-1 (Fisher-Yates).
func randomPerm(n int) ([]int, error) {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return nil, err
		}
		p[i], p[j.Int64()] = p[j.Int64()], p[i]
	}
	return p, nil
}
