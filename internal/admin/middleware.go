package admin

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const claimsKey contextKey = "claims"

func claimsFromCtx(r *http.Request) *Claims {
	if c, ok := r.Context().Value(claimsKey).(*Claims); ok {
		return c
	}
	return nil
}

// requireAuth validates the Bearer token from the Authorization header
// and injects the claims into the request context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
			return
		}
		claims, err := s.parseToken(strings.TrimPrefix(auth, "Bearer "))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole returns a middleware that requires the caller to have one of
// the specified roles (checked after requireAuth).
func requireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromCtx(r)
			if claims == nil || !allowed[claims.Role] {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
