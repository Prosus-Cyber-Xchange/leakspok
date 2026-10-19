---
artifact: design
change: 2026-10-16-hash-map-rule-matching-cache
status: approved
created: 2026-10-16
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
decision: accepted
route_unit: null
---

# Design: Token-Hash Data Layout for the Rule-Matching Cache

## Context

The canonical rule-matching-cache spec (.kepler/specs/rule-matching-cache.md) documents today's contract: results stored keyed by entity and data, negative-only persistence, opt-in coalescing of identical concurrent miss computations and jittered TTLs, and client-side caching of reads. In code, the storage is one string key per (entity, token) pair (`entity:token` → "0"/"1") reached through the CacheStore port (`analyzer/cache/store.go`: GetMatch/SaveMatch); both rule runners query per token per rule (`analyzer/serial_runner.go`, `analyzer/concurrent_runner.go`), the Valkey adapter issues one GET or SET per call (`analyzer/cache/valkey.go`), and the tracer decorator emits one span per call (`analyzer/cache/tracer.go`). The vendored valkey-go 1.0.75 supports client-side caching on HGETALL (`HgetallKey.Cache()`), multi-field HSET in a single command, and EXPIRE with the NX flag (verified in `vendor/github.com/valkey-io/valkey-go`). The serial runner awaits each lookup, so one token costs M sequential round trips whenever the client-side cache is cold; auto-pipelining coalesces in-flight commands on the concurrent path, but the command count remains N×M per request. Operating context from the primary consumer: excellent hit rate; the number of cache requests is the cost driver.

## Goals / Non-Goals

**Goals**

- One read command per token and at most one write command per token (requirements/rule-matching-cache.md).
- Preserve every canonical behavior: negative-only persistence, first-match short-circuit, exception handling, coalescing at token scope, jittered TTLs, client-side caching, and log-and-continue degradation on backend errors.

**Non-Goals**

- Positive-result caching; new cache backends; consumer-side changes; data migration; changes to factory wiring or configuration knobs.

## Domain Model

The cache's model shifts from a set of (Entity, Token) pairs with boolean outcomes to a map from Token to a set of (Entity → boolean outcome). Token and Entity keep their glossary meanings in the analyzer context; the word "key" shifts from meaning a pair to meaning a token under the cache namespace, and documents that cite cache keys must read it that way after this change. Rule ownership is unchanged: the runners own what gets persisted (negatives only, after exception checks) and the order of rule evaluation; the cache owns storage mechanics (key encoding, command shape, TTL). Canonical invariants stay owned where they are today: "positive matches are never cached" is enforced by the runners' save path, and "no coalescing without a backend" is enforced by the runner wiring.

## Architecture & Components

Boundaries and dependency direction are unchanged: both runners depend on the CacheStore abstraction (the outbound port of the analyzer core) and adapters implement it. The port's surface changes from per-pair methods to token-scoped methods; the Valkey adapter's command shape changes accordingly; the noop adapter and tracer decorator follow the port; the singleflight coalescer stays in the runners with a token-level flight key. The factory (`analyzer/factory.go`) keeps its wiring and knobs; only runner construction arguments change if the new flow requires them.

### Packages

| Package | Change | Responsibility | Domain / context and role | Exposes | Depends on |
| --- | --- | --- | --- | --- | --- |
| analyzer/cache | modified | Storage contract and Valkey mechanics for match results | Infrastructure role serving the analyzer cache context | Token-scoped CacheStore, options, noop backend, tracer, mocks | valkey-go |
| analyzer | modified | Rule evaluation flow: consult cache once per token, evaluate misses, batch negatives, first-match short-circuit | Core detection subdomain (analyzer context) | RuleRunner implementations, ByteAnalyzer/StringAnalyzer, coalescing helper | analyzer/cache port |
| monitoring | modified | Cache span helpers | Generic observability | Token-granular cache trace helpers | — |

### Ports & Adapters

| Port / contract | Direction and owner | Consumers / providers | Adapter and translation | Source / proposed change |
| --- | --- | --- | --- | --- |
| CacheStore | Outbound port owned by the analyzer core | Consumers: both rule runners; providers: RuleMatchingCache, NoopRuleMatchingCache, CacheStoreTracer, generated mocks | Valkey adapter translates token-scoped calls to HGETALL and multi-field HSET + EXPIRE NX; map[Entity]bool translates to and from hash fields | Existing contract modified (proposed): GetMatches(ctx, data) (map[pattern.Entity]bool, error); SaveMatches(ctx, data, map[pattern.Entity]bool) error |

### Code Attribution

| Code | Existing / proposed / removed | Domain / context or infrastructure role | Responsibility | Source |
| --- | --- | --- | --- | --- |
| analyzer/cache/store.go | existing, modified | Infrastructure (cache context) | CacheStore becomes token-scoped; GetMatch/SaveMatch are removed | analyzer/cache/store.go |
| analyzer/cache/valkey.go | existing, modified | Infrastructure (cache context) | Key encoding (namespace + token), HGETALL via DoCache when client-side caching is enabled, batched HSET + EXPIRE NX via DoMulti, jitter at establishment | analyzer/cache/valkey.go |
| analyzer/cache/noop.go | existing, modified | Infrastructure | Returns empty results (every entity a miss), discards batches | analyzer/cache/noop.go |
| analyzer/cache/tracer.go | existing, modified | Infrastructure | One read span per token read, one write span per token write | analyzer/cache/tracer.go |
| analyzer/serial_runner.go | existing, modified | Core (analyzer context) | Fetch once per token; consult the map per rule in order; evaluate misses after exception checks; flush the batch, including before an early return on a positive match | analyzer/serial_runner.go |
| analyzer/concurrent_runner.go | existing, modified | Core (analyzer context) | Pre-fetch before submitting workers; submit only missing entities; keep first-match cancellation; flush collected negatives | analyzer/concurrent_runner.go |
| analyzer/singleflight_coalescer.go | existing, modified | Core (analyzer context) | Flight key becomes the token; the flight wraps the token's miss evaluation and batched save | analyzer/singleflight_coalescer.go |
| analyzer/cache/mocks | existing, regenerated | Infrastructure | Regenerated for the new interface | analyzer/cache/mocks |
| monitoring/cache.go | existing, modified | Generic observability | Span helpers take token-scoped tags | monitoring/cache.go |

### Quality Lens

- Responsibility: the cache keeps one reason to change (storage mechanics); the runners keep one (evaluation flow); no unit gains an "and".
- Boundaries and dependencies: runners depend only on the CacheStore abstraction; the Valkey adapter is the only unit that knows command shapes; tests substitute the port (existing generated mock) or a real backend (existing Testcontainers helper in analyzer/cache/testing).
- Right-sizing: no new layers or extension points; the interface shrinks from two per-pair methods to two token-scoped methods, so callers depend only on token-scoped operations.
- Accepted smells: none outstanding.

## Decisions

- One hash per token, keyed by the raw token content under a `leakspok:m:` namespace. The namespace prevents collisions with other applications sharing the backend and marks the new format; existing unprefixed pair keys are never read again and expire by TTL.
- Reads: HGETALL through DoCache when client-side caching is enabled and the base TTL is positive; plain Do otherwise. An empty map means nothing cached and replaces ErrCacheNotFound.
- Values: "0"/"1" strings preserve the existing boolean encoding; only "0" is written today, enforcing the canonical negative-only invariant from the runners' save path.
- Writes: one multi-field HSET carrying every negative result of the token, issued together with EXPIRE NX in a DoMulti so the TTL (jittered at establishment) is applied once and never extended; a zero base TTL skips EXPIRE entirely.
- Runner flow: fetch once per token; a rule whose entity is present in the map uses the cached value; absent entities are evaluated after their exception checks; negatives accumulate in a batch flushed with one SaveMatches call, including a flush before an early return on a positive match, preserving today's behavior of persisting negatives computed before the match.
- Concurrent runner: the map is fetched before workers are submitted; only missing entities are submitted; first-match cancellation is unchanged; negatives collected before cancellation or match are flushed once.
- Coalescing: the flight key becomes the token; the flight wraps the token's miss evaluation and batched save; exception checks remain outside the flight, as the canonical design requires.
- Tracing: one read span and at most one write span per token, tagged with a short non-reversible SHA-256 digest prefix of the token — never the raw token content. Token text is unsanitized PII candidate material and must not be recorded in APM; the digest preserves token-granular span correlation without it.

## Testing Strategy

- Runner behavior (unit, mock CacheStore): a cached hit skips the matcher; a miss evaluates and batches; exception results are never persisted; an early positive match still flushes accumulated negatives; a backend read error degrades to full evaluation; partially cached tokens evaluate only absent entities.
- Adapter behavior (unit plus Testcontainers Valkey via the existing analyzer/cache/testing helper): HGETALL returns the stored map; the multi-field HSET writes all fields; EXPIRE NX establishes the TTL once and a second write does not change it; a zero base TTL writes without expiry; a repeated read is served from the client-side cache.
- Command-count success criterion: a runner-level spy asserts one GetMatches and at most one SaveMatches per token; the adapter tests assert the command shape that yields the backend-level reduction.
- Concurrency: the existing -race suite runs over the restructured runners; coalescing tests cover token-granularity bursts (one shared evaluation and one save; a cancelled waiter unblocks; a save failure is shared).
- Gates: task lint and task test/unit must pass; focused go test runs serve as per-task Verify.

## Constraints & Boundaries

- valkey-go 1.0.75 stays vendored; no new dependencies.
- Canonical invariants are restated and preserved: positive matches are never cached; coalescing does not occur without a backend; both knobs at their zero values keep behavior identical apart from layout and command shape; completed coalesced computations are never memoized.
- The cache namespace (`leakspok:m:`) belongs to the cache adapter; no other package constructs cache keys.
- **BREAKING** CacheStore change; the project is pre-1.0 and consumers adopt by re-vendoring.

## Risks / Trade-offs

- Per-pair TTL granularity is lost: expiry moves to the token key, established once with jitter. Accepted because a token's entries share one evaluation recency, and EXPIRE NX prevents warm-up writes from extending life.
- HGETALL returns the whole map per token; reply size is O(entities) per token — the same order as the M single GETs it replaces, without per-command overhead.
- Client-side cache invalidation moves to token keys: one invalidation per batched write instead of per pair, which is equal or fewer invalidation messages.
- Observability dashboards keyed on per-entity cache spans need remapping; the coordination need is recorded in the proposal's Impact.
- Alternatives rejected: an entity-keyed hash of hashes (reads stay per-entity, still M commands per request); MGET over per-entity keys (cluster mode rejects cross-slot multi-key commands, and explicit per-slot splitting degenerates to roughly M commands); document-level caching (a different hit profile; out of scope).
