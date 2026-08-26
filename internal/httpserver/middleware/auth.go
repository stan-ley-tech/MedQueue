package middleware

import (
	"net/http"
	"strings"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/auth"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

// Authenticate requires a valid "Authorization: Bearer <token>" header,
// parses it, and attaches the resulting claims to the request context.
// Handlers that need to know who's calling read them via
// httpserver.ClaimsFromContext.
func Authenticate(issuer *auth.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				httpserver.WriteError(w, r, apperr.Unauthorized("missing or malformed Authorization header"))
				return
			}

			claims, err := issuer.ParseAccessToken(token)
			if err != nil {
				httpserver.WriteError(w, r, apperr.Unauthorized("invalid or expired access token"))
				return
			}

			ctx := httpserver.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole restricts a route to one of the given roles. It must run
// after Authenticate.
func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	allowed := make(map[domain.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := httpserver.ClaimsFromContext(r.Context())
			if !ok {
				httpserver.WriteError(w, r, apperr.Unauthorized(""))
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				httpserver.WriteError(w, r, apperr.Forbidden("your role does not have access to this operation"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
