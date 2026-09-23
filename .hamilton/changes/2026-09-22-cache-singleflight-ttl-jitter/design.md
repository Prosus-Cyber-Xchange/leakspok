---
artifact: design
change: 2026-09-22-cache-singleflight-ttl-jitter
status: draft
created: 2026-09-22
author: OpenCode
decision: accepted
route_unit: null
---

# Design: Cache Singleflight Coalescing and TTL Jitter

## Context

Leakspok's rule-matching cache (analyzer/cache, backed by valkey-go) is queried per token per rule through two runners: SerialRulesRunner (serial_runner.go) and ConcurrentRulesRunner (concurrent_runner.go). Both runners implement the same miss sequence: GetMatch on the CacheStore, exception check, Matcher.Match, and a SaveMatch of the negative result when nothing matched. Under replica-read caching with replication lag, a burst of N concurrent requests for the same entity+data performs N identical regex matches and issues N identical SETs back to the primary, amplifying the write storm; a fixed TTL on every write then makes those keys expire in lockstep. This change adds two opt-in knobs: coalescing of the identical concurrent miss computation per cache key, implemented inside the runners, and per-write TTL jitter implemented inside the cache. The exported CacheStore interface is not modified; the cache gains no knowledge of computation.

## Goals / Non-Goals

**Goals**

- When coalescing is enabled, the match-and-save segment of a miss runs once per entity+data key among concurrent callers in the same process, and at most one negative result is persisted per burst.
- When jitter is enabled, every server-side SET PX and every client-side cache read uses a TTL within the configured percentage band of the base TTL.
- Zero configuration produces behavior identical to today, for both runners and the cache.

**Non-Goals**

- No interface change to CacheStore and no computation-aware methods on the cache or its decorators.
- No coalescing when the cache backend is disabled, no caching of positive matches, no cross-process coordination, no change to replica-routing behavior, and no wiring in the anonymizer service.

## Decisions

### Decision: Coalescing lives in the runners, not the cache

- Choice: each runner wraps its match-and-save segment in a per-runner singleflight flight keyed by entity+data. The cache layer is untouched except for jitter.
- Alternatives considered: a GetOrCompute method on RuleMatchingCache or a coalescing decorator over CacheStore — both make the cache depend on a compute callback, expanding the storage contract to own orchestration; extending the CacheStore interface with GetOrCompute was rejected as a breaking change that also forces Noop and the tracer to implement orchestration. All were dismissed on the responsibility boundary: the runner already owns the miss sequence, so coalescing it is the runner's job.
- Rationale: the cache remains a pure storage contract; the flight key only needs to distinguish (entity, data) pairs within the process and need not match the cache's key format, so there is no coupling or key-format duplication. A side benefit: the DataDog spans on GetMatch and SaveMatch keep firing per operation with zero tracer changes, and the runner's logger is where the save error is reported.

### Decision: One shared helper used by both runners

- Choice: a small unexported helper in the analyzer package, singleflightCoalescer, holds a singleflight.Group plus the enabled flag and exposes Do(ctx, key, fn) with context-aware waiting via DoChan; both runners call it for the match-and-save segment.
- Alternatives considered: independent flight logic inside each runner (duplicates the DoChan cancellation handling in two places); a cache-level decorator (rejected above).
- Rationale: one implementation, two call sites; the runner-specific code around it stays the runners' own orchestration. The helper depends only on golang.org/x/sync/singleflight and the standard library, with the computation injected as a function — it is unit-testable with no Valkey server.

### Decision: The shared flight never memoizes results

- Choice: after the flight function completes, the helper calls singleflight's Forget for that key.
- Alternatives considered: relying on singleflight's default behavior, which retains completed results so later calls replay the stored outcome without re-running — wrong here, because a stored outcome would go stale against the cache and the map would grow without bound.
- Rationale: coalescing must apply only to the in-flight window; the next burst re-checks the cache and re-computes as today. A sequential-burst test proves the second burst computes again.

### Decision: Opt-in flag shape

- Choice: CacheOptions.SingleflightEnabled (bool, default false) and CacheOptions.TTLJitterPercentage (float64, default 0). The factory consults SingleflightEnabled only when caching is enabled; with the noop cache the flag is ignored.
- Alternatives considered: placing the flag in ConcurrencyOptions (it is a concurrency mechanism) was rejected because it exists to change cache-miss behavior, consumers reason about it as a cache knob, and it must be ignored when caching is off anyway.
- Rationale: the naming matches the documented CACHE_SINGLEFLIGHT_ENABLED knob; zero values preserve current behavior, satisfying the opt-in requirement.

### Decision: Jitter implementation in the cache

- Choice: RuleMatchingCache stores the configured percentage and derives every effective TTL through a jittered helper using math/rand/v2's package-level functions (auto-seeded, concurrency-safe). SaveMatch's Px and GetMatch's DoCache both use it. TTLJitterPercentage <= 0 disables jitter; CacheTTL == 0 keeps the existing no-expiry path with no jitter.
- Alternatives considered: crypto randomness (unnecessary for spreading expiry); per-cache rand sources (extra state for no measurable benefit); jittering only the server write (leaves the local client-side cache expiring in lockstep and re-missing together).
- Rationale: the two TTL sites are both in RuleMatchingCache, which is the one place that knows them; a single percentage drives both so the mitigation cannot be half-applied.

### Decision: Both runners take RunnerOptions (breaking constructor change accepted)

- Choice: NewSerialRulesRuner changes to (logger, options RunnerOptions, cache CacheStore), matching the concurrent runner's existing (logger, options, cache, pool) shape; both runners read SingleflightEnabled from options.Cache. No functional-option variant is introduced.
- Alternatives considered: a variadic functional option on NewSerialRulesRuner to preserve source compatibility, rejected because it leaves the two runners with asymmetric plumbing for the same flag and introduces a one-case option type; keeping the two-argument constructor and gating coalescing elsewhere, rejected because the flag must reach the runner that owns the miss path.
- Rationale: uniform construction and one way to read the flag win over source compatibility; leakspok is pre-1.0, and the break is a one-line mechanical change for direct callers, which the factory absorbs internally.

## Architecture & Components

| Unit | Responsibility | Interface | Dependencies |
|---|---|---|---|
| singleflightCoalescer (new, analyzer) | Coalesce one computation per key across concurrent callers while enabled; honor caller cancellation while waiting; never memoize completed keys | Do(ctx, key string, fn func() (bool, error)) (bool, error) | golang.org/x/sync/singleflight, stdlib |
| SerialRulesRunner (modified) | Orchestrate rules sequentially; on a cache miss, run match-and-save through the coalescer | unchanged Process/Stop contract; constructor now takes RunnerOptions | CacheStore, coalescer |
| ConcurrentRulesRunner (modified) | Same, per rule worker | unchanged Process/Stop contract | CacheStore, coalescer |
| analyzer.CacheOptions (modified) | + SingleflightEnabled, + TTLJitterPercentage | additive fields | — |
| RuleMatchingCache (modified) | Store/retrieve match results; jitter every effective TTL | unchanged CacheStore contract | valkey-go, math/rand/v2 |
| CacheStoreTracer (unchanged) | Tracing decorator | unchanged | — |

### Quality Lens

- Responsibility: the cache keeps its single reason to change (storage semantics); coalescing sits with the runners, whose reason is rule-evaluation orchestration. The helper's only reason to change is the coalescing policy itself.
- Boundaries & dependencies: the helper is pure policy — computation and save arrive as an injected function, which is the testable seam (a counting closure stands in for the matcher; no Valkey needed for coalescing tests). Runners still depend only on the CacheStore interface.
- Right-sizing: no interface changes, no new decorators, no tracer edits, no new config beyond the two approved knobs; the helper is one small type.
- Accepted smells: the coalescing call is made from two runners — mitigated by sharing one implementation, leaving only thin call sites. The exported-constructor break is deliberate and recorded under Risks / Trade-offs. The serial runner receives the full RunnerOptions while reading only Cache.SingleflightEnabled from it — accepted for uniform construction with the concurrent runner.

## Data & Flow

Happy path for a burst of N concurrent misses on the same entity+data with coalescing enabled: each caller performs its own GetMatch (N reads, pipelined by valkey-go as today); each misses and enters the flight; the leader runs the exception check per its own request and then the matcher; on no match the leader issues one SaveMatch(false); all callers receive the shared boolean and, when the save failed, the shared save error to log and continue. On a match the result is shared and nothing is saved, preserving the existing rule that positive matches are not cached. Jitter: each SET carries PX in [base*(1-P), base*(1+P)] and each DoCache reads with a local TTL in the same band.

## Error Handling & Edge Cases

| Failure | Behavior |
|---|---|
| GetMatch returns a non-miss error (e.g., Valkey timeout) | Per-caller: logged, then proceed to compute exactly as today; the error never enters the shared flight |
| SaveMatch fails inside the flight | Result is kept; the error is shared to waiters, each logs and continues (log-and-continue convention; worst case equals today's N failed saves → N logs) |
| A waiting caller's context is cancelled | That caller returns its context error via DoChan select; the shared computation continues for the rest |
| Second sequential burst on the same key | Forget ensures the memoized result is dropped; the new burst re-checks the cache and re-computes |
| Coalescing disabled | The helper runs the function directly per caller, reproducing current behavior |
| Cache backend disabled | Factory installs the noop cache and ignores the flag; no coalescing |
| CacheTTL == 0 | Plain SET without expiry and the non-DoCache read path; jitter never applied |
| TTLJitterPercentage <= 0 | Base TTL used exactly |

## Testing Strategy

Unit tests for the coalescer with a counting injected function: one compute across N concurrent callers and one shared result; waiter cancellation unblocks that waiter while the computation completes; disabled mode runs per caller; sequential bursts re-compute (no memoization). Unit tests for jitter: many-sample bounds within the configured band for both TTL sites; zero percentage yields the exact base TTL; zero CacheTTL takes the no-expiry path. Runner-level tests extend the existing mock-based runner behavior tests to assert the matcher runs once per key under concurrency with the flag on, and that flag-off and noop-cache behavior is unchanged. Existing testcontainers-based cache tests cover the valkey round trip. Gates: task test (with race detector) and task lint on every task; tactical hamilton-code / hamilton-code-feedback during implementation; hamilton-review and hamilton-finish-work at the end.

## Constraints & Boundaries

- Always: run task test and task lint before marking any task done; add tests in the same task as the behavior they pin.
- Ask first: changing any exported signature other than the approved additive CacheOptions fields and the approved NewSerialRulesRuner RunnerOptions parameter; touching vendored code by hand (use task vendor).
- Never: modify the CacheStore interface or the tracer; change replica-routing behavior; edit anything outside leakspok-ce (the anonymizer wiring is a separate change).

## Risks / Trade-offs

- [Breaking change to NewSerialRulesRuner] -> Accepted: pre-1.0 library, one-line mechanical update for direct callers; called out in the release notes and the constructor doc comment.
- [Shared save error produces one log line per waiter] -> Bounded by today's worst case; accepted for simplicity.
- [math/rand/v2 global source] -> The package-level functions are concurrency-safe and auto-seeded; contention is negligible at cache-operation rates.
- [Jittered client TTL can diverge from server PX] -> Benign: valkey-go's client-side caching invalidates on server-driven pushes and misses only re-fetch; worst case is an extra GET.
- [singleflight memoization leak] -> Explicitly countered by Forget; covered by a dedicated test.
- [Per-process only] -> N pods mean at most N computes per key during a burst, already a large reduction; cross-pod coordination is out of scope.

## Migration / Rollout

One breaking change: direct callers of NewSerialRulesRuner must add the RunnerOptions argument; behavior with zero options is unchanged. Existing consumers see identical cache behavior until they set the knobs. Rollback of the mitigations is setting both knobs to zero without a library change. The anonymizer service wiring and dependency bump that actually enable the mitigations in production is a separate follow-up change.

## Open Questions

- None.
