# Plan: Valkey Configuration Mutator

## Overview

- Change: `.hamilton/changes/2026-08-31-valkey-config-mutator/`
- Goal: Expose one optional callback that lets applications mutate the valkey-go `ClientOption` used by Leakspok's rule-matching cache, including replica-routing configuration, while retaining existing defaults when it is absent. Verify both option ordering and observable end-to-end propagation to a single-node Valkey server.
- Test: `go test ./analyzer/...`
- Build / typecheck: `go build ./...`
- Context notes: Follow `AGENTS.md` for Go tests and formatting. Implement the boundary in `analyzer/factory.go`, `analyzer/cache/options.go`, and `analyzer/cache/valkey.go` as described by `design.md`; requirements are in `requirements/valkey-client-configuration.md`.
- Quality notes: Task 1 owns the public-to-cache configuration seam and its isolated unit behavior; Task 2 owns server-observable integration proof. No structural smell is accepted.

## Tasks

### Task 1: Add and apply the Valkey client configuration mutator

- Depends on: none
- Files:
  - Created: none
  - Modified: `analyzer/factory.go`, `analyzer/cache/options.go`, `analyzer/cache/valkey.go`, unit test file(s) covering cache construction and analyzer factory configuration
  - Deleted: none
- Acceptance:
  - Satisfies “Optional Valkey Client Mutation” scenarios in `requirements/valkey-client-configuration.md`: `analyzer.CacheOptions` exposes optional `ValkeyConfigMutator func(*valkey.ClientOption)`, and a nil callback preserves the established option mapping.
  - Satisfies “Mutator Ordering and Invocation” scenarios: a non-nil callback observes mapped defaults, runs exactly once, and can override a valid mapped client option before client creation.
  - Satisfies “Upstream Validation Propagation”: an invalid option combination supplied by the callback fails cache construction with the existing client-creation error context.
- Steps:
  1. Write focused failing tests that construct a cache through the public factory path and record the callback’s invocation count plus selected mapped values; cover a valid override and a valkey-go-rejected option combination.
  2. Add `ValkeyConfigMutator` to `analyzer.CacheOptions` and the cache-package option transfer type, importing valkey-go only where each public type requires it.
  3. Forward the callback unchanged in `buildCacheStore`.
  4. In `NewRuleMatchingCache`, invoke a non-nil callback exactly once after its `valkey.ClientOption` literal and conditional Leakspok mappings are complete, and immediately before `valkey.NewClient`.
  5. Run the focused tests and refactor only for clarity; do not add replica-specific options or Leakspok-side validation.
- Verify: `go test ./analyzer/... -run 'Test.*ValkeyConfigMutator|Test.*Cache'` → focused tests pass, including nil, ordering, once-only, override, and invalid-upstream-option cases.
- Commit: `feat(cache): expose Valkey client configuration mutator`

### Task 2: Verify mutator propagation with a single-node Valkey integration test

- Depends on: Task 1
- Files:
  - Created: none
  - Modified: `analyzer/cache/rule_matching_cache_test.go` and, if required to exercise the public factory, the appropriate existing analyzer test file
  - Deleted: none
- Acceptance:
  - Satisfies “A mutator configures an upstream option” in `requirements/valkey-client-configuration.md`: a `ClientName` assigned by the callback is observable on a disposable single-node Valkey server.
  - The test uses the public analyzer cache configuration path when feasible, performs a cache operation, and does not require or claim multi-node replica-routing proof.
- Steps:
  1. Write a failing Testcontainers-backed test using the existing Valkey container helper. Configure a deterministic `ClientName` through `ValkeyConfigMutator`, construct the cache or analyzer through the public factory path, and perform a cache operation.
  2. Create a separate inspection client connected to the same container. Query `CLIENT LIST` filtered by the configured name and assert that Valkey reports the Leakspok client connection.
  3. Keep the test independent of private cache internals; do not use `CLIENT GETNAME` from the inspection connection because it cannot report another connection’s name.
  4. Run the package integration tests and the analyzer test suite.
- Verify: `go test ./analyzer/...` → all analyzer and cache tests pass against the local Docker/Testcontainers environment.
- Commit: `test(cache): verify Valkey client option propagation`

## Done when

- All tasks implemented and recorded in `progress.md`.
- `go test ./analyzer/...` passes and `go build ./...` completes successfully.
- The mutator is optional, is invoked once after Leakspok mappings, and is proven to propagate an upstream option to Valkey.
- All review feedback has been addressed.
