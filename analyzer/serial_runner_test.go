package analyzer_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	cachemock "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache/mocks"
	analyzermock "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/mocks"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSerialRulesRunner_ProcessSingleMatchingRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "matching-rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessNoMatchingRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "non-matching-rule",
		Matcher: newNonMatchingMatcher(ctrl),
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessMultipleRulesFirstMatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rules := []analyzer.Rule{
		{
			Name:    "first-matching-rule",
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "second-matching-rule",
			Matcher: newMatchingMatcher(ctrl),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	// Serial runner always returns the first matching rule
	assert.Equal(t, "first-matching-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessMultipleRulesSecondMatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	// Each rule carries a distinct entity, mirroring production rules: under
	// the token-scoped contract one entity has one outcome shared by every
	// rule claiming it, so mixed outcomes need distinct entities.
	rules := []analyzer.Rule{
		{
			Name:    "non-matching-rule",
			Matcher: newEntityMatcher(ctrl, pattern.EntityEmail, false),
		},
		{
			Name:    "matching-rule",
			Matcher: newEntityMatcher(ctrl, pattern.EntityCPF, true),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessEmptyRulesList(t *testing.T) {
	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{}, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessWithEmptyData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newConditionalMatcher(ctrl, []byte{}),
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte{})

	assert.True(t, found)
	assert.Equal(t, "rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessWithNilData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newNonMatchingMatcher(ctrl),
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, nil)

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessLargeRuleSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	// Create a large set of non-matching rules, each with a distinct entity
	// so every rule evaluates independently under the token-scoped contract.
	rules := make([]analyzer.Rule, 100)
	for i := 0; i < 99; i++ {
		rules[i] = analyzer.Rule{
			Name:    fmt.Sprintf("non-matching-rule-%d", i),
			Matcher: newEntityMatcher(ctrl, pattern.Entity(fmt.Sprintf("entity-%d", i)), false),
		}
	}
	// Add one matching rule at the end
	rules[99] = analyzer.Rule{
		Name:    "matching-rule",
		Matcher: newEntityMatcher(ctrl, pattern.EntityCPF, true),
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessPreservesRuleData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	originalRule := analyzer.Rule{
		Name:    "test-rule",
		Matcher: newMatchingMatcher(ctrl),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact: &analyzer.RedactSettings{
				Placeholder: "[REDACTED]",
			},
		},
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{originalRule}, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, originalRule.Name, matchedRule.Name)
	assert.Equal(t, originalRule.Settings.Strategy, matchedRule.Settings.Strategy)
	assert.Equal(t, originalRule.Settings.Redact.Placeholder, matchedRule.Settings.Redact.Placeholder)
}

func TestSerialRulesRunner_ProcessMultipleCallsIndependent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule1 := analyzer.Rule{
		Name:    "rule-1",
		Matcher: newMatchingMatcher(ctrl),
	}

	rule2 := analyzer.Rule{
		Name:    "rule-2",
		Matcher: newNonMatchingMatcher(ctrl),
	}

	// First call
	matched1, found1 := runner.Process(t.Context(), []analyzer.Rule{rule1}, []byte("data1"))
	assert.True(t, found1)
	assert.Equal(t, "rule-1", matched1.Name)

	// Second call should be independent
	matched2, found2 := runner.Process(t.Context(), []analyzer.Rule{rule2}, []byte("data2"))
	assert.False(t, found2)
	assert.Equal(t, "", matched2.Name)

	// Third call should still work
	matched3, found3 := runner.Process(t.Context(), []analyzer.Rule{rule1}, []byte("data3"))
	assert.True(t, found3)
	assert.Equal(t, "rule-1", matched3.Name)
}

func TestSerialRulesRunner_ProcessReturnsEmptyRuleOnNoMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rules := []analyzer.Rule{
		{
			Name:    "rule-1",
			Matcher: newNonMatchingMatcher(ctrl),
		},
		{
			Name:    "rule-2",
			Matcher: newNonMatchingMatcher(ctrl),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.False(t, found)
	// Rule should be empty struct
	assert.Equal(t, "", matchedRule.Name)
	assert.Nil(t, matchedRule.Matcher)
}

func TestSerialRulesRunner_ProcessDisabledRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rules := []analyzer.Rule{
		{
			Name:    "disabled-rule",
			Disable: true,
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "enabled-rule",
			Matcher: newMatchingMatcher(ctrl),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	// Should skip disabled rule and match the enabled one
	assert.Equal(t, "enabled-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessAllDisabledRules(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rules := []analyzer.Rule{
		{
			Name:    "disabled-rule-1",
			Disable: true,
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "disabled-rule-2",
			Disable: true,
			Matcher: newMatchingMatcher(ctrl),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessWithException(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	exceptionMatcher := newExceptionMatcher(ctrl, "safe@company.com")

	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: newMatchingMatcher(ctrl),
		Exceptions: []analyzer.Exception{
			{
				Reason:  "Whitelisted email",
				Matcher: exceptionMatcher,
			},
		},
	}

	// When data matches the exception, rule should be skipped
	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("safe@company.com"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessExceptionDoesNotMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	exceptionMatcher := newExceptionMatcher(ctrl, "safe@company.com")

	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: newMatchingMatcher(ctrl),
		Exceptions: []analyzer.Exception{
			{
				Reason:  "Whitelisted email",
				Matcher: exceptionMatcher,
			},
		},
	}

	// When data doesn't match the exception, rule should be matched
	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("other@domain.com"))

	assert.True(t, found)
	assert.Equal(t, "email-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessMultipleExceptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: newMatchingMatcher(ctrl),
		Exceptions: []analyzer.Exception{
			{
				Reason:  "Support email",
				Matcher: newExceptionMatcher(ctrl, "support@company.com"),
			},
			{
				Reason:  "Admin email",
				Matcher: newExceptionMatcher(ctrl, "admin@company.com"),
			},
		},
	}

	// Test first exception
	_, found1 := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("support@company.com"))
	assert.False(t, found1)

	// Test second exception
	_, found2 := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("admin@company.com"))
	assert.False(t, found2)

	// Test non-exception
	matched3, found3 := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("user@external.com"))
	assert.True(t, found3)
	assert.Equal(t, "email-rule", matched3.Name)
}

func TestSerialRulesRunner_ProcessDeterministicOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rules := []analyzer.Rule{
		{
			Name:    "rule-1",
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "rule-2",
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "rule-3",
			Matcher: newMatchingMatcher(ctrl),
		},
	}

	// Serial runner should always return the first matching rule in order
	for i := 0; i < 10; i++ {
		matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))
		assert.True(t, found)
		assert.Equal(t, "rule-1", matchedRule.Name)
	}
}

func TestSerialRulesRunner_Stop(_ *testing.T) {
	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	// Stop should be a no-op for serial runner
	runner.Stop()
	// If we get here without panic, the test passes
}

func TestSerialRulesRunner_ProcessWithContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	matchedRule, found := runner.Process(ctx, []analyzer.Rule{rule}, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessReturnsFirstMatchRegardlessOfOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	// Mix of matching and non-matching rules, each with a distinct entity so
	// every rule evaluates independently under the token-scoped contract.
	rules := []analyzer.Rule{
		{
			Name:    "non-matching-1",
			Matcher: newEntityMatcher(ctrl, pattern.EntityEmail, false),
		},
		{
			Name:    "non-matching-2",
			Matcher: newEntityMatcher(ctrl, pattern.EntityCPF, false),
		},
		{
			Name:    "matching-1",
			Matcher: newEntityMatcher(ctrl, pattern.EntityPhone, true),
		},
		{
			Name:    "matching-2",
			Matcher: newEntityMatcher(ctrl, pattern.EntityCreditCard, true),
		},
		{
			Name:    "non-matching-3",
			Matcher: newEntityMatcher(ctrl, pattern.EntityIPAddress, false),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	// Should return the first matching rule
	assert.Equal(t, "matching-1", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessDisabledAndEnabledMix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	// Each rule carries a distinct entity so the enabled rules evaluate
	// independently under the token-scoped contract.
	rules := []analyzer.Rule{
		{
			Name:    "disabled-matching-1",
			Disable: true,
			Matcher: newEntityMatcher(ctrl, pattern.EntityEmail, true),
		},
		{
			Name:    "non-matching",
			Matcher: newEntityMatcher(ctrl, pattern.EntityCPF, false),
		},
		{
			Name:    "disabled-matching-2",
			Disable: true,
			Matcher: newEntityMatcher(ctrl, pattern.EntityPhone, true),
		},
		{
			Name:    "enabled-matching",
			Matcher: newEntityMatcher(ctrl, pattern.EntityCreditCard, true),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	// Should skip all disabled rules and return the first enabled matching rule
	assert.Equal(t, "enabled-matching", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessExceptionAndDisabledCombined(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	exceptionMatcher := newExceptionMatcher(ctrl, "safe@company.com")

	rules := []analyzer.Rule{
		{
			Name:    "rule-with-exception",
			Matcher: newMatchingMatcher(ctrl),
			Exceptions: []analyzer.Exception{
				{
					Reason:  "Whitelisted",
					Matcher: exceptionMatcher,
				},
			},
		},
		{
			Name:    "disabled-rule",
			Disable: true,
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "normal-rule",
			Matcher: newMatchingMatcher(ctrl),
		},
	}

	// Data matches exception, so first rule is skipped
	matchedRule, found := runner.Process(t.Context(), rules, []byte("safe@company.com"))

	assert.True(t, found)
	// Should skip the first rule (due to exception), skip disabled rule, and match the normal rule
	assert.Equal(t, "normal-rule", matchedRule.Name)
}

// TestSerialRulesRunner_ConcurrentProcessCalls verifies that many goroutines
// calling Process simultaneously on the same SerialRulesRunner all receive
// correct results without data races.
func TestSerialRulesRunner_ConcurrentProcessCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "matching-rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	const goroutines = 50
	results := make([]bool, goroutines)

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("test data"))
			results[idx] = found
		}(i)
	}

	wg.Wait()

	for i := range goroutines {
		assert.True(t, results[i], "goroutine %d should have found a match", i)
	}
}

// TestSerialRulesRunner_ConcurrentMixedProcessCalls verifies that concurrent
// calls with both matching and non-matching rules each receive the correct
// independent result.
func TestSerialRulesRunner_ConcurrentMixedProcessCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	matchingRule := analyzer.Rule{Name: "matching", Matcher: newMatchingMatcher(ctrl)}
	nonMatchingRule := analyzer.Rule{Name: "non-matching", Matcher: newNonMatchingMatcher(ctrl)}

	const n = 40
	hitResults := make([]bool, n)
	missResults := make([]bool, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			_, found := runner.Process(context.Background(), []analyzer.Rule{matchingRule}, []byte("data"))
			hitResults[idx] = found
		}(i)
		go func(idx int) {
			defer wg.Done()
			_, found := runner.Process(context.Background(), []analyzer.Rule{nonMatchingRule}, []byte("data"))
			missResults[idx] = found
		}(i)
	}

	wg.Wait()

	for i := range n {
		assert.True(t, hitResults[i], "concurrent call %d with matching rule should find match", i)
		assert.False(t, missResults[i], "concurrent call %d with non-matching rule should not match", i)
	}
}

// ─── Token-scoped cache contract (spy-based) ─────────────────────────────────

// newEntityMatcher builds a mock matcher for the given entity whose Match
// always returns matched.
func newEntityMatcher(ctrl *gomock.Controller, entity pattern.Entity, matched bool) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(entity).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).Return(matched).AnyTimes()
	return mock
}

// newTokenScopedSerialRunner builds a SerialRulesRunner wired to the given
// cache spy, discarding all log output.
func newTokenScopedSerialRunner(options analyzer.RunnerOptions, cache analyzercache.CacheStore) analyzer.SerialRulesRunner {
	return analyzer.NewSerialRulesRuner(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		options,
		cache,
	)
}

// TestSerialRulesRunner_ProcessIssuesOneTokenScopedRead pins down the spy
// criterion "exactly one GetMatches and at most one SaveMatches per Process
// call": the runner reads the token's cached results once and persists the
// evaluated negatives in one batch; no legacy per-pair call is ever issued.
func TestSerialRulesRunner_ProcessIssuesOneTokenScopedRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []byte("test data")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(map[pattern.Entity]bool{}, nil).Times(1)
	cache.EXPECT().SaveMatches(gomock.Any(), data,
		map[pattern.Entity]bool{pattern.EntityEmail: false}, //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	).Return(nil).Times(1)

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{}, cache)

	rule := analyzer.Rule{
		Name:    "non-matching-rule",
		Matcher: newEntityMatcher(ctrl, pattern.EntityEmail, false),
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, data)

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

// TestSerialRulesRunner_ProcessCachedHitSkipsMatcher pins down that a cached
// hit never invokes that entity's matcher and persists nothing.
func TestSerialRulesRunner_ProcessCachedHitSkipsMatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []byte("cached data")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(map[pattern.Entity]bool{pattern.EntityEmail: true}, nil).Times(1) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	// No SaveMatches expectation: a cached hit evaluates and persists nothing.

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{}, cache)

	// No EXPECT for Match — if the matcher runs, gomock fails the test.
	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()

	rule := analyzer.Rule{Name: "cached-rule", Matcher: mockMatcher}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, data)

	assert.True(t, found)
	assert.Equal(t, "cached-rule", matchedRule.Name)
}

// TestSerialRulesRunner_ProcessCachedNegativeHitSkipsMatcherAndSave pins down
// that an entity already persisted as a negative is neither re-evaluated nor
// re-persisted.
func TestSerialRulesRunner_ProcessCachedNegativeHitSkipsMatcherAndSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []byte("cached data")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(map[pattern.Entity]bool{pattern.EntityEmail: false}, nil).Times(1) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	// No SaveMatches expectation: the negative is already persisted.

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{}, cache)

	// No EXPECT for Match — if the matcher runs, gomock fails the test.
	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()

	rule := analyzer.Rule{Name: "cached-rule", Matcher: mockMatcher}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, data)

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

// TestSerialRulesRunner_ProcessPartiallyCachedTokenEvaluatesOnlyAbsent pins
// down that a partially cached token evaluates only absent entities: the cached
// entity's matcher never runs, the absent entity is evaluated once and its
// negative is the only persisted field.
func TestSerialRulesRunner_ProcessPartiallyCachedTokenEvaluatesOnlyAbsent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []byte("partially cached data")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(map[pattern.Entity]bool{pattern.EntityEmail: false}, nil).Times(1) //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	cache.EXPECT().SaveMatches(gomock.Any(), data,
		map[pattern.Entity]bool{pattern.EntityCPF: false}, //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	).Return(nil).Times(1)

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{}, cache)

	// Cached entity: Entity only — if Match runs, gomock fails the test.
	cachedMatcher := analyzermock.NewMockMatcher(ctrl)
	cachedMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()

	// Absent entity: evaluated exactly once.
	absentMatcher := analyzermock.NewMockMatcher(ctrl)
	absentMatcher.EXPECT().Entity().Return(pattern.EntityCPF).AnyTimes()
	absentMatcher.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).Times(1)

	rules := []analyzer.Rule{
		{Name: "email-rule", Matcher: cachedMatcher},
		{Name: "cpf-rule", Matcher: absentMatcher},
	}

	matchedRule, found := runner.Process(t.Context(), rules, data)

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

// TestSerialRulesRunner_ProcessExceptedEntityNeverEvaluatedOrPersisted pins
// down that an excepted entity is never evaluated and never persisted.
func TestSerialRulesRunner_ProcessExceptedEntityNeverEvaluatedOrPersisted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []byte("safe@company.com")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(map[pattern.Entity]bool{}, nil).Times(1)
	// No SaveMatches expectation: the excepted entity is never persisted.

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{}, cache)

	// No EXPECT for Match — the excepted entity is never evaluated.
	mockMatcher := analyzermock.NewMockMatcher(ctrl)
	mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()

	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: mockMatcher,
		Exceptions: []analyzer.Exception{
			{
				Reason:  "Whitelisted email",
				Matcher: newExceptionMatcherForRunner(ctrl, "safe@company.com"),
			},
		},
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, data)

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

// TestSerialRulesRunner_ProcessPositiveMatchStillFlushesNegatives pins down
// that negatives accumulated before a positive match are still persisted in
// the one batched save, and the positive match itself is never cached.
func TestSerialRulesRunner_ProcessPositiveMatchStillFlushesNegatives(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []byte("matching data")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(map[pattern.Entity]bool{}, nil).Times(1)
	cache.EXPECT().SaveMatches(gomock.Any(), data,
		map[pattern.Entity]bool{pattern.EntityEmail: false}, //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	).Return(nil).Times(1)

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{}, cache)

	rules := []analyzer.Rule{
		{Name: "non-matching-rule", Matcher: newEntityMatcher(ctrl, pattern.EntityEmail, false)},
		{Name: "matching-rule", Matcher: newEntityMatcher(ctrl, pattern.EntityCPF, true)},
	}

	matchedRule, found := runner.Process(t.Context(), rules, data)

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

// TestSerialRulesRunner_ProcessBackendReadErrorDegradesToFullEvaluation pins
// down the "Backend error during read" scenario: the runner logs the error and
// treats every entity as a cache miss, proceeding with full evaluation — the
// matcher still runs and its negative result is still persisted.
func TestSerialRulesRunner_ProcessBackendReadErrorDegradesToFullEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []byte("test data")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(nil, assert.AnError).Times(1)
	cache.EXPECT().SaveMatches(gomock.Any(), data,
		map[pattern.Entity]bool{pattern.EntityEmail: false}, //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	).Return(nil).Times(1)

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{}, cache)

	// The matcher runs exactly once despite the read error: every entity is a
	// miss, so evaluation proceeds and the negative result is persisted.
	matcher := analyzermock.NewMockMatcher(ctrl)
	matcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	matcher.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).Times(1)

	rule := analyzer.Rule{
		Name:    "non-matching-rule",
		Matcher: matcher,
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, data)

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

// TestSerialRulesRunner_ProcessTokenCoalescingSharesEvaluationAndSave pins
// down the "Burst of concurrent misses on the same token" scenario with the
// cache spy: a burst of identical misses shares one evaluation and one batched
// save under the token-level flight.
func TestSerialRulesRunner_ProcessTokenCoalescingSharesEvaluationAndSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const callers = 10

	data := []byte("burst data")
	cache := cachemock.NewMockCacheStore(ctrl)
	cache.EXPECT().GetMatches(gomock.Any(), data).
		Return(map[pattern.Entity]bool{}, nil).Times(callers)
	cache.EXPECT().SaveMatches(gomock.Any(), data,
		map[pattern.Entity]bool{pattern.EntityEmail: false}, //nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses
	).Return(nil).Times(1)

	var computes atomic.Int32
	matcherEntered := make(chan struct{})
	releaseMatcher := make(chan struct{})
	burstMatcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		if computes.Add(1) == 1 {
			close(matcherEntered)
		}
		// Hold the flight open so every caller in the burst joins it before
		// the single computation completes.
		<-releaseMatcher
		return false
	}))
	rule := analyzer.Rule{Name: "burst-rule", Matcher: burstMatcher}

	runner := newTokenScopedSerialRunner(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{Enabled: true, SingleflightEnabled: true},
	}, cache)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			runner.Process(context.Background(), []analyzer.Rule{rule}, data)
		}()
	}

	// Release the whole burst at once. The first caller to arrive becomes the
	// flight leader and blocks in the matcher; give the rest time to join the
	// flight before the computation is released. The bounded wait keeps a
	// leader that never reaches the matcher (for example a spy failure) from
	// deadlocking the test.
	close(start)
	select {
	case <-matcherEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("the flight leader never entered the matcher; see cache spy failures above")
	}
	time.Sleep(50 * time.Millisecond)
	close(releaseMatcher)
	wg.Wait()

	assert.Equal(t, int32(1), computes.Load(), "one burst on the same token must evaluate the missing entity exactly once")
}

func TestSerialRulesRunner_ProcessMultipleCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{}, analyzercache.NewNoopRuleMatchingCache())

	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	// Make multiple sequential calls
	for i := 0; i < 5; i++ {
		matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte(fmt.Sprintf("data-%d", i)))
		assert.True(t, found)
		assert.Equal(t, "rule", matchedRule.Name)
	}
}
