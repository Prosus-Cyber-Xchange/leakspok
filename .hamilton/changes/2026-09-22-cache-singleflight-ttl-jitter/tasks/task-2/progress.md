---
artifact: task-progress
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 2
status: done
updated: 2026-09-22
decision: accepted
---

# Task Progress: Task 2 — Map TTLJitterPercentage through the analyzer factory

## Attempt 1 — 2026-09-22

- Outcome: done
- Created: none (extended the existing `analyzer/coverage_gap_test.go`)
- Modified: `analyzer/factory.go`, `analyzer/coverage_gap_test.go`
- Deleted: none
- Verification:
  - `go test -race -count=1 -run TestMakeByteAnalyzer_TTLJitterPercentageInBand ./analyzer/` — red before the mapping (build failed: `unknown field TTLJitterPercentage in struct literal of type analyzer.CacheOptions`), green after (PASS)
  - `go test -race -count=1 ./analyzer/...` — all pass
  - `go test -race -count=1 ./...` — all pass (full suite)
  - `go build ./...` — success
  - `task lint` — identical 11 pre-existing findings as at the task checkpoint, all in files/lines outside this task's scope (verified by linting the clean checkpoint via stash); zero new findings in task-owned files
- Notes:
  - The plan's step 2 prescribes running the new integration test and watching it fail before the mapping exists, so the test was authored and run first: it failed to build exactly as prescribed (`TTLJitterPercentage` absent from `analyzer.CacheOptions`). Plan step 1's field and `buildCacheStore` forwarding were then implemented, turning the test green. The prescribed test and band semantics were not changed.
  - The PTTL read is compensated with the time elapsed since the write (`pttl + elapsed`) because PTTL only decays, mirroring Task 1's cache-level jitter tests; the ±15% band on the 30s base TTL is 9000 ms wide, so ms-scale decay and integer rounding cannot affect the assertion.
  - Zero-value behavior is preserved: a zero `TTLJitterPercentage` forwards 0 into `RuleMatchingCacheOptions`, where `jitteredTTL()` returns the base TTL unchanged, satisfying requirement "Zero configuration preserves existing behavior".
  - Testcontainers integration tests require `DOCKER_HOST=unix:///Users/caio.cavalcante/.colima/default/docker.sock` and `TESTCONTAINERS_RYUK_DISABLED=true` in this environment (colima); pre-existing environment characteristic, not a code change.
