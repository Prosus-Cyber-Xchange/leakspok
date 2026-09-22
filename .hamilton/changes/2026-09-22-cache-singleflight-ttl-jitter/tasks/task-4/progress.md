---
artifact: task-progress
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 4
status: done
updated: 2026-09-22
decision: accepted
---

# Task Progress: Task 4 — Coalesce serial-runner cache misses with singleflight

## Attempt 1 — 2026-09-22

Outcome: blocked

### Changed paths

- Created:
  - `analyzer/singleflight_coalescer.go` — the `singleflightCoalescer` type, copied verbatim from plan.md Step 1 (struct `{ group singleflight.Group; enabled bool }`, `newSingleflightCoalescer`, and `do` with a value receiver).
  - `analyzer/serial_runner_coalescing_test.go` — package `analyzer_test`; counting `CacheStore` fake (`countingCache`, always-miss `GetMatch`, sync-protected save counter with configurable `saveErr`), `cachedFalseCache` (cached false hit), `newSerialRunnerWithCache` factory, counting matcher via `pattern.PatternFunc` with an atomic counter, and the five required tests (a) burst of 20 identical concurrent misses, (b) two sequential rounds recompute, (c) disabled computes per caller, (d) cached-hit short-circuit skips matcher, (e) cancelled waiter unblocks promptly without computing.
- Modified:
  - `analyzer/serial_runner.go` — `SerialRulesRunner` gains a `coalescer *singleflightCoalescer` field built in `NewSerialRulesRuner` from `options.Cache.SingleflightEnabled`; the miss path (after the exception check) wraps matcher + conditional `SaveMatch(false)` in `coalescer.do` keyed by `string(rule.Matcher.Entity()) + ":" + string(data)`; `GetMatch` logging and the exception check untouched; on coalescer error, `ErrorContext` logs and the rule is skipped.
  - `go.mod` — `golang.org/x/sync v0.20.0` promoted from indirect to direct requirement (via `task vendor`).
  - `go.sum` — unchanged: the v0.20.0 checksums were already present from the module's prior indirect use, so `go mod tidy` added nothing.
  - `vendor/modules.txt` and vendored sources — `golang.org/x/sync/singleflight` added; unrelated vendored files reformatted by `go mod vendor` under Go 1.25.0 (doc-comment normalization).
- Deleted: none

### Verification

- `go test -race -count=1 -run 'TestSerialRulesRunner_CoalescesConcurrentMisses' ./analyzer/` (before wiring) → FAIL as expected: `expected: 1, actual: 20` for both computes and saves, proving the coalescing assertions fail against the unwired runner.
- `go test -race -count=1 ./analyzer/...` (after wiring) → FAIL in exactly the two tests that require cross-goroutine coalescing: `TestSerialRulesRunner_CoalescesConcurrentMisses` (20 computes/20 saves instead of 1/1) and `TestSerialRulesRunner_CoalescingCancelledWaiterSkipsCompute` (2 computes instead of 1; waiter times out at 2s instead of unblocking on cancel). The disabled, sequential-burst, cached-hit, and save-error tests pass because they do not depend on sharing a flight across goroutines.
- `go vet ./analyzer/` → FAIL: `analyzer/singleflight_coalescer.go:24:9: do passes lock by value: ... singleflightCoalescer contains golang.org/x/sync/singleflight.Group contains sync.Mutex`.
- `golangci-lint run -c .golangci.yml ./analyzer/` → FAIL: the same `copylocks` finding plus `analyzer/singleflight_coalescer.go:37:10: Error return value is not checked (errcheck)` (the unchecked `res.Val.(bool)` type assertion; `.golangci.yml` sets `errcheck.check-type-assertions: true`).
- Isolated experiment (temp module outside the repo, same verbatim `do` shape): 20 goroutines with a value-receiver `do` on a shared coalescer value → **20 computes**; with an otherwise identical pointer-receiver `do` → **1 compute**. This proves the defect is the verbatim receiver, not the wiring or the test.
- `go build ./...` → succeeds; the code compiles.

### Notes

- **Blocker**: plan.md Step 1 mandates the coalescer type verbatim ("no design additions"), but that verbatim type cannot satisfy the task's own Acceptance ("burst of identical concurrent misses — one compute, one save", "waiting caller cancels — waiter unblocks without computing") or its Verify (`task lint` → clean). The `do` method has a **value receiver** over a struct containing `singleflight.Group` (which embeds `sync.Mutex`), so every call copies the flight's mutex and map into a private copy; no two goroutines ever share an in-flight call, so coalescing can never occur (empirically 20 computes, not 1). The same verbatim code fails `copylocks` (govet, enabled by default in `.golangci.yml`) and `errcheck` (unchecked `res.Val.(bool)` type assertion with `check-type-assertions: true`). The minimal fixes — pointer receiver `func (c *singleflightCoalescer) do(...)` and a checked type assertion — are design additions to the mandated verbatim type, which the task explicitly forbids, so they were not applied.
- Production edits from this attempt are left uncommitted for a later retry: `analyzer/singleflight_coalescer.go` (created), `analyzer/serial_runner_coalescing_test.go` (created), `analyzer/serial_runner.go` (modified), `go.mod`/`go.sum`/`vendor/` (modified by `task vendor`). The task log and root row carry the blocked record only.
- Checkpoint `tasks/task-4/.base` (`baab2c2f041a09b72a37b2a51375bd17a39a237e`) was already established by the orchestrator, validated (resolves to a commit, is an ancestor of current `HEAD`, precedes this first attempt) and preserved; it remains untracked and untouched.
- `task lint` reports the two findings above exclusively in `analyzer/singleflight_coalescer.go`, the file whose exact content is fixed by plan.md Step 1. The remaining `analyzer` lint baseline is unchanged (the same pre-existing findings Task 3 documented).

## Attempt 2 — 2026-09-22

Outcome: done

### Changed paths

- Created:
  - `analyzer/singleflight_coalescer.go` — the `singleflightCoalescer` type per the amended plan.md Step 1 verbatim: `do` has a **pointer receiver** (`func (c *singleflightCoalescer) do(...)`), `newSingleflightCoalescer` returns `*singleflightCoalescer`, the `res.Val.(bool)` assertion is **checked** (`matched, ok := res.Val.(bool)`; on failure returns a `fmt.Errorf` with the concrete type), and `fmt` is imported. Diffed against the plan snippet: identical modulo whitespace.
  - `analyzer/serial_runner_coalescing_test.go` — package `analyzer_test`; counting `CacheStore` fake (`countingCache`, always-miss `GetMatch`, sync-protected save counter with configurable `saveErr`), `cachedFalseCache` (cached false hit), `newSerialRunnerWithCache` factory, counting matcher via `pattern.PatternFunc` with an atomic counter, and the six tests: (a) burst of 20 identical concurrent misses, (b) two sequential rounds recompute, (c) disabled computes per caller, (d) cached-hit short-circuit skips matcher, (e) cancelled waiter unblocks promptly without computing, (f) coalesced save error is logged and the rule is skipped. The burst test (a) holds the flight open with a blocking matcher until all 20 callers join (release after the leader enters the matcher plus a 50ms grace), so exactly-one-compute is deterministic under `-race` (verified `-count=10`); the original instant-matcher version was scheduling-racy (flight closed before stragglers arrived, yielding 5 computes).
  - `vendor/golang.org/x/sync/singleflight/` — vendored via `task vendor`; `singleflight.go` diffed against the module cache (`golang.org/x/sync@v0.20.0`) and identical.
- Modified:
  - `analyzer/serial_runner.go` — `SerialRulesRunner` gains a `coalescer *singleflightCoalescer` field constructed **exactly once** via `newSingleflightCoalescer(options.Cache.SingleflightEnabled)` in `NewSerialRulesRuner` (the `&coalescer` double-pointer from the partial attempt was fixed to store the constructor's pointer directly); the miss path (after the exception check) wraps the matcher + conditional `SaveMatch(false)` inside `coalescer.do` keyed by `string(rule.Matcher.Entity()) + ":" + string(data)`; `GetMatch` logging and the exception check untouched; on coalescer error, `ErrorContext` logs and the rule is skipped with no match. The match-and-save segment was extracted into a private `matchAndSave` helper to keep `Process`'s cognitive complexity at 20 (the inline flight closure pushed it to 21, a new gocognit finding; `go vet` copylocks and errcheck are clean).
  - `go.mod` — `golang.org/x/sync v0.20.0` promoted from indirect to direct requirement (via `task vendor`).
  - `vendor/modules.txt` — adds `golang.org/x/sync/singleflight`.
- Deleted: none
- Reverted: every `vendor/` file whose only diff was gofmt doc-comment whitespace churn from the local toolchain's `task vendor` run (~200 files) — restored to committed state via `git checkout`; only `go.mod`, `vendor/modules.txt`, and `vendor/golang.org/x/sync/singleflight/` carry vendor changes. `go.sum` is unchanged (v0.20.0 checksums already present from prior indirect use).

### Verification

- `go build ./...` → success.
- `go vet ./analyzer/` → clean (no copylocks; checked type assertion passes).
- `go test -race -count=1 ./analyzer/...` → PASS (all packages: `analyzer` and `analyzer/cache`). All six coalescing tests pass: burst exactly-one-compute/exactly-one-save, sequential-burst recompute, disabled per-caller compute, cached-hit zero matcher calls, cancelled waiter unblocks without computing, save-error log-and-continue.
- `go test -race -count=1 ./...` → PASS (root, analyzer, analyzer/cache, monitoring, pattern; examples carry no tests). Ran with `DOCKER_HOST=unix:///Users/caio.cavalcante/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true` (testcontainers requires colima's docker socket in this environment — same pre-existing environmental requirement documented by Task 3).
- `go test -race -count=10 -run 'TestSerialRulesRunner_CoalescesConcurrentMisses' ./analyzer/` → PASS 10/10 (deterministic exactly-one-compute after the flight-hold fix).
- `task lint` → 11 findings, all the pre-existing baseline documented by Task 3 (outside this task's lane: `analyzer/coverage_gap_test.go`, `analyzer/byte_analyzer_test.go`, `analyzer/cache/store.go`, `analyzer/cache/tracer.go`, `pattern/matcher_test.go`, `monitoring/tracing_test.go`). Zero findings in `analyzer/serial_runner.go`, `analyzer/singleflight_coalescer.go`, or `analyzer/serial_runner_coalescing_test.go`.

### Notes

- **Resolution of Attempt 1 blocker**: plan.md was amended (commit `a115a63`) to a pointer receiver + checked type assertion, an explicit once-constructed stored-pointer step, and the vendor whitespace-churn revert instruction. The partial production edits from Attempt 1 were reconciled in place: the coalescer was rewritten to the amended verbatim (pointer receiver, checked assertion, `fmt` import), the `NewSerialRulesRuner` construction was corrected to store the pointer directly, and the ~200 cosmetic-churn vendor files were reverted to HEAD. Checkpoint `tasks/task-4/.base` (`baab2c2f041a09b72a37b2a51375bd17a39a237e`) was validated (resolves, ancestor of current `HEAD`, precedes the first attempt) and preserved untracked.
- The burst test's blocking matcher deviates from a naive instant-counting matcher because an instant flight closes before 20 racing goroutines join, fragmenting the burst (empirically 5 computes). Holding the flight open until the burst joins is the only way to deterministically assert the requirement's "compute function runs exactly once" under the race detector; the assertion itself is unchanged.
- `matchAndSave` was extracted from `Process` to keep gocognit ≤ 20 (inline closure pushed it to 21 — a new lint finding this task would otherwise introduce). Behavior is identical; the plan's requirement that matcher + conditional save run inside `coalescer.do` is preserved (the helper is the `do` callback).

## Attempt 3 — 2026-09-22

Outcome: done

### Changed paths

- Created: none
- Modified:
  - `analyzer/serial_runner.go` — `NewSerialRulesRuner` now gates coalescing on the cache backend: `newSingleflightCoalescer(options.Cache.Enabled && options.Cache.SingleflightEnabled)` (with a doc comment explaining the noop-cache case), per feedback Pass 1's blocking finding. The coalescer type, key shape, miss-path wiring, and log-and-continue handling are unchanged.
  - `analyzer/serial_runner_coalescing_test.go` — added `TestSerialRulesRunner_CoalescingIgnoredWhenCacheDisabled` with two subtests: (a) "noop cache backend" passes `analyzercache.NewNoopRuleMatchingCache()` with `SingleflightEnabled: true` (mirrors the factory path where `Enabled=false` installs the noop cache) and asserts 20 concurrent calls produce 20 computes; (b) "enabled flag false" passes `Enabled: false, SingleflightEnabled: true` with the counting cache and asserts 20 computes and 20 saves, consistent with the existing disabled-mode test. The five coalescing-ON tests now set `Enabled: true` alongside `SingleflightEnabled: true` so they exercise the flight under the new gate.
- Deleted: none

### Verification

- `go test -race -count=1 ./analyzer/...` → PASS (all packages: `analyzer` and `analyzer/cache`). All six coalescing tests plus the two new disabled-backend subtests pass.
- Pre-fix proof (gate reverted, new test kept): `go test -race -count=1 -run 'TestSerialRulesRunner_CoalescingIgnoredWhenCacheDisabled' ./analyzer/` → FAIL as expected — noop-cache subtest observed **2 computes instead of 20**, enabled-flag-false subtest observed **7 computes instead of 20** — proving the test pins the bug.
- `go test -race -count=1 ./...` → PASS (root, analyzer, analyzer/cache, monitoring, pattern; examples carry no tests). Ran with `DOCKER_HOST=unix:///Users/caio.cavalcante/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true` (testcontainers requires colima's docker socket — same environmental requirement documented by Tasks 3–4). The `ld:` malformed `LC_DYSYMTAB` warnings are benign darwin linker warnings from race-instrumented test binaries, present throughout this worktree's test runs.
- `go test -race -count=5 -run 'TestSerialRulesRunner_CoalescingIgnoredWhenCacheDisabled|TestSerialRulesRunner_CoalescesConcurrentMisses' ./analyzer/` → PASS 5/5 (deterministic).
- `go build ./...` → success.
- `task lint` → 11 findings, all the pre-existing baseline documented by Task 3 / Attempt 2 (outside this task's lane: `analyzer/coverage_gap_test.go`, `analyzer/byte_analyzer_test.go`, `analyzer/cache/store.go`, `analyzer/cache/tracer.go`, `pattern/matcher_test.go`, `monitoring/tracing_test.go`). Zero findings in `analyzer/serial_runner.go`, `analyzer/singleflight_coalescer.go`, or `analyzer/serial_runner_coalescing_test.go`.

### Notes

- **Deviation (feedback-required bounded fix)**: plan.md Step 3's literal `newSingleflightCoalescer(s.options.Cache.SingleflightEnabled)` is amended to `newSingleflightCoalescer(options.Cache.Enabled && options.Cache.SingleflightEnabled)` per feedback Pass 1 (verdict changes-requested). This satisfies requirement scenario "cache backend disabled (noop cache): no coalescing occurs" (requirements/rule-matching-cache.md), design.md "Opt-in flag shape" and its error-handling row "Cache backend disabled | Factory installs the noop cache and ignores the flag; no coalescing", and the `CacheOptions.SingleflightEnabled` doc contract "It is ignored when Enabled is false" (analyzer/factory.go). It is the only deviation; everything else in the task's Steps is unchanged.
- With the gate in place, the five coalescing-ON tests were updated to set `Enabled: true` — a test-only consequence of the corrected semantics, not a behavior change in the runner's miss path.
- Feedback Pass 1's optional (non-blocking) suggestion to extend the save-error test to multiple waiters sharing one save error was considered and deliberately not applied, keeping this correction bounded to the blocking fix; the single-caller save-error test still passes and the shared-error semantics remain pinned by the requirement text.
- Checkpoint `tasks/task-4/.base` (`baab2c2f041a09b72a37b2a51375bd17a39a237e`) validated (resolves, is an ancestor of current `HEAD` and of feedback Head `db948ff551d43ab2ac93d17d80eebc47e4da3e65`, precedes the first attempt) and preserved untracked; the working tree had no pre-existing task-owned changes before this attempt.
- The comment block added to `NewSerialRulesRuner` initially produced a `gci` "File is not properly formatted" finding (struct-literal field alignment); resolved with `gofmt -w` and re-verified lint-clean for this file.



