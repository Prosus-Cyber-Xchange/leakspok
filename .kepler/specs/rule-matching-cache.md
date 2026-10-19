---
artifact: requirements-spec
capability: rule-matching-cache
status: current
updated: 2026-10-16
author: OpenCode
decision: accepted
---

# Capability: rule-matching-cache

## Overview

The rule-matching cache stores rule-match results in one hash per token: the hash key is the token content under the `leakspok:m:` namespace, and each field is an evaluated entity with a boolean-coded value ("1" match, "0" no match). An entity absent from a token's hash is a cache miss; a token with no hash is entirely uncached. Reads are a single HGETALL per token (through client-side caching when enabled); writes are one multi-field HSET per token that batches every negative result of that token's evaluation, with the key's TTL established exactly once at first write. Positive matches are never cached. Two optional mitigations sit on top of the storage contract: coalescing of identical concurrent miss computations per token (a burst of callers that miss the same token evaluates its missing entities once), and a jittered TTL applied when the token key's expiry is established (so entries written during a burst do not expire in lockstep). Both are opt-in; with both knobs at their zero values the cache behaves exactly as it did before either existed, apart from the token-hash layout.

## Contract

| field | type | notes |
|-------|------|-------|
| `analyzer.CacheStore.GetMatches` | `func(ctx, data) (map[pattern.Entity]bool, error)` | Returns exactly the entities cached for the token; an empty map means nothing is cached. Legacy per-pair methods were removed (pre-1.0 breaking change). |
| `analyzer.CacheStore.SaveMatches` | `func(ctx, data, results) error` | Persists a batch of results for a token in one command; callers own the persistence policy (negative-only today). |
| `analyzer.CacheOptions.SingleflightEnabled` | `bool` | Optional, default false. When true and caching is enabled, concurrent callers that miss the same token share one evaluation of its missing entities. Ignored when caching is disabled. |
| `analyzer.CacheOptions.TTLJitterPercentage` | `float64` | Optional, default 0. A fraction, e.g. 0.15 means ±15%. Values ≤ 0 disable jitter. Applies to the token key's establishment TTL and the client-side cache read TTL. |
| `RuleMatchingCacheOptions.TTLJitterPercentage` | `float64` | Same semantics as the analyzer-level knob; the cache-level option the factory maps the analyzer knob into. |

## Behavior

The cache is consulted once per token through the rule runners: a token's hash is read with a single HGETALL (through client-side caching when enabled), each rule's entity is then looked up in the returned map, and only entities absent from the map are evaluated — after their per-caller exception checks, whose outcomes are never persisted. Every negative result produced while evaluating the token is persisted for that token in one multi-field HSET; a positive match is returned to the caller and never cached. A backend read error degrades to full evaluation with the error logged and evaluation proceeding.

When coalescing is enabled, a burst of concurrent callers that miss on the same token share one in-flight evaluation of that token's missing entities: the flight leader evaluates its own missing set in rule order, persists at most one batched negative-result write, and shares the results and save error with waiters, each of which merges them and evaluates any residual misses inline — every caller issues at most one write for the token. A caller whose context is cancelled while waiting unblocks with its context error while the shared computation continues for the rest. If the negative-result save fails, every caller still receives the computed results together with the save error, which each caller logs and continues past. Coalescing covers only the in-flight window: when a burst completes and a later burst misses again, the later burst recomputes rather than replaying a memoized earlier result. With coalescing disabled, or whenever the cache backend is disabled, each miss computes and saves independently, exactly as before.

When jitter is configured at P percent, the token key's TTL at establishment lies within [base×(1−P), base×(1+P)]: each first write issues EXPIRE NX with a jittered TTL (whole-second granularity, clamped into the band), later writes never extend the key's life, and each client-side cache read uses a jittered local TTL. Zero jitter uses the base TTL exactly; a zero base TTL keeps the plain no-expiry path with no jitter applied.

**Examples**

- repeat evaluation of a known token -> cached results returned per entity; unmatched entities re-evaluated only if absent from the hash
- token evaluated against M rules with no matches -> one HGETALL read + one multi-field HSET write
- token with a positive match at rule k -> negatives accumulated before the match persisted; the positive never cached
- hot token -> reads served from the process-local client-side cache, invalidated by server push when the token's hash changes
- coalescing enabled, N concurrent misses on the same token -> the missing-entity evaluation runs exactly once, one batched save, all N callers share the results
- cancelled waiter -> unblocks with its context error; shared computation continues
- save failure -> every caller receives the results and the save error, each logs and continues
- sequential bursts on the same token -> each burst recomputes (no memoization of completed keys)
- jitter at P percent, base TTL T -> establishment TTL within [T×(1−P), T×(1+P)], established once and never extended
- zero jitter / zero base TTL -> exact base TTL / no expiry, no jitter applied

## Invariants

- Positive matches MUST NEVER be cached.
- When the cache backend is disabled, coalescing MUST NOT occur.
- When both knobs are at their zero values, cache behavior MUST be identical to the pre-mitigation release apart from the token-hash storage layout and command shape.
- Completed coalesced computations MUST NOT be memoized; a later burst MUST recompute against the current cache state.
- Exception outcomes MUST NEVER be cached or shared: persisted values encode matcher results only.
- A token key's TTL MUST be established exactly once and MUST NOT be extended by later writes to the same hash.

## Decisions

- Coalescing lives in the rule runners, not the cache: each runner wraps its token's miss evaluation and batched save in a shared token-keyed singleflight while the cache stays a pure storage contract with no computation-aware methods.
- One shared coalescing helper is used by both runners, with the computation injected as a function, so the coalescing policy exists once and is unit-testable without a Valkey server.
- The shared flight never memoizes: completed keys are forgotten, so coalescing applies only to the in-flight window and the next burst re-checks the cache.
- The knobs are boolean (`SingleflightEnabled`) and fraction (`TTLJitterPercentage`), both defaulting to their zero values, so unset knobs preserve existing behavior exactly.
- Jitter is implemented in the cache so the TTL sites (token-key establishment via EXPIRE NX and the client-side read) are driven by a single percentage and the mitigation cannot be half-applied; the random source is a concurrency-safe auto-seeded package-level generator, not crypto — the goal is spreading expiry, not unpredictability.
- Both rule runners take `RunnerOptions` at construction; the token-hash layout replaced the per-pair string keys in a pre-1.0 breaking change (the CacheStore contract is token-scoped), and existing string-format entries expire by TTL with no migration.
- Coalescing is per-process only; there is no cross-pod or cross-process coordination.
- The token-hash layout (one hash per token under the `leakspok:m:` namespace) replaced per-(entity, token) string keys so a request issues one read and at most one write per token; cluster-mode multi-key restrictions and per-pair TTL granularity were consciously traded away (key-level TTL, established once).
- Cache span tags carry a short non-reversible SHA-256 digest prefix of the token — never raw token text (unsanitized PII candidate material must not be recorded in APM).
