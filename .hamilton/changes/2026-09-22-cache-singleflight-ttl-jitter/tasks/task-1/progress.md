---
artifact: task-progress
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 1
status: done
updated: 2026-09-22
decision: accepted
---

# Task Progress: Task 1 — Add TTL jitter to the rule-matching cache

## Attempt 1 — 2026-09-22

- Outcome: done
- Created: `analyzer/cache/ttl_jitter_test.go`
- Modified: `analyzer/cache/options.go`, `analyzer/cache/valkey.go`
- Deleted: none
- Verification:
  - `go test -race -count=1 ./analyzer/cache/...` — pass, including the four new jitter tests (PTTL in-band, zero jitter preserves base, no-expiry unaffected, client-side read path)
  - `go test -race -count=1 ./...` — pass (full suite)
  - `go build ./...` — success
  - `task lint` — no new findings in task-owned files; `analyzer/cache/valkey.go:96` G404 suppressed with a `//nolint:gosec` comment (design decision mandates `math/rand/v2`, explicitly not crypto); the 11 remaining findings are pre-existing at the task checkpoint in files outside this task's scope (verified via stash comparison against HEAD)
- Notes:
  - The helper was added verbatim from the plan, with a single `//nolint:gosec` annotation on the `rand.Float64()` line matching the repo's existing precedent (`valkey.go:59`, `pattern/regex.go:11`); the design decision "Jitter implementation in the cache" explicitly rejects crypto randomness for expiry spreading.
  - The `cacheTTL == 0` guards in `SaveMatch`/`GetMatch` were left unchanged, so the no-expiry path applies no jitter as required.
  - Testcontainers integration tests require `DOCKER_HOST=unix:///Users/caio.cavalcante/.colima/default/docker.sock` and `TESTCONTAINERS_RYUK_DISABLED=true` in this environment (colima; the testcontainers reaper could not start). This is a pre-existing environment characteristic affecting all container-backed cache tests, not a code change.
