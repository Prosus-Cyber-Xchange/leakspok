---
artifact: requirements-change
capability: rule-matching-cache
change: 2026-09-22-cache-singleflight-ttl-jitter
status: approved
created: 2026-09-22
author: OpenCode
decision: accepted
---

# Capability: rule-matching-cache

The rule-matching cache serves cached rule-match results from Valkey, coalesces identical concurrent miss computations when enabled, and spreads entry expiry with jittered per-write TTLs when enabled.

## ADDED Requirements

### Requirement: Opt-in coalescing of identical concurrent miss computations

The system SHALL, when coalescing is enabled, run the match computation for a given entity+data cache key at most once among concurrent callers that all missed the cache, and SHALL persist at most one negative result for that key.

- Priority: must
- Rationale: under replication lag, per-request miss handling turns one slow miss into N duplicate regex runs and N duplicate SETs, amplifying the write storm; coalescing removes the amplification at the compute and write layers while leaving reads to valkey-go's existing auto-pipelining.

#### Scenario: burst of identical concurrent misses

- WHEN coalescing is enabled and N concurrent requests resolve the same entity+data key that is not present in the cache
- THEN the compute function runs exactly once, all N callers receive the same boolean result, and exactly one negative-result save is issued for the key

#### Scenario: cached hit during a burst

- WHEN coalescing is enabled and the key is already present in the cache
- THEN the cached result is returned to each caller and the compute function runs zero times

#### Scenario: positive match remains uncached

- WHEN the coalesced compute returns true
- THEN the result is shared with all waiters and no save is issued, preserving the existing rule that positive matches are not cached

#### Scenario: coalesced save fails

- WHEN the coalesced compute returns false and the negative-result save fails
- THEN every caller still receives the computed false result together with the save error, which callers log and continue on, matching today's log-and-continue convention

#### Scenario: waiting caller cancels

- WHEN a caller's context is cancelled while it waits on a shared computation
- THEN that caller unblocks with its context error and the shared computation continues for the remaining callers

#### Scenario: sequential bursts recompute

- WHEN a burst completes and a later, separate burst of the same key misses the cache again
- THEN the later burst computes again rather than replaying a memoized earlier result

#### Scenario: coalescing disabled

- WHEN coalescing is disabled
- THEN each miss computes and saves independently, exactly as today

#### Scenario: cache backend disabled

- WHEN the cache backend is disabled (noop cache)
- THEN no coalescing occurs and each request computes independently

### Requirement: Opt-in per-write TTL jitter

The system SHALL, when a jitter percentage greater than zero is configured, randomize the TTL of every cache write and of every client-side cache read so each effective TTL lies within the configured percentage band around the base TTL.

- Priority: must
- Rationale: a fixed TTL on every write makes burst-written keys expire in lockstep, producing thundering-herd expiry waves and periodic write storms; jitter spreads expirations uniformly so a load surge cannot produce self-sustaining waves.

#### Scenario: server-side write jitter

- WHEN jitter is configured at P percent of a positive base TTL
- THEN each negative-result save issues SET PX with an effective TTL within [base*(1-P), base*(1+P)]

#### Scenario: client-side cache TTL jitter

- WHEN jitter is configured at P percent and client-side caching is enabled
- THEN each client-side cache read uses a local TTL within [base*(1-P), base*(1+P)]

#### Scenario: zero jitter preserves fixed TTL

- WHEN no jitter is configured (zero)
- THEN saves and client-side reads use the base TTL exactly, as today

#### Scenario: no-expiry TTL unaffected

- WHEN the base TTL is zero (entries never expire)
- THEN writes use a plain SET without expiry and no jitter is applied

### Requirement: Zero configuration preserves existing behavior

The system SHALL preserve current cache behavior exactly when both knobs are at their zero values.

- Priority: must
- Rationale: leakspok is a public library with consumers that did not opt into these mitigations; an additive change must not alter their observable behavior.

#### Scenario: unset knobs

- WHEN an application constructs an analyzer without setting the coalescing or jitter knobs
- THEN cache reads, writes, expiry, and concurrency behavior are identical to the previous release
