package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
)

// WriteJSON writes v as a JSON response with the given status code. It is
// the single place response encoding happens, so every endpoint produces
// consistent Content-Type headers and error handling on write failure.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("httpserver: failed to encode response", "error", err)
	}
}

// WriteError translates err into the application's consistent error
// envelope: {"code": ..., "message": ..., "fields": {...}}. Unrecognized
// errors are logged with full detail and returned to the client as an
// opaque 500 so internal details never leak.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		appErr = apperr.Internal(err)
	}

	if appErr.Status >= 500 {
		slog.Error("httpserver: request failed",
			"method", r.Method, "path", r.URL.Path,
			"code", appErr.Code, "error", appErr.Cause(),
		)
	}

	WriteJSON(w, appErr.Status, appErr)
}
