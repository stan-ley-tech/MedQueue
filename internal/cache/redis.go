// Package cache wraps the Redis client used for the queue cache, pub/sub
// fan-out to WebSocket clients, rate limiting, and read-through caching of
// reference data such as departments and doctors.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	*redis.Client
}

type Options struct {
	Addr     string
	Password string
	DB       int
}

func Connect(ctx context.Context, opts Options) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("cache: ping: %w", err)
	}

	return &Client{rdb}, nil
}
