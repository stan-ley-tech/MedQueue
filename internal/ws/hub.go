// Package ws implements the WebSocket fan-out for real-time queue
// updates. The hub subscribes once to Redis pub/sub (so it receives
// queue events regardless of which API replica produced them) and
// rebroadcasts each event to every client currently watching that
// department.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/stan-ley-tech/medqueue/internal/cache"
)

type Hub struct {
	cache *cache.QueueCache
	log   *slog.Logger

	mu      sync.RWMutex
	clients map[string]map[*Client]struct{} // departmentID -> client set
}

func NewHub(qc *cache.QueueCache, log *slog.Logger) *Hub {
	return &Hub{
		cache:   qc,
		log:     log,
		clients: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.departmentID] == nil {
		h.clients[c.departmentID] = make(map[*Client]struct{})
	}
	h.clients[c.departmentID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[c.departmentID], c)
	if len(h.clients[c.departmentID]) == 0 {
		delete(h.clients, c.departmentID)
	}
}

func (h *Hub) broadcast(departmentID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[departmentID] {
		c.send(message)
	}
}

// Run subscribes to the Redis queue-events channel and blocks, forwarding
// every message to the department's connected clients, until ctx is
// cancelled. It is meant to run for the lifetime of the process as a
// single goroutine started from main.
func (h *Hub) Run(ctx context.Context) {
	sub := h.cache.SubscribeAll(ctx)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event cache.QueueEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				h.log.Error("ws: failed to unmarshal queue event", "error", err)
				continue
			}
			h.broadcast(event.DepartmentID, []byte(msg.Payload))
		}
	}
}
