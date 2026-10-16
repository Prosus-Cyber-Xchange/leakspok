package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/valkey-io/valkey-go"
)

// matchKeyNamespace prefixes every token hash key. It marks the token-hash data
// layout and prevents collisions with other applications sharing the backend;
// the encoding lives only in this adapter.
const matchKeyNamespace = "leakspok:m:"

// RuleMatchingCache caches rule matching results using valkey-go.
//
// It benefits from two valkey-go features with zero extra code:
//   - Auto pipelining: concurrent Do/DoCache calls from different goroutines are
//     automatically batched into a single network round-trip.
//   - Client-side caching: DoCache stores results in local process memory and the
//     server pushes invalidation messages when keys change, eliminating round-trips
//     for hot keys on subsequent requests.
//
// Client-side caching is disabled when RuleMatchingCacheOptions.DisableInMemoryCache is true.
type RuleMatchingCache struct {
	client     valkey.Client
	cacheTTL   time.Duration
	jitterPct  float64
	disableCSC bool
	keyBufPool sync.Pool
}

// NewRuleMatchingCache creates a RuleMatchingCache from the provided options.
func NewRuleMatchingCache(ctx context.Context, options RuleMatchingCacheOptions) (*RuleMatchingCache, error) {
	r := options.Redis

	opt := valkey.ClientOption{
		InitAddress:       strings.Split(r.Addr, ","),
		Username:          r.Username,
		Password:          r.Password,
		ForceSingleClient: r.DisableClusterMode,
		DisableCache:      options.DisableInMemoryCache,
		Dialer: net.Dialer{
			Timeout: r.DialTimeout,
		},
	}

	if r.WriteTimeout > 0 {
		opt.ConnWriteTimeout = r.WriteTimeout
	}

	if r.PoolSize > 0 {
		opt.BlockingPoolSize = r.PoolSize
	}

	if r.Password != "" {
		opt.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: r.InsecureSkipVerify, //nolint:gosec // Controlled by caller; disabled by default
		}
	}

	if options.ValkeyConfigMutator != nil {
		options.ValkeyConfigMutator(&opt)
	}

	client, err := valkey.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	if r.PingOnConnect {
		if pingErr := client.Do(ctx, client.B().Ping().Build()).Error(); pingErr != nil {
			client.Close()
			return nil, fmt.Errorf("valkey connection check failed: %w", pingErr)
		}
	}

	return &RuleMatchingCache{
		client:     client,
		cacheTTL:   options.CacheTTL,
		jitterPct:  options.TTLJitterPercentage,
		disableCSC: options.DisableInMemoryCache,
	}, nil
}

// jitteredTTL returns the base TTL randomized within +/-jitterPct.
// A jitterPct of zero returns the base TTL unchanged. Callers already
// guard the no-expiry case (CacheTTL == 0) before calling.
func (r *RuleMatchingCache) jitteredTTL() time.Duration {
	if r.jitterPct <= 0 {
		return r.cacheTTL
	}
	delta := time.Duration(float64(r.cacheTTL) * r.jitterPct * (2*rand.Float64() - 1)) //nolint:gosec // Expiry spreading only; not security-sensitive
	return r.cacheTTL + delta
}

// useClientSideCache reports whether reads may be served from the process-local
// client-side cache: it requires CSC to be enabled and a positive base TTL for
// the jittered local expiry.
func (r *RuleMatchingCache) useClientSideCache() bool {
	return !r.disableCSC && r.cacheTTL > 0
}

// GetMatch retrieves a cached rule matching result.
// Uses DoCache for client-side caching unless disabled; falls back to Do otherwise.
// Auto pipelining coalesces concurrent calls into a single network round-trip.
func (r *RuleMatchingCache) GetMatch(ctx context.Context, entity pattern.Entity, data []byte) (bool, error) {
	key := r.key(entity, data)

	var result valkey.ValkeyResult

	if !r.useClientSideCache() {
		result = r.client.Do(ctx, r.client.B().Get().Key(key).Build())
	} else {
		result = r.client.DoCache(ctx, r.client.B().Get().Key(key).Cache(), r.jitteredTTL())
	}

	val, err := result.ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return false, ErrCacheNotFound
		}

		return false, err
	}

	return val == "1", nil
}

// SaveMatch caches a rule matching result.
// Auto pipelining coalesces concurrent writes into a single network round-trip.
func (r *RuleMatchingCache) SaveMatch(ctx context.Context, entity pattern.Entity, data []byte, matched bool) error {
	key := r.key(entity, data)

	val := "0"
	if matched {
		val = "1"
	}

	var cmd valkey.Completed
	if r.cacheTTL > 0 {
		cmd = r.client.B().Set().Key(key).Value(val).Px(r.jitteredTTL()).Build()
	} else {
		cmd = r.client.B().Set().Key(key).Value(val).Build()
	}

	return r.client.Do(ctx, cmd).Error()
}

// matchKey encodes a token's hash key: the token content under the namespace.
func matchKey(data []byte) string {
	return matchKeyNamespace + string(data)
}

// GetMatches retrieves every rule-matching result stored for the token's hash
// with a single HGETALL command, using DoCache for client-side caching when it
// is enabled and the base TTL is positive, and plain Do otherwise. An empty
// hash yields an empty map and a nil error: nothing is cached for the token.
func (r *RuleMatchingCache) GetMatches(ctx context.Context, data []byte) (map[pattern.Entity]bool, error) {
	key := matchKey(data)

	var result valkey.ValkeyResult

	if !r.useClientSideCache() {
		result = r.client.Do(ctx, r.client.B().Hgetall().Key(key).Build())
	} else {
		result = r.client.DoCache(ctx, r.client.B().Hgetall().Key(key).Cache(), r.jitteredTTL())
	}

	fields, err := result.AsMap()
	if err != nil {
		return nil, err
	}

	results := make(map[pattern.Entity]bool, len(fields))
	for field, message := range fields {
		value, err := message.ToString()
		if err != nil {
			return nil, err
		}
		results[pattern.Entity(field)] = value == "1"
	}

	return results, nil
}

// SaveMatches persists a batch of rule-matching results for the token's hash in
// a single multi-field HSET. When the base TTL is positive the HSET is issued
// together with EXPIRE NX in one DoMulti, establishing the jittered TTL exactly
// once at the key's first write while existing expiries are never extended; a
// zero base TTL writes the hash without expiry. An empty batch is a no-op.
func (r *RuleMatchingCache) SaveMatches(ctx context.Context, data []byte, results map[pattern.Entity]bool) error {
	if len(results) == 0 {
		return nil
	}

	key := matchKey(data)

	hset := r.client.B().Hset().Key(key).FieldValue()
	for entity, matched := range results {
		value := "0"
		if matched {
			value = "1"
		}
		hset = hset.FieldValue(string(entity), value)
	}

	if r.cacheTTL == 0 {
		return r.client.Do(ctx, hset.Build()).Error()
	}

	resp := r.client.DoMulti(ctx,
		hset.Build(),
		r.client.B().Expire().Key(key).Seconds(r.expireSeconds()).Nx().Build(),
	)
	if err := resp[0].Error(); err != nil {
		return err
	}

	return resp[1].Error()
}

// expireSeconds converts the jittered base TTL into whole seconds for the
// EXPIRE command, clamped into the jitter band [base×(1−P), base×(1+P)] so the
// second-granularity command cannot fall outside it. Callers already guard the
// no-expiry case (CacheTTL == 0) before calling.
func (r *RuleMatchingCache) expireSeconds() int64 {
	seconds := int64(math.Round(r.jitteredTTL().Seconds()))

	lower := int64(math.Ceil(float64(r.cacheTTL) * (1 - r.jitterPct) / float64(time.Second)))
	upper := int64(math.Floor(float64(r.cacheTTL) * (1 + r.jitterPct) / float64(time.Second)))

	// Second granularity may round the jittered TTL just outside the band; the
	// clamps put it back in. When the band holds no whole second (sub-second
	// base TTLs), lower exceeds upper and the clamps collapse onto the nearest
	// representable second.
	if seconds < lower {
		seconds = lower
	}
	if seconds > upper {
		seconds = upper
	}

	// EXPIRE 0 deletes the key immediately; that must never happen.
	if seconds < 1 {
		seconds = 1
	}

	return seconds
}

// key generates a cache key in "entity:data" format, reusing a sync.Pool buffer.
func (r *RuleMatchingCache) key(x pattern.Entity, y []byte) string {
	n := len(x) + 1 + len(y)

	var buf []byte

	if raw := r.keyBufPool.Get(); raw != nil {
		//nolint:forcetypeassert,errcheck // sync.Pool stores []byte, type assertion is safe
		buf = raw.([]byte)
	}

	if cap(buf) < n {
		buf = make([]byte, n)
	}
	buf = buf[:n]

	i := copy(buf, x)
	buf[i] = ':'
	copy(buf[i+1:], y)

	s := string(buf)

	//nolint:staticcheck // SA6002: using non-pointer intentionally for buffer reuse
	r.keyBufPool.Put(buf)

	return s
}
