package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

// QueueCache is a thin cache-aside layer over the department queue plus
// the pub/sub channel used to fan out queue events to WebSocket clients
// across API replicas. PostgreSQL (via SELECT ... FOR UPDATE SKIP LOCKED
// in QueueRepository.CallNext) remains the single source of truth for
// queue ordering and concurrency; Redis never decides who gets called
// next; it only caches the result of that decision and broadcasts it.
type QueueCache struct {
	client *Client
	ttl    time.Duration
}

func NewQueueCache(client *Client) *QueueCache {
	return &QueueCache{client: client, ttl: 30 * time.Second}
}

const eventsChannelPattern = "queue:%s:events"

func snapshotKey(departmentID string) string {
	return fmt.Sprintf("queue:%s:snapshot", departmentID)
}

func (c *QueueCache) GetSnapshot(ctx context.Context, departmentID string) (*domain.QueueSnapshot, bool, error) {
	raw, err := c.client.Get(ctx, snapshotKey(departmentID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("queue_cache: get: %w", err)
	}

	var snapshot domain.QueueSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, false, fmt.Errorf("queue_cache: unmarshal: %w", err)
	}
	return &snapshot, true, nil
}

func (c *QueueCache) SetSnapshot(ctx context.Context, snapshot domain.QueueSnapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("queue_cache: marshal: %w", err)
	}
	if err := c.client.Set(ctx, snapshotKey(snapshot.DepartmentID), raw, c.ttl).Err(); err != nil {
		return fmt.Errorf("queue_cache: set: %w", err)
	}
	return nil
}

func (c *QueueCache) InvalidateSnapshot(ctx context.Context, departmentID string) error {
	if err := c.client.Del(ctx, snapshotKey(departmentID)).Err(); err != nil {
		return fmt.Errorf("queue_cache: invalidate: %w", err)
	}
	return nil
}

func (c *QueueCache) Publish(ctx context.Context, event domain.QueueEvent) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("queue_cache: marshal event: %w", err)
	}
	channel := fmt.Sprintf(eventsChannelPattern, event.DepartmentID)
	if err := c.client.Publish(ctx, channel, raw).Err(); err != nil {
		return fmt.Errorf("queue_cache: publish: %w", err)
	}
	return nil
}

// SubscribeAll subscribes to queue events across every department using a
// pattern subscription, which is how the WebSocket hub learns about
// changes regardless of which API replica handled the write.
func (c *QueueCache) SubscribeAll(ctx context.Context) *redis.PubSub {
	return c.client.PSubscribe(ctx, "queue:*:events")
}
