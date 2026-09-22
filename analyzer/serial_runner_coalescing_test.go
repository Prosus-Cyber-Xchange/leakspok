package analyzer_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
)

// countingCache is a CacheStore test fake. GetMatch always reports a cache
// miss; SaveMatch records how many saves were issued and can be configured to
// fail with saveErr.
type countingCache struct {
	mu      sync.Mutex
	saves   int
	saveErr error
}

func (c *countingCache) GetMatch(_ context.Context, _ pattern.Entity, _ []byte) (bool, error) {
	return false, analyzercache.ErrCacheNotFound
}

func (c *countingCache) SaveMatch(_ context.Context, _ pattern.Entity, _ []byte, _ bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.saves++
	return c.saveErr
}

func (c *countingCache) saveCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saves
}

// cachedFalseCache is a CacheStore test fake whose GetMatch returns a cached
// false result, so the matcher must never run.
type cachedFalseCache struct{}

func (cachedFalseCache) GetMatch(_ context.Context, _ pattern.Entity, _ []byte) (bool, error) {
	return false, nil
}

func (cachedFalseCache) SaveMatch(_ context.Context, _ pattern.Entity, _ []byte, _ bool) error {
	return nil
}

// newSerialRunnerWithCache builds a SerialRulesRunner with the given options
// and cache, discarding all log output.
func newSerialRunnerWithCache(options analyzer.RunnerOptions, cache analyzercache.CacheStore) analyzer.SerialRulesRunner {
	return analyzer.NewSerialRulesRuner(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		options,
		cache,
	)
}

// countingMatcher returns a matcher for entity EntityEmail that counts how many
// times it computed and never matches.
func countingMatcher(computes *atomic.Int32) pattern.PatternMatcher {
	return pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		computes.Add(1)
		return false
	}))
}

func TestSerialRulesRunner_CoalescesConcurrentMisses(t *testing.T) {
	const callers = 20

	cache := &countingCache{}
	var computes atomic.Int32
	matcherEntered := make(chan struct{})
	releaseMatcher := make(chan struct{})
	burstMatcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		if computes.Add(1) == 1 {
			close(matcherEntered)
		}
		// Hold the flight open so every caller in the burst joins it before
		// the single computation completes; otherwise an instant matcher
		// closes the flight before the stragglers arrive and the burst
		// fragments into multiple flights.
		<-releaseMatcher
		return false
	}))
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: burstMatcher,
	}
	runner := newSerialRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{SingleflightEnabled: true},
	}, cache)

	data := []byte("identical data")

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan bool, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, found := runner.Process(context.Background(), []analyzer.Rule{rule}, data)
			results <- found
		}()
	}

	// Release the whole burst at once. The first caller to arrive becomes the
	// flight leader and blocks in the matcher; give the rest time to join the
	// flight before the computation is released.
	close(start)
	<-matcherEntered
	time.Sleep(50 * time.Millisecond)
	close(releaseMatcher)

	wg.Wait()
	close(results)

	for found := range results {
		assert.False(t, found, "matcher never matches, so Process must report no match")
	}
	assert.Equal(t, int32(1), computes.Load(), "one burst of identical misses must compute exactly once")
	assert.Equal(t, 1, cache.saveCount(), "one burst of identical misses must save exactly once")
}

func TestSerialRulesRunner_CoalescingDoesNotMemoizeAcrossBursts(t *testing.T) {
	cache := &countingCache{}
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newSerialRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{SingleflightEnabled: true},
	}, cache)

	data := []byte("identical data")

	_, found := runner.Process(context.Background(), []analyzer.Rule{rule}, data)
	assert.False(t, found)

	_, found = runner.Process(context.Background(), []analyzer.Rule{rule}, data)
	assert.False(t, found)

	assert.Equal(t, int32(2), computes.Load(), "sequential bursts must recompute, never replay a memoized result")
	assert.Equal(t, 2, cache.saveCount())
}

func TestSerialRulesRunner_CoalescingDisabledComputesPerCall(t *testing.T) {
	const callers = 20

	cache := &countingCache{}
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newSerialRunnerWithCache(analyzer.RunnerOptions{}, cache)

	data := []byte("identical data")
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner.Process(context.Background(), []analyzer.Rule{rule}, data)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(callers), computes.Load(), "with coalescing disabled each caller must compute")
	assert.Equal(t, callers, cache.saveCount(), "with coalescing disabled each caller must save")
}

func TestSerialRulesRunner_CoalescingCachedHitSkipsMatcher(t *testing.T) {
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newSerialRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{SingleflightEnabled: true},
	}, cachedFalseCache{})

	matched, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("cached data"))

	assert.False(t, found)
	assert.Equal(t, analyzer.Rule{}, matched)
	assert.Equal(t, int32(0), computes.Load(), "a cached hit must never reach the matcher")
}

func TestSerialRulesRunner_CoalescingCancelledWaiterSkipsCompute(t *testing.T) {
	cache := &countingCache{}

	var computes atomic.Int32
	matcherEntered := make(chan struct{})
	releaseMatcher := make(chan struct{})
	blockingMatcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		if computes.Add(1) == 1 {
			close(matcherEntered)
		}
		<-releaseMatcher
		return false
	}))
	rule := analyzer.Rule{
		Name:    "blocking-rule",
		Matcher: blockingMatcher,
	}
	runner := newSerialRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{SingleflightEnabled: true},
	}, cache)

	data := []byte("identical data")
	leaderResult := make(chan bool, 1)
	go func() {
		_, found := runner.Process(context.Background(), []analyzer.Rule{rule}, data)
		leaderResult <- found
	}()

	// Wait for the leader to enter the matcher: the flight for this key exists.
	<-matcherEntered

	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	waiterResult := make(chan bool, 1)
	waiterStarted := make(chan struct{})
	go func() {
		close(waiterStarted)
		_, found := runner.Process(waiterCtx, []analyzer.Rule{rule}, data)
		waiterResult <- found
	}()

	// Let the waiter join the flight, then cancel its context.
	<-waiterStarted
	time.Sleep(50 * time.Millisecond)
	cancelWaiter()

	select {
	case found := <-waiterResult:
		assert.False(t, found, "a cancelled waiter must report no match")
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled waiter did not unblock promptly")
	}

	// The leader's computation still completes exactly once.
	close(releaseMatcher)

	select {
	case found := <-leaderResult:
		assert.False(t, found, "the leader must report no match")
	case <-time.After(2 * time.Second):
		t.Fatal("leader did not complete after the computation was released")
	}

	assert.Equal(t, int32(1), computes.Load(), "the shared computation must run exactly once")
	assert.Equal(t, 1, cache.saveCount(), "the shared computation must save exactly once")
}

// TestSerialRulesRunner_CoalescingSaveErrorIsLoggedAndContinues pins down that a
// failed negative-result save inside the flight is surfaced to every caller as
// a no-match, following the log-and-continue convention.
func TestSerialRulesRunner_CoalescingSaveErrorIsLoggedAndContinues(t *testing.T) {
	saveErr := assert.AnError
	cache := &countingCache{saveErr: saveErr}
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newSerialRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{SingleflightEnabled: true},
	}, cache)

	matched, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("data"))

	assert.False(t, found)
	assert.Equal(t, analyzer.Rule{}, matched)
	assert.Equal(t, int32(1), computes.Load())
	assert.Equal(t, 1, cache.saveCount())
}
