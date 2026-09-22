---
artifact: feedback
change: 2026-09-22-cache-singleflight-ttl-jitter
task: 4
created: 2026-09-22
status: open
decision: rejected
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
