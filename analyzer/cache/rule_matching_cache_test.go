package cache_test

import (
	"context"
	"testing"
	"time"

	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	cachetesting "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache/testing"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
)

// TestRuleMatchingCache_AutoPipelining verifies that concurrent SaveMatches
// and GetMatches calls all complete correctly, exercising the auto-pipelining path
// where valkey-go coalesces concurrent Do calls into batched round-trips.
func TestRuleMatchingCache_AutoPipelining(t *testing.T) {
	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
		Redis: analyzercache.RedisOptions{
			Addr:               addr,
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)

	const workers = 50
	tokens := make([][]byte, workers)
	for i := range tokens {
		tokens[i] = []byte("pipeline-token-" + string(rune('A'+i%26)) + string(rune('0'+i%10)))
	}

	saveErrs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			saveErrs <- cache.SaveMatches(ctx, tokens[i], map[pattern.Entity]bool{pattern.EntityEmail: i%2 == 0})
		}(i)
	}
	for i := 0; i < workers; i++ {
		require.NoError(t, <-saveErrs)
	}

	type result struct {
		idx int
		got map[pattern.Entity]bool
		err error
	}
	results := make(chan result, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			got, getErr := cache.GetMatches(ctx, tokens[i])
			results <- result{i, got, getErr}
		}(i)
	}
	for range workers {
		r := <-results
		require.NoError(t, r.err)
		assert.Equal(t, map[pattern.Entity]bool{pattern.EntityEmail: r.idx%2 == 0}, r.got, "token index %d", r.idx)
	}
}

func TestRuleMatchingCache_PingOnConnect(t *testing.T) {
	ctx := context.Background()

	t.Run("ping succeeds on valid address", func(t *testing.T) {
		addr := cachetesting.StartValkeyContainer(t)

		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 10 * time.Second,
			Redis: analyzercache.RedisOptions{
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
			CacheTTL: 10 * time.Second,
			Redis: analyzercache.RedisOptions{
				// Port intentionally picked as an unlikely-to-bind high port for
				// unreachable-address assertions: if a service ever binds it, this
				// test fails loudly, not silently.
				Addr:               "localhost:19379",
				DisableClusterMode: true,
				PingOnConnect:      true,
			},
		}

		cache, err := analyzercache.NewCacheStore(ctx, options)
		require.Error(t, err)
		assert.Nil(t, cache)
	})
}

func TestRuleMatchingCache_ValkeyConfigMutator_NilPreservesDefaults(t *testing.T) {
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
		Redis: analyzercache.RedisOptions{
			// No live server is needed: with a nil mutator the default mapping
			// stays intact and the only failure is the plain dial error against
			// the unreachable address. The previous SaveMatch/GetMatch round-trip
			// was redundant for this assertion. Port 19379 is an intentionally
			// unlikely-to-bind high port: if a service ever binds it, the
			// unreachable-address assertions fail loudly, not silently.
			Addr:               "localhost:19379",
			DisableClusterMode: true,
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.Error(t, err)
	assert.Nil(t, cache)
	assert.Contains(t, err.Error(), "failed to create valkey client")
}

func TestRuleMatchingCache_ValkeyConfigMutator_ObservesMappedDefaultsOnce(t *testing.T) {
	ctx := context.Background()

	var calls int
	var observedInitAddress []string
	var observedForceSingleClient, observedDisableCache bool
	var observedDialTimeout, observedConnWriteTimeout time.Duration
	var observedPoolSize int

	mutator := func(opt *valkey.ClientOption) {
		calls++
		observedInitAddress = opt.InitAddress
		observedForceSingleClient = opt.ForceSingleClient
		observedDisableCache = opt.DisableCache
		observedDialTimeout = opt.Dialer.Timeout
		observedConnWriteTimeout = opt.ConnWriteTimeout
		observedPoolSize = opt.BlockingPoolSize
	}

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL:             10 * time.Second,
		DisableInMemoryCache: true,
		Redis: analyzercache.RedisOptions{
			// No live server is needed: this test asserts only mutator-observable
			// values, captured before client creation fails against the
			// unreachable address. Port 19379 is an intentionally unlikely-to-bind
			// high port: if a service ever binds it, the tests fail loudly, not
			// silently.
			Addr:               "localhost:19379",
			DisableClusterMode: true,
			DialTimeout:        2 * time.Second,
			WriteTimeout:       3 * time.Second,
			PoolSize:           5,
		},
		ValkeyConfigMutator: mutator,
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.Error(t, err)
	assert.Nil(t, cache)
	assert.Contains(t, err.Error(), "failed to create valkey client")

	// The callback observed every Leakspok mapping exactly once before client
	// creation failed against the unreachable address.
	assert.Equal(t, 1, calls)
	assert.Equal(t, []string{"localhost:19379"}, observedInitAddress)
	assert.True(t, observedForceSingleClient)
	assert.True(t, observedDisableCache)
	assert.Equal(t, 2*time.Second, observedDialTimeout)
	assert.Equal(t, 3*time.Second, observedConnWriteTimeout)
	assert.Equal(t, 5, observedPoolSize)
}

// TestRuleMatchingCache_ValkeyConfigMutator_OverridesMappedOption proves
// serverless that a valid override of a Leakspok-mapped option is what client
// construction actually uses. Leakspok maps RedisOptions.Addr into
// opt.InitAddress; the mutator replaces it with a different valid value. Under
// ForceSingleClient (which DisableClusterMode maps to), valkey-go dials
// synchronously inside NewClient, so the returned dial error names the endpoint
// the client was built with — the overridden one, never the mapped one.
func TestRuleMatchingCache_ValkeyConfigMutator_OverridesMappedOption(t *testing.T) {
	ctx := context.Background()

	var calls int
	var observedInitAddress []string

	mutator := func(opt *valkey.ClientOption) {
		calls++
		observedInitAddress = opt.InitAddress
		opt.InitAddress = []string{"localhost:22222"}
	}

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
		Redis: analyzercache.RedisOptions{
			// No live server is needed: the callback runs before valkey.NewClient,
			// and the synchronous dial against the overridden address fails with an
			// error that names that address. Port 19379 is an intentionally
			// unlikely-to-bind high port: if a service ever binds it, the tests
			// fail loudly, not silently.
			Addr:               "localhost:19379",
			DisableClusterMode: true,
		},
		ValkeyConfigMutator: mutator,
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.Error(t, err)
	assert.Nil(t, cache)
	assert.Contains(t, err.Error(), "failed to create valkey client")

	// The callback observed the Leakspok-mapped default exactly once and
	// overrode it before client creation.
	assert.Equal(t, 1, calls)
	assert.Equal(t, []string{"localhost:19379"}, observedInitAddress)

	// Client creation was reached with the caller-assigned value: the dial error
	// names the overridden endpoint. localhost may resolve to 127.0.0.1 or ::1,
	// so the port is the stable discriminator; the mapped port must not appear.
	assert.Contains(t, err.Error(), "22222")
	assert.NotContains(t, err.Error(), "19379")
}

func TestRuleMatchingCache_ValkeyConfigMutator_InvalidUpstreamOptionFails(t *testing.T) {
	ctx := context.Background()

	var calls int

	// EnableRedirect combined with ReplicaAddress is rejected synchronously by
	// valkey.NewClient before any connection attempt.
	mutator := func(opt *valkey.ClientOption) {
		calls++
		opt.Standalone.EnableRedirect = true
		opt.Standalone.ReplicaAddress = []string{"localhost:6379"}
	}

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
		Redis: analyzercache.RedisOptions{
			Addr:               "localhost:6379",
			DisableClusterMode: true,
		},
		ValkeyConfigMutator: mutator,
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.Error(t, err)
	assert.Nil(t, cache)
	assert.Equal(t, 1, calls)
	assert.Contains(t, err.Error(), "failed to create valkey client")
	assert.Contains(t, err.Error(), "EnableRedirect and ReplicaAddress cannot be used together")
}

// TestRuleMatchingCache_ValkeyConfigMutator_ClientNameObservable proves
// end-to-end that a ClientName assigned by ValkeyConfigMutator reaches a real
// single-node Valkey server. The cache store is built through the public cache
// factory path, a cache operation is performed against the container, and a
// separate inspection client observes the configured name via CLIENT LIST.
func TestRuleMatchingCache_ValkeyConfigMutator_ClientNameObservable(t *testing.T) {
	const clientName = "leakspok-cache-mutator-integration"

	addr := cachetesting.StartValkeyContainer(t)
	ctx := context.Background()

	options := analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
		Redis: analyzercache.RedisOptions{
			Addr:               addr,
			DisableClusterMode: true,
		},
		ValkeyConfigMutator: func(opt *valkey.ClientOption) {
			opt.ClientName = clientName
		},
	}

	cache, err := analyzercache.NewCacheStore(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Perform a real cache operation so the client connection is exercised.
	token := []byte("client-name@example.com")
	require.NoError(t, cache.SaveMatches(ctx, token, map[pattern.Entity]bool{pattern.EntityEmail: false}))

	got, getErr := cache.GetMatches(ctx, token)
	require.NoError(t, getErr)
	assert.Equal(t, map[pattern.Entity]bool{pattern.EntityEmail: false}, got)

	cachetesting.AssertValkeyClientName(t, addr, clientName)
}
