---
artifact: requirements-change
capability: rule-matching-cache
change: 2026-10-16-hash-map-rule-matching-cache
status: approved
created: 2026-10-16
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
decision: accepted
---

# Capability: rule-matching-cache

The rule-matching cache stores rule-match results so repeated evaluations of the same input skip re-running matchers. This change modifies the capability's storage layout, read and write command shape, coalescing scope, and TTL application point, while preserving every documented behavior of the canonical spec (.kepler/specs/rule-matching-cache.md): negative-only persistence, opt-in coalescing and jitter, client-side caching, and identical behavior when both knobs sit at their zero values. Scope: supporting subdomain in the analyzer cache context.

## ADDED Requirements

- None.

## MODIFIED Requirements

### Requirement: Cache Storage Layout

The system SHALL store rule-match results in one hash per token: the hash key is the token content under the cache namespace, and each field is an evaluated entity with a boolean-coded value ("1" for match, "0" for no match).

- Priority: must
- Rationale: collapses the per-request cache command count from tokens × entities to tokens, enabling single-command reads and batched writes.

#### Scenario: First evaluation of a token

- WHEN a token is evaluated and its hash contains no entry for an entity
- THEN that entity is a cache miss and the runner evaluates that entity's matcher.

#### Scenario: Repeat evaluation of a known token

- WHEN a token's hash contains an entry for an entity
- THEN the runner SHALL use the cached value and SHALL NOT run that entity's matcher.

#### Scenario: Partially evaluated token

- WHEN a token's hash contains entries for only a subset of entities because evaluation stopped at a positive match
- THEN entities absent from the hash SHALL be treated as cache misses on subsequent evaluations.

### Requirement: Token-Scoped Cache Reads

The system SHALL retrieve all cached results for a token with a single HGETALL command, using client-side caching when it is enabled, and SHALL treat an empty result as "nothing cached".

- Priority: must
- Rationale: one read command per token on the hot path; client-side caching keeps hot tokens off the network with server-pushed invalidation.

#### Scenario: Hot token with client-side caching enabled

- WHEN the same token is read repeatedly while client-side caching is enabled
- THEN repeat reads SHALL be served from process memory and invalidated by server-pushed messages when the token's hash changes.

#### Scenario: Backend error during read

- WHEN the token read fails
- THEN the runner SHALL log the error and treat every entity as a cache miss, proceeding with evaluation.

### Requirement: Batched Negative-Result Writes

The system SHALL persist all negative results produced during one token's evaluation in a single multi-field HSET command, and SHALL establish the hash key's TTL exactly once at the key's first write.

- Priority: must
- Rationale: one write command per token replaces one write per entity; a per-key TTL replaces per-pair TTLs.

#### Scenario: Token with no matches

- WHEN evaluation completes for a token whose evaluated entities all produced negative results
- THEN one HSET SHALL persist every evaluated entity's negative result and the key's TTL SHALL be established once.

#### Scenario: Token with a positive match

- WHEN evaluation stops at a positive match
- THEN negative results accumulated before the match SHALL be persisted, and the positive match SHALL NOT be cached.

#### Scenario: Later additions to an existing token hash

- WHEN a subsequent evaluation persists additional entities to an existing token hash
- THEN the key's expiry SHALL remain as originally established (writes SHALL NOT extend the TTL).

### Requirement: Coalescing of Concurrent Misses

When coalescing is enabled, the system SHALL coalesce identical concurrent miss computations per token: a burst of concurrent callers that miss the cache on the same token SHALL run the evaluation of that token's missing entities exactly once, SHALL persist at most one batched negative-result write, and SHALL share the results with every caller. A caller whose context is cancelled while waiting SHALL unblock with its context error while the shared computation continues. If the negative-result save fails, every caller SHALL still receive the computed results together with the save error, which each caller logs and continues past. Completed coalesced computations SHALL NOT be memoized: a later burst SHALL recompute against the current cache state. When the cache backend is disabled, coalescing SHALL NOT occur.

- Priority: must
- Rationale: preserves the canonical coalescing contract while moving its scope from entity+token to token.

#### Scenario: Burst of concurrent misses on the same token

- WHEN N concurrent callers miss the cache on the same token while coalescing is enabled
- THEN the evaluation of that token's missing entities runs exactly once and all N callers receive the shared results.

#### Scenario: Cancelled waiter

- WHEN a caller's context is cancelled while it waits for a shared computation
- THEN that caller SHALL unblock with its context error while the shared computation continues for the remaining callers.

#### Scenario: Save failure

- WHEN the batched negative-result save fails after a shared computation
- THEN every caller SHALL receive the computed results together with the save error, and each SHALL log the error and continue past.

#### Scenario: Sequential bursts on the same token

- WHEN a burst completes and a later burst misses the same token again
- THEN the later burst SHALL recompute rather than replay the completed computation.

### Requirement: Jittered Token TTL

When jitter is configured at P percent, the token hash key's TTL at establishment SHALL lie within [base×(1−P), base×(1+P)], and client-side cache read TTLs SHALL use the jittered local TTL as they do today. Zero jitter SHALL use the base TTL exactly, and a zero base TTL SHALL keep the no-expiry path with no jitter applied.

- Priority: must
- Rationale: preserves the canonical jitter mitigation at token-key granularity.

#### Scenario: Jittered establishment

- WHEN a token hash is first written with jitter configured at P percent and base TTL T
- THEN the key's TTL SHALL be within [T×(1−P), T×(1+P)].

#### Scenario: Zero jitter and zero base TTL

- WHEN jitter is zero
- THEN the base TTL is used exactly, and WHEN the base TTL is zero
- THEN the key is written without expiry and no jitter is applied.

## REMOVED Requirements

- None.

## RENAMED Requirements

- None.
