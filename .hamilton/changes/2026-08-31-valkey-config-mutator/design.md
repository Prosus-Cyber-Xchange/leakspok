# Design: Valkey Configuration Mutator

## Context

`analyzer.CacheOptions` currently exposes a fixed set of Redis/Valkey settings. `buildCacheStore` maps these fields into cache-package options, and `NewRuleMatchingCache` constructs a `valkey.ClientOption` from that reduced set. Consequently, applications cannot configure valkey-go replica routing (`SendToReplicas` with `ReadNodeSelector`), standalone replica addresses, or any other upstream option that Leakspok does not mirror. The public configuration must remain backward-compatible for callers that do not require customization.

## Goals / Non-Goals

**Goals**

- Provide one additive functional option that exposes the complete valkey-go client-option surface.
- Apply it after Leakspok defaults and before valkey-go construction.
- Prove the configured option reaches a real Valkey server without requiring a replica topology.

**Non-Goals**

- Introduce individual replica-specific configuration fields.
- Change default replica routing or client-side caching behavior.
- Add independent validation for upstream valkey-go options.

## Decisions

### Decision: Use a forwarded functional option

- Choice: Add `ValkeyConfigMutator func(*valkey.ClientOption)` to `analyzer.CacheOptions` and the internal cache options, then forward the same callback through `buildCacheStore`.
- Alternatives considered: Replica-specific Leakspok fields were rejected because they duplicate an evolving upstream API and constrain callers. Accepting a replacement `valkey.ClientOption` was rejected because it obscures Leakspok defaults and makes accidental loss of required initialization easier.
- Rationale: A single callback provides full upstream configuration access while retaining the existing public factory and default mapping as the baseline.

### Decision: Apply mutation at the client-construction boundary

- Choice: `NewRuleMatchingCache` will build its local `valkey.ClientOption`, apply a non-nil mutator once, then call `valkey.NewClient`.
- Alternatives considered: Applying it in the analyzer factory would require duplicating client construction details there. Applying it after `valkey.NewClient` is impossible because initialization and validation have already occurred.
- Rationale: The cache constructor owns the concrete option and is the narrowest point at which the callback can observe and override every default before upstream validation.

### Decision: Test configuration propagation with ClientName

- Choice: Add unit tests for nil handling, once-only invocation, default observation, and override ordering; add a single-node integration test that sets `ClientName` in the mutator and checks the active connection with `CLIENT GETNAME`.
- Alternatives considered: A multi-node cluster test would demonstrate actual replica selection but adds topology complexity beyond this change's contract. Pure unit testing cannot demonstrate that valkey-go received and applied a valid mutated option.
- Rationale: `ClientName` is observable from a single-node Valkey server and proves the mutator survives the complete Leakspok-to-client construction path.

## Architecture & Components

| Component | Responsibility | Boundary |
|---|---|---|
| `analyzer.CacheOptions` | Public configuration contract for analyzer callers. | Owns the exported mutator field and forwards it; it does not create Valkey clients. |
| `cache.RuleMatchingCacheOptions` | Cache-layer configuration transfer object. | Carries the callback without interpreting it. |
| `cache.NewRuleMatchingCache` | Maps Leakspok defaults, invokes the callback, and creates the Valkey client. | Owns concrete `valkey.ClientOption` construction; valkey-go remains responsible for option validation and routing semantics. |
| Cache tests | Verify ordering and end-to-end option propagation. | Substitute callback state in unit tests and use a disposable single-node Valkey server for integration behavior. |

### Quality Lens

The callback is the one configuration extension point, so no parallel replica-specific structures can drift from it. The public factory depends only on the callback type and forwards it unchanged; concrete valkey-go construction stays in the cache boundary. Tests use callback-observable state and an isolated container instead of reaching into client internals. No new abstraction is added beyond this required extension seam.

## Data & Flow

1. The application sets `CacheOptions.ValkeyConfigMutator` while configuring an analyzer.
2. `buildCacheStore` copies the callback into `RuleMatchingCacheOptions`.
3. `NewRuleMatchingCache` maps Leakspok settings into a local `valkey.ClientOption`.
4. If non-nil, the callback mutates that option exactly once.
5. `valkey.NewClient` validates and initializes the mutated configuration.

## Error Handling & Edge Cases

| Failure or edge case | Behavior |
|---|---|
| Mutator is nil | Client construction follows existing behavior and the callback is not invoked. |
| Mutator overrides a default with a valid value | The override is passed directly to valkey-go. |
| Mutator creates invalid or incompatible valkey-go options | `valkey.NewClient` returns an error; Leakspok wraps it as a client-creation failure. |
| Standalone client with client-side caching and replica options | Leakspok passes the caller's configuration through unchanged; valkey-go's standalone `DoCache` behavior remains an upstream limitation and is not altered by this change. |

## Testing Strategy

- Add focused unit tests around cache construction to assert the mutator receives Leakspok-mapped defaults, is called once, and can override a mapped option before construction.
- Extend the existing Testcontainers-backed cache integration tests. Configure a deterministic `ClientName` through the public `analyzer.CacheOptions` factory path, then issue `CLIENT GETNAME` on the active Valkey connection or otherwise inspect the server-observable name. This validates end-to-end propagation with one Valkey node.
- Run the focused cache and analyzer tests. A multi-node cluster is not required because replica selection itself is delegated to valkey-go and is outside this extension seam's behavioral proof.

## Constraints & Boundaries

- Always: preserve the existing defaults when no mutator is supplied and use valkey-go's returned errors.
- Ask first: expand the public API beyond the single mutator field or add a multi-node topology test.
- Never: add replica-routing policy, duplicate the full valkey-go option schema, or modify vendored valkey-go code.

## Risks / Trade-offs

- [Callers can override fields Leakspok normally controls] -> The documented ordering makes this deliberate; callers own the validity of their overrides and valkey-go validates them.
- [The upstream option API becomes part of Leakspok's public compile-time surface] -> This is intentional to provide complete configuration access and is preferable to maintaining a stale duplicate model.
- [Single-node integration does not prove replica traffic] -> The test proves mutation propagation; valkey-go owns routing behavior and its configuration is made available unchanged.

## Migration / Rollout

The change is additive. Existing applications require no migration. Applications that need replica reads can set `ValkeyConfigMutator` with the valkey-go routing options appropriate to their deployment.

## Open Questions

- None.
