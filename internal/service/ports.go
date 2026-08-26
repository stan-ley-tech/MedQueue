package service

import (
	"context"

	"github.com/stan-ley-tech/medqueue/internal/domain"
)

// QueueCache abstracts the Redis-backed cache-aside/pub-sub layer that
// QueueService uses for fast snapshot reads and real-time event fan-out.
// Defining it here (rather than depending on *cache.QueueCache directly)
// keeps the service package testable with an in-memory fake and free of
// any import on the Redis client.
type QueueCache interface {
	GetSnapshot(ctx context.Context, departmentID string) (*domain.QueueSnapshot, bool, error)
	SetSnapshot(ctx context.Context, snapshot domain.QueueSnapshot) error
	InvalidateSnapshot(ctx context.Context, departmentID string) error
	Publish(ctx context.Context, event domain.QueueEvent) error
}
