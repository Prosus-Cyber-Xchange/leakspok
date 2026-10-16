# Capability: valkey-client-configuration

This capability allows applications using Leakspok's analyzer factory to configure the exact valkey-go client options used by the rule-matching cache.

## ADDED Requirements

### Requirement: Optional Valkey Client Mutation

The system SHALL expose an optional `ValkeyConfigMutator` callback in `analyzer.CacheOptions` with the signature `func(*valkey.ClientOption)`.

- Priority: must
- Rationale: Callers need access to all current and future upstream valkey-go client options without Leakspok duplicating that configuration surface.

#### Scenario: No mutator is configured

- WHEN an application creates an analyzer with caching enabled and leaves `ValkeyConfigMutator` nil
- THEN Leakspok SHALL construct the Valkey client using its existing default mapping and behavior.

#### Scenario: A mutator configures an upstream option

- WHEN an application configures `ValkeyConfigMutator` to set a valid valkey-go client option
- THEN Leakspok SHALL use that modified option when it creates the Valkey client.

### Requirement: Mutator Ordering and Invocation

The system SHALL invoke a non-nil `ValkeyConfigMutator` exactly once after Leakspok maps cache settings into `valkey.ClientOption` and before it calls `valkey.NewClient`.

- Priority: must
- Rationale: The caller must be able to inspect, extend, or deliberately override the mapped client configuration before valkey-go validates and initializes it.

#### Scenario: A mutator observes Leakspok defaults

- WHEN a mutator reads a client option that Leakspok maps from cache settings
- THEN it SHALL observe the mapped value.

#### Scenario: A mutator overrides a Leakspok default

- WHEN a mutator assigns a different valid value to a client option mapped by Leakspok
- THEN the client SHALL be created using the caller-assigned value.

### Requirement: Upstream Validation Propagation

The system SHALL delegate validation of mutated valkey-go options to `valkey.NewClient` and SHALL return its failure wrapped as cache-client creation failure.

- Priority: must
- Rationale: Valkey-go owns the semantics and compatibility rules for its complete option surface.

#### Scenario: A mutator creates invalid upstream configuration

- WHEN a mutator creates a client-option combination rejected by valkey-go
- THEN cache construction SHALL fail and return an error describing the Valkey client creation failure.

## MODIFIED Requirements

- None.

## REMOVED Requirements

- None.

## RENAMED Requirements

- None.
