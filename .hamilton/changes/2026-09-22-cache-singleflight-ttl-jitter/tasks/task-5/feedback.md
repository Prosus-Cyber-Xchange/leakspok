---
artifact: feedback
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 5
created: 2026-09-22
status: open
decision: accepted
---

# Code Feedback: Task 5 — Coalesce concurrent-runner cache misses with singleflight

## Pass 1 — 2026-09-22

Base: c8c222c0f594347b97778ed44e5e1e08e6f5d8d0
Head: a0573630827b5bc4796f2ab4f9b06393897fa3c8
Verdict: approved

### Blocking

- None.

### Suggestions

- [analyzer/concurrent_runner_coalescing_test.go] The coalesced-save-error branch (every caller of a burst receives the shared false result plus the save error and continues) is not pinned for the concurrent runner; it is enforced by the shared helper and the unchanged `if coalesceErr != nil` log-and-continue path, but a burst-style test with a `countingCache{saveErr: ...}` would pin that requirement scenario end to end for this runner.
- [analyzer/concurrent_runner_coalescing_test.go] The noop-cache gate (`Enabled=false` with `SingleflightEnabled=true` computing per caller) is not asserted here; it is enforced by the identical `Cache.Enabled && Cache.SingleflightEnabled` construction gate approved for the serial runner (Task 4) and pinned by `TestSerialRulesRunner_CoalescingIgnoredWhenCacheDisabled`, so coverage for the concurrent runner remains optional.

## Pass 2 — 2026-09-22

Base: c8c222c0f594347b97778ed44e5e1e08e6f5d8d0
Head: 6d54f66178852111dabe6dfc902115b28ade55f3
Verdict: approved

### Blocking

- None.

### Suggestions

- [range extension] Evidence-only range extension for the finish gate: `git diff a0573630827b5bc4796f2ab4f9b06393897fa3c8 6d54f66178852111dabe6dfc902115b28ade55f3 -- analyzer/concurrent_runner.go analyzer/concurrent_runner_coalescing_test.go` is empty — Task 5's production files are byte-identical to the approved Pass 1 head. The range's other commits are evidence-only: task-progress attempt-record canonicalization (4957f2a), Task 1 feedback repair/re-approvals (3d82b89, a24c6f5), Task 3/4 feedback re-approvals (27e6288, 6d54f66), and the whole-branch review (e1927c9), whose review.md suggests no changes to Task 5's lane beyond the already-recorded optional coverage.
