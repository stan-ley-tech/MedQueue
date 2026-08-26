package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

// RateLimit enforces a fixed-window request limit per client, keyed by
// authenticated user ID when available and falling back to remote IP for
// anonymous routes (login, health checks). A fixed window is a
// deliberate simplification over a sliding-window/token-bucket: it can
// allow up to 2x the limit across a window boundary, which is an
// acceptable trade-off for a clinic-scale internal API given how much
// simpler it is to reason about and debug; a sliding window would be the
// next step if this needed to defend a public internet-facing edge.
func RateLimit(rdb *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "ratelimit:" + clientKey(r)

			count, err := incrWithExpiry(r.Context(), rdb, key, window)
			if err != nil {
				// Redis being unavailable should degrade to "allow", not
				// take the whole API down.
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			remaining := int64(limit) - count
			if remaining < 0 {
				remaining = 0
			}
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			if count > int64(limit) {
				httpserver.WriteError(w, r, apperr.RateLimited(""))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func incrWithExpiry(ctx context.Context, rdb *redis.Client, key string, window time.Duration) (int64, error) {
	pipe := rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

func clientKey(r *http.Request) string {
	if claims, ok := httpserver.ClaimsFromContext(r.Context()); ok {
		return "user:" + claims.UserID
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}
