package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

// Recover turns a panic in any downstream handler into a 500 response
// instead of taking down the whole server, and logs the stack trace so
// the underlying bug is still visible.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						"request_id", httpserver.RequestIDFromContext(r.Context()),
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					httpserver.WriteError(w, r, apperr.Internal(fmt.Errorf("panic: %v", rec)))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
