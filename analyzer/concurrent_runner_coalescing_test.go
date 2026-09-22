package analyzer_test

import (
	"bytes"
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
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
)

// newConcurrentRunnerWithCache builds a ConcurrentRulesRunner with the given
// options and cache, backed by a pool large enough to hold poolSize concurrent
// tasks, discarding all log output.
func newConcurrentRunnerWithCache(options analyzer.RunnerOptions, cache analyzercache.CacheStore, poolSize int) *analyzer.ConcurrentRulesRunner {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, err := analyzer.NewAntsWorkerPool(poolSize, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to create worker pool: %v", err))
	}

	return analyzer.NewConcurrentRulesRunner(logger, options, cache, pool)
}

// TestConcurrentRulesRunner_CoalescesConcurrentMisses pins down the requirement
// scenario "burst of identical concurrent misses": a burst of identical misses
// across Process calls must compute once and save once.
func TestConcurrentRulesRunner_CoalescesConcurrentMisses(t *testing.T) {
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
	// The coalescer is gated on Cache.Enabled too (it is a cache-miss
	// behavior), so coalescing engages only when Enabled is true.
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{Enabled: true, SingleflightEnabled: true},
	}, cache, callers)
	defer runner.Stop()

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

// TestConcurrentRulesRunner_CoalescingDisabledComputesPerCall pins down the
// requirement scenario "coalescing disabled": with zero-value options each miss
// computes and saves independently.
func TestConcurrentRulesRunner_CoalescingDisabledComputesPerCall(t *testing.T) {
	const callers = 20

	cache := &countingCache{}
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, callers)

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
	runner.Stop()

	assert.Equal(t, int32(callers), computes.Load(), "with coalescing disabled each caller must compute")
	assert.Equal(t, callers, cache.saveCount(), "with coalescing disabled each caller must save")
}

// TestConcurrentRulesRunner_CoalescingDifferentKeysIndependent pins down that
// the flight key includes the data: bursts on different data keys do not
// share a flight, and each key computes and saves exactly once.
func TestConcurrentRulesRunner_CoalescingDifferentKeysIndependent(t *testing.T) {
	const callersPerKey = 8

	cache := &countingCache{}
	dataA := []byte("data key A")
	dataB := []byte("data key B")

	var computes atomic.Int32
	firstAEntered := make(chan struct{})
	firstBEntered := make(chan struct{})
	releaseMatcher := make(chan struct{})
	var onceA, onceB sync.Once
	matcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, data []byte) bool {
		computes.Add(1)
		if bytes.Equal(data, dataA) {
			onceA.Do(func() { close(firstAEntered) })
		} else {
			onceB.Do(func() { close(firstBEntered) })
		}
		<-releaseMatcher
		return false
	}))
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: matcher,
	}

	poolSize := 2 * callersPerKey
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{Enabled: true, SingleflightEnabled: true},
	}, cache, poolSize)
	defer runner.Stop()

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range callersPerKey {
		for _, data := range [][]byte{dataA, dataB} {
			wg.Add(1)
			go func(data []byte) {
				defer wg.Done()
				<-start
				_, found := runner.Process(context.Background(), []analyzer.Rule{rule}, data)
				assert.False(t, found, "matcher never matches, so Process must report no match")
			}(data)
		}
	}

	// Wait until each key's flight leader has entered the matcher, then give
	// the remaining callers time to join their key's flight before releasing.
	close(start)
	<-firstAEntered
	<-firstBEntered
	time.Sleep(50 * time.Millisecond)
	close(releaseMatcher)

	wg.Wait()

	assert.Equal(t, int32(2), computes.Load(), "two different data keys must compute once each, independently")
	assert.Equal(t, 2, cache.saveCount(), "two different data keys must save once each")
}
