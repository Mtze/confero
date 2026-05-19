package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey int

const claimsKey contextKey = 0

// RequireToken verifies the session JWT from the cookie and populates
// the request context with SessionClaims. Returns 401 on failure.
func RequireToken(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				writeUnauthorized(w, "authentication required")
				return
			}
			claims, err := tm.Verify(cookie.Value)
			if err != nil {
				writeUnauthorized(w, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireMember returns 403 if the session does not include the member role.
func RequireMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || !hasMemberRole(claims) {
			writeForbidden(w, "member role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin returns 403 if the session does not include the admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || !hasAdminRole(claims) {
			writeForbidden(w, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClaimsFromContext extracts SessionClaims from the request context.
func ClaimsFromContext(ctx context.Context) (SessionClaims, bool) {
	c, ok := ctx.Value(claimsKey).(SessionClaims)
	return c, ok
}

func hasMemberRole(c SessionClaims) bool {
	for _, r := range c.Roles {
		if r == "member" {
			return true
		}
	}
	return false
}

func hasAdminRole(c SessionClaims) bool {
	for _, r := range c.Roles {
		if r == "admin" {
			return true
		}
	}
	return false
}

func writeUnauthorized(w http.ResponseWriter, detail string) {
	writeProblem(w, http.StatusUnauthorized, "Unauthorized", detail)
}

func writeForbidden(w http.ResponseWriter, detail string) {
	writeProblem(w, http.StatusForbidden, "Forbidden", detail)
}

func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"title":  title,
		"status": status,
		"detail": detail,
	})
}
