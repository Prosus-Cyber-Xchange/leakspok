---
artifact: plan
change: 2026-09-22-cache-singleflight-ttl-jitter
status: approved
created: 2026-09-22
author: OpenCode
decision: accepted
route_unit: null
---

# Plan: Cache Singleflight Coalescing and TTL Jitter

## Overview

- Change: `.hamilton/changes/2026-09-22-cache-singleflight-ttl-jitter/`
- Goal: Add two opt-in knobs to the analyzer cache configuration — per-key singleflight coalescing of identical concurrent miss computations (implemented inside the rule runners) and per-write TTL jitter (implemented inside the rule-matching cache) — so replica-lag miss bursts and lockstep expirations stop amplifying. Zero configuration preserves current behavior exactly.
- Test: `task test` (race detector, whole suite); `task test/unit` for the fast gate.
- Build / typecheck: `go build ./...`
- Context notes: Upstream artifacts `design.md` and `requirements/rule-matching-cache.md` are authoritative for why and what; this plan only records how. Key constraints from the repo: tests must live in external `_test` packages (testpackage linter), so the unexported coalescer is verified through the runners' public API; cache integration tests use `cachetesting.StartValkeyContainer` (testcontainers, skip under `-short`); strict golangci config via `task lint`; dependencies are vendored via `task vendor`. The breaking `NewSerialRulesRuner` signature change is pre-approved in the design and must update every call site listed in Task 3.
- Quality notes: The coalescing call site exists in two runners by design — mitigated by one shared `singleflightCoalescer` implementation (thin call sites, accepted smell recorded in design.md). The serial runner receiving the full `RunnerOptions` while reading only `Cache.SingleflightEnabled` is also a recorded, accepted over-breadth. Cache stays computation-unaware; no `CacheStore` interface changes anywhere in this plan.

## Tasks

### Task 1: Add TTL jitter to the rule-matching cache

- Depends on: none
- Files:
  - Created: `analyzer/cache/ttl_jitter_test.go`
  - Modified: `analyzer/cache/options.go`, `analyzer/cache/valkey.go`
  - Deleted: none
- Acceptance:
  - `RuleMatchingCacheOptions` gains `TTLJitterPercentage float64` (fraction, e.g. 0.15 = ±15%; `<= 0` disables).
  - `RuleMatchingCache` computes every effective TTL through one jittered helper used by both `SaveMatch`'s `Px` and `GetMatch`'s `DoCache`, satisfying requirement "Opt-in per-write TTL jitter" scenarios server-side write jitter and client-side cache TTL jitter.
  - Zero percentage uses the base TTL exactly; `CacheTTL == 0` keeps the existing no-expiry path with no jitter (scenarios zero jitter preserves fixed TTL and no-expiry TTL unaffected).
- Steps:
  1. Add `TTLJitterPercentage` to `RuleMatchingCacheOptions` with a doc comment stating it is a fraction, `<= 0` disables, and that it applies to both the server SET PX and the client-side cache TTL.
  2. Write `analyzer/cache/ttl_jitter_test.go` (package `cache_test`, mirroring `rule_matching_cache_test.go`): a PTTL-in-band test that saves a false match through a `NewCacheStore` built with `CacheTTL` (e.g. 10s) and `TTLJitterPercentage: 0.15`, then opens a plain valkey-go client to the container address and asserts `PTTL` of the key `entity + ":" + data` lies within `[base*(1-0.15), base*(1+0.15)]`; a zero-jitter test asserting PTTL equals the base within a small ms tolerance; a no-expiry test with `CacheTTL: 0` and a non-zero percentage asserting the SET has no expiry (PTTL `-1`) and reads still round-trip; a read-path test with client-side caching enabled (`DisableInMemoryCache` false) and jitter on, asserting a saved false match reads back correctly. Run it and watch the jitter assertions fail against the fixed-TTL implementation.
  3. Implement: add a `jitterPct float64` field to `RuleMatchingCache`, populate it from the option in `NewRuleMatchingCache`, and add the helper verbatim:
     ```go
     // jitteredTTL returns the base TTL randomized within +/-jitterPct.
     // A jitterPct of zero returns the base TTL unchanged. Callers already
     // guard the no-expiry case (CacheTTL == 0) before calling.
     func (r *RuleMatchingCache) jitteredTTL() time.Duration {
         if r.jitterPct <= 0 {
             return r.cacheTTL
         }
         delta := time.Duration(float64(r.cacheTTL) * r.jitterPct * (2*rand.Float64() - 1))
         return r.cacheTTL + delta
     }
     ```
     using `math/rand/v2` (package-level `rand.Float64()`; do not add a rand field or crypto source). Replace `r.cacheTTL` with `r.jitteredTTL()` in `SaveMatch`'s `Px` and in `GetMatch`'s `DoCache` TTL argument only — the `cacheTTL == 0` guards stay as they are.
  4. Run `go test -race -count=1 ./analyzer/cache/...` — expect green, including the four new jitter tests.
- Verify: `go test -race -count=1 ./analyzer/cache/...` → all pass; `task lint` → clean.
- Commit: `feat(cache): jitter rule-matching cache TTL writes`

### Task 2: Map TTLJitterPercentage through the analyzer factory

- Depends on: Task 1
- Files:
  - Created: none (extend existing test file)
  - Modified: `analyzer/factory.go`, `analyzer/coverage_gap_test.go`
  - Deleted: none
- Acceptance:
  - `analyzer.CacheOptions` gains `TTLJitterPercentage float64` and `buildCacheStore` forwards it into `RuleMatchingCacheOptions`; a factory-level integration test observes a jittered PTTL in band after `MakeByteAnalyzer` writes a negative result.
- Steps:
  1. Add `TTLJitterPercentage` to `analyzer.CacheOptions` with the same semantics doc comment as the cache option, and map it in `buildCacheStore` next to `CacheTTL`.
  2. Add an integration test to `analyzer/coverage_gap_test.go` modeled on the existing "client name observable on server" subtest: start a container via `cachetesting.StartValkeyContainer`, build a `MakeByteAnalyzer` with `Cache.Enabled`, `TTL: 30*time.Second`, `TTLJitterPercentage: 0.15`, `DisableInMemoryCache: true`, and the container address; anonymize a plain input with an email rule (this writes the negative result); open a valkey-go client to the address and assert `PTTL` of the `entity:data` key is within `[TTL*(1-0.15), TTL*(1+0.15)]`. Run it and watch it fail before the mapping exists.
  3. Run `go test -race -count=1 ./analyzer/...` — expect green.
- Verify: `go test -race -count=1 ./analyzer/...` → all pass; `task lint` → clean.
- Commit: `feat(analyzer): map TTL jitter percentage through factory`

### Task 3: Pass RunnerOptions to the serial rules runner (breaking)

- Depends on: none
- Files:
  - Created: none
  - Modified: `analyzer/serial_runner.go`, `analyzer/factory.go`, `examples/basic/main.go`, `examples/custom-rules/main.go`, `analyzer/serial_runner_test.go`, `analyzer/coverage_gap_test.go`, `analyzer/runner_behavior_test.go`, `analyzer/byte_analyzer_test.go`
  - Deleted: none
- Acceptance:
  - `NewSerialRulesRuner(logger, options RunnerOptions, cache)` compiles; `SerialRulesRunner` stores the options for the coalescing work in Tasks 4–5. `CacheOptions` gains `SingleflightEnabled bool` (default false) with a doc comment stating it only takes effect when caching is enabled.
  - Every call site in the repo compiles against the new signature; full suite green; no behavior change (flag not yet read).
- Steps:
  1. Add `SingleflightEnabled bool` to `analyzer.CacheOptions` with a doc comment: when true, identical concurrent cache misses for the same entity+data are coalesced into one computation per key; ignored when `Enabled` is false.
  2. Change `NewSerialRulesRuner(logger *slog.Logger, options RunnerOptions, cache analyzercache.CacheStore) SerialRulesRunner` and store the options on the struct; update its doc comment.
  3. Update both factory call sites (`factory.go` around lines 205 and 244) to pass `options`.
  4. Update every remaining call site: the two examples (`NewSerialRulesRuner(logger, analyzercache.NewNoopRuleMatchingCache())` gains `analyzer.RunnerOptions{}` between the arguments), and the mechanical `analyzer.NewSerialRulesRuner(...)` edits in `serial_runner_test.go` (about thirty sites), `coverage_gap_test.go` (two), `runner_behavior_test.go` (one), `byte_analyzer_test.go` (two). Let the compiler list them; do not touch any `NewConcurrentRulesRunner` call.
  5. Run `go build ./...` then `go test -race -count=1 ./...` — expect green with zero behavior changes.
- Verify: `go build ./...` → success; `go test -race -count=1 ./...` → all pass; `task lint` → clean.
- Commit: `refactor(analyzer): pass RunnerOptions to serial rules runner (breaking)`

### Task 4: Coalesce serial-runner cache misses with singleflight

- Depends on: Task 3
- Files:
  - Created: `analyzer/singleflight_coalescer.go`, `analyzer/serial_runner_coalescing_test.go`, vendored `golang.org/x/sync/singleflight` via `task vendor`
  - Modified: `analyzer/serial_runner.go`, `go.mod`, `go.sum`, `vendor/modules.txt`
  - Deleted: none
- Acceptance:
  - The unexported `singleflightCoalescer` exists exactly as specified below and the serial runner wraps only its match-and-save segment with it, keyed by `entity + ":" + data`, honoring requirement "Opt-in coalescing of identical concurrent miss computations" scenarios: burst of identical concurrent misses (one compute, one save), waiting caller cancels (waiter unblocks without computing), coalescing disabled (per-request), sequential bursts recompute (no memoization), and cached-hit short-circuit unchanged.
- Steps:
  1. Create `analyzer/singleflight_coalescer.go` with the type verbatim (no design additions):
     ```go
     package analyzer

     import (
         "context"

         "golang.org/x/sync/singleflight"
     )

     // singleflightCoalescer coalesces one computation per key across concurrent
     // callers while enabled. Results are shared only while the call is in flight;
     // the key is forgotten on completion so a later burst recomputes.
     type singleflightCoalescer struct {
         group   singleflight.Group
         enabled bool
     }

     func newSingleflightCoalescer(enabled bool) singleflightCoalescer {
         return singleflightCoalescer{enabled: enabled}
     }

     // do runs fn once per key among concurrent callers and shares its result.
     // When disabled, fn runs once per caller. A caller whose context is
     // cancelled while waiting returns ctx.Err() without running fn.
     func (c singleflightCoalescer) do(ctx context.Context, key string, fn func() (bool, error)) (bool, error) {
         if !c.enabled {
             return fn()
         }

         ch := c.group.DoChan(key, func() (any, error) {
             defer c.group.Forget(key)
             matched, err := fn()
             return matched, err
         })

         select {
         case res := <-ch:
             return res.Val.(bool), res.Err
         case <-ctx.Done():
             return false, ctx.Err()
         }
     }
     ```
  2. Write `analyzer/serial_runner_coalescing_test.go` (package `analyzer_test`). Define a small counting fake implementing `analyzercache.CacheStore` in the file: `GetMatch` always returns `ErrCacheNotFound`, `SaveMatch` records the save count per key (sync-protected) and returns a configurable error. Use a counting matcher built from `pattern.PatternFunc` (atomic counter) and a real `analyzer.Rule`. Write tests: (a) enabled, N=20 goroutines calling `Process` concurrently with the same data — matcher computed exactly once, saves exactly one, all return no match; (b) enabled, two sequential rounds — matcher count is two (no memoization); (c) disabled — N concurrent calls produce N computations; (d) hit short-circuit — a fake that returns a cached false match results in zero matcher calls; (e) cancellation — matcher blocks on a channel and signals entry, a second caller joins the flight, its context is cancelled, it returns `(Rule{}, false)` promptly while the blocked computation completes once for the leader. Run and watch the coalescing assertions fail against the current serial runner.
  3. Wire the serial runner: construct the coalescer from `s.options.Cache.SingleflightEnabled`; build the key as `string(rule.Matcher.Entity()) + ":" + string(data)`; on the miss path, after the exception check, run the matcher and the conditional `SaveMatch(false)` inside `coalescer.do`, keeping the exception check outside the flight and keeping the existing `GetMatch` logging untouched. On a coalescer error, log with `ErrorContext` and continue to the next rule with no match — cancelled waiters therefore never compute.
  4. Run `task vendor` to promote `golang.org/x/sync` to a direct requirement and vendor `singleflight`; commit the `go.mod`, `go.sum`, and `vendor/` changes with this task.
  5. Run `go test -race -count=1 ./analyzer/...` — expect green.
- Verify: `go test -race -count=1 ./analyzer/...` → all pass, new coalescing tests included; `task lint` → clean.
- Commit: `feat(analyzer): coalesce serial runner cache misses with singleflight`

### Task 5: Coalesce concurrent-runner cache misses with singleflight

- Depends on: Task 3, Task 4
- Files:
  - Created: `analyzer/concurrent_runner_coalescing_test.go`
  - Modified: `analyzer/concurrent_runner.go`
  - Deleted: none
- Acceptance:
  - `ConcurrentRulesRunner.processRule` wraps its match-and-save segment in the shared `singleflightCoalescer` built from `options.Cache.SingleflightEnabled`, so a concurrent burst of identical misses across `Process` calls computes once and saves once (requirement scenario burst of identical concurrent misses), disabled mode stays per-request, and the exception check remains outside the flight.
- Steps:
  1. Write `analyzer/concurrent_runner_coalescing_test.go` (package `analyzer_test`, reusing the counting fake from Task 4's test file): build `ConcurrentRulesRunner` via its constructor with a pool (see `runner_behavior_test.go`'s factory) and `RunnerOptions{Cache: CacheOptions{SingleflightEnabled: true}}`; assert (a) N concurrent `Process` calls with the same data and a counting matcher produce exactly one compute and one save; (b) disabled produces N computes; (c) different data keys are independent. Run and watch the assertions fail against the current runner.
  2. Wire `processRule`: construct the coalescer alongside the runner (store it like the serial runner does, or build the key and call the same `do` helper the same way); replace only the `rule.Matcher.Match` plus conditional `SaveMatch(false)` segment with the coalesced call, keeping the early-exit selects, the `GetMatch` logging, and the exception check exactly where they are. On a coalescer error, log with `ErrorContext` and report no match for that rule.
  3. Run `go test -race -count=1 ./analyzer/...` — expect green.
- Verify: `go test -race -count=1 ./analyzer/...` → all pass; `task lint` → clean.
- Commit: `feat(analyzer): coalesce concurrent runner cache misses with singleflight`

## Done when

- All five tasks implemented and recorded in `progress.md` as done
- `go build ./...` clean; `go test -race -count=1 ./...` green; `task lint` clean
- All hamilton-code-feedback findings and hamilton-review feedback addressed
