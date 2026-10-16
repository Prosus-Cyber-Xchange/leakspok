---
artifact: proposal
change: 2026-10-16-hash-map-rule-matching-cache
status: approved
decision: accepted
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
created: 2026-10-16
route_unit: null
---

# Proposal: Token-Hash Data Layout for the Rule-Matching Cache

## Why

The rule-matching cache currently stores one string key per entity and token pair (`entity:token` → "0"/"1"). The ByteAnalyzer tokenizes input into N tokens and evaluates each token against M rules, and every rule performs its own cache lookup, so one request issues up to N×M cache reads and, on tokens not yet cached, up to N×M writes. With an excellent hit rate (operating experience from the anonymizer, the primary consumer), the dominant cost is the number of cache commands per request — server CPU, RESP overhead, and per-command APM spans — not cache misses.

## Goals & Success Criteria

- Cache traffic per request scales with the number of tokens, not tokens times entities: exactly one read command per token and at most one write command per token.
- Cached behavior is preserved as documented in the canonical rule-matching-cache spec: negative-only results, first-match short-circuit, exception handling, coalescing of identical concurrent miss computations, jittered TTLs, and client-side caching of hot tokens.
- Success criteria: with caching enabled, evaluating a document of N tokens against M enabled rules issues N read commands (one HGETALL per token) and at most N write commands (one batched HSET per token), verified by runner-level spies against the cache contract and by adapter tests against a real Valkey; all canonical invariants hold — positive matches are never cached, completed coalesced computations are never memoized, coalescing does not occur when the backend is disabled, and both knobs at their zero values keep behavior identical apart from the storage layout and command shape.

## Non-Goals

- No change to what is cached: results remain negative-only; positive matches are never written to the cache.
- No change to the public analyzer API other than the CacheStore interface; that interface change is **BREAKING** for CacheStore implementers, consistent with the pre-1.0 precedent set by the coalescing change.
- No changes to consumers in this repository's scope: the anonymizer adopts by re-vendoring a future leakspok release, with no consumer code changes beyond mechanical updates.
- No new cache backend: Valkey through valkey-go remains the only backend.
- No migration tooling: existing `entity:token` string entries are simply never read again and expire by TTL.

## Proposed Change

Restructure the cache data layout from one string key per (entity, token) pair to one hash per token: the key is the token content under a `leakspok:m:` namespace and the value is a hash map of entity → "0"/"1". Reads become one HGETALL per token, wrapped in client-side caching so hot tokens keep being served from process memory with server-pushed invalidation (supported for HGETALL by the vendored valkey-go 1.0.75). Writes become one multi-field HSET per token that batches every negative result produced while evaluating that token, with the key's TTL established once at first write (EXPIRE NX, jitter applied at establishment). The CacheStore contract becomes token-scoped — retrieve all cached results for a token in one call, persist a batch of results for a token in one call — and both rule runners fetch once per token and evaluate only the missing entities. The coalescing key moves from entity+token to token. **BREAKING**: the CacheStore interface changes shape; existing string-format entries expire naturally.

## Domain Context

### Ubiquitous Language

| Term | Meaning in this change | Bounded context | Source |
| --- | --- | --- | --- |
| Token | Segment of input produced by tokenization; becomes the unit the cache is keyed by | Project-wide (analyzer) | .kepler/specs/glossary.md |
| Entity | Classification a matcher reports (for example CPF, Email); becomes the hash field name | Project-wide (analyzer, pattern) | .kepler/specs/glossary.md |
| Rule-matching cache | The cache of rule matching results used by the analyzer so repeated evaluations of the same input skip the matcher | Project-wide (analyzer, cache) | .kepler/specs/glossary.md; .kepler/specs/rule-matching-cache.md |
| Matcher | The component of a rule that decides whether a piece of input matches | Project-wide (analyzer, pattern) | .kepler/specs/glossary.md |
| Exception | Whitelist mechanism excluding specific input from a rule; exception results are never cached | Project-wide (analyzer) | .kepler/specs/glossary.md |
| Valkey | The Redis-compatible cache backend, accessed through the valkey-go client | Project-wide (cache) | .kepler/specs/glossary.md |

### Subdomains and Bounded Contexts

| Subdomain | Bounded context | Responsibility | Involvement in the change |
| --- | --- | --- | --- |
| Detection and anonymization (core) | analyzer | Tokenization, rule evaluation in order, first-match short-circuit, exception checks | Both rule runners consult the cache once per token instead of once per rule |
| Caching (supporting) | analyzer/cache | Storage of match results, TTL and jitter mechanics, backend commands | Storage layout changes from per-pair string keys to one hash per token |
| Observability (generic) | monitoring | Cache tracing spans | Span granularity changes from per-entity lookups to per-token reads and writes |

## Capabilities

### New

- None.

### Modified

- `rule-matching-cache`: supporting subdomain in the analyzer cache context; the storage contract changes from per-(entity, token) string keys to one hash per token with batched writes, the coalescing scope moves to token granularity, and the TTL applies per token key.

### Removed

- None.

## Impact

Code affected: `analyzer/cache` (store interface, valkey backend, noop backend, tracer decorator, mocks, tests), `analyzer` (serial runner, concurrent runner, coalescer key), and `monitoring` (cache span helpers). The public CacheStore interface changes shape (**BREAKING**, pre-1.0). Existing string-format entries expire by TTL; no data migration is needed. The anonymizer and any other consumers adopt by re-vendoring the next release; the factory path (`MakeByteAnalyzer` / `MakeStringAnalyzer`) and its configuration knobs are unchanged. Observability dashboards keyed on per-entity cache span counts need remapping to token-granular spans.

### Affected Boundaries

| Boundary | Current responsibility or contract | Proposed impact | Coordination needed |
| --- | --- | --- | --- |
| analyzer ↔ cache | CacheStore exposes per-(entity, token) GetMatch/SaveMatch | CacheStore exposes token-scoped GetMatches/SaveMatches | **BREAKING** interface change; pre-1.0 precedent; consumers re-vendor |
| cache ↔ Valkey | Up to N×M GET/SET commands per request | N HGETALL reads plus at most N multi-field HSET writes per request | Server-side command count drops; client-side cache invalidation now happens per token key |
| cache ↔ monitoring | Two spans per (token, rule) cache operation | One read span per token read and one write span per token write | Dashboard remapping for per-entity span metrics |

## Open Questions

- None.
