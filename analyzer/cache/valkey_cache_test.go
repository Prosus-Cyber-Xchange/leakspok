package cache_test

import (
	"context"
	"testing"
	"time"

	analyzercache "github.com/ifood/leakspok/analyzer/cache"
	"github.com/ifood/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// startValkeyContainer starts a Valkey container using the testcontainers redis module.
// Valkey is a Redis-compatible fork, so the redis module works transparently.
func startValkeyContainer(t *testing.T) (addr string) {
	t.Helper()

	ctx := context.Background()

	container, err := tcredis.Run(ctx, "docker.io/valkey/valkey:8")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(ctx))
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	return host + ":" + port.Port()
}

func TestValkeyRuleMatchingCache_BasicOperations(t *testing.T) {
	addr := startValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		Backend:  analyzercache.BackendValkey,
		CacheTTL: 10 * time.Second,
		Redis: analyzercache.RedisOptions{
			Enabled:            true,
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, cache)

	t.Run("cache miss returns not-found error", func(t *testing.T) {
		_, missErr := cache.GetMatch(ctx, pattern.EntityEmail, []byte("never-saved@example.com"))
		require.Error(t, missErr)
		assert.True(t, analyzercache.IsCacheNotFoundError(missErr))
	})

	t.Run("saves and retrieves true", func(t *testing.T) {
		data := []byte("match@example.com")
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, true))

		matched, getErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("saves and retrieves false", func(t *testing.T) {
		data := []byte("no-match-value")
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, false))

		matched, getErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
		require.NoError(t, getErr)
		assert.False(t, matched)
	})

	t.Run("different entities are independent keys", func(t *testing.T) {
		data := []byte("shared-token")
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, true))
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityPhone, data, false))

		matchedEmail, err := cache.GetMatch(ctx, pattern.EntityEmail, data)
		require.NoError(t, err)
		assert.True(t, matchedEmail)

		matchedPhone, err := cache.GetMatch(ctx, pattern.EntityPhone, data)
		require.NoError(t, err)
		assert.False(t, matchedPhone)
	})

}

func TestValkeyRuleMatchingCache_TTLExpiry(t *testing.T) {
	addr := startValkeyContainer(t)
	ctx := context.Background()

	ttl := 200 * time.Millisecond

	options := analyzercache.RuleMatchingCacheOptions{
		Backend:  analyzercache.BackendValkey,
		CacheTTL: ttl,
		Redis: analyzercache.RedisOptions{
			Enabled:            true,
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	data := []byte("ttl-test@example.com")
	require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, true))

	matched, getErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
	require.NoError(t, getErr)
	assert.True(t, matched)

	time.Sleep(ttl + 100*time.Millisecond)

	_, missErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
	require.Error(t, missErr)
	assert.True(t, analyzercache.IsCacheNotFoundError(missErr))
}

func TestValkeyRuleMatchingCache_NoTTL(t *testing.T) {
	addr := startValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		Backend:  analyzercache.BackendValkey,
		CacheTTL: 0, // no expiry
		Redis: analyzercache.RedisOptions{
			Enabled:            true,
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	data := []byte("persistent@example.com")
	require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, true))

	matched, getErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
	require.NoError(t, getErr)
	assert.True(t, matched)
}

// TestValkeyRuleMatchingCache_ClientSideCachingDisabled verifies that setting
// Memory.CacheSize == -1 disables client-side caching and the backend still
// works correctly (using plain Do instead of DoCache).
// With CSC disabled every read goes to the server, so overwrite semantics are
// immediately consistent.
func TestValkeyRuleMatchingCache_ClientSideCachingDisabled(t *testing.T) {
	addr := startValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		Backend:  analyzercache.BackendValkey,
		CacheTTL: 10 * time.Second,
		Memory: analyzercache.MemoryOptions{
			CacheSize: -1, // disable client-side caching
		},
		Redis: analyzercache.RedisOptions{
			Enabled:            true,
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	t.Run("saves and retrieves value", func(t *testing.T) {
		data := []byte("no-csc@example.com")
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, true))

		matched, getErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("overwrites previous value for same key", func(t *testing.T) {
		// Without CSC every read goes straight to the server, so overwrites are
		// immediately visible with no invalidation race.
		data := []byte("overwrite-me")
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityCPF, data, true))

		matched, err := cache.GetMatch(ctx, pattern.EntityCPF, data)
		require.NoError(t, err)
		assert.True(t, matched)

		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityCPF, data, false))

		matched, err = cache.GetMatch(ctx, pattern.EntityCPF, data)
		require.NoError(t, err)
		assert.False(t, matched)
	})
}

// TestValkeyRuleMatchingCache_AutoPipelining verifies that concurrent SaveMatch
// and GetMatch calls all complete correctly, exercising the auto-pipelining path
// where valkey-go coalesces concurrent Do calls into batched round-trips.
func TestValkeyRuleMatchingCache_AutoPipelining(t *testing.T) {
	addr := startValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		Backend:  analyzercache.BackendValkey,
		CacheTTL: 10 * time.Second,
		Redis: analyzercache.RedisOptions{
			Enabled:            true,
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	const workers = 50
	keys := make([][]byte, workers)
	for i := range keys {
		keys[i] = []byte("pipeline-token-" + string(rune('A'+i%26)) + string(rune('0'+i%10)))
	}

	// Write all entries concurrently.
	saveErrs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			saveErrs <- cache.SaveMatch(ctx, pattern.EntityEmail, keys[i], i%2 == 0)
		}(i)
	}
	for i := 0; i < workers; i++ {
		require.NoError(t, <-saveErrs)
	}

	// Read all entries concurrently and verify values.
	type result struct {
		idx     int
		matched bool
		err     error
	}
	results := make(chan result, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			matched, err := cache.GetMatch(ctx, pattern.EntityEmail, keys[i])
			results <- result{i, matched, err}
		}(i)
	}
	for range workers {
		r := <-results
		require.NoError(t, r.err)
		assert.Equal(t, r.idx%2 == 0, r.matched, "key index %d", r.idx)
	}
}

func TestValkeyRuleMatchingCache_PingOnConnect(t *testing.T) {
	ctx := context.Background()

	t.Run("ping succeeds on valid address", func(t *testing.T) {
		addr := startValkeyContainer(t)

		options := analyzercache.RuleMatchingCacheOptions{
			Backend:  analyzercache.BackendValkey,
			CacheTTL: 10 * time.Second,
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               addr,
				DisableClusterMode: true,
				PingOnConnect:      true,
			},
		}

		cache, err := analyzercache.NewCacheStore(ctx, options)
		require.NoError(t, err)
		require.NotNil(t, cache)
	})

	t.Run("ping fails on unreachable address", func(t *testing.T) {
		options := analyzercache.RuleMatchingCacheOptions{
			Backend:  analyzercache.BackendValkey,
			CacheTTL: 10 * time.Second,
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               "localhost:19379", // nothing listening here
				DisableClusterMode: true,
				PingOnConnect:      true,
			},
		}

		cache, err := analyzercache.NewCacheStore(ctx, options)
		require.Error(t, err)
		assert.Nil(t, cache)
	})
}
