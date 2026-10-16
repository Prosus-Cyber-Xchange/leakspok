---
artifact: feedback
change: 2026-10-16-hash-map-rule-matching-cache
task: 4
created: 2026-10-16
updated: 2026-10-16
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
status: resolved
decision: accepted
---

# Code Feedback: Task 4 — Remove the legacy per-pair cache contract

## Pass 1 — 2026-10-16

Base: b03cd8ff12af520a8ccd870bd8c89bb8a905fe9d
Head: 39cec6c46554857f82acbbde31aef286c97a7259
Verdict: approved
Scope: full
Target: refs/heads/hash-map-rule-matching-cache

### Blocking

- None.

### Suggestions

- [task range b03cd8ff..39cec6c4] Verified scope: the range is the single task commit 39cec6c4 "refactor(cache): remove legacy per-pair cache contract" (no peer commits inside the range — the base is Task 3's approved feedback commit b03cd8ff; the token-scoped surface being preserved was approved in Task 1 feedback f589f25d..a734b25d and Task 2/3 feedbacks, all verified readback as approved), judged against Task 4 of plan.md, the approved intent at 2269fc9c1b0471ca2e87bf4dce004a58f1239865, design.md Code Attribution ("CacheStore becomes token-scoped; GetMatch/SaveMatch are removed"; mocks regenerated; monitoring span helpers token-scoped), and AGENTS.md standards. Verification re-run independently: the acceptance grep `git grep -n 'GetMatch(\|SaveMatch(\|ErrCacheNotFound\|IsCacheNotFoundError\|TraceCacheGet(\|TraceCacheSave(\|singleflightKey' -- analyzer monitoring` → zero matches (exit 1); `go test ./analyzer/... -count=1` → 272 passed in 5 packages (Docker available, so the container-backed Valkey tests ran); `go test ./monitoring/ -count=1` → 9 passed; `go build ./...` OK; `go vet ./analyzer/... ./monitoring/` clean. A repo-wide sweep (excluding vendor) finds the removed identifiers only in historical change artifacts of the completed 2026-09-22 change and this change's own plan/feedback/progress prose — no production, test, examples/, README, or rules.go references; `go build ./examples/...` confirms the three examples compile against the shrunken interface. Preflight `kepler workbench lint --change-dir` succeeded before authoring.
- [task range b03cd8ff..39cec6c4] Verified removal completeness and safety: the interface shrank in store.go with its doc updated; the Valkey adapter lost GetMatch/SaveMatch, the per-pair `key()` builder, the now-orphaned `keyBufPool` field, and the `sync` import together (no half-removed state); noop lost its legacy pair; the tracer lost the per-pair delegation and its doc now names the token-scoped `cache.token` tag; errors.go is fully deleted; TraceCacheGet/TraceCacheSave are gone from monitoring/cache.go while TraceCacheGetMatches/TraceCacheSaveMatches remain; the stale factory.go CacheOptions comment is gone; the mock is regenerated consistently (zero legacy identifiers; header destination path normalized to the repo-root `go tool mockgen` invocation). Every deleted test was enumerated and confirmed legacy-only: all four tests in cache_tracer_test.go and all four in ttl_jitter_test.go exercised only the removed per-pair surface, the four legacy cache cases (BasicOperations, TTLExpiry, NoTTL, InMemoryCacheDisabled) and the two legacy trace finish-func tests covered only removed methods. No token-scoped coverage was lost: TestTokenHash_TTL/_JitterBand/_ClientSideReads/_TracerGetMatches/_TracerSaveMatches remain, and AutoPipelining plus ValkeyConfigMutator_ClientNameObservable were rewritten onto the surviving surface with equal assertion strength. The spy fakes' pairReads/pairSaves counters and assert.Zero pair guards were correctly removed: with the fake no longer implementing the removed methods those guards are structurally impossible to violate, and the acceptance grep is the remaining guard.
- [analyzer/cache/rule_matching_cache_token_test.go:308] Judged the recorded gopls `DuplicateDecl cacheTracerTestKey` diagnostic a confirmed stale-workspace false positive, not a defect: exactly one declaration exists repo-wide (`rg -n 'cacheTracerTestKey' -g '*.go'` → one `type` at line 308 plus its two usages; the historical second declaration site was the git-removed cache_tracer_test.go), and the package compiles and passes under `go build ./...`, `go vet ./analyzer/...`, and the 272-test analyzer suite — a true duplicate declaration would fail compilation. Recording the anomaly with compiler evidence instead of suppressing diagnostics or editing the file is the right handling: it preserves the fresh-LSP-diagnostic verification trail without masking future real diagnostics. The token test file's only other changes are comment-only ("legacy" wording dropped) and the relocation of the shared context key, so the Task 1–3 token-scoped surface is byte-for-byte behavior-unchanged.
- [tasks/task-4/progress.md:16] Minor evidence precision note (no action required; progress artifacts are not editable by feedback): the Red record says "81 references across 11 files"; the verified base-state count at b03cd8ff is 81 references across 14 files (`git grep -c … b03cd8ff -- analyzer monitoring`). The acceptance-relevant number (81 → 0) is exact, so the plan's Red step is satisfied; only the incidental file-count phrase is imprecise.
