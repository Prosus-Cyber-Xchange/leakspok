# Proposal: Valkey Configuration Mutator

| Field | Value |
|---|---|
| Change | 2026-08-31-valkey-config-mutator |
| Status | approved |
| Author | OpenCode |
| Created | 2026-08-31 |

## Why

Leakspok maps only a small, fixed subset of valkey-go client configuration into its rule-matching cache. Applications therefore cannot configure replica routing or other supported valkey-go behavior, causing read requests to continue using primary nodes even when their Valkey deployment provides replicas.

## Goals & Success Criteria

- Applications can mutate the exact `valkey.ClientOption` used to construct Leakspok's cache client.
- The mutator is optional and preserves existing client construction behavior when absent.
- The mutator runs after Leakspok's defaults are mapped and before valkey-go creates the client, so callers can configure or override any supported valkey-go option.
- Automated tests demonstrate that a mutation reaches a real single-node Valkey server and that the public factory preserves the configured behavior.

## Non-Goals

- Leakspok will not add separate replica-routing fields or policies.
- Leakspok will not validate, normalize, or selectively restrict valkey-go options beyond valkey-go's own validation.
- This change will not prove traffic distribution across a multi-node replica topology.

## Proposed Change

Add an optional `ValkeyConfigMutator` functional option to the public analyzer cache configuration and forward it into the cache package. The cache package will invoke it once against the locally constructed `*valkey.ClientOption` after applying Leakspok defaults and before `valkey.NewClient`. Callers can use valkey-go options such as `SendToReplicas`, `ReadNodeSelector`, and standalone replica addresses without Leakspok mirroring the upstream configuration API.

## Capabilities

### New

- `valkey-client-configuration`: Application-provided configuration of the valkey-go client used by the rule-matching cache.

### Modified

- None.

### Removed

- None.

## Impact

This is an additive public API change in `analyzer.CacheOptions` and introduces the valkey-go option type into that package's public configuration surface. Internal cache construction receives and invokes the same callback. Existing callers that do not set the option retain current behavior. The implementation and tests affect `analyzer/factory.go`, `analyzer/cache/options.go`, `analyzer/cache/valkey.go`, and cache tests.

## Open Questions

- None.
