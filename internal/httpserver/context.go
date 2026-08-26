package httpserver

import (
	"context"

	"github.com/stan-ley-tech/medqueue/internal/auth"
)

type ctxKey int

const (
	ctxKeyClaims ctxKey = iota
	ctxKeyRequestID
)

func WithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, ctxKeyClaims, claims)
}

func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(ctxKeyClaims).(*auth.Claims)
	return claims, ok
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}
