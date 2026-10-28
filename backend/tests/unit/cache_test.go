package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app/internal/cache"
)

func TestMemoryCache_Expiry(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()

	require.NoError(t, c.Set(ctx, "short", 1, 20*time.Millisecond))
	require.NoError(t, c.Set(ctx, "forever", 2, 0))

	var got int
	require.NoError(t, c.Get(ctx, "short", &got))
	assert.Equal(t, 1, got)

	time.Sleep(30 * time.Millisecond)

	assert.ErrorIs(t, c.Get(ctx, "short", &got), cache.ErrKeyNotFound, "gone after its TTL")
	require.NoError(t, c.Get(ctx, "forever", &got), "a TTL of zero never expires")
	assert.Equal(t, 2, got)
}

func TestMemoryCache_RememberCachesUntilForgotten(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()
	calls := 0
	load := func() (interface{}, error) { calls++; return calls, nil }

	var got int
	require.NoError(t, c.Remember(ctx, "k", time.Minute, load, &got))
	require.NoError(t, c.Remember(ctx, "k", time.Minute, load, &got))
	assert.Equal(t, 1, got)

	require.NoError(t, c.Forget(ctx, "k"))
	require.NoError(t, c.Remember(ctx, "k", time.Minute, load, &got))
	assert.Equal(t, 2, got)
}
