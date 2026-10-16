---
artifact: task-progress
change: 2026-10-16-hash-map-rule-matching-cache
task: 1
status: done
updated: 2026-10-16
decision: accepted
---

# Task Progress: Task 1 — Extend the cache contract with token-scoped hash storage

## Attempt 1 — 2026-10-16

- Outcome: done
- Changed:
  - Created: analyzer/cache/rule_matching_cache_token_test.go
  - Modified: analyzer/cache/store.go, analyzer/cache/valkey.go, analyzer/cache/noop.go, analyzer/cache/tracer.go, monitoring/cache.go, analyzer/cache/mocks/mock_cache_store.go (regenerated), analyzer/coverage_gap_test.go and analyzer/serial_runner_coalescing_test.go (goal-serving unlisted edits: minimal GetMatches/SaveMatches shims on the hand-rolled CacheStore test fakes required for compilation after the port grew; their legacy semantics are untouched and Tasks 2–3 rework them with the runner flows)
  - Deleted: none
- Verified:
  - Red: `go test ./analyzer/cache/ -run 'TestTokenHash'` → failed for the intended reason only: `cache.SaveMatches undefined (type cache.CacheStore has no field or method SaveMatches)` / `cache.GetMatches undefined ...` (intended new-API compilation Red). One test-harness bug of the check itself (ToString on a non-addressable map value) was fixed before Green so Red failed solely for the intended reason.
  - Green: same command → 15 passed (TestTokenHash_HashStorage, TestTokenHash_TTL, TestTokenHash_JitterBand, TestTokenHash_ClientSideReads, TestTokenHash_TracerGetMatches, TestTokenHash_TracerSaveMatches; 9 subtests). One assertion discriminator of my own check was corrected in this phase (TTL-not-reset check now compares the remaining TTL against the decayed first reading instead of an elapsed-compensated 0.95×base threshold); the implementation was not behavior-corrected.
  - Refactor: extracted `useClientSideCache()` naming the shared CSC selection policy of legacy GetMatch and new GetMatches; no other real duplication (legacy pair-key and token-hash encodings share nothing; `jitteredTTL()` already drives both TTL sites). Same command → 15 passed; `gofmt -l` clean; `go vet ./analyzer/cache/... ./monitoring/` clean.
  - Adapter suite: `go test ./analyzer/cache/...` → 48 passed in 3 packages (container-backed; `docker info` OK from the start, no colima start needed).
  - Cross-package sanity: `go build ./...` → OK; `go test ./analyzer/ -run 'TestSerialRulesRunner|TestConcurrentRulesRunner' -count=1` → 68 passed (legacy contract intact for existing callers).
  - Final Verify: `go test ./analyzer/cache/ -run 'TestTokenHash' -count=1` → 15 passed; fresh pi-lens LSP diagnostics (source=lsp, mode=full, severity=all) over analyzer/cache/store.go, analyzer/cache/valkey.go, analyzer/cache/noop.go, analyzer/cache/tracer.go, monitoring/cache.go, analyzer/cache/rule_matching_cache_token_test.go → 0 diagnostics (clean=6, findings=0); the two compile-fixed analyzer test files also show 0 errors (only pre-existing style hints/typo informationals on lines the edits did not touch).
- Notes:
  - Quality constraints — interface segregation and DRY: the `leakspok:m:` key encoding lives only in the Valkey adapter (`matchKeyNamespace`/`matchKey` in analyzer/cache/valkey.go); tests reference the literal solely to observe server-side state. The adapter never decides what is cached: SaveMatches persists exactly the caller's batch (observed by the "positive results are boolean-coded as \"1\"" check); the negative-only policy stays with callers.
  - Decision context — vendored valkey-go v1.0.75 capabilities verified under vendor/: reads use `Hgetall().Key(k).Cache()` through DoCache when client-side caching is enabled and the base TTL is positive, plain Do otherwise; writes issue one multi-field `Hset().Key(k).FieldValue()...` together with `Expire().Key(k).Seconds(n).Nx()` in one DoMulti when the base TTL is positive (jittered TTL established once at first write; NX never extends an existing expiry) and a plain HSET at zero base TTL.
  - Bounded judgment within approved intent: EXPIRE is second-granular, so `expireSeconds()` rounds the jittered TTL and clamps it into the integer jitter band [ceil(base×(1−P)), floor(base×(1+P))] so the establishment TTL stays inside [base×(1−P), base×(1+P)] (observed: 20-token ±15% band test on a 10s base); sub-second base TTLs collapse onto the nearest representable second, documented in code. An empty SaveMatches batch is a no-op (HSET requires at least one field); an empty GetMatches result is an empty map with a nil error, replacing ErrCacheNotFound per design.
  - Canonical invariants preserved unchanged: positive matches are never cached (runner-owned save path, out of this task's scope), both knobs at their zero values keep behavior identical (zero-jitter and zero-base-TTL checks), and coalescing is untouched here.
  - Plan-scoped deferral honored: full `go test ./...` and `task lint` remain with the coordinator's later workflow stages per the plan's deferred-verification note; gofmt/go vet/go build ran as in-task sanity instead.
  - Known pi-lens false positive ("map write inside goroutine" in analyzer/serial_runner_test.go and analyzer/byte_analyzer_test.go): those files were not modified; fresh full LSP scans over the changed files returned zero diagnostics.
