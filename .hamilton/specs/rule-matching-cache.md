---
artifact: requirements-spec
capability: rule-matching-cache
status: current
updated: 2026-09-22
author: OpenCode
decision: accepted
---

# Capability: rule-matching-cache

## Overview

The rule-matching cache stores rule-match results keyed by entity and data so repeated requests for the same input skip re-running the matcher. It is backed by Valkey and exposed to applications through the analyzer cache configuration. Two optional mitigations sit on top of the plain storage contract: coalescing of identical concurrent miss computations (so a burst of callers that all miss the cache computes once instead of N times), and jittered per-write TTLs (so entries written during a burst do not expire in lockstep). Both are opt-in; with both knobs at their zero values the cache behaves exactly as it did before either existed.

## Contract

| field | type | notes |
|-------|------|-------|
| `analyzer.CacheOptions.SingleflightEnabled` | `bool` | Optional, default false. When true and caching is enabled, identical concurrent cache misses for the same entity+data are coalesced into one computation. Ignored when caching is disabled. |
| `analyzer.CacheOptions.TTLJitterPercentage` | `float64` | Optional, default 0. A fraction, e.g. 0.15 means ±15%. Values ≤ 0 disable jitter. Applies to both the server-side write TTL and the client-side cache read TTL. |
| `RuleMatchingCacheOptions.TTLJitterPercentage` | `float64` | Same semantics as the analyzer-level knob; the cache-level option the factory maps the analyzer knob into. |

## Behavior

The cache is queried per token per rule through the rule runners. A hit short-circuits: the cached result is returned and the matcher runs zero times. A miss runs the exception check, then the match computation, and persists a negative result when nothing matched — positive matches are never cached.

When coalescing is enabled, a burst of concurrent callers that miss on the same entity+data key run the match computation exactly once and persist at most one negative result. Every caller receives the shared boolean; a caller whose context is cancelled while waiting unblocks with its context error while the shared computation continues for the rest. If the negative-result save fails, every caller still receives the computed result together with the save error, which each caller logs and continues past. Coalescing covers only the in-flight window: when a burst completes and a later burst misses again, the later burst recomputes rather than replaying a memoized earlier result. With coalescing disabled, or whenever the cache backend is disabled, each miss computes and saves independently, exactly as before.

When jitter is configured at P percent, every effective TTL lies within [base×(1−P), base×(1+P)]: each negative-result save issues a server SET PX with a jittered TTL, and each client-side cache read uses a jittered local TTL. Zero jitter uses the base TTL exactly; a zero base TTL (entries never expire) keeps the plain no-expiry path with no jitter applied.

**Examples**

- cache hit on a burst of N concurrent callers -> cached result returned to each caller, compute runs zero times
- coalescing enabled, N concurrent misses on the same key -> compute runs exactly once, one negative-result save, all N callers receive the same boolean
- coalesced compute returns true -> result shared with all waiters, no save issued
- coalesced save fails -> every caller receives the computed result and the save error, each logs and continues
- waiting caller's context cancelled -> that caller unblocks with its context error, the shared computation continues
- sequential bursts on the same key -> each burst computes again (no memoization of completed keys)
- coalescing disabled or cache backend disabled -> each miss computes and saves independently
- jitter at P percent, base TTL T -> server SET PX and client-side read TTL within [T×(1−P), T×(1+P)]
- zero jitter -> base TTL used exactly
- base TTL zero -> plain SET without expiry, no jitter applied
- both knobs at zero -> cache reads, writes, expiry, and concurrency behavior identical to the pre-mitigation release

## Invariants

- Positive matches MUST NEVER be cached.
- When the cache backend is disabled, coalescing MUST NOT occur.
- When both knobs are at their zero values, cache behavior MUST be identical to the pre-mitigation release.
- Completed coalesced computations MUST NOT be memoized; a later burst MUST recompute against the current cache state.

## Decisions

- Coalescing lives in the rule runners, not the cache: each runner wraps its match-and-save segment in a shared per-key singleflight while the cache stays a pure storage contract with no computation-aware methods.
- One shared coalescing helper is used by both runners, with the computation injected as a function, so the coalescing policy exists once and is unit-testable without a Valkey server.
- The shared flight never memoizes: completed keys are forgotten, so coalescing applies only to the in-flight window and the next burst re-checks the cache.
- The knobs are boolean (`SingleflightEnabled`) and fraction (`TTLJitterPercentage`), both defaulting to their zero values, so unset knobs preserve existing behavior exactly.
- Jitter is implemented in the cache so the two TTL sites (server SET and client-side read) are driven by a single percentage and the mitigation cannot be half-applied; the random source is a concurrency-safe auto-seeded package-level generator, not crypto — the goal is spreading expiry, not unpredictability.
- Both rule runners take `RunnerOptions` at construction, making the serial runner's constructor signature breaking (matching the concurrent runner's existing shape); leakspok is pre-1.0 and the change is a one-line mechanical update for direct callers.
- Coalescing is per-process only; there is no cross-pod or cross-process coordination.
