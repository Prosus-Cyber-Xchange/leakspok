---
artifact: proposal
change: 2026-09-22-cache-singleflight-ttl-jitter
status: approved
decision: accepted
author: OpenCode
created: 2026-09-22
route_unit: null
---

# Proposal: Cache Singleflight Coalescing and TTL Jitter

## Why

Under replica-read caching, a replication-lag window turns every cache miss into a self-amplifying loop: N concurrent requests for the same entity+data each run their own regex match and each issue their own SET back to the primary, feeding more replication stream. Every write also carries a fixed TTL, so keys written during a burst expire in lockstep and produce synchronized expiry waves. Leakspok's rule-matching cache has no coalescing of identical concurrent misses and no TTL jitter, which makes both effects worse. This change gives Leakspok applications opt-in knobs to coalesce identical concurrent miss computations per cache key and to randomize per-write TTLs so expiry spreads instead of aligning.

## Goals & Success Criteria

- When enabled, concurrent identical cache misses run the match computation exactly once per key and persist exactly one negative result, instead of once per request.
- When enabled, each cache write and each client-side cache read uses a TTL jittered within a configured percentage of the base TTL, so expiry waves cannot form.
- Both behaviors are opt-in: with the knobs at their zero values, cache behavior is byte-for-byte identical to today.
- The existing cache semantics are preserved: hits short-circuit without computation, positive matches are still not cached, and cache-disabled operation still performs no coalescing.
- Automated tests prove single execution of the compute function under concurrency, TTL bounds under jitter, and unchanged behavior at zero configuration.

## Non-Goals

- No change to replica-read routing or staleness semantics (the false-miss window itself remains; it just becomes non-amplifying).
- No caching of positive matches.
- No coalescing when the cache backend is disabled (Noop cache stays a pure pass-through).
- No cross-pod or cross-process coordination: coalescing is per-process.
- No change in the anonymizer service or any other consumer repository; wiring the new knobs there is a separate follow-up.
- No hard guarantee on exact compute counts under pathological races beyond the singleflight contract, and no attempt to bound queued waiters per key.

## Proposed Change

Add two opt-in fields to the analyzer cache configuration: a boolean to enable per-key singleflight coalescing of identical concurrent miss computations, and a float percentage that jitters every per-operation TTL within ±N% of the configured base. The rule runners coalesce the match-and-save segment of a miss per entity+data key when the boolean is enabled and keep their current behavior otherwise; the cache stays a pure storage contract. The rule-matching cache computes a jittered TTL per operation for both server-side SET expiry and client-side cache reads. Zero values for both knobs preserve today's behavior exactly.

## Capabilities

### New

- `rule-matching-cache`: Opt-in coalescing of identical concurrent cache-miss computations and opt-in per-write TTL jitter for the rule-matching cache.

### Modified

- None.

### Removed

- None.

## Impact

Affects `analyzer/cache` (RuleMatchingCache and its options, jitter only) and `analyzer` (both runners' miss paths, a new coalescing helper, and the factory that maps configuration). Adds one direct dependency: `golang.org/x/sync/singleflight` (the module is already an indirect dependency, so this only promotes the existing version and vendors one package). The exported `CacheStore` interface is unchanged; new capability surface is additive (`analyzer.CacheOptions` fields). The serial runner constructor gains a `RunnerOptions` parameter — a breaking change for direct callers, matching the concurrent runner's existing shape. Existing callers that set nothing retain identical behavior.

## Open Questions

- None.
