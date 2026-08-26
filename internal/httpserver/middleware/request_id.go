package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

const RequestIDHeader = "X-Request-ID"

// RequestID assigns a request ID (reusing an inbound one if the caller,
// e.g. an upstream gateway, already set one) and stashes it in context and
// the response header so it can be correlated across logs.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := httpserver.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(buf)
}
