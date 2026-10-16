---
artifact: feedback
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 2
created: 2026-09-22
status: resolved
decision: accepted
---

# Code Feedback: Task 2 — Map TTLJitterPercentage through the analyzer factory

## Pass 1 — 2026-09-22

Base: 71a5320834161aad7a88c209d4b64e979eea7cdd
Head: 078d90cf830879b364a83269f7f23affd97f4ef7
Verdict: approved

### Blocking

- None.

### Suggestions

- [analyzer/coverage_gap_test.go:118-181] Plan/design constraint (recorded per rubric): the plan-prescribed in-band assertion on a 30 s base at ±15 % cannot distinguish jitter from a fixed TTL — the fixed 30000 ms value lies inside [25500, 34500] — so this factory-level test proves the plumbing path (MakeByteAnalyzer → buildCacheStore → RuleMatchingCacheOptions → SET PX) plus the decay-compensated band read, while actual jitter spread is pinned by Task 1's cache-level spread probe. If factory-level jitter detection is ever required, add a spread probe like Task 1's; out of scope for this task as planned.
- [analyzer/coverage_gap_test.go:155,173-180] Verified coverage worth keeping: the `savedAt`-based elapsed compensation (`pttl + elapsed`) mirrors Task 1's approved decay-safe measurement, and the 9000 ms band at ±15 % on the 30 s base makes the assertion robust to ms-scale decay and rounding.
