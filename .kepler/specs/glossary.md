---
artifact: requirements-spec
capability: glossary
status: current
updated: 2026-10-16
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
decision: accepted
---

# Capability: glossary

## Overview

The project's canonical, context-scoped ubiquitous language. Leakspok is a single-context Go library for detecting and anonymizing sensitive data; entries below are seeded from established project documentation (AGENTS.md, README.md, and the canonical specs in `.kepler/specs/`). Code-derived meanings were kept out. The requester confirmed this language on 2026-10-16.

## Contract

| Term | Definition | Bounded context | Aliases / terms to avoid | Source |
| --- | --- | --- | --- | --- |
| Leakspok | A Go library that detects and anonymizes personally identifiable information (PII) and sensitive data in strings and byte slices, for defensive security and compliance use. | Project-wide (the library itself) | none | AGENTS.md Project Overview; README.md |
| Token | A segment of input produced by tokenization (input split on delimiters); matchers are evaluated against tokens individually, never against the full input. | Project-wide (analyzer) | chunk, fragment | AGENTS.md Tokenization (byte_analyzer.go) |
| Rule | A detection unit combining a matcher, zero or more exceptions, and anonymization settings, with a name and description. | Project-wide (analyzer) | detector | AGENTS.md Rule Definition |
| Matcher | The component of a rule that decides whether a piece of input matches, exposing both an entity classification and a match operation. | Project-wide (analyzer, pattern) | pattern (do not use interchangeably with Matcher; a Pattern is the composable matching unit a Matcher may wrap) | AGENTS.md Rule Definition; pattern package docs |
| Entity | The classification a matcher reports for what a match represents (for example CPF, Email, CNPJ, CreditCard). | Project-wide (analyzer, pattern) | type, kind | AGENTS.md; pattern package docs |
| Exception | A whitelist mechanism that excludes specific input from a rule so known-safe values are not anonymized. | Project-wide (analyzer) | whitelist (acceptable synonym), ignore-list | AGENTS.md Rule Definition; README Defensive Security Notes |
| Pattern | A composable matching unit that can be combined with logical operations (And, Or, Not, Threshold) to build complex detection logic. | Project-wide (pattern) | regex (only one specialized kind), rule (a Rule is not a Pattern) | AGENTS.md Pattern Matching; pattern package docs |
| REDACT | An anonymization strategy that replaces an entire matched token with a fixed placeholder such as `[REDACTED]`. | Project-wide (analyzer) | remove, delete | AGENTS.md Anonymization Strategies |
| MASK | An anonymization strategy that replaces the first N characters of a matched token with a masking character, keeping the remainder. | Project-wide (analyzer) | redact (use REDACT for the whole-token strategy) | AGENTS.md Anonymization Strategies |
| Rule-matching cache | The cache of rule matching results used by the analyzer so repeated evaluations of the same input are avoided. | Project-wide (analyzer, cache) | result cache | .kepler/specs/valkey-client-configuration.md |
| Valkey | The Redis-compatible cache backend, accessed through the valkey-go client, behind the rule-matching cache. | Project-wide (cache) | Redis (acceptable when referring to protocol compatibility, not the product) | .kepler/specs/valkey-client-configuration.md |

## Behavior

Read a term within its named context. Leakspok currently documents one shared context (the analyzer library); entries marked project-wide carry the same meaning across analyzer, pattern, cache, and monitoring usage because the project documentation defines them once. Distinct contexts may use the same word for different concepts in the future; cross-context documents should then identify the meanings and their translation rather than assume equivalence.

## Invariants

- A canonical entry states an established meaning with its scope and source.
- A conflicting or unresolved proposed definition does not replace established language.
- Matcher and Pattern are not interchangeable: a Rule uses a Matcher; a Pattern is a composable matching unit.
- Code identifiers (Go type and function names) are not canonical vocabulary; entries describe meanings, not symbols.

## Decisions

- Seeded only from established documentation (AGENTS.md, README.md, canonical specs). No code-derived definition was promoted without requester confirmation; the requester confirmed the language on 2026-10-16 (attended review).
