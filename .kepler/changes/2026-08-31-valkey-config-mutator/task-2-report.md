# Task 2 Report: Verify mutator propagation with a single-node Valkey integration test

Change: `.hamilton/changes/2026-08-31-valkey-config-mutator/`
Date: 2026-08-31
Branch: `fix/valkey-read-replicas` (isolated: yes)
Status: **done**

## Task

Plan.md Task 2 — prove server-observable integration of Task 1's
`ValkeyConfigMutator` seam: a deterministic `ClientName` assigned by the callback
must be observable on a disposable single-node Valkey server after a cache
operation, using the public analyzer cache configuration path when feasible, a
distinct inspection client, and `CLIENT LIST` filtered by name — never
`CLIENT GETNAME` on the inspection connection. No multi-node routing assertion.

## Steps executed (in order)

1. **Failing Testcontainers-backed test (written first)** — Added two tests, one
   per file listed in the plan (both files already existed; `Created: none`):
   - `analyzer/cache/rule_matching_cache_test.go` (cache-package public path):
     `TestRuleMatchingCache_ValkeyConfigMutator_ClientNameObservable` — builds the
     cache store through the public `NewCacheStore` factory with a mutator setting
     `opt.ClientName`, performs a real `SaveMatch`/`GetMatch` round trip against the
     container, then inspects via a separate client.
   - `analyzer/coverage_gap_test.go` (public analyzer factory path):
     `TestMakeByteAnalyzer_ValkeyConfigMutator_ClientNameObservable` — configures
     `analyzer.CacheOptions.ValkeyConfigMutator` through the public
     `MakeByteAnalyzer` factory, performs real cache writes (SET) and reads (GET)
     via two `Anonymize` passes over a plain input (the serial runner saves a
     negative result on the first pass and serves it from the cache on the second),
     then inspects via a separate client.
   - The seam already existed (Task 1), so the tests pass immediately; their
     "failing" proof was demonstrated by temporarily neutralizing the mutator
     invocation in `analyzer/cache/valkey.go` (`if false && …`), re-running both
     tests → both FAIL with `CLIENT LIST did not report connection named
     "<name>"`, then restoring the code and re-running → both PASS. The tests
     assert genuinely server-observable behavior and would catch a regression of
     the seam (forwarding, invocation, or ordering).

2. **Separate inspection client + CLIENT LIST filter** — Added
   `assertValkeyClientName(ctx, t, addr, clientName)` (per test file; unexported
   helpers cannot cross packages) which:
   - creates a distinct `valkey.Client` on the same container address (empty
     `ClientName`, so it cannot match the filter),
   - issues `CLIENT LIST` (`inspector.B().ClientList().Build()`),
   - parses the bulk response line-by-line and field-by-field, asserting an exact
     `name=<clientName>` field exists.
   - `CLIENT GETNAME` is never used (documented in the helper comment): run on the
     inspection connection it can only report that connection's own empty name.
   - Note: Valkey 8 rejects the server-side filter `CLIENT LIST NAME <name>`
     ("ERR syntax error", verified empirically against `docker.io/valkey/valkey:8`
     before writing the test), so "filtered by the configured name" is implemented
     by filtering the returned lines client-side on the exact `name=` field — the
     robust, version-portable interpretation.

3. **Independent of private internals** — Both tests use only public surfaces
   (`NewCacheStore`/`RuleMatchingCacheOptions` and `MakeByteAnalyzer`/`CacheOptions`
   plus the `CacheStore` operations performed through `Anonymize`); no cache
   internals are touched.

4. **Run package integration tests and analyzer suite** — See Verification.

## Verification

| Command | Result |
|---|---|
| `go test ./analyzer/... -run 'Test.*ValkeyConfigMutator_ClientNameObservable'` | pass (2/2, one container each) |
| `go test ./analyzer/...` (plan Verify) | pass (analyzer + cache, incl. all container tests) |
| `go test ./...` | pass (all packages) |
| `go build ./...` | pass |
| `gofmt -l analyzer/` | clean |
| `golangci-lint run -c .golangci.yml ./analyzer/...` | diff against clean base → identical findings (only pre-existing `unused-parameter` in test stubs + `cache.CacheStore` stutter; my new `context-as-argument` findings were fixed by moving `ctx` to first parameter) |
| `go test -race ./analyzer/...` | pre-existing flake `TestConcurrentRulesRunner_Stop_IsIdempotentConcurrently` (~1/5 runs) — reproduced identically on the clean base via `git stash` (1/5), unrelated to this task |

## Acceptance check

- "A mutator configures an upstream option": a `ClientName` assigned by the
  callback is observable on a disposable single-node Valkey server. ✅ Both tests
  assert the name appears in Valkey's `CLIENT LIST`; proven failing when the
  mutator invocation is removed.
- "The test uses the public analyzer cache configuration path when feasible,
  performs a cache operation, and does not require or claim multi-node
  replica-routing proof." ✅ `TestMakeByteAnalyzer_…` exercises
  `analyzer.CacheOptions.ValkeyConfigMutator` end-to-end through `MakeByteAnalyzer`
  with real cache SET/GET operations; the cache-layer test covers the
  cache-package public path (`NewCacheStore`). No replica-routing or multi-node
  claim anywhere; the plan's second modified file clause ("if required to exercise
  the public factory") is satisfied by the analyzer-package test.
- Plan step 3: inspection uses a distinct client and `CLIENT LIST` filtered by the
  configured name; `CLIENT GETNAME` is never issued on the inspection connection. ✅

## Code-quality self-review

- Tests assert real server-observable behavior and fail if the seam breaks
  (verified empirically via the temporary `if false` edit).
- Change confined to the two plan-listed test files; zero production changes
  (`git diff` shows only test files); no new dependencies (testcontainers and
  valkey-go already used by the package).
- Naming/structure follow package conventions (`TestRuleMatchingCache_*`,
  `TestMakeByteAnalyzer_*`, `startValkeyContainer` mirror helper); errors use the
  existing require/assert style.
- No dead code, stubs, debug output, or commented-out blocks.

## Notes / concerns

- The analyzer package re-gained a local `startValkeyContainer` mirror helper
  (unexported helpers cannot be shared across packages; the review's note
  anticipated this: "if Task 2 adds more container tests, consider a shared
  test-support helper"). `go test ./analyzer` (single package) now requires Docker
  again — expected, since Task 2's Verify explicitly targets the
  Docker/Testcontainers environment.
- `go test -race ./analyzer/...` has a pre-existing flake in
  `TestConcurrentRulesRunner_Stop_IsIdempotentConcurrently` (concurrent runner
  Stop path, untouched by this change); reproduced on the clean base at the same
  rate. Flagged for review; not addressed here (out of Task 2 scope).
- Valkey 8 does not support the server-side `CLIENT LIST NAME` filter; the test
  filters client-side on the exact `name=` field instead.
