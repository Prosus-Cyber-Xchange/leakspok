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
