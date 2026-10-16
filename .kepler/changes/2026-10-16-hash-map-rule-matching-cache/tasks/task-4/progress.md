---
artifact: task-progress
change: 2026-10-16-hash-map-rule-matching-cache
task: 4
status: done
updated: 2026-10-16
decision: accepted
---

# Task Progress: Task 4 — Remove the legacy per-pair cache contract

## Attempt 1 — 2026-10-16

- Worker: coder (pi subagent session 01a14685, parent coordinator run 01a144e0) under the authorized coordination deviation recorded for this change; dispatch identity readback verified HEAD `b03cd8ff12af520a8ccd870bd8c89bb8a905fe9d`, source `refs/heads/hash-map-rule-matching-cache`, target `refs/heads/main`, linked worktree root, before any mutation.
- Inputs standing: current — plan frontmatter intent approvals at commit `2269fc9c1b0471ca2e87bf4dce004a58f1239865`; input blobs verified (`design.md` `eb9c20a0…`, `AGENTS.md` `5306df3c…` via `git hash-object`).
- Red (exceptional — justified alternative): a conventional failing behavioral check is technically impossible for a pure removal; there is no new runtime behavior to observe before the edit. Repeatable alternative per the plan: `git grep -n 'GetMatch(\|SaveMatch(\|ErrCacheNotFound\|IsCacheNotFoundError\|TraceCacheGet(\|TraceCacheSave(\|singleflightKey' -- analyzer monitoring` → observed 81 references across 11 files (the dead contract surface; count > 0 is the pre-change defect state). `singleflightKey` was already absent (removed by the Task 2/3 flow rework) — nothing to remove there.
- Green:
  - Removed `GetMatch`/`SaveMatch` from the `CacheStore` interface (`analyzer/cache/store.go`) and its doc sentence about the temporary legacy methods.
  - Removed the Valkey adapter implementations `GetMatch`, `SaveMatch`, the per-pair `key()` builder and its now-unused `keyBufPool` field + `sync` import (`analyzer/cache/valkey.go`).
  - Removed the noop implementations (`analyzer/cache/noop.go`).
  - Removed the legacy per-pair span delegation from `CacheStoreTracer` and updated its doc to the token-scoped `cache.token` tag (`analyzer/cache/tracer.go`).
  - Removed the legacy entity-tagged span helpers `TraceCacheGet`/`TraceCacheSave` (`monitoring/cache.go`); the token-scoped `TraceCacheGetMatches`/`TraceCacheSaveMatches` remain.
  - Deleted `analyzer/cache/errors.go` (`ErrCacheNotFound`, `IsCacheNotFoundError`).
  - Dropped the stale factory comment "Each GetMatch and SaveMatch call emits a DataDog span." (`analyzer/factory.go`).
  - Legacy-only tests: deleted `analyzer/cache/cache_tracer_test.go` (all four tests covered only the removed per-pair tracer surface) and `analyzer/cache/ttl_jitter_test.go` (all four tests covered only the removed per-write SET-PX jitter semantics; token-scoped equivalents exist in the Task 1 suite: `TestTokenHash_TTL`, `TestTokenHash_JitterBand`, `TestTokenHash_ClientSideReads`). In `rule_matching_cache_test.go`: deleted legacy-only `TestRuleMatchingCache_BasicOperations`, `_TTLExpiry`, `_NoTTL`, `_InMemoryCacheDisabled`; rewrote `TestRuleMatchingCache_AutoPipelining` to concurrent `SaveMatches`/`GetMatches` over distinct tokens (same auto-pipelining intent, surviving surface); rewrote `TestRuleMatchingCache_ValkeyConfigMutator_ClientNameObservable`'s real cache operation to the token-scoped methods. In `monitoring/tracing_test.go`: deleted legacy-only `TestTraceCacheGet_ReturnsFinishFunc` and `TestTraceCacheSave_ReturnsFinishFunc`.
  - Test fakes that referenced the deleted sentinel or implemented only the removed surface: removed the dead `GetMatch`/`SaveMatch` methods and `pairReads`/`pairSaves` counters plus their now-structurally-impossible assertions from `analyzer/serial_runner_coalescing_test.go`, `analyzer/concurrent_runner_coalescing_test.go`, and `analyzer/coverage_gap_test.go`. The pair-counter assertions existed solely to guard the removed legacy surface and became tautological once the interface shrank; the acceptance grep is the remaining guard.
  - Moved the shared `cacheTracerTestKey` context key into `analyzer/cache/rule_matching_cache_token_test.go` (its only remaining users) so deleting the legacy tracer test file does not break the token-scoped tracer tests.
  - Regenerated the mock: `go tool mockgen -destination=analyzer/cache/mocks/mock_cache_store.go -package=cachemock github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache CacheStore` (go.mod tool directive; zero legacy identifiers in the regenerated mock).
  - Green command 1: `git grep -c … -- analyzer monitoring` → POST_reference_count=0 (grep exit 1: zero matches) — same command as Red, now at the target state.
  - Green command 2: `go test ./analyzer/cache/... ./analyzer/ -run 'TestTokenHash|TestSerialRulesRunner|TestConcurrentRulesRunner' -count=1` → 103 passed in 4 packages.
  - `go build ./...` and `go vet ./analyzer/... ./monitoring/` pass: no indirect compile breaks in `examples/`, root `rules.go`, or any other package.
- Refactor: none beyond the removal — the removal is itself the cleanup; recording that decision per the plan. Verification rerun as part of Verify below (focused suites re-passed within the full suite run).
- Verify:
  - Acceptance grep: `git grep -n 'GetMatch(\|SaveMatch(\|ErrCacheNotFound\|IsCacheNotFoundError\|TraceCacheGet(\|TraceCacheSave(\|singleflightKey' -- analyzer monitoring` → zero references (exit 1). Before: 81; after: 0.
  - `go test ./analyzer/... -count=1` → 272 passed in 5 packages (Docker available; container-backed Valkey tests ran).
  - `go test ./monitoring/ -count=1` → 9 passed in 1 package.
  - Fresh LSP diagnostics (pi-lens, source=lsp, mode=full, severity=error): `monitoring/` → no diagnostics. `analyzer/` (30 files scanned) → one finding: `rule_matching_cache_token_test.go:308 DuplicateDecl cacheTracerTestKey` — confirmed false positive from stale gopls workspace state over the `git rm` of `cache_tracer_test.go` (the other historical declaration site). Compiler evidence the code is clean: exactly one declaration repo-wide (`rg -n 'cacheTracerTestKey' -g '*.go'`), and `go build ./...`, `go vet ./analyzer/... ./monitoring/`, and `go test` all compile that exact package successfully — a true DuplicateDecl makes the package uncompilable. Not modified, per the no-code-change-for-known-pi-lens-false-positives instruction.
- Notes:
  - Quality constraint (DRY / single source of truth) applied: exactly one storage contract remains — the token-scoped `GetMatches`/`SaveMatches`; observed evidence is the zero-reference acceptance grep and the shrunken interface in `store.go`. Decision context applied: design.md Code Attribution marks the legacy surface removed; canonical spec invariants untouched (token-scoped tests pass byte-for-byte in behavior; only the shared test context key relocated and two comments dropped the word "legacy").
  - Scope discipline: pure removal, net −784 lines across 16 files; no behavior change, no new dependencies, `.golangci.yml` and CI workflows untouched (pre-existing lint-schema/zizmor findings in those untouched files are out of this task's LSP scope and behind ask-first boundaries).
  - pi-lens session-cache advisories (ℹ️ context.Background-in-handler, goroutine-loop-capture) in test files are pre-existing idiomatic patterns, not introduced defects; not modified per the same instruction.
- Outcome: done
