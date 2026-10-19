package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	cachemock "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache/mocks"
	cachetesting "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache/testing"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/mock/gomock"
)

// matchHashKey is the hash key the token-scoped adapter contract specifies: the
// token content under the leakspok:m: namespace. The test knows the encoding to
// observe server-side state; the adapter remains its only producer.
func matchHashKey(token []byte) string {
	return "leakspok:m:" + string(token)
}

// startInspector opens an independent valkey client for observing server-side
// key state, mirroring the pattern used by the jitter tests.
func startInspector(t *testing.T, addr string) valkey.Client {
	t.Helper()

	inspector, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{addr},
		ForceSingleClient: true,
	})
	require.NoError(t, err)
	t.Cleanup(inspector.Close)

	return inspector
}

// newTokenCache builds a cache store against addr with the given base TTL,
// jitter fraction, and client-side caching switch.
func newTokenCache(ctx context.Context, t *testing.T, addr string, ttl time.Duration, jitter float64, disableCSC bool) analyzercache.CacheStore {
	t.Helper()

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL:             ttl,
		TTLJitterPercentage:  jitter,
		DisableInMemoryCache: disableCSC,
		Redis: analyzercache.RedisOptions{
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	return cache
}

// remainingTTL reads a token hash key's PTTL in milliseconds.
func remainingTTL(t *testing.T, inspector valkey.Client, token []byte) int64 {
	t.Helper()

	pttl, err := inspector.Do(context.Background(), inspector.B().Pttl().Key(matchHashKey(token)).Build()).ToInt64()
	require.NoError(t, err)

	return pttl
}

// TestTokenHash_HashStorage verifies the "Cache Storage Layout" scenarios
// (first evaluation, repeat evaluation, partially evaluated token) and the
// "Batched Negative-Result Writes" no-match scenario at the adapter level:
// SaveMatches persists one hash key leakspok:m:<token> with one boolean-coded
// field per entity, GetMatches returns exactly the stored fields, and an
// unknown token yields an empty map and a nil error.
func TestTokenHash_HashStorage(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()
	inspector := startInspector(t, addr)
	cache := newTokenCache(ctx, t, addr, 10*time.Second, 0, false)

	token := []byte("hash-storage@example.com")

	t.Run("no-match token persists one hash with one field per entity", func(t *testing.T) {
		require.NoError(t, cache.SaveMatches(ctx, token, map[pattern.Entity]bool{ //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
			pattern.EntityEmail: false,
			pattern.EntityCPF:   false,
		}))

		key := matchHashKey(token)
		exists, err := inspector.Do(ctx, inspector.B().Exists().Key(key).Build()).ToInt64()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists, "exactly the token hash key must exist")

		stored, err := inspector.Do(ctx, inspector.B().Hgetall().Key(key).Build()).AsMap()
		require.NoError(t, err)
		require.Len(t, stored, 2, "one field per evaluated entity")
		for _, entity := range []pattern.Entity{pattern.EntityEmail, pattern.EntityCPF} {
			field := stored[string(entity)]
			value, toStringErr := field.ToString()
			require.NoError(t, toStringErr)
			assert.Equal(t, "0", value, "negative results are boolean-coded as \"0\"")
		}

		legacyKey := string(pattern.EntityEmail) + ":" + string(token)
		legacyExists, err := inspector.Do(ctx, inspector.B().Exists().Key(legacyKey).Build()).ToInt64()
		require.NoError(t, err)
		assert.Equal(t, int64(0), legacyExists, "the legacy per-pair key must not be written")
	})

	t.Run("repeat evaluation returns exactly the stored fields", func(t *testing.T) {
		got, err := cache.GetMatches(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, map[pattern.Entity]bool{ //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
			pattern.EntityEmail: false,
			pattern.EntityCPF:   false,
		}, got)
	})

	t.Run("unknown token yields empty map and nil error", func(t *testing.T) {
		got, err := cache.GetMatches(ctx, []byte("never-written-token"))
		require.NoError(t, err)
		require.NotNil(t, got, "an empty result must be a usable empty map, not nil")
		assert.Empty(t, got)
	})

	t.Run("partially evaluated token returns only the stored subset", func(t *testing.T) {
		partial := []byte("partial-token@example.com")
		require.NoError(t, cache.SaveMatches(ctx, partial, map[pattern.Entity]bool{ //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
			pattern.EntityEmail: false,
		}))

		got, err := cache.GetMatches(ctx, partial)
		require.NoError(t, err)
		assert.Equal(t, map[pattern.Entity]bool{pattern.EntityEmail: false}, got, //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
			"entities absent from the hash are not reported")

		// Later additions extend the stored fields without dropping earlier
		// ones; the adapter codes positives as "1" when the caller provides them.
		require.NoError(t, cache.SaveMatches(ctx, partial, map[pattern.Entity]bool{ //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
			pattern.EntityCPF:   false,
			pattern.EntityPhone: true,
		}))

		stored, err := inspector.Do(ctx, inspector.B().Hgetall().Key(matchHashKey(partial)).Build()).AsMap()
		require.NoError(t, err)
		phoneField := stored[string(pattern.EntityPhone)]
		phoneValue, err := phoneField.ToString()
		require.NoError(t, err)
		assert.Equal(t, "1", phoneValue, "positive results are boolean-coded as \"1\"")

		got, err = cache.GetMatches(ctx, partial)
		require.NoError(t, err)
		assert.Equal(t, map[pattern.Entity]bool{ //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
			pattern.EntityEmail: false,
			pattern.EntityCPF:   false,
			pattern.EntityPhone: true,
		}, got)
	})
}

// TestTokenHash_TTL verifies the TTL scenarios of the token hash key: the
// expiry is established exactly once at first write (later additions keep it),
// zero jitter uses the base TTL exactly, and a zero base TTL writes without
// expiry.
func TestTokenHash_TTL(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()
	inspector := startInspector(t, addr)

	t.Run("second SaveMatches keeps the original expiry", func(t *testing.T) {
		const base = 10 * time.Second
		cache := newTokenCache(ctx, t, addr, base, 0, false)

		token := []byte("ttl-once@example.com")
		require.NoError(t, cache.SaveMatches(ctx, token, map[pattern.Entity]bool{pattern.EntityEmail: false})) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
		pttlFirst := remainingTTL(t, inspector, token)

		const sleep = 200 * time.Millisecond
		time.Sleep(sleep)

		require.NoError(t, cache.SaveMatches(ctx, token, map[pattern.Entity]bool{pattern.EntityCPF: false})) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
		pttlSecond := remainingTTL(t, inspector, token)

		// The remaining TTL must have kept decaying from the first reading
		// instead of being re-established at the base TTL: EXPIRE NX must never
		// extend the expiry. Half the sleep is the midpoint between a kept
		// reading (base − sleep − latency) and a reset reading (base).
		assert.Less(t, float64(pttlSecond), float64(pttlFirst)-0.5*float64(sleep/time.Millisecond),
			"the second write must not re-establish the key's TTL at the base")
	})

	t.Run("zero jitter uses the base TTL exactly", func(t *testing.T) {
		const base = 10 * time.Second
		cache := newTokenCache(ctx, t, addr, base, 0, false)

		token := []byte("ttl-zero-jitter@example.com")
		require.NoError(t, cache.SaveMatches(ctx, token, map[pattern.Entity]bool{pattern.EntityEmail: false})) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
		savedAt := time.Now()

		pttl := remainingTTL(t, inspector, token)
		elapsedMs := float64(time.Since(savedAt)) / float64(time.Millisecond)

		assert.InDelta(t, float64(base/time.Millisecond), float64(pttl)+elapsedMs, 200,
			"without jitter the establishment TTL must equal the base TTL")
	})

	t.Run("zero base TTL writes without expiry", func(t *testing.T) {
		cache := newTokenCache(ctx, t, addr, 0, 0.5, false)

		token := []byte("ttl-no-expiry@example.com")
		require.NoError(t, cache.SaveMatches(ctx, token, map[pattern.Entity]bool{pattern.EntityEmail: false})) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses

		pttl := remainingTTL(t, inspector, token)
		assert.Equal(t, int64(-1), pttl, "a zero base TTL must write the hash without expiry")

		got, err := cache.GetMatches(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, map[pattern.Entity]bool{pattern.EntityEmail: false}, got) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	})
}

// TestTokenHash_JitterBand verifies the "Jittered Token TTL" scenario: with
// jitter configured at P percent, the token hash's establishment TTL lies
// within [base×(1−P), base×(1+P)], and jitter is actually applied: at least
// one key must deviate from the base TTL.
//
// As in the jitter tests, PTTL reports the remaining TTL, so each reading is
// compensated with the time elapsed since its save before the band comparison.
func TestTokenHash_JitterBand(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()
	inspector := startInspector(t, addr)
	cache := newTokenCache(ctx, t, addr, 10*time.Second, 0.15, false)

	const baseMs = 10000.0
	const lower = baseMs * (1 - 0.15)
	const upper = baseMs * (1 + 0.15)
	// bandSlackMs absorbs PTTL's integer-ms rounding and the client-side
	// measurement lag between the save and the PTTL read; the ±15% band is
	// 3000 ms wide, so a few ms of slack does not weaken the band assertion.
	const bandSlackMs = 5.0

	const tokens = 20
	spread := false
	for i := range tokens {
		token := []byte(fmt.Sprintf("jitter-band-%02d@example.com", i))
		require.NoError(t, cache.SaveMatches(ctx, token, map[pattern.Entity]bool{pattern.EntityEmail: false})) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
		savedAt := time.Now()

		pttl := remainingTTL(t, inspector, token)
		elapsedMs := float64(time.Since(savedAt)) / float64(time.Millisecond)
		effectiveMs := float64(pttl) + elapsedMs

		assert.GreaterOrEqual(t, effectiveMs, lower-bandSlackMs, "token %d effective TTL below band", i)
		assert.LessOrEqual(t, effectiveMs, upper+bandSlackMs, "token %d effective TTL above band", i)
		if effectiveMs > baseMs+50 || effectiveMs < baseMs-50 {
			spread = true
		}
	}

	assert.True(t, spread, "expected at least one jittered establishment TTL to deviate from the base TTL")
}

// TestTokenHash_ClientSideReads verifies the "Token-Scoped Cache Reads"
// scenario: with client-side caching enabled, repeated GetMatches calls return
// the stored map, and with the in-memory cache disabled the plain read path
// returns it too.
func TestTokenHash_ClientSideReads(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()

	want := map[pattern.Entity]bool{ //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
		pattern.EntityEmail: false,
		pattern.EntityCPF:   false,
	}

	t.Run("repeated reads with client-side caching return the stored map", func(t *testing.T) {
		cache := newTokenCache(ctx, t, addr, 10*time.Second, 0, false)

		token := []byte("csc-hot@example.com")
		require.NoError(t, cache.SaveMatches(ctx, token, want))

		first, err := cache.GetMatches(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, want, first)

		second, err := cache.GetMatches(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, want, second)
	})

	t.Run("reads with the in-memory cache disabled use the plain path", func(t *testing.T) {
		cache := newTokenCache(ctx, t, addr, 10*time.Second, 0, true)

		token := []byte("csc-disabled@example.com")
		require.NoError(t, cache.SaveMatches(ctx, token, want))

		got, err := cache.GetMatches(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

// cacheTracerTestKey is the context key shared by the tracer delegation tests.
type cacheTracerTestKey struct{}

// TestTokenHash_TracerGetMatches mirrors the tracer contract for the
// token-scoped read: the decorator delegates to the underlying store and
// forwards the span-bearing context so the token-tagged span wraps the call.
func TestTokenHash_TracerGetMatches(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUnderlying := cachemock.NewMockCacheStore(ctrl)

	data := []byte("tracer-token@example.com")
	want := map[pattern.Entity]bool{pattern.EntityEmail: false} //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses

	var capturedCtx context.Context
	mockUnderlying.EXPECT().
		GetMatches(gomock.Any(), data).
		DoAndReturn(func(ctx context.Context, _ []byte) (map[pattern.Entity]bool, error) {
			capturedCtx = ctx
			return want, nil
		})

	callerCtx := context.WithValue(context.Background(), cacheTracerTestKey{}, "marker")
	tracer := analyzercache.NewCacheStoreTracer(mockUnderlying)
	got, err := tracer.GetMatches(callerCtx, data)

	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, "marker", capturedCtx.Value(cacheTracerTestKey{}))
}

// TestTokenHash_TracerSaveMatches mirrors the tracer contract for the
// token-scoped write: the decorator delegates to the underlying store and
// forwards the span-bearing context so the token-tagged span wraps the call.
func TestTokenHash_TracerSaveMatches(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUnderlying := cachemock.NewMockCacheStore(ctrl)

	data := []byte("tracer-token@example.com")
	results := map[pattern.Entity]bool{pattern.EntityEmail: false} //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses

	var capturedCtx context.Context
	mockUnderlying.EXPECT().
		SaveMatches(gomock.Any(), data, results).
		DoAndReturn(func(ctx context.Context, _ []byte, _ map[pattern.Entity]bool) error {
			capturedCtx = ctx
			return nil
		})

	callerCtx := context.WithValue(context.Background(), cacheTracerTestKey{}, "marker")
	tracer := analyzercache.NewCacheStoreTracer(mockUnderlying)
	err := tracer.SaveMatches(callerCtx, data, results)

	require.NoError(t, err)
	assert.Equal(t, "marker", capturedCtx.Value(cacheTracerTestKey{}))
}
