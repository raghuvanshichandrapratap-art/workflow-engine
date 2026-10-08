package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const processingPlaceholder = "processing"

// Idempotency lets a handler recognize a retried request (same client-supplied
// key) and either replay the cached result or avoid starting a duplicate.
type Idempotency struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewIdempotency(client *goredis.Client, ttl time.Duration) *Idempotency {
	return &Idempotency{client: client, ttl: ttl}
}

func idempotencyKey(key string) string {
	return "idempotency:" + key
}

// Claim atomically sets key -> "processing" only if absent (SET NX EX).
// claimed=false means another request already owns this key.
func (i *Idempotency) Claim(ctx context.Context, key string) (bool, error) {
	ok, err := i.client.SetNX(ctx, idempotencyKey(key), processingPlaceholder, i.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// GetResult returns the stored value for key, if any. A value equal to the
// "processing" placeholder means a request with this key is still in flight.
func (i *Idempotency) GetResult(ctx context.Context, key string) (value string, found bool, err error) {
	value, err = i.client.Get(ctx, idempotencyKey(key)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// StoreResult overwrites the placeholder with the final value, refreshing the TTL.
func (i *Idempotency) StoreResult(ctx context.Context, key string, value string) error {
	return i.client.Set(ctx, idempotencyKey(key), value, i.ttl).Err()
}

// Delete releases a claim — used after a failed attempt so a legitimate
// retry isn't permanently blocked by a key that never got a real result.
func (i *Idempotency) Delete(ctx context.Context, key string) error {
	return i.client.Del(ctx, idempotencyKey(key)).Err()
}

// IsProcessing reports whether value is the in-flight placeholder rather
// than a real stored result.
func IsProcessing(value string) bool {
	return value == processingPlaceholder
}
