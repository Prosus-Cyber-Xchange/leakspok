package analyzer_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
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

// ─── Token-scoped cache contract (spy-based) ─────────────────────────────────

// tokenSpyCache is a CacheStore test fake for the token-scoped contract: the
// token-scoped read returns the configured cached map (empty by default), the
// batched save records the persisted fields, and every call — token-scoped and
// legacy per-pair alike — is counted separately, so the spy criteria (exactly
// one GetMatches per Process, at most one SaveMatches, no legacy per-pair
// calls) can be asserted deterministically.
type tokenSpyCache struct {
	mu      sync.Mutex
	cached  map[pattern.Entity]bool
	readErr error
	saveErr error

	tokenReads int
	tokenSaves int
	lastSave   map[pattern.Entity]bool
}

func newTokenSpyCache(cached map[pattern.Entity]bool) *tokenSpyCache {
	return &tokenSpyCache{cached: cached}
}

func (c *tokenSpyCache) GetMatches(_ context.Context, _ []byte) (map[pattern.Entity]bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokenReads++
	if c.readErr != nil {
		return nil, c.readErr
	}
	// Copy: callers must never mutate the fake's cached map.
	out := make(map[pattern.Entity]bool, len(c.cached))
	maps.Copy(out, c.cached)
	return out, nil
}

func (c *tokenSpyCache) SaveMatches(_ context.Context, _ []byte, results map[pattern.Entity]bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokenSaves++
	c.lastSave = maps.Clone(results)
	return c.saveErr
}

// spyCounts snapshots the call counters of a tokenSpyCache.
type spyCounts struct {
	tokenReads int
	tokenSaves int
}

func (c *tokenSpyCache) counts() spyCounts {
	c.mu.Lock()
	defer c.mu.Unlock()
	return spyCounts{
		tokenReads: c.tokenReads,
		tokenSaves: c.tokenSaves,
	}
}

func (c *tokenSpyCache) savedFields() map[pattern.Entity]bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.lastSave)
}

// TestConcurrentRulesRunner_ProcessIssuesOneTokenScopedRead pins down the spy
// criteria "exactly one GetMatches and at most one SaveMatches per Process
// call": the runner reads the token once before submitting workers and
// persists the evaluated negatives in one batch; no legacy per-pair call is
// ever issued.
func TestConcurrentRulesRunner_ProcessIssuesOneTokenScopedRead(t *testing.T) {
	cache := newTokenSpyCache(nil)
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "non-matching-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
	defer runner.Stop()

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, analyzer.Rule{}, matchedRule)
	assert.Equal(t, int32(1), computes.Load())

	counts := cache.counts()
	assert.Equal(t, 1, counts.tokenReads, "Process must issue exactly one token-scoped read")
	assert.Equal(t, 1, counts.tokenSaves, "Process must persist the negatives in exactly one batched save")
	assert.Equal(t, map[pattern.Entity]bool{pattern.EntityEmail: false}, cache.savedFields(),
		"the evaluated negative is the only persisted field")
}

// TestConcurrentRulesRunner_ProcessCachedHitSkipsMatcher pins down that a
// cached hit — positive or negative — never submits a worker for that entity:
// the matcher never runs and nothing is persisted.
func TestConcurrentRulesRunner_ProcessCachedHitSkipsMatcher(t *testing.T) {
	t.Run("cached positive hit", func(t *testing.T) {
		cache := newTokenSpyCache(map[pattern.Entity]bool{pattern.EntityEmail: true})
		var computes atomic.Int32
		rule := analyzer.Rule{Name: "cached-rule", Matcher: countingMatcher(&computes)}
		runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
		defer runner.Stop()

		matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("cached data"))

		assert.True(t, found)
		assert.Equal(t, "cached-rule", matchedRule.Name)
		assert.Equal(t, int32(0), computes.Load(), "a cached hit must never reach the matcher")
		assert.Zero(t, cache.counts().tokenSaves, "a cached hit evaluates and persists nothing")
	})

	t.Run("cached negative hit", func(t *testing.T) {
		cache := newTokenSpyCache(map[pattern.Entity]bool{pattern.EntityEmail: false})
		var computes atomic.Int32
		rule := analyzer.Rule{Name: "cached-rule", Matcher: countingMatcher(&computes)}
		runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
		defer runner.Stop()

		matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("cached data"))

		assert.False(t, found)
		assert.Equal(t, analyzer.Rule{}, matchedRule)
		assert.Equal(t, int32(0), computes.Load(), "a cached hit must never reach the matcher")
		assert.Zero(t, cache.counts().tokenSaves, "a cached negative is already persisted")
	})
}

// TestConcurrentRulesRunner_ProcessPartiallyCachedTokenSubmitsOnlyAbsent pins
// down that a partially cached token submits only absent entities: the cached
// entity's matcher never runs, the absent entity is evaluated once, and its
// negative is the only persisted field.
func TestConcurrentRulesRunner_ProcessPartiallyCachedTokenSubmitsOnlyAbsent(t *testing.T) {
	cache := newTokenSpyCache(map[pattern.Entity]bool{pattern.EntityEmail: false})

	var emailComputes, cpfComputes atomic.Int32
	cpfMatcher := pattern.NewPatternMatcher(pattern.EntityCPF, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		cpfComputes.Add(1)
		return false
	}))

	rules := []analyzer.Rule{
		{Name: "email-rule", Matcher: countingMatcher(&emailComputes)},
		{Name: "cpf-rule", Matcher: cpfMatcher},
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
	defer runner.Stop()

	matchedRule, found := runner.Process(context.Background(), rules, []byte("partially cached data"))

	assert.False(t, found)
	assert.Equal(t, analyzer.Rule{}, matchedRule)
	assert.Equal(t, int32(0), emailComputes.Load(), "the cached entity's matcher must never run")
	assert.Equal(t, int32(1), cpfComputes.Load(), "only the absent entity is submitted for evaluation")
	counts := cache.counts()
	assert.Equal(t, 1, counts.tokenReads)
	assert.Equal(t, 1, counts.tokenSaves)
	assert.Equal(t, map[pattern.Entity]bool{pattern.EntityCPF: false}, cache.savedFields(),
		"only the absent entity's negative is persisted")
}

// TestConcurrentRulesRunner_ProcessExceptedEntityNeverEvaluatedOrPersisted
// pins down that an entity whose rule's own exception check passes is never
// evaluated and never persisted.
func TestConcurrentRulesRunner_ProcessExceptedEntityNeverEvaluatedOrPersisted(t *testing.T) {
	cache := newTokenSpyCache(nil)
	var computes atomic.Int32
	exceptionMatcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, input []byte) bool {
		return string(input) == "safe@company.com"
	}))
	rule := analyzer.Rule{
		Name:    "email-rule",
		Matcher: countingMatcher(&computes),
		Exceptions: []analyzer.Exception{
			{Reason: "Whitelisted email", Matcher: exceptionMatcher},
		},
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
	defer runner.Stop()

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("safe@company.com"))

	assert.False(t, found)
	assert.Equal(t, analyzer.Rule{}, matchedRule)
	assert.Equal(t, int32(0), computes.Load(), "an excepted entity is never evaluated")
	counts := cache.counts()
	assert.Equal(t, 1, counts.tokenReads)
	assert.Zero(t, counts.tokenSaves, "an excepted entity is never persisted")
}

// TestConcurrentRulesRunner_ProcessBackendReadErrorDegradesToFullEvaluation
// pins down the "Backend error during read" scenario: the runner logs the
// error and treats every entity as a cache miss, proceeding with evaluation —
// the matcher still runs and its negative result is still persisted.
func TestConcurrentRulesRunner_ProcessBackendReadErrorDegradesToFullEvaluation(t *testing.T) {
	cache := newTokenSpyCache(nil)
	cache.readErr = assert.AnError
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "non-matching-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
	defer runner.Stop()

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("test data"))

	assert.False(t, found)
	assert.Equal(t, analyzer.Rule{}, matchedRule)
	assert.Equal(t, int32(1), computes.Load(), "every entity is treated as a miss, so the matcher runs")
	counts := cache.counts()
	assert.Equal(t, 1, counts.tokenReads, "exactly one read is attempted even when it fails")
	assert.Equal(t, 1, counts.tokenSaves, "the evaluated negative is still persisted")
	assert.Equal(t, map[pattern.Entity]bool{pattern.EntityEmail: false}, cache.savedFields())
}

// TestConcurrentRulesRunner_ProcessPositiveMatchStillFlushesNegatives pins
// down that negatives collected from completed workers before a positive match
// are persisted in the one batched save, and the positive match itself is
// never cached.
func TestConcurrentRulesRunner_ProcessPositiveMatchStillFlushesNegatives(t *testing.T) {
	cache := newTokenSpyCache(nil)
	var emailComputes atomic.Int32
	// The negative worker must complete its evaluation before the matching
	// worker matches, so the collector receives the negative first (channel
	// FIFO) and the one batched save deterministically carries it. The matching
	// matcher waits for the negative matcher to finish and yields briefly so
	// the negative result's send wins the race before the match is collected.
	emailDone := make(chan struct{})
	emailMatcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		emailComputes.Add(1)
		close(emailDone)
		return false
	}))
	matchingMatcher := pattern.NewPatternMatcher(pattern.EntityCPF, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		<-emailDone
		time.Sleep(time.Millisecond)
		return true
	}))
	rules := []analyzer.Rule{
		{Name: "non-matching-rule", Matcher: emailMatcher},
		{Name: "matching-rule", Matcher: matchingMatcher},
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
	defer runner.Stop()

	matchedRule, found := runner.Process(context.Background(), rules, []byte("matching data"))

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
	assert.Equal(t, int32(1), emailComputes.Load())
	counts := cache.counts()
	assert.Equal(t, 1, counts.tokenSaves)
	assert.Equal(t, map[pattern.Entity]bool{pattern.EntityEmail: false}, cache.savedFields(),
		"negatives accumulated before the match are persisted and the positive match is never cached")
}

// TestConcurrentRulesRunner_ProcessReturnsBeforeSlowWorkerCompletes pins down
// the preserved first-match cancellation: Process returns as soon as a
// collected result matches, the slow in-flight worker is not awaited, and only
// completed workers' negatives would be flushed (none here).
func TestConcurrentRulesRunner_ProcessReturnsBeforeSlowWorkerCompletes(t *testing.T) {
	cache := newTokenSpyCache(nil)

	slowEntered := make(chan struct{})
	releaseSlow := make(chan struct{})
	slowDone := make(chan struct{})
	slowMatcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		close(slowEntered)
		<-releaseSlow
		defer close(slowDone)
		return false
	}))
	fastDone := make(chan struct{})
	fastMatcher := pattern.NewPatternMatcher(pattern.EntityCPF, pattern.PatternFunc(func(_ context.Context, _ []byte) bool {
		// Only match once the slow worker is already inside its matcher, so
		// the first-match cancellation can never bail it out before it runs.
		<-slowEntered
		defer close(fastDone)
		return true
	}))

	rules := []analyzer.Rule{
		{Name: "slow-rule", Matcher: slowMatcher},
		{Name: "matching-rule", Matcher: fastMatcher},
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
	defer runner.Stop()

	done := make(chan struct{})
	var matchedRule analyzer.Rule
	var found bool
	go func() {
		defer close(done)
		matchedRule, found = runner.Process(context.Background(), rules, []byte("matching data"))
	}()

	<-slowEntered
	<-fastDone
	// Give the collector time to receive the completed fast result before
	// requiring Process to have returned.
	time.Sleep(50 * time.Millisecond)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		close(releaseSlow)
		t.Fatal("Process must return as soon as a collected result matches, without awaiting the slow worker")
	}

	assert.True(t, found)
	assert.Equal(t, "matching-rule", matchedRule.Name)
	counts := cache.counts()
	assert.Equal(t, 1, counts.tokenReads)
	assert.Zero(t, counts.tokenSaves, "no negative completed, so nothing may be persisted; positives are never cached")

	close(releaseSlow)
	<-slowDone
}

// TestConcurrentRulesRunner_ProcessTokenCoalescingSharesEvaluationAndSave pins
// down the "Burst of concurrent misses on the same token" scenario: a burst of
// identical misses shares one evaluation and one batched save under the
// token-level flight, while every caller still issues its own token-scoped
// read and no legacy per-pair call is made.
func TestConcurrentRulesRunner_ProcessTokenCoalescingSharesEvaluationAndSave(t *testing.T) {
	const callers = 10

	cache := newTokenSpyCache(nil)
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
	rule := analyzer.Rule{Name: "burst-rule", Matcher: burstMatcher}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{Enabled: true, SingleflightEnabled: true},
	}, cache, callers)
	defer runner.Stop()

	data := []byte("burst data")
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, found := runner.Process(context.Background(), []analyzer.Rule{rule}, data)
			assert.False(t, found, "matcher never matches, so Process must report no match")
		}()
	}

	// Release the whole burst at once. The first caller to arrive becomes the
	// flight leader and blocks in the matcher; give the rest time to join the
	// flight before the computation is released.
	close(start)
	select {
	case <-matcherEntered:
	case <-time.After(2 * time.Second):
		close(releaseMatcher)
		t.Fatal("the flight leader never entered the matcher; see spy failures above")
	}
	time.Sleep(50 * time.Millisecond)
	close(releaseMatcher)
	wg.Wait()

	assert.Equal(t, int32(1), computes.Load(), "one burst on the same token must evaluate the missing entity exactly once")
	counts := cache.counts()
	assert.Equal(t, callers, counts.tokenReads, "every caller issues its own token-scoped read")
	assert.Equal(t, 1, counts.tokenSaves, "one burst on the same token must save exactly once")
}

// TestConcurrentRulesRunner_ProcessTokenCoalescingCancelledWaiter pins down
// the "Cancelled waiter" scenario: a caller whose context is cancelled while
// it waits for the shared computation unblocks promptly with no match while
// the leader's evaluation continues. The pool fits exactly the leader's single
// task, so a waiter that submitted a pool task would deadlock on Submit — this
// also pins that waiters hold no pool slots while waiting on the flight
// channel.
func TestConcurrentRulesRunner_ProcessTokenCoalescingCancelledWaiter(t *testing.T) {
	cache := newTokenSpyCache(nil)
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
	rule := analyzer.Rule{Name: "blocking-rule", Matcher: blockingMatcher}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{
		Cache: analyzer.CacheOptions{Enabled: true, SingleflightEnabled: true},
	}, cache, 1)
	defer runner.Stop()

	data := []byte("identical data")
	leaderDone := make(chan struct{})
	var leaderFound bool
	go func() {
		defer close(leaderDone)
		_, leaderFound = runner.Process(context.Background(), []analyzer.Rule{rule}, data)
	}()

	// Wait for the leader to enter the matcher: the flight for this token
	// exists and its pool task is running.
	select {
	case <-matcherEntered:
	case <-time.After(2 * time.Second):
		close(releaseMatcher)
		t.Fatal("the flight leader never entered the matcher")
	}

	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	waiterDone := make(chan struct{})
	go func() {
		defer close(waiterDone)
		_, _ = runner.Process(waiterCtx, []analyzer.Rule{rule}, data)
	}()

	// Let the waiter join the flight, then cancel its context.
	time.Sleep(50 * time.Millisecond)
	cancelWaiter()

	select {
	case <-waiterDone:
	case <-time.After(2 * time.Second):
		close(releaseMatcher)
		t.Fatal("cancelled waiter did not unblock promptly")
	}

	// The leader's shared computation still completes exactly once.
	close(releaseMatcher)

	select {
	case <-leaderDone:
	case <-time.After(2 * time.Second):
		t.Fatal("leader did not complete after the computation was released")
	}

	assert.False(t, leaderFound, "the leader must report no match")
	assert.Equal(t, int32(1), computes.Load(), "the shared computation must run exactly once")
	assert.Equal(t, 1, cache.counts().tokenSaves, "the shared computation must save exactly once")
}

// TestConcurrentRulesRunner_ProcessSaveErrorIsLoggedAndContinues pins down the
// "Save failure" scenario: a failed batched negative-result save is logged and
// the caller continues past it with the computed results.
func TestConcurrentRulesRunner_ProcessSaveErrorIsLoggedAndContinues(t *testing.T) {
	cache := newTokenSpyCache(nil)
	cache.saveErr = assert.AnError
	var computes atomic.Int32
	rule := analyzer.Rule{
		Name:    "counting-rule",
		Matcher: countingMatcher(&computes),
	}
	runner := newConcurrentRunnerWithCache(analyzer.RunnerOptions{}, cache, 4)
	defer runner.Stop()

	matchedRule, found := runner.Process(context.Background(), []analyzer.Rule{rule}, []byte("data"))

	assert.False(t, found)
	assert.Equal(t, analyzer.Rule{}, matchedRule)
	assert.Equal(t, int32(1), computes.Load())
	assert.Equal(t, 1, cache.counts().tokenSaves)
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
