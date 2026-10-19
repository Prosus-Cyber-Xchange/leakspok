---
artifact: task-progress
change: 2026-10-16-hash-map-rule-matching-cache
task: 6
status: done
updated: 2026-10-19
decision: accepted
---

# Task Progress: Task 6 — Tag cache spans with a token digest instead of raw token text

## Attempt 1 — 2026-10-19

- Outcome: done
- Changed:
  - Created: monitoring/cache_test.go (new monitoring test asserting the `cache.token` tag contract)
  - Modified: monitoring/cache.go (digest-prefix computation shared by both span helpers)
  - Deleted: none
- Red: `go test ./monitoring/ -run 'TestCacheSpan' -count=1` → failed as intended: all 4 subtests of TestCacheSpanTags_TokenDigestPrefix failed because the current implementation tags the raw token (`cache.token tag = "john.doe@example.com"`, `"abc"`) instead of the 12-hex SHA-256 prefix; failure reason is the defect under remediation, not a harness failure. Expected digests are precomputed independently of the implementation (`sha256sum` oracles: john.doe@example.com → 836f82db9912, jane.roe@example.com → 22fff12b355c, abc → ba7816bf8f01) so the assertion cannot pass by mirroring the implementation's own computation; the test also asserts the tag never contains the raw token text and that exactly one span is started per token-scoped operation.
- Green: `go test ./monitoring/ -run 'TestCacheSpan' -count=1` → passed (5 = parent + 4 subtests) after computing the digest prefix inside monitoring/cache.go via a single private helper `tokenDigestPrefix` used by both TraceCacheGetMatches and TraceCacheSaveMatches.
- Refactor: none beyond the small change — decision: the digest computation lives only in monitoring (one private helper, both call sites), satisfying the DRY quality constraint and the design.md Ports & Adapters boundary; the adapter (analyzer/cache/tracer.go) still passes the token and knows nothing of the digest, and was left untouched (verified unchanged via LSP over the file). Rerun: `go test ./monitoring/ -count=1` → 14 passed; `go test ./analyzer/cache/ -run 'TestTokenHash' -count=1` → 15 passed (Task 1 tracer-delegation tests TestTokenHash_TracerGetMatches/TracerSaveMatches pass unchanged — they assert delegation and context forwarding only, no raw tag values, so no update was needed).
- Verified:
  - `go test ./monitoring/ -count=1` → 14 passed
  - `go test ./analyzer/cache/ -run 'TestTokenHash' -count=1` (Docker available) → 15 passed
  - Fresh LSP diagnostics (source=lsp, scope=paths) over monitoring/cache.go, monitoring/cache_test.go, analyzer/cache/tracer.go → 0 errors (only two information-level typo hints on the hex-literal string in the test; no warnings or errors)
  - `task test/unit` (full suite, -short -race) → passed, exit 0
  - `task lint` → failed on one pre-existing, out-of-scope finding: unused nolint directive `//nolint:staticcheck` at analyzer/cache/rule_matching_cache_token_test.go:363 (file unmodified by this task, committed before dispatch at HEAD 3457604; zero findings in monitoring). Not corrected here because editing a sibling task's committed test artifact is outside Task 6's scope; surfaced to the coordinator as a concern.
- Quality constraint & decision context: DRY — digest computation lives only in monitoring per design.md Ports & Adapters (monitoring owns span helpers); observed evidence: single `tokenDigestPrefix` helper in monitoring/cache.go, adapter unchanged. Decision context: revised approved design decision (commit 4a49fb70e8080bbd486a44745020624b828135e9, design.md blob f32300c574d00600c256b3a494d3e6a8df07d89f, requester "Approve revision" 2026-10-16), remediating whole-branch review pass-1-blocking-2; span granularity preserved (one read span and one write span per token-scoped operation, asserted by the test's single-span check).
- Notes: security rationale — raw token text is unsanitized PII candidate material from the tokenizer and must not be recorded in APM; the `cache.token` tag now carries only the non-reversible 12-hex SHA-256 digest prefix, preserving token-granular span correlation without exposing token content. Concern for the coordinator: pre-existing `nolintlint` failure at analyzer/cache/rule_matching_cache_token_test.go:363 (unused `//nolint:staticcheck` directive) predates this task and is not addressable within its scope.

