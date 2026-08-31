# Review: Valkey Configuration Mutator

## Task 1 — 2026-08-31

Verdict: changes-requested

### Blocking

- [analyzer/coverage_gap_test.go:586-633, analyzer/cache/rule_matching_cache_test.go:281-352]
  Task 1's "isolated unit behavior" tests spin up a real Docker Valkey container
  (`startValkeyContainer`, `docker.io/valkey/valkey:8`) where the assertions need no live
  server. valkey-go dials asynchronously, so `NewCacheStore` returns success against an
  unreachable address — the pre-existing `TestRuleMatchingCache_PingOnConnect` "ping fails on
  unreachable address" subtest (rule_matching_cache_test.go:265-278) only errors because
  `PingOnConnect` is set. Concretely:
  - `TestMakeByteAnalyzer_ValkeyConfigMutatorForwarded` (coverage_gap_test.go:586) and
    `TestMakeByteAnalyzer_ValkeyConfigMutatorNilPreservesDefaults` (coverage_gap_test.go:619)
    assert only construction success + callback-observed values; both pass unchanged with a
    dummy address and no container.
  - `TestRuleMatchingCache_ValkeyConfigMutator_NilPreservesDefaults`
    (rule_matching_cache_test.go:281) adds a SaveMatch/GetMatch round-trip whose assertions
    (calls == 0, defaults preserved) do not need the round-trip.
  - `TestRuleMatchingCache_ValkeyConfigMutator_ObservesMappedDefaultsOnce`
    (rule_matching_cache_test.go:308) asserts only mutator-observable values; no server needed.
  - Only `TestRuleMatchingCache_ValkeyConfigMutator_OverridesValidOption`
    (rule_matching_cache_test.go:354) genuinely needs a live endpoint (PingOnConnect proving the
    override won) — and that is precisely the server-observable propagation proof the plan
    assigned to Task 2.
  This contradicts plan.md's quality note ("Task 1 owns the public-to-cache configuration seam
  and its isolated unit behavior; Task 2 owns server-observable integration proof. No structural
  smell is accepted") and regresses the analyzer package's test profile: `go test ./analyzer`
  (single package) now requires Docker, which it did not before. Fix: replace the container with
  an unreachable dummy address (e.g. `"localhost:19379"`) in the four tests above — all their
  assertions hold serverless; keep the container only where the assertion is server-observable
  (the override-wins ping), or defer that proof to Task 2 per the plan split.
  (violates: plan.md "Task 1 owns ... its isolated unit behavior; Task 2 owns server-observable
  integration proof"; design.md Testing Strategy bullet 1 "focused unit tests around cache
  construction")

### Suggestions

- [analyzer/coverage_gap_test.go:565] `startValkeyContainer` duplicates the same helper in
  `analyzer/cache/rule_matching_cache_test.go:16`. Different packages so an unexported helper
  cannot be shared, but if Task 2 adds more container tests, consider a shared test-support
  helper (or the analyzer-package copy disappears if the blocking fix is applied as stated).

- [analyzer/cache/rule_matching_cache_test.go:281-306] The nil-mutator test performs a
  SaveMatch/GetMatch round-trip; the round-trip duplicates cache-behavior coverage that already
  exists in the package's other tests and is Task 2 territory. The nil-preservation assertion
  (calls == 0) is complete without it.

### Verified (from diff + source, not re-run)

- Seam placement: `analyzer/cache/valkey.go:63-65` invokes the mutator after every Leakspok
  mapping (ClientOption literal 37-46, ConnWriteTimeout 48-50, BlockingPoolSize 52-54, TLSConfig
  56-61) and immediately before `valkey.NewClient(opt)` (67); exactly one invocation site,
  nil-guarded, so defaults are preserved and invocation is once-only.
- Public surface: `analyzer.CacheOptions.ValkeyConfigMutator func(*valkey.ClientOption)`
  (factory.go:66-68) with matching cache-package transfer field (options.go), forwarded
  unchanged in `buildCacheStore` (factory.go:113). Signature matches the requirement exactly.
- Upstream validation propagation: valkey-go validates `Standalone.EnableRedirect` +
  `Standalone.ReplicaAddress` synchronously in `NewClient` (vendor .../valkey-go/valkey.go:484-486,
  exact string "EnableRedirect and ReplicaAddress cannot be used together"); Leakspok wraps it as
  "failed to create valkey client" (valkey.go:69) and the factory wraps further as
  "failed to create cache" (factory.go:129). Both wraps asserted in tests.
- Ordering/override tests are self-verifying: if the mutator ran before the mappings, the
  observed-value assertions would fail.
- Constraints honored: no replica-routing policy added, no duplicated valkey-go option schema,
  no vendor changes, no files created in analyzer/ (plan said "Created: none"), scope confined to
  the seam + tests.
- Cannot verify from diff: the passing test-suite results in task-1-report.md are taken from the
  implementer's report; no focused re-run was performed because the code raised no specific doubt
  (the claimed error text, wrap chain, and ordering were confirmed against source).
