# Capability: valkey-client-configuration

## Overview

This capability lets applications using Leakspok's analyzer factory configure the exact valkey-go client options used by the rule-matching cache. It exposes a single optional callback through which callers can inspect, extend, or override any valkey-go client option — including replica routing, standalone replica addresses, client naming, and connection-pool tuning — without Leakspok duplicating the upstream configuration surface.

## Contract

| field | type | notes |
|-------|------|-------|
| `analyzer.CacheOptions.ValkeyConfigMutator` | `func(*valkey.ClientOption)` | Optional. When nil, Leakspok constructs the Valkey client using its established default mapping. When non-nil, invoked with the fully mapped `*valkey.ClientOption` before client creation. |

## Behavior

When an analyzer is created with caching enabled and `ValkeyConfigMutator` is nil, Leakspok constructs the Valkey client using its existing default mapping of cache settings into `valkey.ClientOption` — no callback is invoked and all default behavior is preserved.

When the mutator is non-nil, Leakspok maps its cache settings into a local `valkey.ClientOption`, then invokes the callback exactly once with a pointer to that option. The callback can read any mapped value, extend the option with additional settings, or deliberately override a mapped default. After the callback returns, the (possibly mutated) option is passed to `valkey.NewClient`, which validates and initializes it. Any validation error from valkey-go propagates as a cache-client creation failure.

The callback receives the complete `*valkey.ClientOption` after all Leakspok mappings are applied, so callers can configure replica routing (`SendToReplicas` with `ReadNodeSelector`), standalone replica addresses (`Standalone.ReplicaAddress`), client naming (`ClientName`), and any other supported upstream option. Callers own the validity of their overrides; valkey-go validates the final option combination.

**Examples**

- nil mutator with caching enabled -> client constructed with default Leakspok mapping, callback never invoked
- mutator sets `opt.ClientName` -> Valkey server reports the configured name on the connection
- mutator overrides `opt.InitAddress` -> client dials the overridden address, not the mapped one
- mutator sets incompatible options (e.g. `Standalone.EnableRedirect` + `Standalone.ReplicaAddress`) -> `valkey.NewClient` rejects the combination; cache construction fails with the upstream error wrapped as "failed to create valkey client"
- mutator sets `opt.SendToReplicas` + `opt.ReadNodeSelector` for cluster mode -> read commands route to replicas per the selector (cluster `DoCache` honors `SendToReplicas`; standalone `DoCache` does not — that is an upstream valkey-go limitation, not a Leakspok behavior)

## Invariants

- The mutator MUST be invoked at most once per cache construction, after all Leakspok mappings and before `valkey.NewClient`.
- A nil mutator MUST NOT alter default client construction behavior.

## Decisions

- Leakspok exposes one functional-option callback rather than mirroring valkey-go's option schema. The upstream API evolves independently; duplicating it in Leakspok would create a stale surface and require library releases for every new upstream option. The callback gives callers complete access to current and future options at the cost of making the upstream type part of Leakspok's public compile-time surface — an intentional trade-off.
- The mutator is applied at the client-construction boundary (`NewRuleMatchingCache`), not in the analyzer factory. The cache constructor owns the concrete `valkey.ClientOption` and is the narrowest point at which the callback can observe and override every default before upstream validation. Applying it in the factory would require duplicating construction details; applying it after `valkey.NewClient` is impossible because initialization has already occurred.
- Leakspok delegates all validation of mutated options to `valkey.NewClient`. Valkey-go owns the semantics and compatibility rules for its complete option surface; adding Leakspok-side validation would duplicate that responsibility and could diverge from upstream.
