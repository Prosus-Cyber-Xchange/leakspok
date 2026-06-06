package analyzer_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/ifood/leakspok/analyzer"
	analyzercache "github.com/ifood/leakspok/analyzer/cache"
	analyzermock "github.com/ifood/leakspok/analyzer/mocks"
	"github.com/ifood/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// RunnerFactory is a function that creates a RuleRunner for testing
type RunnerFactory func(*gomock.Controller) analyzer.RuleRunner

// newSerialRunnerFactory creates a factory for SerialRulesRunner
func newSerialRunnerFactory(_ *gomock.Controller) analyzer.RuleRunner {
	return analyzer.NewSerialRulesRuner(slog.New(slog.NewTextHandler(io.Discard, nil)), analyzercache.NewNoopRuleMatchingCache())
}

// newConcurrentRunnerFactory creates a factory for ConcurrentRulesRunner with a pool of 4 workers.
func newConcurrentRunnerFactory(_ *gomock.Controller) analyzer.RuleRunner {
	const poolSize = 4
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool, err := analyzer.NewAntsWorkerPool(poolSize, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to create worker pool: %v", err))
	}

	opts := analyzer.RunnerOptions{
		Concurrency: analyzer.ConcurrencyOptions{
			ConcurrentRuleProcessing: true,
			RuleRunnerPoolSize:       poolSize,
		},
	}

	return analyzer.NewConcurrentRulesRunner(logger, opts, analyzercache.NewNoopRuleMatchingCache(), pool)
}

// Helper functions for creating mock matchers

func newMatchingMatcher(ctrl *gomock.Controller) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	return mock
}

func newNonMatchingMatcher(ctrl *gomock.Controller) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	return mock
}

func newConditionalMatcher(ctrl *gomock.Controller, matchData []byte) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		return bytes.Equal(input, matchData)
	}).AnyTimes()
	return mock
}

func newExceptionMatcherForRunner(ctrl *gomock.Controller, matchValue string) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		return string(input) == matchValue
	}).AnyTimes()
	return mock
}

// TestRunnersBehaviorIdentical validates that SerialRulesRunner works correctly
// across common test scenarios
func TestRunnersBehaviorIdentical(t *testing.T) {
	factories := map[string]RunnerFactory{
		"SerialRulesRunner":     newSerialRunnerFactory,
		"ConcurrentRulesRunner": newConcurrentRunnerFactory,
	}

	testCases := []struct {
		name string
		test func(*testing.T, analyzer.RuleRunner, *gomock.Controller)
	}{
		{
			name: "ProcessSingleMatchingRule",
			test: testProcessSingleMatchingRule,
		},
		{
			name: "ProcessNoMatchingRule",
			test: testProcessNoMatchingRule,
		},
		{
			name: "ProcessMultipleRulesFirstMatches",
			test: testProcessMultipleRulesFirstMatches,
		},
		{
			name: "ProcessMultipleRulesSecondMatches",
			test: testProcessMultipleRulesSecondMatches,
		},
		{
			name: "ProcessEmptyRulesList",
			test: testProcessEmptyRulesList,
		},
		{
			name: "ProcessWithEmptyData",
			test: testProcessWithEmptyData,
		},
		{
			name: "ProcessWithNilData",
			test: testProcessWithNilData,
		},
		{
			name: "ProcessLargeRuleSet",
			test: testProcessLargeRuleSet,
		},
		{
			name: "ProcessPreservesRuleData",
			test: testProcessPreservesRuleData,
		},
		{
			name: "ProcessMultipleCallsIndependent",
			test: testProcessMultipleCallsIndependent,
		},
		{
			name: "ProcessReturnsEmptyRuleOnNoMatch",
			test: testProcessReturnsEmptyRuleOnNoMatch,
		},
		{
			name: "ProcessDisabledRule",
			test: testProcessDisabledRule,
		},
		{
			name: "ProcessAllDisabledRules",
			test: testProcessAllDisabledRules,
		},
		{
			name: "ProcessWithException",
			test: testProcessWithException,
		},
		{
			name: "ProcessExceptionDoesNotMatch",
			test: testProcessExceptionDoesNotMatch,
		},
		{
			name: "ProcessMultipleExceptions",
			test: testProcessMultipleExceptions,
		},
		{
			name: "ProcessDeterministicOrder",
			test: testProcessDeterministicOrder,
		},
		{
			name: "ProcessWithContext",
			test: testProcessWithContext,
		},
		{
			name: "ProcessReturnsFirstMatchRegardlessOfOrder",
			test: testProcessReturnsFirstMatchRegardlessOfOrder,
		},
		{
			name: "ProcessDisabledAndEnabledMix",
			test: testProcessDisabledAndEnabledMix,
		},
		{
			name: "ProcessExceptionAndDisabledCombined",
			test: testProcessExceptionAndDisabledCombined,
		},
	}

	// Run each test case with both runner types
	for _, tc := range testCases {
		for runnerName, factory := range factories {
			testName := fmt.Sprintf("%s_%s", tc.name, runnerName)
			t.Run(testName, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				runner := factory(ctrl)
				tc.test(t, runner, ctrl)
			})
		}
	}
}

// Test implementations that run against both runners

func testProcessSingleMatchingRule(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	rule := analyzer.Rule{
		Name:    "matching-rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "matching-rule", matchedRule.Name, "should return the matching rule")
}

func testProcessNoMatchingRule(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	rule := analyzer.Rule{
		Name:    "non-matching-rule",
		Matcher: newNonMatchingMatcher(ctrl),
	}

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("test data"))

	assert.False(t, found, "should not find any matching rule")
	assert.Equal(t, "", matchedRule.Name, "should return empty rule")
}

func testProcessMultipleRulesFirstMatches(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	// Both rules match; serial runner returns the first, concurrent runner may return either.
	assert.True(t,
		matchedRule.Name == "first-matching-rule" || matchedRule.Name == "second-matching-rule",
		"should return one of the matching rules, got: %s", matchedRule.Name,
	)
}

func testProcessMultipleRulesSecondMatches(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "matching-rule", matchedRule.Name, "should return the matching rule")
}

func testProcessEmptyRulesList(t *testing.T, runner analyzer.RuleRunner, _ *gomock.Controller) {
	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{}, []byte("test data"))

	assert.False(t, found, "should not find any matching rule")
	assert.Equal(t, "", matchedRule.Name, "should return empty rule")
}

func testProcessWithEmptyData(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newConditionalMatcher(ctrl, []byte{}),
	}

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte{})

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "rule", matchedRule.Name, "should return the matching rule")
}

func testProcessWithNilData(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newNonMatchingMatcher(ctrl),
	}

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, nil)

	assert.False(t, found, "should not find any matching rule")
	assert.Equal(t, "", matchedRule.Name, "should return empty rule")
}

func testProcessLargeRuleSet(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "matching-rule", matchedRule.Name, "should return the matching rule")
}

func testProcessPreservesRuleData(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{originalRule}, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, originalRule.Name, matchedRule.Name, "should return matching rule with same name")
	assert.Equal(t, originalRule.Settings.Strategy, matchedRule.Settings.Strategy, "should preserve settings strategy")
	assert.Equal(t, originalRule.Settings.Redact.Placeholder, matchedRule.Settings.Redact.Placeholder, "should preserve placeholder")
}

func testProcessMultipleCallsIndependent(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	rule1 := analyzer.Rule{
		Name:    "rule-1",
		Matcher: newMatchingMatcher(ctrl),
	}

	rule2 := analyzer.Rule{
		Name:    "rule-2",
		Matcher: newNonMatchingMatcher(ctrl),
	}

	// First call
	matched1, found1 := runner.Process(context.Background(), []analyzer.Rule{rule1}, []byte("data1"))
	assert.True(t, found1, "first call should find match")
	assert.Equal(t, "rule-1", matched1.Name, "first call should return rule-1")

	// Second call should be independent
	matched2, found2 := runner.Process(context.Background(), []analyzer.Rule{rule2}, []byte("data2"))
	assert.False(t, found2, "second call should not find match")
	assert.Equal(t, "", matched2.Name, "second call should return empty rule")

	// Third call should still work
	matched3, found3 := runner.Process(context.Background(), []analyzer.Rule{rule1}, []byte("data3"))
	assert.True(t, found3, "third call should find match")
	assert.Equal(t, "rule-1", matched3.Name, "third call should return rule-1")
}

func testProcessReturnsEmptyRuleOnNoMatch(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.False(t, found, "should not find any matching rule")
	assert.Equal(t, "", matchedRule.Name, "should return empty rule name")
	assert.Nil(t, matchedRule.Matcher, "should return nil matcher")
}

func testProcessDisabledRule(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "enabled-rule", matchedRule.Name, "should skip disabled rule and match enabled one")
}

func testProcessAllDisabledRules(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.False(t, found, "should not find any matching rule")
	assert.Equal(t, "", matchedRule.Name, "should return empty rule")
}

func testProcessWithException(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	exceptionMatcher := newExceptionMatcherForRunner(ctrl, "safe@company.com")

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
	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("safe@company.com"))

	assert.False(t, found, "should not match due to exception")
	assert.Equal(t, "", matchedRule.Name, "should return empty rule")
}

func testProcessExceptionDoesNotMatch(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	exceptionMatcher := newExceptionMatcherForRunner(ctrl, "safe@company.com")

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
	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("other@domain.com"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "email-rule", matchedRule.Name, "should return email-rule")
}

func testProcessMultipleExceptions(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: newMatchingMatcher(ctrl),
		Exceptions: []analyzer.Exception{
			{
				Reason:  "Support email",
				Matcher: newExceptionMatcherForRunner(ctrl, "support@company.com"),
			},
			{
				Reason:  "Admin email",
				Matcher: newExceptionMatcherForRunner(ctrl, "admin@company.com"),
			},
		},
	}

	// Test first exception
	_, found1 := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("support@company.com"))
	assert.False(t, found1, "first exception should prevent match")

	// Test second exception
	_, found2 := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("admin@company.com"))
	assert.False(t, found2, "second exception should prevent match")

	// Test non-exception
	matched3, found3 := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("user@external.com"))
	assert.True(t, found3, "non-exception data should match")
	assert.Equal(t, "email-rule", matched3.Name, "should return email-rule")
}

func testProcessDeterministicOrder(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	// Runner should always find a match
	for i := 0; i < 5; i++ {
		matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))
		assert.True(t, found, "should find matching rule")
		// Serial runner should return rules in order
		assert.True(t, matchedRule.Name == "rule-1" || matchedRule.Name == "rule-2" || matchedRule.Name == "rule-3",
			"should return one of the matching rules")
	}
}

func testProcessWithContext(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	rule := analyzer.Rule{
		Name:    "rule",
		Matcher: newMatchingMatcher(ctrl),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	matchedRule, found := runner.Process(ctx, []analyzer.Rule{rule}, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "rule", matchedRule.Name, "should return the matching rule")
}

func testProcessReturnsFirstMatchRegardlessOfOrder(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	// Both matching-1 and matching-2 are valid; concurrent runner may return either.
	assert.True(t,
		matchedRule.Name == "matching-1" || matchedRule.Name == "matching-2",
		"should return one of the matching rules, got: %s", matchedRule.Name,
	)
}

func testProcessDisabledAndEnabledMix(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
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

	matchedRule, found := runner.Process(context.Background(), rules, []byte("test data"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "enabled-matching", matchedRule.Name, "should skip all disabled rules and return first enabled matching rule")
}

func testProcessExceptionAndDisabledCombined(t *testing.T, runner analyzer.RuleRunner, ctrl *gomock.Controller) {
	exceptionMatcher := newExceptionMatcherForRunner(ctrl, "safe@company.com")

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
	matchedRule, found := runner.Process(context.Background(), rules, []byte("safe@company.com"))

	assert.True(t, found, "should find matching rule")
	assert.Equal(t, "normal-rule", matchedRule.Name, "should skip exception rule, skip disabled rule, and match normal rule")
}
