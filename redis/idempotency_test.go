package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) *goredis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	return goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
}

func TestIdempotency_ClaimThenGetResult(t *testing.T) {
	ctx := context.Background()
	idem := NewIdempotency(newTestClient(t), time.Minute)

	claimed, err := idem.Claim(ctx, "key-1")
	require.NoError(t, err)
	assert.True(t, claimed)

	value, found, err := idem.GetResult(ctx, "key-1")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, IsProcessing(value))

	require.NoError(t, idem.StoreResult(ctx, "key-1", `{"status":"approved"}`))

	value, found, err = idem.GetResult(ctx, "key-1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, `{"status":"approved"}`, value)
	assert.False(t, IsProcessing(value))
}

func TestIdempotency_SecondClaimFails(t *testing.T) {
	ctx := context.Background()
	idem := NewIdempotency(newTestClient(t), time.Minute)

	claimed1, err := idem.Claim(ctx, "key-2")
	require.NoError(t, err)
	assert.True(t, claimed1)

	claimed2, err := idem.Claim(ctx, "key-2")
	require.NoError(t, err)
	assert.False(t, claimed2, "a second claim on the same key must fail")
}

func TestIdempotency_GetResult_NotFound(t *testing.T) {
	ctx := context.Background()
	idem := NewIdempotency(newTestClient(t), time.Minute)

	_, found, err := idem.GetResult(ctx, "never-claimed")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIdempotency_DeleteAllowsReclaim(t *testing.T) {
	ctx := context.Background()
	idem := NewIdempotency(newTestClient(t), time.Minute)

	_, err := idem.Claim(ctx, "key-3")
	require.NoError(t, err)

	require.NoError(t, idem.Delete(ctx, "key-3"))

	claimed, err := idem.Claim(ctx, "key-3")
	require.NoError(t, err)
	assert.True(t, claimed, "after Delete, the key should be claimable again")
}
