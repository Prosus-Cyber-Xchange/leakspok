---
artifact: task-progress
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 5
status: pending
updated: 2026-09-22
decision: accepted
---

# Task Progress: Task 5 — Coalesce concurrent-runner cache misses with singleflight

## Attempt 1 — 2026-09-22

- **Outcome**: done
- **Created paths**:
  - `analyzer/concurrent_runner_coalescing_test.go`
- **Modified paths**:
  - `analyzer/concurrent_runner.go`
- **Deleted paths**:
  - none
- **Verification**:
  - `go test -race -count=1 -run 'TestConcurrentRulesRunner_Coalesces|TestConcurrentRulesRunner_CoalescingDisabled|TestConcurrentRulesRunner_CoalescingDifferent' ./analyzer/` → the three new coalescing tests fail against the unwired runner (burst computes 20× instead of 1; two keys compute 16× instead of 2), then pass after wiring
  - `go test -race -count=1 ./analyzer/...` → all pass (with `DOCKER_HOST=unix:///Users/caio.cavalcante/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true` for the cache integration tests)
  - `go test -race -count=1 ./...` → all pass
  - `go build ./...` → success
  - `go vet ./analyzer/` → clean
  - `gofmt -l analyzer/concurrent_runner.go analyzer/concurrent_runner_coalescing_test.go` → no output
  - `task lint` → fails only on pre-existing findings: the finding set is byte-for-byte identical to the base commit `c8c222c` (verified via a scratch worktree at the checkpoint); none of the findings reference Task 5's files
- **Notes**:
  - Wired `processRule` exactly per plan Step 2: the coalescer is stored on the `ConcurrentRulesRunner` struct (one shared instance across all `Process` calls) and constructed with `newSingleflightCoalescer(options.Cache.Enabled && options.Cache.SingleflightEnabled)` — the same construction gate the serial runner uses, satisfying the Task 4 review requirement "cache backend disabled (noop cache): no coalescing occurs" (requirements/rule-matching-cache.md) and the `CacheOptions.SingleflightEnabled` doc contract "It is ignored when Enabled is false".
  - Only the `rule.Matcher.Match` + conditional `SaveMatch(false)` segment is inside the flight; the early-exit selects, `GetMatch` logging, and the exception check remain exactly where they were. On a coalescer error, `ErrorContext` logs and the rule reports no match, matching the log-and-continue convention.
  - Plan Step 1's literal options text (`RunnerOptions{Cache: CacheOptions{SingleflightEnabled: true}}`) omits `Enabled: true`; under the mandated `Enabled && SingleflightEnabled` gate that would leave coalescing off, so the enabled tests set `Enabled: true` exactly like the serial runner's test file does. Disabled behavior is covered by the zero-value `RunnerOptions{}` test.
  - Test file reuses the `countingCache` / `countingMatcher` fakes from Task 4's `serial_runner_coalescing_test.go` (same `analyzer_test` package); pool size is set to the caller count so every burst caller runs concurrently and joins the flight before it completes.

