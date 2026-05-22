// Package middleware provee middlewares HTTP para Nodal.
// auth.go implementa RequireAuth (validación JWT Bearer) y RequireRole (RBAC).
package middleware

import (
	"context"
	"net/http"
	"strings"

	"nodal/internal/auth"
)

// contextKey es un tipo privado para las claves de contexto de Nodal.
// Usar un tipo propio (no string) evita colisiones con otros paquetes.
type contextKey string

const (
	// ClaimsContextKey es la clave bajo la que se inyectan los Claims JWT en el contexto.
	ClaimsContextKey contextKey = "nodal_auth_claims"
)

// ClaimsFromContext extrae los Claims JWT del contexto de la request.
// Devuelve nil si no hay claims (request no autenticada).
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(ClaimsContextKey).(*auth.Claims)
	return claims
}

// RequireAuth es un middleware que exige autenticación JWT válida.
// Acepta el token en dos formas (en orden de prioridad):
//  1. Header Authorization: Bearer <token>  — para clientes API
//  2. Cookie nodal_session                  — para el navegador (htmx)
//
// Si el token es inválido o falta, responde con 401.
// Si el token es válido, los Claims se inyectan en el contexto.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// 1. Intentar Bearer header (clientes API, curl, etc.)
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// 2. Fallback: cookie de sesión del navegador
		if tokenString == "" {
			if cookie, err := r.Cookie("nodal_session"); err == nil {
				tokenString = strings.TrimSpace(cookie.Value)
			}
		}

		if tokenString == "" {
			http.Error(w, `{"error":"autenticación requerida"}`, http.StatusUnauthorized)
			return
		}

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, `{"error":"token inválido o expirado"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole devuelve un middleware que verifica que el usuario autenticado tenga
// uno de los roles permitidos. Si el rol no está en la lista, responde con 403.
// Este middleware DEBE ir anidado dentro de RequireAuth.
//
// Uso:
//
//	handler := middleware.RequireAuth(middleware.RequireRole("owner", "moderator")(h))
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				// RequireAuth no se ejecutó antes — error de configuración del servidor
				http.Error(w, `{"error":"autenticación requerida"}`, http.StatusUnauthorized)
				return
			}

			if _, ok := allowed[claims.Role]; !ok {
				http.Error(w, `{"error":"permisos insuficientes"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
