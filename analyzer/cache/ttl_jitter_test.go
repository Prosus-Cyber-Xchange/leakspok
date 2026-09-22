package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	cachetesting "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache/testing"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
)

// TestRuleMatchingCache_TTLJitter_ServerWriteInBand verifies that with a jitter
// percentage configured, each negative-result save issues SET PX with an
// effective TTL inside [base*(1-P), base*(1+P)] and that jitter is actually
// applied: across several writes, at least one PTTL must deviate from the base
// TTL. Against the fixed-TTL implementation all PTTLs equal the base, so the
// spread assertion fails before jitter exists.
func TestRuleMatchingCache_TTLJitter_ServerWriteInBand(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()

	const base = 10 * time.Second
	const jitter = 0.15
	const keys = 20

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL:            base,
		TTLJitterPercentage: jitter,
		Redis: analyzercache.RedisOptions{
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	data := make([][]byte, keys)
	for i := range data {
		data[i] = []byte(fmt.Sprintf("jitter-no-match-%02d@example.com", i))
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data[i], false))
	}

	inspector, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{addr},
		ForceSingleClient: true,
	})
	require.NoError(t, err)
	t.Cleanup(inspector.Close)

	baseMs := float64(base / time.Millisecond)
	lower := baseMs * (1 - jitter)
	upper := baseMs * (1 + jitter)

	spread := false
	for _, d := range data {
		key := string(pattern.EntityEmail) + ":" + string(d)
		pttl, pttlErr := inspector.Do(ctx, inspector.B().Pttl().Key(key).Build()).ToInt64()
		require.NoError(t, pttlErr)

		assert.GreaterOrEqual(t, float64(pttl), lower, "key %q PTTL below band", key)
		assert.LessOrEqual(t, float64(pttl), upper, "key %q PTTL above band", key)
		if deviated := pttl > int64(baseMs)+50 || pttl < int64(baseMs)-50; deviated {
			spread = true
		}
	}

	assert.True(t, spread, "expected at least one jittered PTTL to deviate from the base TTL")
}

// TestRuleMatchingCache_TTLJitter_ZeroPreservesBaseTTL verifies that without
// jitter (the zero value), the server-side PTTL equals the base TTL within a
// small millisecond tolerance, exactly as before jitter existed.
func TestRuleMatchingCache_TTLJitter_ZeroPreservesBaseTTL(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()

	const base = 10 * time.Second

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL: base,
		Redis: analyzercache.RedisOptions{
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	data := []byte("zero-jitter@example.com")
	require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, false))

	inspector, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{addr},
		ForceSingleClient: true,
	})
	require.NoError(t, err)
	t.Cleanup(inspector.Close)

	key := string(pattern.EntityEmail) + ":" + string(data)
	pttl, pttlErr := inspector.Do(ctx, inspector.B().Pttl().Key(key).Build()).ToInt64()
	require.NoError(t, pttlErr)

	const toleranceMs = 200
	assert.InDelta(t, float64(base/time.Millisecond), float64(pttl), toleranceMs, "PTTL must equal the base TTL without jitter")
}

// TestRuleMatchingCache_TTLJitter_NoExpiryUnaffected verifies that with
// CacheTTL == 0 the no-expiry path is preserved even when a jitter percentage
// is configured: the write carries no PX (PTTL -1) and reads still round-trip.
func TestRuleMatchingCache_TTLJitter_NoExpiryUnaffected(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL:            0,
		TTLJitterPercentage: 0.5,
		Redis: analyzercache.RedisOptions{
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	data := []byte("persistent-jitter@example.com")
	require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, false))

	matched, getErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
	require.NoError(t, getErr)
	assert.False(t, matched)

	inspector, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{addr},
		ForceSingleClient: true,
	})
	require.NoError(t, err)
	t.Cleanup(inspector.Close)

	key := string(pattern.EntityEmail) + ":" + string(data)
	pttl, pttlErr := inspector.Do(ctx, inspector.B().Pttl().Key(key).Build()).ToInt64()
	require.NoError(t, pttlErr)
	assert.Equal(t, int64(-1), pttl, "no-expiry keys must carry no PX regardless of jitter")
}

// TestRuleMatchingCache_TTLJitter_ClientSideReadPath verifies that with
// client-side caching enabled (DisableInMemoryCache false) and jitter on, the
// read path still works: a saved false match is read back correctly through
// DoCache with a jittered local TTL.
func TestRuleMatchingCache_TTLJitter_ClientSideReadPath(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL:            10 * time.Second,
		TTLJitterPercentage: 0.15,
		Redis: analyzercache.RedisOptions{
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	data := []byte("client-side-jitter@example.com")
	require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data, false))

	matched, getErr := cache.GetMatch(ctx, pattern.EntityEmail, data)
	require.NoError(t, getErr)
	assert.False(t, matched)
}
