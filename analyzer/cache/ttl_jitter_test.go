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
// applied: across several writes, at least one effective TTL must deviate from
// the base TTL. Against the fixed-TTL implementation all effective TTLs equal
// the base, so the spread assertion fails before jitter exists.
//
// PTTL reports the *remaining* TTL, which only decays, so each reading is
// compensated with the time elapsed since its save before the band comparison:
// pttl + elapsed approximates the TTL set at write time and is immune to
// ms-scale decay between write and read.
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
	savedAt := make([]time.Time, keys)
	for i := range data {
		data[i] = []byte(fmt.Sprintf("jitter-no-match-%02d@example.com", i))
		require.NoError(t, cache.SaveMatch(ctx, pattern.EntityEmail, data[i], false))
		savedAt[i] = time.Now()
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
	// bandSlackMs absorbs PTTL's integer-ms rounding and the client-side
	// measurement lag between the save and the PTTL read; the ±15% band is
	// 3000 ms wide, so a few ms of slack does not weaken the band assertion.
	const bandSlackMs = 5.0

	spread := false
	for i, d := range data {
		key := string(pattern.EntityEmail) + ":" + string(d)
		pttl, pttlErr := inspector.Do(ctx, inspector.B().Pttl().Key(key).Build()).ToInt64()
		require.NoError(t, pttlErr)

		elapsedMs := float64(time.Since(savedAt[i])) / float64(time.Millisecond)
		effectiveMs := float64(pttl) + elapsedMs

		assert.GreaterOrEqual(t, effectiveMs, lower-bandSlackMs, "key %q effective TTL below band", key)
		assert.LessOrEqual(t, effectiveMs, upper+bandSlackMs, "key %q effective TTL above band", key)
		if deviated := effectiveMs > baseMs+50 || effectiveMs < baseMs-50; deviated {
			spread = true
		}
	}

	assert.True(t, spread, "expected at least one jittered effective TTL to deviate from the base TTL")
}

// TestRuleMatchingCache_TTLJitter_ZeroPreservesBaseTTL verifies that without
// jitter (the zero value), the server-side effective TTL equals the base TTL
// within a small millisecond tolerance, exactly as before jitter existed. As
// with the in-band test, the remaining TTL read is compensated with the time
// elapsed since the save so ms-scale decay cannot push it below the base.
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
	savedAt := time.Now()

	inspector, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{addr},
		ForceSingleClient: true,
	})
	require.NoError(t, err)
	t.Cleanup(inspector.Close)

	key := string(pattern.EntityEmail) + ":" + string(data)
	pttl, pttlErr := inspector.Do(ctx, inspector.B().Pttl().Key(key).Build()).ToInt64()
	require.NoError(t, pttlErr)

	elapsedMs := float64(time.Since(savedAt)) / float64(time.Millisecond)

	const toleranceMs = 200
	assert.InDelta(t, float64(base/time.Millisecond), float64(pttl)+elapsedMs, toleranceMs, "effective TTL must equal the base TTL without jitter")
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
