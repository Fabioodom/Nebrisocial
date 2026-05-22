// Package crypto provee utilidades de seguridad para el proyecto Nodal.
// Implementa hashing de contraseñas con Argon2id (RFC 9106) tal como exige el PRD (Sección 9).
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parámetros Argon2id recomendados por OWASP (2024):
// - Memory: 64 MB, Iterations: 3, Parallelism: 2, KeyLength: 32 bytes
const (
	argon2Memory      uint32 = 64 * 1024 // 64 MB
	argon2Iterations  uint32 = 3
	argon2Parallelism uint8  = 2
	argon2KeyLength   uint32 = 32
	argon2SaltLength  int    = 16
)

var (
	ErrInvalidHash         = errors.New("crypto: el formato del hash es inválido")
	ErrIncompatibleVersion = errors.New("crypto: versión de argon2 incompatible")
)

// HashPassword genera un hash Argon2id para la contraseña en texto plano proporcionada.
// El hash incluye la sal aleatoria y los parámetros, y es seguro para almacenarse en BD.
// Formato de salida: $argon2id$v=<ver>$m=<mem>,t=<iter>,p=<par>$<salt_b64>$<hash_b64>
func HashPassword(plain string) (string, error) {
	// Generar sal criptográficamente aleatoria
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("crypto: fallo al generar sal: %w", err)
	}

	// Derivar la clave con Argon2id
	hash := argon2.IDKey(
		[]byte(plain),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)

	// Codificar en base64 sin padding (URL-safe no es necesario aquí)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Construir el string de hash en formato estándar PHC
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// CheckPassword compara una contraseña en texto plano con un hash Argon2id almacenado.
// Devuelve true si coinciden. Usa comparación en tiempo constante para prevenir timing attacks.
func CheckPassword(plain, hash string) bool {
	// Parsear el hash almacenado
	params, salt, storedHash, err := parseHash(hash)
	if err != nil {
		// No revelar detalles del error al llamador
		return false
	}

	// Derivar el hash de la contraseña candidata con los mismos parámetros
	candidateHash := argon2.IDKey(
		[]byte(plain),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLength,
	)

	// Comparación en tiempo constante (previene timing attacks)
	return subtle.ConstantTimeCompare(storedHash, candidateHash) == 1
}

// argon2Params agrupa los parámetros extraídos de un hash almacenado.
type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	keyLength   uint32
}

// parseHash decodifica un hash en formato PHC y devuelve sus componentes.
func parseHash(encoded string) (*argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// Formato esperado: ["", "argon2id", "v=19", "m=65536,t=3,p=2", "<salt>", "<hash>"]
	if len(parts) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	if parts[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	params := &argon2Params{}
	var parallelism int
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d",
		&params.memory, &params.iterations, &parallelism); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	params.parallelism = uint8(parallelism)

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	params.keyLength = uint32(len(hash))

	return params, salt, hash, nil
}
