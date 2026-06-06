package analyzer_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSerialRulesRunner_ProcessSingleMatchingRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

	rules := []analyzer.Rule{
		{
			Name:    "non-matching-rule",
			Matcher: newNonMatchingMatcher(ctrl),
		},
		{
			Name:    "matching-rule",
			Matcher: newMatchingMatcher(ctrl),
		},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessEmptyRulesList(t *testing.T) {
	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{}, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessWithEmptyData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

	// Create a large set of non-matching rules
	rules := make([]analyzer.Rule, 100)
	for i := 0; i < 99; i++ {
		rules[i] = analyzer.Rule{
			Name:    fmt.Sprintf("non-matching-rule-%d", i),
			Matcher: newNonMatchingMatcher(ctrl),
		}
	}
	// Add one matching rule at the end
	rules[99] = analyzer.Rule{
		Name:    "matching-rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

func TestSerialRulesRunner_ProcessPreservesRuleData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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
	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

	// Stop should be a no-op for serial runner
	runner.Stop()
	// If we get here without panic, the test passes
}

func TestSerialRulesRunner_ProcessWithContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

	// Mix of matching and non-matching rules
	rules := []analyzer.Rule{
		{
			Name:    "non-matching-1",
			Matcher: newNonMatchingMatcher(ctrl),
		},
		{
			Name:    "non-matching-2",
			Matcher: newNonMatchingMatcher(ctrl),
		},
		{
			Name:    "matching-1",
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "matching-2",
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "non-matching-3",
			Matcher: newNonMatchingMatcher(ctrl),
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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

	rules := []analyzer.Rule{
		{
			Name:    "disabled-matching-1",
			Disable: true,
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "non-matching",
			Matcher: newNonMatchingMatcher(ctrl),
		},
		{
			Name:    "disabled-matching-2",
			Disable: true,
			Matcher: newMatchingMatcher(ctrl),
		},
		{
			Name:    "enabled-matching",
			Matcher: newMatchingMatcher(ctrl),
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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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

func TestSerialRulesRunner_ProcessMultipleCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())

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
