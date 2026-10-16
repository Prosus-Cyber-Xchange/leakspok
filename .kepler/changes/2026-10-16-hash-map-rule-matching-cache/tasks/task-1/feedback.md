---
artifact: feedback
change: 2026-10-16-hash-map-rule-matching-cache
task: 1
created: 2026-10-16
status: resolved
decision: accepted
---

# Code Feedback: Task 1 — Extend the cache contract with token-scoped hash storage

## Pass 1 — 2026-10-16

Base: f589f25d40afab9b3ae515eced42a4a66b220301
Head: a734b25d750d33f5f0c5f55269fae9cc4b6805e0
Verdict: approved
Scope: full
Target: refs/heads/hash-map-rule-matching-cache

### Blocking

- None.

### Suggestions

- [task range f589f25..a734b25] Verified scope: the range is the single task commit a734b25 "feat(cache): add token-scoped hash storage contract" (no peer commits to certify), judged against Task 1 of plan.md, the approved intent at 2269fc9c1b0471ca2e87bf4dce004a58f1239865 (requirements scenarios "Cache Storage Layout", "Token-Scoped Cache Reads", "Batched Negative-Result Writes", "Jittered Token TTL"; design.md Architecture, Ports & Adapters, and Decisions), the canonical invariants in .kepler/specs/rule-matching-cache.md, and AGENTS.md standards.
- [analyzer/cache/store.go] Verified Red plausibility: at Base the CacheStore interface has only GetMatch/SaveMatch, so the new test file could only have failed on the missing GetMatches/SaveMatches — the intended new-API compilation Red recorded in progress.md.
- [analyzer/cache] Verified by re-run at Head (Docker 27.4.0 available throughout): `go test ./analyzer/cache/ -run 'TestTokenHash' -count=1` → 15 passed, matching progress.md; `go build ./...` OK; `gofmt -l` clean; `go vet ./analyzer/cache/... ./monitoring/` clean; `go run go.uber.org/mock/mockgen` output differs from the committed mock only in the header destination-path comment (faithful regeneration); `go test ./analyzer/ -run 'TestSerialRulesRunner|TestConcurrentRulesRunner' -count=1` → 68 passed with the legacy methods retained for existing callers.
- [analyzer/cache/rule_matching_cache_token_test.go] Verified behavior against server-side state via an independent inspector client: exactly one hash key `leakspok:m:<token>` with one "0"-coded field per evaluated entity ("1" when the caller provides a positive), the legacy per-pair key not written, GetMatches returning exactly the stored fields, an unknown token yielding a non-nil empty map with nil error (replacing ErrCacheNotFound per design), a second SaveMatches leaving the remaining TTL decaying (EXPIRE NX never extends), zero jitter landing on the base TTL exactly, a zero base TTL writing PTTL −1 even with jitter configured, both read paths (DoCache when CSC is enabled with a positive base TTL, plain Do otherwise) exercised, and tracer spans forwarding the caller context token-tagged.
- [analyzer/cache/valkey.go:expireSeconds] Verified by inspection: round-then-clamp into [ceil(base×(1−P)), floor(base×(1+P))] keeps every whole-second band inside the required [base×(1−P), base×(1+P)] (tested at ±15% on a 10s base over 20 tokens, elapsed-compensated PTTL, with a spread assertion proving jitter is applied); the EXPIRE-0 guard (seconds < 1 → 1) ensures the key is never deleted; zero-jitter whole-second bases reproduce the base exactly.
- [analyzer/cache/valkey.go] Verified the `useClientSideCache()` refactor: widening legacy GetMatch's plain-Do condition from `cacheTTL == 0` to `cacheTTL > 0` is behavior-identical across the documented input domain (0 = no expiry, positive = TTL) and harmonizes the read path with the write path's existing non-positive treatment.
- [analyzer/coverage_gap_test.go, analyzer/serial_runner_coalescing_test.go] Verified the goal-serving unlisted compile shims: minimal neutral implementations on the hand-rolled CacheStore test fakes (empty-map/nil token-scoped behavior, save counters preserved, comments keeping legacy semantics on the legacy methods), necessary after the port grew, bounded to exactly the files implementing CacheStore by hand, and reworked by Tasks 2–3 with the runner flows — accepted as within approved intent and dependency-mode scope integrity.
- [analyzer, monitoring] Verified hygiene: fresh LSP diagnostics (source=lsp, mode=full, severity=all) over the eight changed Go files show zero errors, only pre-existing waitgroupgo hints (analyzer/serial_runner_coalescing_test.go:130/194/228/256) and `ba` typo informationals (analyzer/coverage_gap_test.go) on lines the edits did not touch; no stubs, dead code, TODOs, debug output, or weakened tests; the `leakspok:m:` encoding is produced only by the adapter (tests reference the literal solely to observe server-side state, per the task's quality note).
- [analyzer/cache/valkey.go:expireSeconds] Sub-second base TTLs collapse onto the nearest representable second and can land outside the literal jitter band (e.g. a 0.5s base yields EXPIRE 1s); this is inherent to the approved second-granularity EXPIRE decision and is documented in code. If sub-second TTLs ever become a real configuration, PEXPIRE NX would satisfy the band exactly — a design change, not a requested correction.
- [analyzer/cache/rule_matching_cache_token_test.go:TestTokenHash_ClientSideReads] The CSC test matches the legacy suite's depth (repeated reads return the stored map); a future enhancement could additionally observe server-push invalidation by mutating the hash through the inspector client and re-reading.
