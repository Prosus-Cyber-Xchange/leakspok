---
artifact: feedback
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 1
created: 2026-09-22
status: open
decision: rejected
---

# Code Feedback: Task 1 — Add TTL jitter to the rule-matching cache

## Pass 1 — 2026-09-22

Base: 0e9b6308f42139f54b90512615af26fb6bc825c7
Head: fb531a62a79bc342f997d3e4b8abf03d92bd0bec
Verdict: changes-requested

### Blocking

- [analyzer/cache/ttl_jitter_test.go:66] `TestRuleMatchingCache_TTLJitter_ServerWriteInBand` asserts the band against `PTTL`, which measures *remaining* TTL and only decays: when a jittered TTL lands near the band's low edge (8500 ms for base 10 s at ±15 %), a few milliseconds of decay before the key's PTTL is read pushes the value below the asserted floor. Empirically flaky on this machine: failed 3 of ~11 runs, including PTTL 8483 and 8491 observed in 2 of 8 isolated `-count=8` iterations (plus one combined-suite run), so the task's own Verify gate (`go test -race -count=1 ./analyzer/cache/...`) is intermittently red and the progress claim of a green run is not reproducible. The decay also degrades the spread pin at ttl_jitter_test.go:68 (`±50 ms` around base): once a later key's read is >50 ms after its write, a fixed-TTL implementation would spuriously look "spread". Fix within the plan's binding intent (the band assertion must stay): record each key's save time and assert `pttl + elapsed` against `[base*(1-P), base*(1+P)]`, or use a base TTL well above the plan's "e.g. 10 s" example so in-band decay is negligible, keeping the `assert.True(spread)` regression probe intact. (violates: rubric "Behavioral tests" — the PTTL-in-band test must reliably pass on a correct implementation and reliably fail on the fixed-TTL regression; plan step 2's PTTL requirement holds only when the measurement accounts for decay)

### Suggestions

- [analyzer/cache/ttl_jitter_test.go:107-109] `TestRuleMatchingCache_TTLJitter_ZeroPreservesBaseTTL` uses the same remaining-TTL measurement; its 200 ms tolerance is comfortable at the current 10 s base but inherits the decay race if the base is lowered — fold in elapsed-time compensation alongside the ServerWriteInBand fix for consistency.
- [plan.md step 2] Plan/design constraint (recorded per rubric): the plan's prescribed raw-PTTL in-band assertion over a 10 s base at ±15 % is inherently decay-racy because the floor coincides with the band's low edge; consider amending the example to a decay-compensated band assertion so Task 1's Verify gate is deterministic. The blocking fix above already stays within this binding intent.
