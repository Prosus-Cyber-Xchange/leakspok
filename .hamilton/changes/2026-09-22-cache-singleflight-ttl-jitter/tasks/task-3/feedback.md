---
artifact: feedback
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 3
created: 2026-09-22
status: open
decision: rejected
---

# Code Feedback: Task 3 — Pass RunnerOptions to the serial rules runner (breaking)

## Pass 1 — 2026-09-22

Base: ed1aaf8a5f50d12889e00002905af68626e1a97f
Head: ebb5c2b8c14d252681c5b13ca3565190069e74a7
Verdict: changes-requested

### Blocking

- [README.md:82, README.md:120] Both snippets still call the old two-argument constructor `analyzer.NewSerialRulesRuner(logger, nil)`. At the checkpoint base this was type-valid against `(logger *slog.Logger, cache analyzercache.CacheStore)`; after this task's breaking change the constructor takes three arguments, so both documented examples no longer compile. `README.md` is the library's primary public usage surface and the canonical direct-caller example the design explicitly promises to make "a one-line mechanical change for direct callers", yet it is absent from Task 3's Files list and was left untouched. Insert `analyzer.RunnerOptions{}` as the second argument in both snippets (minimal fix: `analyzer.NewSerialRulesRuner(logger, analyzer.RunnerOptions{}, nil)`) and add `README.md` to the task's Files so the propagated call sites are complete. (violates: Task 3 acceptance "Every call site in the repo compiles against the new signature"; rubric "Scope integrity"/"Correctness" — the breaking change left a named, unchanged documented call site stale. Plan/design constraint: the plan's step 4 file enumeration omitted README.md, which is the causal omission; the fix stays within the pre-approved break's intent.)

### Suggestions

- None.

## Pass 2 — 2026-09-22

Base: ed1aaf8a5f50d12889e00002905af68626e1a97f
Head: fad32242f02d9484bbca824c6a6b7f563436d999
Verdict: approved

### Blocking

- None.

### Suggestions

- [README.md:82, README.md:120] Verified coverage: both snippets now call the three-argument constructor exactly as Pass 1 prescribed, the full suite is green under `go test -race -count=1 ./...` (all packages), and `SingleflightEnabled` is confirmed defined-but-unread (grep shows it only in `analyzer/factory.go`), preserving the no-behavior-change criterion.

## Pass 3 — 2026-09-22

Base: ed1aaf8a5f50d12889e00002905af68626e1a97f
Head: a24c6f55061ed3910efb9aff99d5be84297b99d3
Verdict: approved

### Blocking

- None.

### Suggestions

- [range extension] Head extended from Pass 2's fad32242f02d9484bbca824c6a6b7f563436d999 to a24c6f55061ed3910efb9aff99d5be84297b99d3 to cover the latest progress-touching and bookkeeping commits (task-progress canonicalization 4957f2a, whole-branch review e1927c9, Task 1 feedback re-approval a24c6f5). Task 3's own production files (analyzer/factory.go, examples/basic/main.go, examples/custom-rules/main.go, analyzer/serial_runner_test.go, analyzer/coverage_gap_test.go, analyzer/runner_behavior_test.go, analyzer/byte_analyzer_test.go, README.md) are byte-identical to the Pass 2 head; the only Task 3-lane file differing in the range is analyzer/serial_runner.go, whose changes are Task 4's coalescing (db948ff, cae34f5) already covered by Task 4's own feedback passes and whole-branch review e1927c9. Re-verified at the new head: three-argument `NewSerialRulesRuner` signature with options stored, `CacheOptions.SingleflightEnabled` with the required doc comment, `go build ./...` clean, and `go test -race -count=1 ./...` fully green.
