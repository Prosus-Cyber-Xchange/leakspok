# Task 1 Review-Fix Report: serverless ValkeyConfigMutator unit tests

Change: `.hamilton/changes/2026-08-31-valkey-config-mutator/`
Date: 2026-08-31
Branch: `fix/valkey-read-replicas` (isolated: yes)
Status: **done**

## Scope

Bounded correction of the review (review.md, blocking item): Task 1's "isolated unit
behavior" tests must not start Docker containers. Server-observable mutation proof belongs
exclusively to Task 2. Production seam untouched.

## Changes

### `analyzer/coverage_gap_test.go`

- Removed the `startValkeyContainer` helper and the now-unused `tcredis` import — the
  analyzer package (`go test ./analyzer`) no longer requires Docker.
- `TestMakeByteAnalyzer_ValkeyConfigMutatorForwarded` — uses the dummy unreachable address
  `localhost:19379`; keeps every seam assertion (callback forwarded unchanged through
  `buildCacheStore`, invoked exactly once, observes mapped `InitAddress` + `ForceSingleClient`).
- `TestMakeByteAnalyzer_ValkeyConfigMutatorNilPreservesDefaults` — uses the dummy address;
  verifies nil mutator preserves the default path.
- `TestMakeByteAnalyzer_ValkeyConfigMutatorInvalidUpstreamPropagates` — unchanged (already
  serverless).

### `analyzer/cache/rule_matching_cache_test.go`

- `TestRuleMatchingCache_ValkeyConfigMutator_NilPreservesDefaults` — uses the dummy address;
  removed the redundant SaveMatch/GetMatch round trip (review suggestion 2); keeps the
  nil-preservation assertion (`calls == 0`).
- `TestRuleMatchingCache_ValkeyConfigMutator_ObservesMappedDefaultsOnce` — uses the dummy
  address; keeps every mutator-observable value assertion (once-only, mapped defaults).
- Deleted `TestRuleMatchingCache_ValkeyConfigMutator_OverridesValidOption` — the one test that
  genuinely needed a live endpoint (PingOnConnect proving the override won); server-observable
  propagation is Task 2 territory per plan.md and the instruction.
- `TestRuleMatchingCache_ValkeyConfigMutator_InvalidUpstreamOptionFails` — unchanged (already
  serverless).

## Deviation from the review's literal mechanism (important)

The review asserted: "valkey-go dials asynchronously, so `NewCacheStore` returns success
against an unreachable address." That premise does not hold for the vendored valkey-go:

- `ClientOption.ForceSingleClient` (which `DisableClusterMode: true` maps to) makes
  `NewClient` take the `newSingleClient` path (vendor `valkey.go:539-541`).
- `newSingleClient` calls `conn.Dial()` **synchronously** (vendor `client.go:31-38`) — a TCP
  connect plus the HELLO/SETINFO handshake — and returns the dial error, which `NewClient`
  propagates.
- Leakspok's `NewRuleMatchingCache` treats that as a fatal client-creation failure, so
  construction against `localhost:19379` returns
  `failed to create valkey client: dial tcp ... connect: connection refused`.

Empirically confirmed: with the review's suggested mechanism, all four tests failed with that
exact error. `require.NoError`/`assert.NotNil` construction-success assertions are therefore
impossible serverless in this valkey-go version, and no production seam change was permitted.

Resolution: the four tests keep their full seam assertions — which are the actual point of
the tests and are fully observable without a server, because the mutator runs *before*
`valkey.NewClient` — and additionally assert that real client creation was reached (the error
contains `failed to create valkey client`). Concretely:

- Forwarded / ObservesMappedDefaultsOnce: `require.Error` (dial failure) + mutator invoked
  exactly once with the mapped Leakspok defaults.
- Nil tests: `require.Error` (plain default-path dial failure) + callback never invoked.

All original seam assertions (invocation count, observed mapped values, nil-not-invoked) are
retained; only the construction-success assertion changed to the honest serverless
observation.

## Verification

| Command | Result |
|---|---|
| `go test ./analyzer/... -run 'Test.*ValkeyConfigMutator' -count=1` | pass (6/6, no Docker) |
| `go test ./analyzer/... -run 'Test.*ValkeyConfigMutator\|Test.*Cache' -count=1` | pass |
| `go test ./analyzer -count=1` | pass (single package, Docker-free) |
| `go test ./analyzer/... -count=1` | pass (full, incl. pre-existing container tests) |
| `go build ./...` | pass |
| `gofmt -l analyzer/` | clean |

## Acceptance check

- Four flagged tests use a dummy unreachable address (`localhost:19379`) and start no
  container. ✅
- Redundant SaveMatch/GetMatch round trip removed from the nil test. ✅
- No server-dependent override test retained in Task 1 (deleted; Task 2 owns it). ✅
- Production seam (`analyzer/factory.go`, `analyzer/cache/options.go`,
  `analyzer/cache/valkey.go`) untouched. ✅
- `go test ./analyzer` (single package) no longer requires Docker; the analyzer package's
  pre-Task-1 test profile is restored. ✅

## Notes / concerns

- The assertion-shape change (construction success → construction failure with the plain dial
  error) is a direct consequence of the vendored valkey-go dialing synchronously under
  `ForceSingleClient`; it is the minimal honest adaptation of the review's intended
  correction and is documented here and in `progress.md` for the reviewer.
- The cache package keeps its pre-existing Testcontainers-backed tests (BasicOperations, TTL,
  AutoPipelining, etc.) and its own `startValkeyContainer` helper — unchanged, as those
  assertions are server-observable and predate Task 1.
