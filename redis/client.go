package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// NewClient connects to Redis at addr (e.g. "localhost:6379") and pings it
// once so callers fail fast on a bad address, the same way config.ConnectDB
// already fails fast for Postgres.
func NewClient(addr string) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis at %s: %w", addr, err)
	}

	return client, nil
}
