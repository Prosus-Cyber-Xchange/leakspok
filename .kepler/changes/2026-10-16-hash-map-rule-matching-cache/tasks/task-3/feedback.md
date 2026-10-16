---
artifact: feedback
change: 2026-10-16-hash-map-rule-matching-cache
task: 3
created: 2026-10-16
updated: 2026-10-16
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
status: resolved
decision: accepted
---

# Code Feedback: Task 3 — Route the concurrent runner through token-scoped evaluation

## Pass 1 — 2026-10-16

Base: 50adc309c3c9854e240c762db622a4978e03c241
Head: a55fae54422dada0cc80e687d812fd505a71d63f
Verdict: approved
Scope: full
Target: refs/heads/hash-map-rule-matching-cache

### Blocking

- None.

### Suggestions

- [task range 50adc309..a55fae5] Verified scope: the range is the single task commit a55fae5 "feat(analyzer): token-scoped evaluation in the concurrent runner" (the amended continuation of the preserved WIP dispatch b8940fb; no peer commits inside the range — the base is Task 2's approved feedback commit), judged against Task 3 of plan.md, the approved intent at 2269fc9c1b0471ca2e87bf4dce004a58f1239865 (requirements scenarios "Cache Storage Layout", "Token-Scoped Cache Reads", "Batched Negative-Result Writes", "Coalescing of Concurrent Misses"; design.md Decisions on the concurrent runner flow, first-match cancellation unchanged, and token-scope coalescing), the canonical invariants in .kepler/specs/rule-matching-cache.md, and AGENTS.md standards. Verification re-run independently: `go test ./analyzer/ -run 'TestConcurrentRulesRunner' -race -count=1` → 44 passed; full `go test ./analyzer/ -count=1` → 248 passed (Docker available, so the container-backed TTL-jitter and TestRunnersBehaviorIdentical cases ran); `gofmt -l analyzer/` clean; `go vet ./analyzer/ ./analyzer/cache/` clean; preflight `kepler workbench lint --change-dir` succeeded before authoring.
- [analyzer/concurrent_runner.go:evaluateMissing] Verified the concurrency surface: the recorded cancellation-path correction is sound — the guard ordering (found short-circuit first, then the `ctx.Err()` residual skip, then inline residual evaluation) both preserves a genuinely collected match under cancellation and breaks the circular wait where an abandoned matcher blocked until after Process returned; the resultCh buffer is sized to the full missing set and every worker sends exactly once, so no send can block on the match, cancellation, or pool-closed exit paths (no goroutine leak); the non-blocking drain persists only completed workers' negatives in the one batched save, positives are never cached (saveNegatives filters to negatives and skips empty batches), and a pool-closed submission aborts the whole token's evaluation with no results and no matcher run. The pool profile matches the coordination constraint: the leader is the caller goroutine (holding no runner-pool slot) submitting one task per missing entity, and waiters hold no pool slots while blocked on the flight channel — pinned by TestConcurrentRulesRunner_ProcessTokenCoalescingCancelledWaiter with pool size 1. The shared results map is read-only for waiters and merges go into fresh maps; the focused suite is -race clean.
- [analyzer/concurrent_runner.go:Process] Verified the winner-selection judgment call as faithful to the approved design: on the match path the first collected evaluated match wins and cached positives are consulted only when no missing entity matched, which follows from the design's "only missing entities are submitted" (cached positives are never collected results, so the acceptance's "Process returns as soon as a collected result matches" holds literally); the pre-change concurrent runner's winner among multiple simultaneous matches was already nondeterministic (any collected match could win, cached hits included), so no documented rule-identity parity is lost — TestRunnersBehaviorIdentical pins deterministic single-match scenarios, and serial-order parity is kept on the no-evaluated-match path and for cached positives. Optional follow-up (non-blocking): a spy test explicitly pinning the cached-positive-versus-evaluated-match precedence would document the deterministic winner choice if the team wants that guarantee recorded.
- [analyzer/concurrent_runner.go:collectEvaluations] Recording the chosen reading of the design sentence "negatives collected before cancellation or match are flushed once": on a caller-context cancellation mid-collection the collected outcomes are returned without persisting, while the first-match-cancellation/match path drains completed workers and issues the one save — no requirement scenario covers caller cancellation, positives are never cached, and lost negatives recompute on the next read, so this is judged in-intent. Note the progress phrasing "matching the previous cancellation profile" is imprecise (the base persisted per-pair negatives opportunistically for workers completing before cancellation through their own SaveMatch); worth keeping in mind when kepler-finish-work folds the requirements into the canonical spec.
