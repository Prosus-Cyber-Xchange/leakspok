---
artifact: review
change: 2026-09-22-cache-singleflight-ttl-jitter
created: 2026-09-22
status: complete
decision: accepted
---

# Whole-branch Review: Cache Singleflight Coalescing and TTL Jitter

## Pass 1 — 2026-09-22

Base: 1052a80292fa63306c5a2e4a172546ea3ec56aeb
Head: 864e29f761c80df72e38af553cfd453849cb2060
Verdict: approved

### Blocking

- None.

### Suggestions

- [focused verification] Resolved behavioral doubts with narrow checks only: `go build ./...` succeeds; `gofmt -l` clean; `go vet ./analyzer/` clean; `go test -race -count=1 -run 'Coalesc|Coalescing' ./analyzer/` passes all 10 serial/concurrent coalescing tests (burst one-compute/one-save, sequential-burst recompute, disabled per-caller, noop-cache gate, cached-hit zero computes, cancelled-waiter unblock, save-error log-and-continue); `DOCKER_HOST=unix:///Users/caio.cavalcante/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test -race -count=1 -run 'TestRuleMatchingCache_TTLJitter' ./analyzer/cache/` passes all four jitter tests against a real Valkey container (in-band, zero-preserves-base, no-expiry, client-side read); `DOCKER_HOST=... TESTCONTAINERS_RYUK_DISABLED=true go test -race -count=1 -run 'TestMakeByteAnalyzer_TTLJitterPercentageInBand' ./analyzer/` passes the factory-level plumbing test. Full-suite and full lint runs remain finish-work's gate.
- [AGENTS.md:230] Pre-existing stale example calls `analyzer.NewSerialRulesRunner()` — a constructor name that does not exist in any revision (the exported function is `NewSerialRulesRuner`); it was already non-compiling at the merge base, so no change in this branch made it stale. Optional: correct it to the three-argument form (`analyzer.NewSerialRulesRuner(logger, analyzer.RunnerOptions{}, ...)`) the next time project guidance docs are touched, for consistency with the README sweep in Task 3.
- [analyzer/serial_runner_coalescing_test.go] Requirement scenario "positive match remains uncached" is pinned only indirectly in the coalesced path — no test matcher ever returns true, so the no-save-on-match branch of `matchAndSave` is untested. A matcher-returns-true burst test asserting waiters receive the shared match result while zero saves are issued would pin it end to end (already recorded in Task 4 feedback Pass 2).
- [analyzer/concurrent_runner_coalescing_test.go] The coalesced save-error branch (all waiters receive the shared false result plus the save error) and the noop-cache gate are not pinned for the concurrent runner; both are enforced by the shared `singleflightCoalescer` and the identical `Enabled && SingleflightEnabled` construction gate already pinned by `TestSerialRulesRunner_CoalescingIgnoredWhenCacheDisabled`, so coverage remains optional.
- [analyzer/coverage_gap_test.go] The factory-level jitter test's in-band assertion cannot distinguish jitter from a fixed TTL (the fixed 30000 ms value lies inside [25500, 34500]); it proves the plumbing path (MakeByteAnalyzer → buildCacheStore → RuleMatchingCacheOptions → SET PX) plus the decay-compensated band read, while actual jitter spread is pinned by Task 1's cache-level spread probe (already recorded in Task 2 feedback).
