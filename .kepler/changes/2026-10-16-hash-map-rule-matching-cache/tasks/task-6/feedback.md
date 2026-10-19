---
artifact: feedback
change: 2026-10-16-hash-map-rule-matching-cache
task: 6
created: 2026-10-16
updated: 2026-10-16
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
status: resolved
decision: accepted
---

# Code Feedback: Task 6 — Tag cache spans with a token digest instead of raw token text

## Pass 1 — 2026-10-16

Base: 3457604360f2b859f3ae5bd1f9c74f4f338d05af
Head: 2417ca9d0ec03379a26852967a17e70fccb4c4c4
Verdict: approved
Scope: full
Target: refs/heads/hash-map-rule-matching-cache

### Blocking

- None.

### Suggestions

- [task range 3457604..2417ca9] Verified coverage: the range is the single task commit 2417ca9 "fix(monitoring): tag cache spans with token digest prefix" (subject matches the plan's Commit field exactly; no peer commits inside the range), judged against Task 6 of plan.md, the revised approved design decision (design.md blob f32300c574d00600c256b3a494d3e6a8df07d89f, Tracing bullet; intent commit 4a49fb70e8080bbd486a44745020624b828135e9), and whole-branch review pass-1-blocking-2. All claimed verification re-ran independently at the head: `go test ./monitoring/ -count=1` → 14 passed; `go test ./analyzer/cache/ -run 'TestTokenHash' -count=1` (Docker 27.4.0) → 15 test-level passes including both Task 1 tracer-delegation tests unchanged. Digest oracles were verified independently of the implementation via `sha256sum` (john.doe@example.com → 836f82db9912, jane.roe@example.com → 22fff12b355c, abc → ba7816bf8f01), so the new monitoring test's assertions cannot pass by mirroring the implementation's own computation; the test asserts both directions of the tag contract (equality to the 12-hex SHA-256 prefix and `strings.Contains` rejection of the raw token) and pins exactly one span per token-scoped operation, so span granularity is covered. The digest derives from the full 256-bit SHA-256 of the token (hex-encoded, then truncated to 12 hex chars), not from truncating raw token text, and is deterministic. All `cache.token` `SetTag` sites were enumerated: exactly two, both in monitoring/cache.go, both routed through the unexported `tokenDigestPrefix` helper — the digest computation lives only in monitoring per the design.md Ports & Adapters boundary, and the adapter analyzer/cache/tracer.go passes the token verbatim and was untouched within the range. Fresh LSP diagnostics (source=lsp) over monitoring/cache.go, monitoring/cache_test.go, and analyzer/cache/tracer.go → 0 findings. The security rationale is recorded in the helper's doc comment, the test's doc comment, and the task progress record: raw token text is unsanitized PII candidate material and must not reach APM. `task lint` was not treated as this task's gate: its current findings live in files outside the task diff and remain deferred to whole-branch review scope; no lint finding is attributed to Task 6. `kepler workbench lint --change-dir` passed (exit 0) at preflight.
