package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

const IdempotencyKeyHeader = "Idempotency-Key"
const idempotencyKeyTTL = 24 * time.Hour

// Idempotency makes a write endpoint safe to retry: a client that resends
// the same request (network timeout, double-tap on a slow connection)
// with the same Idempotency-Key gets back the original response instead
// of performing the operation twice — critical for "book appointment" and
// "check in", where a duplicate would double-book a slot or double-enqueue
// a patient. It is mounted only on the specific routes that need it, not
// globally, since GETs and idempotent-by-nature operations don't.
func Idempotency(repo repository.IdempotencyRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(IdempotencyKeyHeader)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				httpserver.WriteError(w, r, apperr.Validation("failed to read request body", nil))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			sum := sha256.Sum256(body)
			requestHash := hex.EncodeToString(sum[:])

			found, statusCode, responseBody, err := repo.Reserve(r.Context(), key, r.URL.Path, requestHash, idempotencyKeyTTL)
			if err != nil {
				httpserver.WriteError(w, r, err)
				return
			}
			if found {
				if statusCode == 0 {
					httpserver.WriteError(w, r, apperr.IdempotencyInProgress())
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Idempotency-Replayed", "true")
				w.WriteHeader(statusCode)
				_, _ = w.Write(responseBody)
				return
			}

			rec := &responseCapture{ResponseWriter: w, status: http.StatusOK, body: &bytes.Buffer{}}
			next.ServeHTTP(rec, r)

			if err := repo.Complete(r.Context(), key, rec.status, rec.body.Bytes()); err != nil {
				// The response was already sent to the client; a failure
				// to persist it just means a retried request will redo
				// the work instead of replaying, which is safe, if
				// wasteful.
				return
			}
		})
	}
}

type responseCapture struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *responseCapture) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseCapture) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
