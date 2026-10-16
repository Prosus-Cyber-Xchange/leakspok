# Task 1 Report: Add and apply the Valkey client configuration mutator

Change: `.hamilton/changes/2026-08-31-valkey-config-mutator/`
Date: 2026-08-31
Branch: `fix/valkey-read-replicas` (isolated: yes)
Status: **done**

## Task

Plan.md Task 1 — expose an optional `ValkeyConfigMutator func(*valkey.ClientOption)` on
`analyzer.CacheOptions`, forward it unchanged through `buildCacheStore` to the cache-package
options, and invoke it exactly once in `NewRuleMatchingCache` after Leakspok's complete default
mappings and immediately before `valkey.NewClient`.

## Steps executed (in order)

1. **Failing tests** — Wrote focused tests first, in two existing test files (respecting the
   task's "Created: none"):
   - `analyzer/cache/rule_matching_cache_test.go` (cache construction, package `cache_test`):
     - `TestRuleMatchingCache_ValkeyConfigMutator_NilPreservesDefaults`
     - `TestRuleMatchingCache_ValkeyConfigMutator_ObservesMappedDefaultsOnce`
     - `TestRuleMatchingCache_ValkeyConfigMutator_OverridesValidOption`
     - `TestRuleMatchingCache_ValkeyConfigMutator_InvalidUpstreamOptionFails`
   - `analyzer/coverage_gap_test.go` (public analyzer factory path, package `analyzer_test`):
     - `TestMakeByteAnalyzer_ValkeyConfigMutatorForwarded`
     - `TestMakeByteAnalyzer_ValkeyConfigMutatorNilPreservesDefaults`
     - `TestMakeByteAnalyzer_ValkeyConfigMutatorInvalidUpstreamPropagates`
   - Confirmed the expected failing state: `unknown field ValkeyConfigMutator` compile errors.

2. **Implement the seam** —
   - `analyzer/cache/options.go`: added `ValkeyConfigMutator func(*valkey.ClientOption)` to
     `RuleMatchingCacheOptions`; imported valkey-go (the only file in the cache package besides
     `valkey.go` that needs it).
   - `analyzer/factory.go`: added the field to `analyzer.CacheOptions`; imported valkey-go; the
     field is doc-commented as forwarded unchanged. No other valkey-go usage leaks into the
     analyzer package.
   - `analyzer/cache/valkey.go`: forwarded the callback from `buildCacheStore` into
     `RuleMatchingCacheOptions` (no copy/no reinterpretation).

3. **Forward in `buildCacheStore`** — `RuleMatchingCacheOptions{ValkeyConfigMutator:
   options.ValkeyConfigMutator, ...}`.

4. **Invoke at the construction boundary** — in `NewRuleMatchingCache`, after the
   `valkey.ClientOption` literal and the conditional Leakspok mappings (WriteTimeout, PoolSize,
   TLS) and immediately before `valkey.NewClient(opt)`:
   ```go
   if options.ValkeyConfigMutator != nil {
       options.ValkeyConfigMutator(&opt)
   }
   ```
   Exactly once; nil short-circuits to the existing behavior.

5. **Focused tests + refactor only for clarity** — no replica-specific options, no Leakspok-side
   validation added.

## Verification

| Command | Result |
|---|---|
| `go test ./analyzer/... -run 'Test.*ValkeyConfigMutator\|Test.*Cache'` | pass (7 mutator tests + existing Cache tests) |
| `go test ./analyzer/...` | pass |
| `go test ./...` | pass (all packages) |
| `go build ./...` | pass |
| `go test -race -count=1 ./analyzer/...` | pass (benign macOS linker warning only) |
| `go vet ./analyzer/...` | clean |
| `gofmt -l analyzer/` | clean |
| `golangci-lint run -c .golangci.yml ./analyzer/...` | only pre-existing `unused-parameter` (revive) findings in test helper stubs; byte-identical to base output (verified via `git stash` diff) |

## Acceptance check

- Optional Valkey Client Mutation: `analyzer.CacheOptions` exposes `ValkeyConfigMutator
  func(*valkey.ClientOption)`; nil preserves the established mapping — covered by
  `..._NilPreservesDefaults` (cache + factory) and existing factory default paths. ✅
- Mutator Ordering and Invocation: non-nil callback observes mapped defaults (`InitAddress`,
  `ForceSingleClient`, `DisableCache`, timeouts, pool size), runs exactly once (`calls == 1` in
  every non-nil test), and can override a valid mapped option before client creation
  (`OverridesValidOption`: mapped address is unreachable, mutator overrides `InitAddress` to the
  container, construction + `PingOnConnect` succeed only because the override won). ✅
- Upstream Validation Propagation: invalid combination (`Standalone.EnableRedirect = true` +
  `Standalone.ReplicaAddress` non-empty) fails construction synchronously with the existing
  `failed to create valkey client: …EnableRedirect and ReplicaAddress cannot be used together`
  wrap, at both the cache and public-factory (wrapped further as `failed to create cache`) level. ✅

## Code-quality self-review

- Tests assert observable behavior (invocation counts, mapped values, server connectivity via
  ping, error text) and would fail if the seam moved, was skipped, doubled, or dropped the wrap.
- Change confined to the seam + tests; no scope creep, no new files in `analyzer/`.
- Naming and structure follow the package's existing conventions; errors reuse the existing wrap.
- No dead code, stubs, debug output, or commented-out blocks.

## Notes / concerns

- Construction-success tests use the existing Testcontainers helper
  (`docker.io/valkey/valkey:8`); this matches the `analyzer/cache` package's existing test
  profile, which already required Docker for `go test ./analyzer/...`. The factory-level tests
  now also use it, so `go test ./analyzer` (single package) now requires Docker — consistent with
  CI's `-short ./...` run which already executed container tests.
- `go.mod` declares `go 1.25.0` and the local toolchain is 1.25.0; no new dependencies added.
