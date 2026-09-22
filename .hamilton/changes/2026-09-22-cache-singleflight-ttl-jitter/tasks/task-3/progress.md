---
artifact: task-progress
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 3
status: done
updated: 2026-09-22
decision: accepted
---

# Task Progress: Task 3 — Pass RunnerOptions to the serial rules runner (breaking)

## Attempt 1 — 2026-09-22

Outcome: done

### Changed paths

- Created: none
- Modified:
  - `analyzer/serial_runner.go` — `NewSerialRulesRuner` signature changed to `(logger *slog.Logger, options RunnerOptions, cache analyzercache.CacheStore)`; the `options` field is stored on `SerialRulesRunner` for the coalescing work in Tasks 4–5; constructor doc comment updated.
  - `analyzer/factory.go` — `CacheOptions` gains `SingleflightEnabled bool` (default false) with a doc comment stating it coalesces identical concurrent misses for the same entity+data key and is ignored when `Enabled` is false; both `MakeByteAnalyzer` and `MakeStringAnalyzer` serial call sites pass `options`.
  - `examples/basic/main.go` — call site gains `analyzer.RunnerOptions{}`.
  - `examples/custom-rules/main.go` — call site gains `analyzer.RunnerOptions{}`.
  - `analyzer/serial_runner_test.go` — 25 mechanical call-site edits.
  - `analyzer/coverage_gap_test.go` — 2 mechanical call-site edits.
  - `analyzer/runner_behavior_test.go` — 1 mechanical call-site edit.
  - `analyzer/byte_analyzer_test.go` — 2 mechanical call-site edits.
- Deleted: none

### Verification

- `go build ./...` → success
- `go test -race -count=1 ./...` → all packages pass (root, analyzer, analyzer/cache, monitoring, pattern; examples carry no tests). Ran with `DOCKER_HOST=unix:///Users/caio.cavalcante/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true` because the plain run fails in `analyzer/cache` with a testcontainers "rootless Docker not found" panic — the default docker socket is absent in this environment (colima). The same failure reproduces on the checkpoint base with the task changes stashed, confirming it is pre-existing and environmental, not introduced by this task.
- `task lint` → 11 findings, all pre-existing: identical findings exist at the checkpoint base (`ed1aaf8`) and at the branch merge-base (`1052a80`, before this change series); diffing the lint output before/after this task shows only line-number shifts of the same findings (the two added lines in `coverage_gap_test.go`), i.e. zero new findings introduced by this task. The flagged lines are in files/lines outside this task's lane (`analyzer/cache/store.go`, `analyzer/cache/tracer.go`, `pattern/matcher_test.go`, `monitoring/tracing_test.go`, and pre-existing test-helper parameters), so they are not touched per the task's Files list.

### Notes

- Acceptance met: `NewSerialRulesRuner(logger, options RunnerOptions, cache)` compiles; `SerialRulesRunner` stores the options; `CacheOptions.SingleflightEnabled bool` (default false) with the required doc comment; every call site in the repo compiles against the new signature; full suite green; no behavior change (the flag is not read by anyone yet — verified `SingleflightEnabled` appears only in `analyzer/factory.go`).
- No `NewConcurrentRulesRunner` call was touched.
- Checkpoint `tasks/task-3/.base` (`ed1aaf8a5f50d12889e00002905af68626e1a97f`) validated and preserved; it is an ancestor of current `HEAD` and precedes this first attempt.

