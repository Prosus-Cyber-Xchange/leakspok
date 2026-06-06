package analyzer_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ifood/leakspok/analyzer"
	analyzercache "github.com/ifood/leakspok/analyzer/cache"
	analyzermock "github.com/ifood/leakspok/analyzer/mocks"
	"github.com/ifood/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// newConcurrentRunner creates a ConcurrentRulesRunner for unit testing with the given pool size.
func newConcurrentRunner(workers int) *analyzer.ConcurrentRulesRunner {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, err := analyzer.NewAntsWorkerPool(workers, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to create worker pool: %v", err))
	}

	return analyzer.NewConcurrentRulesRunner(
		logger,
		analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       workers,
			},
		},
		analyzercache.NewNoopRuleMatchingCache(),
		pool,
	)
}

// newSlowMatcher returns a mock matcher that signals on `called` when Match is invoked,
// then blocks until `unblock` is closed, then returns `result`.
func newSlowMatcher(ctrl *gomock.Controller, called chan<- struct{}, unblock <-chan struct{}, result bool) *analyzermock.MockMatcher {
	m := analyzermock.NewMockMatcher(ctrl)
	m.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	m.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ []byte) bool {
		select {
		case called <- struct{}{}:
		default:
		}
		<-unblock
		return result
	}).AnyTimes()

	return m
}

// ─── Standard Functional Tests ───────────────────────────────────────────────

func TestConcurrentRulesRunner_ProcessSingleMatchingRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rule := analyzer.Rule{Name: "matching-rule", Matcher: newMatchingMatcher(ctrl)}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessNoMatchingRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rule := analyzer.Rule{Name: "non-matching-rule", Matcher: newNonMatchingMatcher(ctrl)}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessEmptyRulesList(t *testing.T) {
	runner := newConcurrentRunner(4)
	defer runner.Stop()

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{}, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessWithNilData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rule := analyzer.Rule{Name: "rule", Matcher: newNonMatchingMatcher(ctrl)}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, nil)

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessAllDisabledRules(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rules := []analyzer.Rule{
		{Name: "disabled-1", Disable: true, Matcher: newMatchingMatcher(ctrl)},
		{Name: "disabled-2", Disable: true, Matcher: newMatchingMatcher(ctrl)},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessDisabledRulesAreSkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rules := []analyzer.Rule{
		{Name: "disabled", Disable: true, Matcher: newMatchingMatcher(ctrl)},
		{Name: "enabled", Matcher: newMatchingMatcher(ctrl)},
	}

	_, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found, "enabled rule should still be matched")
}

func TestConcurrentRulesRunner_ProcessWithException(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: newMatchingMatcher(ctrl),
		Exceptions: []analyzer.Exception{
			{Reason: "Whitelisted", Matcher: newExceptionMatcherForRunner(ctrl, "safe@company.com")},
		},
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("safe@company.com"))

	assert.False(t, found, "exception should prevent the rule from matching")
	assert.Equal(t, "", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessExceptionDoesNotMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: newMatchingMatcher(ctrl),
		Exceptions: []analyzer.Exception{
			{Reason: "Whitelisted", Matcher: newExceptionMatcherForRunner(ctrl, "safe@company.com")},
		},
	}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("other@domain.com"))

	assert.True(t, found)
	assert.Equal(t, "email-rule", matchedRule.Name)
}

func TestConcurrentRulesRunner_ProcessPreservesRuleData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	original := analyzer.Rule{
		Name:    "test-rule",
		Matcher: newMatchingMatcher(ctrl),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact:   &analyzer.RedactSettings{Placeholder: "[REDACTED]"},
		},
	}

	matched, found := runner.Process(t.Context(), []analyzer.Rule{original}, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, original.Name, matched.Name)
	assert.Equal(t, original.Settings.Strategy, matched.Settings.Strategy)
	assert.Equal(t, original.Settings.Redact.Placeholder, matched.Settings.Redact.Placeholder)
}

func TestConcurrentRulesRunner_ProcessReturnsEmptyRuleOnNoMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rules := []analyzer.Rule{
		{Name: "rule-1", Matcher: newNonMatchingMatcher(ctrl)},
		{Name: "rule-2", Matcher: newNonMatchingMatcher(ctrl)},
	}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, "", matchedRule.Name)
	assert.Nil(t, matchedRule.Matcher)
}

func TestConcurrentRulesRunner_ProcessLargeRuleSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(8)
	defer runner.Stop()

	rules := make([]analyzer.Rule, 100)
	for i := range 99 {
		rules[i] = analyzer.Rule{
			Name:    fmt.Sprintf("non-matching-%d", i),
			Matcher: newNonMatchingMatcher(ctrl),
		}
	}
	rules[99] = analyzer.Rule{Name: "matching-rule", Matcher: newMatchingMatcher(ctrl)}

	matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
}

func TestConcurrentRulesRunner_DifferentWorkerCounts(t *testing.T) {
	for _, workers := range []int{1, 2, 4, 8} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			runner := newConcurrentRunner(workers)
			defer runner.Stop()

			rules := make([]analyzer.Rule, 20)
			for i := range 19 {
				rules[i] = analyzer.Rule{
					Name:    fmt.Sprintf("non-matching-%d", i),
					Matcher: newNonMatchingMatcher(ctrl),
				}
			}
			rules[19] = analyzer.Rule{Name: "target", Matcher: newMatchingMatcher(ctrl)}

			matchedRule, found := runner.Process(t.Context(), rules, []byte("test data"))

			assert.True(t, found)
			assert.Equal(t, "target", matchedRule.Name)
		})
	}
}

// ─── Concurrent-Specific Tests ────────────────────────────────────────────────

// TestConcurrentRulesRunner_Stop_IsIdempotent verifies that calling Stop multiple
// times sequentially does not panic (sync.Once protection).
func TestConcurrentRulesRunner_Stop_IsIdempotent(_ *testing.T) {
	runner := newConcurrentRunner(4)

	for range 10 {
		runner.Stop() // must not panic
	}
}

// TestConcurrentRulesRunner_Stop_IsIdempotentConcurrently verifies that calling Stop
// from many goroutines simultaneously does not cause a double-close panic.
func TestConcurrentRulesRunner_Stop_IsIdempotentConcurrently(_ *testing.T) {
	runner := newConcurrentRunner(4)

	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner.Stop()
		}()
	}

	wg.Wait()
}

// TestConcurrentRulesRunner_ProcessAfterStop verifies that Process returns (Rule{}, false)
// immediately after Stop has been called, without attempting to use the worker pool.
func TestConcurrentRulesRunner_ProcessAfterStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	runner.Stop()

	// The matcher must NOT be called — workers are gone.
	rule := analyzer.Rule{Name: "matching-rule", Matcher: newMatchingMatcher(ctrl)}

	matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))

	assert.False(t, found, "Process must return false after Stop")
	assert.Equal(t, "", matchedRule.Name)
}

// TestConcurrentRulesRunner_ProcessMultipleCallsReusesWorkerPool verifies that the
// lazy-initialized worker pool is reused across many sequential Process calls.
func TestConcurrentRulesRunner_ProcessMultipleCallsReusesWorkerPool(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	rule := analyzer.Rule{Name: "rule", Matcher: newMatchingMatcher(ctrl)}

	for i := range 30 {
		matchedRule, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte(fmt.Sprintf("data-%d", i)))
		assert.True(t, found, "call %d should find a match", i)
		assert.Equal(t, "rule", matchedRule.Name, "call %d should return the correct rule", i)
	}
}

// TestConcurrentRulesRunner_ConcurrentProcessCalls verifies that many goroutines
// calling Process simultaneously all receive correct results with no data races.
func TestConcurrentRulesRunner_ConcurrentProcessCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(8)
	defer runner.Stop()

	rule := analyzer.Rule{Name: "matching-rule", Matcher: newMatchingMatcher(ctrl)}

	const goroutines = 50
	results := make([]bool, goroutines)
	var wg sync.WaitGroup

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, found := runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("test data"))
			results[idx] = found
		}(i)
	}

	wg.Wait()

	for i, found := range results {
		assert.True(t, found, "goroutine %d should have found a match", i)
	}
}

// TestConcurrentRulesRunner_ConcurrentMixedResults verifies that concurrent calls with
// both matching and non-matching rules each receive the correct independent result.
func TestConcurrentRulesRunner_ConcurrentMixedResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

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
			_, found := runner.Process(t.Context(), []analyzer.Rule{matchingRule}, []byte("data"))
			hitResults[idx] = found
		}(i)
		go func(idx int) {
			defer wg.Done()
			_, found := runner.Process(t.Context(), []analyzer.Rule{nonMatchingRule}, []byte("data"))
			missResults[idx] = found
		}(i)
	}

	wg.Wait()

	for i := range n {
		assert.True(t, hitResults[i], "concurrent call %d with matching rule should find match", i)
		assert.False(t, missResults[i], "concurrent call %d with non-matching rule should not match", i)
	}
}

// TestConcurrentRulesRunner_ContextCancellation verifies that cancelling the caller's
// context causes Process to return promptly even when workers are still in flight.
func TestConcurrentRulesRunner_ContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)
	defer runner.Stop()

	called := make(chan struct{}, 1)
	unblock := make(chan struct{})

	rule := analyzer.Rule{
		Name:    "slow-rule",
		Matcher: newSlowMatcher(ctrl, called, unblock, false),
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{})
	var matched bool
	go func() {
		defer close(done)
		_, matched = runner.Process(ctx, []analyzer.Rule{rule}, []byte("data"))
	}()

	// Ensure the matcher is actively blocking before cancelling.
	select {
	case <-called:
	case <-t.Context().Done():
		close(unblock)
		t.Fatal("slow matcher was never called")
	}

	cancel() // cancel the caller's context

	// Process must return promptly after cancellation.
	select {
	case <-done:
	case <-t.Context().Done():
		close(unblock)
		t.Fatal("Process did not return after context cancellation")
	}

	assert.False(t, matched)

	close(unblock) // let the blocked goroutine in the worker exit cleanly
}

// TestConcurrentRulesRunner_StopWhileProcessRunning verifies that calling Stop while
// Process has work in flight causes Process to return (Rule{}, false) without hanging.
func TestConcurrentRulesRunner_StopWhileProcessRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)

	called := make(chan struct{}, 1)
	unblock := make(chan struct{})

	rule := analyzer.Rule{
		Name:    "slow-rule",
		Matcher: newSlowMatcher(ctrl, called, unblock, false),
	}

	done := make(chan struct{})
	var found bool
	go func() {
		defer close(done)
		_, found = runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("data"))
	}()

	// Wait until work is actually in flight (matcher has been invoked).
	select {
	case <-called:
	case <-t.Context().Done():
		close(unblock)
		t.Fatal("slow matcher was never called")
	}

	// Stop the runner while the matcher is still blocking.
	runner.Stop()

	// Unblock the matcher so the worker goroutine can exit cleanly.
	close(unblock)

	// Process must return (found=false) after Stop.
	select {
	case <-done:
	case <-t.Context().Done():
		t.Fatal("Process did not return after Stop")
	}

	assert.False(t, found, "Process should return false after Stop")
}

// TestConcurrentRulesRunner_StopAndProcessRace exercises the Stop/Process boundary
// from many goroutines concurrently to ensure no panic or deadlock occurs.
func TestConcurrentRulesRunner_StopAndProcessRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runner := newConcurrentRunner(4)

	rule := analyzer.Rule{Name: "rule", Matcher: newMatchingMatcher(ctrl)}

	var wg sync.WaitGroup

	// Spawn goroutines that keep calling Process.
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Result may be true or false depending on whether Stop raced ahead.
			runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("data"))
		}()
	}

	// Stop concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runner.Stop()
	}()

	wg.Wait()
}

// TestConcurrentRulesRunner_Stop_NoGoroutineLeak verifies that all worker goroutines
// exit after Stop is called, preventing goroutine leaks.
func TestConcurrentRulesRunner_Stop_NoGoroutineLeak(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Capture baseline goroutine count.
	runtime.GC()
	baseline := runtime.NumGoroutine()

	const workers = 8
	runner := newConcurrentRunner(workers)

	rule := analyzer.Rule{Name: "rule", Matcher: newMatchingMatcher(ctrl)}

	// Exercise the runner to ensure workers are started.
	for range 10 {
		_, _ = runner.Process(t.Context(), []analyzer.Rule{rule}, []byte("data"))
	}

	runner.Stop()

	// Wait for goroutines to exit. Workers should exit promptly after the pool is released.
	assert.Eventually(t, func() bool {
		runtime.GC()
		return runtime.NumGoroutine() <= baseline+1
	}, 2*time.Second, 10*time.Millisecond,
		"goroutine count should return to near baseline after Stop; leaked workers detected",
	)
}
