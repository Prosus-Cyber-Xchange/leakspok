package analyzer_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	analyzermock "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/mocks"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ─── Factory Error Paths ─────────────────────────────────────────────────────

func TestMakeByteAnalyzer_RuleRunnerPoolSizeZero(t *testing.T) {
	_, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                  true,
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       0, // invalid
			},
		})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RuleRunnerPoolSize must be > 0")
}

func TestMakeByteAnalyzer_TokenPoolSizeZero(t *testing.T) {
	_, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                   true,
				ConcurrentTokenProcessing: true,
				TokenPoolSize:             0, // invalid
			},
		})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TokenPoolSize must be > 0")
}

func TestMakeStringAnalyzer_RuleRunnerPoolSizeZero(t *testing.T) {
	_, err := analyzer.MakeStringAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                  true,
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       0, // invalid
			},
		})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RuleRunnerPoolSize must be > 0")
}

func TestMakeByteAnalyzer_ConcurrentRuleProcessingSuccess(t *testing.T) {
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                  true,
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       4,
			},
		})
	require.NoError(t, err)

	// Verify it was created with concurrency (pool set only for token processing)
	assert.NotNil(t, ba)
}

func TestMakeByteAnalyzer_ConcurrentTokenProcessingWithTimeout(t *testing.T) {
	// Tests poolOptions when MaxGoroutineIdleTimeout is set
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                   true,
				ConcurrentTokenProcessing: true,
				TokenPoolSize:             4,
				MaxGoroutineIdleTimeout:   5 * time.Second,
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, ba)
}

func TestMakeStringAnalyzer_ConcurrentRuleProcessingSuccess(t *testing.T) {
	sa, err := analyzer.MakeStringAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                  true,
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       4,
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, sa)
}

// ─── buildCacheStore Paths ───────────────────────────────────────────────────

func TestMakeByteAnalyzer_CacheEnabledGocache(t *testing.T) {
	// Tests buildCacheStore with Cache.Enabled=true (gocache backend)
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Cache: analyzer.CacheOptions{
				Enabled: true,
				TTL:     30 * time.Second,
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, ba)
}

func TestMakeByteAnalyzer_CacheEnabledWithTracing(t *testing.T) {
	// Tests buildCacheStore with TracingEnabled=true
	// DefaultTracer is the noop tracer (build tag: !datadog), so this path works.
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Cache: analyzer.CacheOptions{
				Enabled:        true,
				TracingEnabled: true,
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, ba)
}

func TestMakeByteAnalyzer_CacheWithMemorySizeZero(t *testing.T) {
	// MemorySize=0 means use default (10MB), Memory.Enabled defaults to true
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Cache: analyzer.CacheOptions{
				Enabled:    true,
				MemorySize: 0, // default 10MB
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, ba)
}

// ─── ByteAnalyzer.Stop with pool set ─────────────────────────────────────────

func TestByteAnalyzer_StopWithPool(t *testing.T) {
	// Create a ByteAnalyzer with ConcurrentTokenProcessing so that ba.pool is non-nil.
	// Calling Stop must then exercise the pool.ReleaseContext branch.
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                   true,
				ConcurrentTokenProcessing: true,
				TokenPoolSize:             2,
			},
		})
	require.NoError(t, err)

	// Should not panic, should exercise pool != nil branch in Stop.
	ba.Stop()

	// Second Stop should also be safe (ReleaseContext is idempotent).
	ba.Stop()
}

func TestByteAnalyzer_StopWithConcurrentRuleProcessing(t *testing.T) {
	// Create with ConcurrentRuleProcessing (runner pool set, token pool nil).
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                  true,
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       2,
			},
		})
	require.NoError(t, err)

	// Stop must release the runner pool and not panic.
	ba.Stop()
}

// ─── writeToOutput error path ────────────────────────────────────────────────

// errorWriter is an io.Writer that always returns an error.
type errorWriter struct{}

func (e *errorWriter) Write([]byte) (int, error) {
	return 0, io.ErrShortWrite
}

func TestByteAnalyzer_WriteToOutputErrorPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{})
	require.NoError(t, err)

	// Use a real matcher so the pipeline actually tries to write output.
	rules := []analyzer.Rule{
		{
			Name:    "email-rule",
			Matcher: newEmailMatcher(ctrl),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact:   &analyzer.RedactSettings{Placeholder: "[REDACTED]"},
			},
		},
	}

	ctx := context.Background()
	// errorWriter always fails Write; the logger should log a warning but not panic.
	ew := &errorWriter{}
	details := ba.Anonymize(ctx, rules, ew, []byte("user@test.com"))

	// Even though writing failed, the detection should still work.
	assert.True(t, details.HasFindings)
}

// ─── SerialRulesRunner.Process cache hit (positive match) ────────────────────

// prePopulatedCache wraps a NoopRuleMatchingCache but can return a pre-set match.
type prePopulatedCache struct {
	analyzercache.CacheStore
	matchResult   bool
	matchReturned bool
}

func (p *prePopulatedCache) GetMatch(ctx context.Context, entity pattern.Entity, data []byte) (bool, error) {
	if !p.matchReturned {
		p.matchReturned = true
		return p.matchResult, nil
	}
	return false, store.NotFound{}
}

func TestSerialRulesRunner_ProcessCachePositiveHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := &prePopulatedCache{
		CacheStore:  analyzercache.NewNoopRuleMatchingCache(),
		matchResult: true,
	}

	runner := analyzer.NewSerialRulesRuner(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache,
	)

	// Entity() is called by cache.GetMatch for key construction, but Match() should NOT be called.
	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	// No EXPECT for Match — if Match is called, gomock will fail the test.

	rule := analyzer.Rule{
		Name:    "cached-rule",
		Matcher: mockMatcher,
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("any data"))
	assert.True(t, found)
	assert.Equal(t, "cached-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessCacheNegativeHitIsSkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := &prePopulatedCache{
		CacheStore:  analyzercache.NewNoopRuleMatchingCache(),
		matchResult: false,
	}

	runner := analyzer.NewSerialRulesRuner(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache,
	)

	// When cache returns a negative hit (matched=false, nil error), the rule
	// is skipped — Match is never called but Entity is called for cache key.
	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	// No EXPECT on Match — if called, gomock will fail.

	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: mockMatcher,
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))
	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

// ─── anonymizeConcurrent pool.Submit error path ─────────────────────────────

func TestByteAnalyzer_AnonymizeConcurrentPoolSubmitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create with concurrent token processing.
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                   true,
				ConcurrentTokenProcessing: true,
				TokenPoolSize:             2,
			},
		})
	require.NoError(t, err)

	rules := []analyzer.Rule{
		{
			Name:    "email-rule",
			Matcher: newEmailMatcher(ctrl),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact:   &analyzer.RedactSettings{Placeholder: "[REDACTED]"},
			},
		},
	}

	// Stop first to close the pool, then call Anonymize.
	// Some goroutines may already be in-flight, so output may be partial.
	// The key assertion: no panic and detection works (or not, depending on timing).
	ba.Stop()

	ctx := context.Background()
	output := &bytes.Buffer{}

	// Must not panic.
	require.NotPanics(t, func() {
		ba.Anonymize(ctx, rules, output, []byte("user@test.com extra text here"))
	})
}

// ─── poolOptions zero timeout ───────────────────────────────────────────────

func TestMakeByteAnalyzer_ConcurrentTokenProcessingZeroIdleTimeout(t *testing.T) {
	// MaxGoroutineIdleTimeout=0 means no timeout option is appended.
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                   true,
				ConcurrentTokenProcessing: true,
				TokenPoolSize:             4,
				MaxGoroutineIdleTimeout:   0, // should not append option
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, ba)
}

// ─── StringAnalyzer.Stop ─────────────────────────────────────────────────────

func TestStringAnalyzer_Stop(t *testing.T) {
	sa, err := analyzer.MakeStringAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{})
	require.NoError(t, err)

	// Should not panic
	sa.Stop()
}

// ─── ConcurrentRulesRunner processRule cache error path ──────────────────────

// errorCache returns an error on every GetMatch call.
type errorCache struct{}

func (e *errorCache) GetMatch(ctx context.Context, entity pattern.Entity, data []byte) (bool, error) {
	return false, assert.AnError
}

func (e *errorCache) SaveMatch(ctx context.Context, entity pattern.Entity, data []byte, matched bool) error {
	return nil
}

func TestConcurrentRulesRunner_ProcessRuleCacheError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, err := analyzer.NewAntsWorkerPool(2, logger)
	require.NoError(t, err)

	runner := analyzer.NewConcurrentRulesRunner(
		logger,
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       2,
			},
		},
		&errorCache{},
		pool,
	)
	defer runner.Stop()

	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	// The processRule will log the cache error but still evaluate the matcher.
	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))
	assert.True(t, found)
	assert.Equal(t, "rule", matchedRule.Name)
}

// ─── processRule cache positive hit path ────────────────────────────────────

// singleHitCache returns a positive match exactly once, then reverts to not-found.
type singleHitCache struct {
	hit  bool
	used bool
}

func (s *singleHitCache) GetMatch(ctx context.Context, entity pattern.Entity, data []byte) (bool, error) {
	if !s.used {
		s.used = true
		return s.hit, nil
	}
	return false, store.NotFound{}
}

func (s *singleHitCache) SaveMatch(ctx context.Context, entity pattern.Entity, data []byte, matched bool) error {
	return nil
}

func TestConcurrentRulesRunner_ProcessRuleCachePositiveHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, err := analyzer.NewAntsWorkerPool(2, logger)
	require.NoError(t, err)

	cache := &singleHitCache{hit: true}
	runner := analyzer.NewConcurrentRulesRunner(
		logger,
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       2,
			},
		},
		cache,
		pool,
	)
	defer runner.Stop()

	// Entity() is called by cache.GetMatch for key construction, but Match() should NOT be called.
	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	// No EXPECT for Match — if Match is called, gomock will fail.

	rule := analyzer.Rule{
		Name:    "cached-rule",
		Matcher: mockMatcher,
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("any data"))
	assert.True(t, found)
	assert.Equal(t, "cached-rule", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessRuleCacheNegativeHitIsSkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, err := analyzer.NewAntsWorkerPool(2, logger)
	require.NoError(t, err)

	cache := &singleHitCache{hit: false}
	runner := analyzer.NewConcurrentRulesRunner(
		logger,
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       2,
			},
		},
		cache,
		pool,
	)
	defer runner.Stop()

	// When cache returns a negative hit (matched=false, nil error), the rule is
	// skipped — Match is never called.
	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	// No EXPECT on Match — if called, gomock will fail.

	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: mockMatcher,
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))
	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

// ─── ByteAnalyzer Anonymize with no findings exercises writeToOutput path ─────

func TestByteAnalyzer_AnonymizeNoFindingsWritesOriginal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{})
	require.NoError(t, err)

	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mockMatcher.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	rules := []analyzer.Rule{
		{
			Name:    "no-match",
			Matcher: mockMatcher,
		},
	}

	ctx := context.Background()
	output := &bytes.Buffer{}
	input := []byte("plain text without any PII")
	details := ba.Anonymize(ctx, rules, output, input)

	assert.False(t, details.HasFindings)
	assert.Equal(t, input, output.Bytes())
}

// ─── CacheOptions with MemorySize=-1 (disable in-memory cache) ──────────────

func TestMakeByteAnalyzer_CacheMemoryDisabled(t *testing.T) {
	ba, err := analyzer.MakeByteAnalyzer(context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		analyzer.RunnerOptions{
			Cache: analyzer.CacheOptions{
				Enabled:    true,
				MemorySize: -1, // disables in-memory caching
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, ba)
}
