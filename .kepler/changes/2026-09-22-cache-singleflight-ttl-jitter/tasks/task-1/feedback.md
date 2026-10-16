---
artifact: feedback
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 1
created: 2026-09-22
status: resolved
decision: accepted
---

# Code Feedback: Task 1 — Add TTL jitter to the rule-matching cache

## Pass 1 — 2026-09-22

Base: 0e9b6308f42139f54b90512615af26fb6bc825c7
Head: fb531a62a79bc342f997d3e4b8abf03d92bd0bec
Verdict: changes-requested

### Blocking

- [analyzer/cache/ttl_jitter_test.go:66] `TestRuleMatchingCache_TTLJitter_ServerWriteInBand` asserts the band against `PTTL`, which measures *remaining* TTL and only decays: when a jittered TTL lands near the band's low edge (8500 ms for base 10 s at ±15 %), a few milliseconds of decay before the key's PTTL is read pushes the value below the asserted floor. Empirically flaky on this machine: failed 3 of ~11 runs, including PTTL 8483 and 8491 observed in 2 of 8 isolated `-count=8` iterations (plus one combined-suite run), so the task's own Verify gate (`go test -race -count=1 ./analyzer/cache/...`) is intermittently red and the progress claim of a green run is not reproducible. The decay also degrades the spread pin at ttl_jitter_test.go:68 (`±50 ms` around base): once a later key's read is over 50 ms after its write, a fixed-TTL implementation would spuriously look "spread". Fix within the plan's binding intent (the band assertion must stay): record each key's save time and assert `pttl + elapsed` against `[base*(1-P), base*(1+P)]`, or use a base TTL well above the plan's "e.g. 10 s" example so in-band decay is negligible, keeping the `assert.True(spread)` regression probe intact. (violates: rubric "Behavioral tests" — the PTTL-in-band test must reliably pass on a correct implementation and reliably fail on the fixed-TTL regression; plan step 2's PTTL requirement holds only when the measurement accounts for decay)

### Suggestions

- [analyzer/cache/ttl_jitter_test.go:107-109] `TestRuleMatchingCache_TTLJitter_ZeroPreservesBaseTTL` uses the same remaining-TTL measurement; its 200 ms tolerance is comfortable at the current 10 s base but inherits the decay race if the base is lowered — fold in elapsed-time compensation alongside the ServerWriteInBand fix for consistency.
- [plan.md step 2] Plan/design constraint (recorded per rubric): the plan's prescribed raw-PTTL in-band assertion over a 10 s base at ±15 % is inherently decay-racy because the floor coincides with the band's low edge; consider amending the example to a decay-compensated band assertion so Task 1's Verify gate is deterministic. The blocking fix above already stays within this binding intent.

## Pass 2 — 2026-09-22

Base: 0e9b6308f42139f54b90512615af26fb6bc825c7
Head: 27645c6cd69de446b4b3526e1f8fa1f458c86e7c
Verdict: approved

### Blocking

- None.

### Suggestions

- [analyzer/cache/ttl_jitter_test.go] Pass 1's blocking finding verified resolved: `TestRuleMatchingCache_TTLJitter_ServerWriteInBand` and `TestRuleMatchingCache_TTLJitter_ZeroPreservesBaseTTL` now assert decay-compensated `pttl + elapsed` against the band (5 ms slack) and base (200 ms tolerance), the `spread` regression probe is intact against the compensated value, and the previously flaky tests are empirically stable: 4 consecutive `go test -race -count=1 ./analyzer/cache/...` runs green, the `-count=8` isolation (previously PTTL 8483/8491) green, full `go test -race -count=1 ./...` green, and `golangci-lint` reports no findings in the three task-owned files.

## Pass 3 — 2026-09-22

Base: 0e9b6308f42139f54b90512615af26fb6bc825c7
Head: 3d82b89111d4d16f1717ad0f70ec698ab878be0e
Verdict: approved

### Blocking

- None.

### Suggestions

- [feedback.md Pass 1] Re-approval pass for the finish gate. `git diff 27645c6..3d82b89 -- '*.go'` is non-empty, but only because the range carries the Tasks 2-5 implementation (analyzer/factory.go, serial/concurrent runner coalescing, vendored singleflight, call-site updates) and README/example updates — none of Task 1's files (`analyzer/cache/options.go`, `analyzer/cache/valkey.go`, `analyzer/cache/ttl_jitter_test.go`) changed since the approved Pass 2 head, so Task 1's acceptance criteria hold byte-for-byte at this head; the Tasks 2-5 code has its own per-task feedback passes and the whole-branch review approval (e1927c9). The remaining range commits are evidence-only: the repaired `>` finding text (3d82b89, now "over 50 ms") and the progress-record canonicalization (4957f2a).
