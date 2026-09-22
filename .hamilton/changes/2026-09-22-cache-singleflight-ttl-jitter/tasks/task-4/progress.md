---
artifact: task-progress
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 4
status: pending
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

