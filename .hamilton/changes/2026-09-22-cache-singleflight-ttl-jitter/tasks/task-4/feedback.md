---
artifact: feedback
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 4
created: 2026-09-22
status: open
decision: accepted
---

# Code Feedback: Task 4 — Coalesce serial-runner cache misses with singleflight

## Pass 1 — 2026-09-22

Base: baab2c2f041a09b72a37b2a51375bd17a39a237e
Head: db948ff551d43ab2ac93d17d80eebc47e4da3e65
Verdict: changes-requested

### Blocking

- [analyzer/serial_runner.go:29] The coalescer is built from `options.Cache.SingleflightEnabled` alone, with no gate on `options.Cache.Enabled`. Through the factory path, `Cache.Enabled=false` installs the noop cache (`buildCacheStore` returns `NewNoopRuleMatchingCache()` at analyzer/factory.go:115), yet with `SingleflightEnabled=true` the runner still wraps every miss in `coalescer.do`, so N concurrent identical misses compute once and save once even though the cache backend is disabled. This contradicts the cited must-priority requirement scenario "cache backend disabled (noop cache): no coalescing occurs, each request computes independently" (requirements/rule-matching-cache.md), the design commitment "The factory consults SingleflightEnabled only when caching is enabled; with the noop cache the flag is ignored" and its error-handling row "Cache backend disabled | Factory installs the noop cache and ignores the flag; no coalescing" (design.md), and the shipped contract on the flag itself — the `CacheOptions.SingleflightEnabled` doc comment states "It is ignored when Enabled is false" (analyzer/factory.go:33-35), and this task is where the flag is first read. Fix within the plan's intent: gate the construction in `NewSerialRulesRuner`, e.g. `newSingleflightCoalescer(options.Cache.Enabled && options.Cache.SingleflightEnabled)`, and add a test asserting a noop cache (or `Enabled=false`) with the flag set computes per caller with no coalescing. (violates: requirements/rule-matching-cache.md scenario "cache backend disabled (noop cache)"; design.md "Opt-in flag shape" decision and error-handling table; `CacheOptions.SingleflightEnabled` doc contract "ignored when Enabled is false"; rubric "Explicit failure and edge handling")

### Suggestions

- [analyzer/serial_runner_coalescing_test.go] The coalesced-save-error scenario is pinned only for a single caller (`TestSerialRulesRunner_CoalescingSaveErrorIsLoggedAndContinues`); the requirement that "every caller still receives the computed false result together with the save error" relies on upstream `DoChan` sharing, which is untested here. A multi-waiter variant asserting every caller receives the shared save error and reports no-match would pin that scenario end to end.

## Pass 2 — 2026-09-22

Base: baab2c2f041a09b72a37b2a51375bd17a39a237e
Head: cae34f5feb10b5ceaa8d159dedf84e4f75ea58dd
Verdict: approved

### Blocking

- None.

### Suggestions

- [analyzer/serial_runner_coalescing_test.go] The "coalesced save fails" requirement scenario is pinned by a single caller only (`TestSerialRulesRunner_CoalescingSaveErrorIsLoggedAndContinues`); the "every caller still receives the computed false result together with the save error" half still relies on upstream `DoChan` sharing. A multi-waiter variant asserting every caller reports no-match while exactly one compute and one save occurred would pin the shared-error path end to end.
- [analyzer/serial_runner_coalescing_test.go] The "positive match remains uncached" requirement scenario is exercised only indirectly (no test matcher ever returns true), so the no-save-on-match branch of `matchAndSave` is untested in the coalesced path. A test whose matcher returns true would assert waiters receive the shared match result while zero saves are issued.

## Pass 3 — 2026-09-22

Base: baab2c2f041a09b72a37b2a51375bd17a39a237e
Head: 27e62882c402f771548c37e664f9e1892d7fd5ef
Verdict: approved

### Blocking

- None.

### Suggestions

- Evidence-only range extension: this pass re-approves the same Task 4 head after progress-touching evidence commits (4957f2a task-progress canonicalization, Task 1/3 feedback re-approvals, whole-branch review e1927c9) extended the task range. `git diff cae34f5feb10b5ceaa8d159dedf84e4f75ea58dd 27e62882c402f771548c37e664f9e1892d7fd5ef -- analyzer/serial_runner.go analyzer/singleflight_coalescer.go analyzer/serial_runner_coalescing_test.go go.mod vendor/` is empty — Task 4's production files are byte-identical to the approved Pass 2 head; the remaining range changes belong to Task 5 and evidence records already covered by their own passes and the whole-branch review.
