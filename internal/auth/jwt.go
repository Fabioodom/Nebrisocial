// Package auth implementa la lógica de autenticación de Nodal:
// generación y validación de tokens JWT con la librería golang-jwt/jwt/v5.
package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Duraciones de validez de los tokens
const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// Errores públicos de validación de JWT
var (
	ErrTokenExpired   = errors.New("jwt: token expirado")
	ErrTokenInvalid   = errors.New("jwt: token inválido")
	ErrMissingSecret  = errors.New("jwt: JWT_SECRET no configurado")
)

// Claims define los claims personalizados incluidos en cada token de Nodal.
// Implementa la interfaz jwt.Claims mediante embedding de jwt.RegisteredClaims.
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// jwtSecret devuelve la clave secreta desde la variable de entorno JWT_SECRET.
// Devuelve error si no está configurada (fallo temprano de configuración).
func jwtSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, ErrMissingSecret
	}
	return []byte(secret), nil
}

// TokenPair agrupa el access token y el refresh token generados juntos.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// GenerateTokenPair crea un par de tokens JWT firmados con HS256:
//   - AccessToken: expira en 15 minutos, contiene user_id y role.
//   - RefreshToken: expira en 7 días, contiene user_id y role.
//
// La clave secreta se lee de la variable de entorno JWT_SECRET.
// NUNCA se loguea el token completo ni el secreto.
func GenerateTokenPair(userID, role string) (*TokenPair, error) {
	secret, err := jwtSecret()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// ── Access Token ──────────────────────────────────────────────────────────
	accessClaims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "nodal",
			Subject:   userID,
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(secret)
	if err != nil {
		return nil, fmt.Errorf("jwt: error al firmar access token: %w", err)
	}

	// ── Refresh Token ─────────────────────────────────────────────────────────
	refreshClaims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "nodal",
			Subject:   userID,
		},
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(secret)
	if err != nil {
		return nil, fmt.Errorf("jwt: error al firmar refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateToken parsea y valida la firma y la expiración de un token JWT.
// Devuelve los Claims extraídos o un error tipado (ErrTokenExpired, ErrTokenInvalid).
func ValidateToken(tokenString string) (*Claims, error) {
	secret, err := jwtSecret()
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(t *jwt.Token) (interface{}, error) {
			// Verificar que el algoritmo de firma sea exactamente HS256
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("jwt: algoritmo inesperado: %v", t.Header["alg"])
			}
			return secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuedAt(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}
