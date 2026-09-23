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

## Attempt 2 — 2026-09-22

- Outcome: done
- Created: none
- Modified: `analyzer/cache/ttl_jitter_test.go`
- Deleted: none
- Verification:
  - `go test -race -count=1 ./analyzer/cache/...` — pass, repeated 5 consecutive runs (flake previously observed here is gone)
  - `go test -race -count=8 -run 'TestRuleMatchingCache_TTLJitter_(ServerWriteInBand|ZeroPreservesBaseTTL)$' ./analyzer/cache/...` — pass (the exact `-count=8` isolation that produced PTTL 8483/8491 before the fix)
  - `go test -race -count=1 ./...` — pass (full suite)
  - `go build ./...` — success
  - `task lint` — the 11 remaining findings are unchanged and pre-existing at the task checkpoint (`0e9b630`) in files outside this task's scope (`store.go`, `tracer.go`, `coverage_gap_test.go`, `byte_analyzer_test.go`, `matcher_test.go`, `tracing_test.go`), verified via `git diff 0e9b630..HEAD` on those files being empty; no findings in task-owned files
- Notes:
  - Context: correction after changes-requested feedback.
  - Blocking feedback fix: `TestRuleMatchingCache_TTLJitter_ServerWriteInBand` no longer asserts the band against raw PTTL (which only decays). Each save now records `savedAt`, and the assertion compares `pttl + elapsed` (effective TTL at write time) against `[base*(1-P), base*(1+P)]`, with a 5 ms slack (`bandSlackMs`) absorbing PTTL's integer-ms rounding and client-side measurement lag — negligible against the 3000 ms-wide band. The `assert.True(spread)` regression probe is intact, now evaluated against the decay-compensated value so a fixed-TTL implementation cannot spuriously look "spread" from read lag.
  - Suggestion applied: `TestRuleMatchingCache_TTLJitter_ZeroPreservesBaseTTL` uses the same `pttl + elapsed` compensation for consistency, keeping its 200 ms tolerance.
  - Plan step 2's raw-PTTL example was recorded as inherently decay-racy in feedback (Pass 1); plan.md was not edited per the feedback note.
  - Checkpoint `.base` (`0e9b630`) was validated (resolves, ancestor of HEAD and feedback `Head:`, matches feedback `Base:`) and reused untouched.
