---
artifact: plan
change: 2026-10-16-hash-map-rule-matching-cache
status: approved
created: 2026-10-16
author: Caio Cavalcante <caio.cavalcante@ifood.com.br>
decision: accepted
execution: {version: 2, mode: dependencies}
route_unit: null
input_impact: {"version":1,"previousPlanCommit":"f589f25d40afab9b3ae515eced42a4a66b220301","reviewReference":"01a15408-5884-70b4-bb82-daec2f8685dd impact-review: sound","changes":[{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/design.md","previousBlob":"eb9c20a068f02f63698193f547ac91804748332a","currentBlob":"f32300c574d00600c256b3a494d3e6a8df07d89f","affectedTasks":[],"unaffectedTasks":[1,2,3,4],"newTasks":[5,6],"unresolvedIntent":false,"rationale":"Whole-branch review pass-1-blocking-2 (intent-defect): the approved tracer decision tagged spans with raw token text (PII channel into APM). Requester approved the revised digest-prefix decision at 4a49fb70e8080bbd486a44745020624b828135e9 before this record was committed. Tasks 1-4 are complete and frozen with fresh approved feedback; their implementations and approvals stand (no acceptance criterion pins the tag value). The correction obligation is new work: Task 6 applies the digest-prefix tag; Task 5 remediates the separate pass-1-blocking-1 code-defect (untrack .pi/artifacts/TODO.md) and is a transparent coordination append to this record. Impact classification independently adjudicated sound."}]}
intent_approvals:
  - path: proposal.md
    reference: "Requester (Caio Cavalcante) approved the exact candidate bytes in the planning session on 2026-10-16 ('Approve as written')"
    commit: 2269fc9c1b0471ca2e87bf4dce004a58f1239865
  - path: requirements/rule-matching-cache.md
    reference: "Requester (Caio Cavalcante) approved the exact candidate bytes in the planning session on 2026-10-16 ('Approve as written')"
    commit: 2269fc9c1b0471ca2e87bf4dce004a58f1239865
  - path: design.md
    reference: "Requester (Caio Cavalcante) approved the revised tracer decision bytes in the orchestration session on 2026-10-16 ('Approve revision'; remediation of whole-branch review pass-1-blocking-2)"
    commit: 4a49fb70e8080bbd486a44745020624b828135e9
initialization:
  version: 1
  base: 2269fc9c1b0471ca2e87bf4dce004a58f1239865
  authority: "Requester (Caio Cavalcante) confirmed the task breakdown in the planning session on 2026-10-16 ('Approve breakdown'); intent authority recorded in intent_approvals"
  breakdown: 12929e5d7a702944796932dd03ae4c038082b40c49d8d18c37c0e948de3feb3e
  taskIds: [1, 2, 3, 4]
---

# Plan: Token-Hash Data Layout for the Rule-Matching Cache

## Overview

- Change: .kepler/changes/2026-10-16-hash-map-rule-matching-cache/
- Goal: replace the rule-matching cache's per-(entity, token) string entries with one hash per token so a request issues exactly one read and at most one write per token, preserving every canonical behavior of the rule-matching-cache capability.
- Spec impact: rule-matching-cache — approved requirements delta at .kepler/changes/2026-10-16-hash-map-rule-matching-cache/requirements/rule-matching-cache.md, committed at 2269fc9c1b0471ca2e87bf4dce004a58f1239865; kepler-finish-work folds it into .kepler/specs/rule-matching-cache.md.
- Related tests: `go test ./analyzer/cache/ -run 'TestTokenHash'` (container-backed, requires Docker), `go test ./analyzer/ -run 'TestSerialRulesRunner|TestConcurrentRulesRunner' -race`, focused per-task commands in each task's Verify.
- LSP diagnostics: pi-lens LSP over the changed Go files under `analyzer/` and `monitoring/` plus affected dependents after each task; require no errors; record unavailable coverage rather than claiming a clean result.
- Deferred verification: full `go test ./...` and `task lint` run in later workflow stages, not in task Verify.
- Context notes: project standards in AGENTS.md (commands, boundaries — `task gen` regenerates mocks via `mockgen -destination=mocks/mock_cache_store.go -package=cachemock github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache CacheStore`); architecture and flow live in design.md; canonical invariants live in .kepler/specs/rule-matching-cache.md. Vendored valkey-go v1.0.75 supports Cacheable HGETALL (`HgetallKey.Cache()`), multi-field HSET, and `ExpireConditionNx` (verified under vendor/github.com/valkey-io/valkey-go/internal/cmds/). Container-backed cache tests have no `-short` guard and require Docker; when Docker is unavailable the worker records the environment outcome instead of claiming success. Task-definition digests in each task's Notes computed by kepler over each task block excluding non-fenced Notes-JSON lines, joined with newlines and right-trimmed; the breakdown digest hashes the `## Tasks` section as committed.
- Quality notes: tasks follow the design's seams — adapter storage (Task 1), serial flow (Task 2), concurrent flow (Task 3), legacy removal (Task 4); each lands one cohesive unit testable in isolation through the CacheStore port (spy mock or Testcontainers backend); no structural smell is accepted.
- Intent approvals: proposal.md, requirements/rule-matching-cache.md — approved 2026-10-16 at 2269fc9c1b0471ca2e87bf4dce004a58f1239865; design.md — originally approved at 2269fc9c1b0471ca2e87bf4dce004a58f1239865, revised and re-approved at 4a49fb70e8080bbd486a44745020624b828135e9 ('Approve revision'; remediation of whole-branch review pass-1-blocking-2); see frontmatter `intent_approvals`.
- Input impact review: 01a15408-5884-70b4-bb82-daec2f8685dd impact-review: sound
- Input impact previous plan: f589f25d40afab9b3ae515eced42a4a66b220301

## Tasks

### Task 1: Extend the cache contract with token-scoped hash storage

- Goal: CacheStore exposes token-scoped storage — `GetMatches(ctx context.Context, data []byte) (map[pattern.Entity]bool, error)` returning only the entities present in the token's hash, and `SaveMatches(ctx context.Context, data []byte, results map[pattern.Entity]bool) error` persisting a batch in one command — implemented by the Valkey adapter (reads: one HGETALL, through DoCache when client-side caching is enabled and the base TTL is positive, plain Do otherwise; writes: one multi-field HSET issued together with EXPIRE NX in a DoMulti when the base TTL is positive, the jittered TTL established once at first write, a plain HSET when the base TTL is zero), the noop adapter (empty map, discard batches), and the tracer (one read span and one write span per call, token-tagged through new monitoring helpers). The legacy GetMatch/SaveMatch methods remain in place so existing callers keep compiling; this contract is the result every later task consumes, which is why it exists first.
- Depends on: none
- Files: optional guidance
  - Created: analyzer/cache/rule_matching_cache_token_test.go
  - Modified: analyzer/cache/store.go, analyzer/cache/valkey.go, analyzer/cache/noop.go, analyzer/cache/tracer.go, monitoring/cache.go, analyzer/cache/mocks/mock_cache_store.go (regenerated)
  - Deleted: none
- Acceptance:
  - Satisfies the requirement scenarios "Cache Storage Layout" (first evaluation, repeat evaluation, partially evaluated token), "Token-Scoped Cache Reads" (hot token with client-side caching enabled), "Batched Negative-Result Writes" (no-match token; later additions keep the original expiry), and "Jittered Token TTL" (jittered establishment, zero jitter, zero base TTL) from requirements/rule-matching-cache.md.
  - Adapter tests against a real Valkey (analyzer/cache/testing.StartValkeyContainer) show: SaveMatches writes exactly one hash key `leakspok:m:<token>` with one field per entity; GetMatches returns exactly the stored fields; an unknown token yields an empty map and a nil error; a second SaveMatches on the same token does not change the key's TTL; a zero base TTL writes without expiry; the jittered establishment TTL stays within [base×(1−P), base×(1+P)]; repeated reads with client-side caching enabled return the stored map.
  - Quality constraints: interface segregation and DRY — the `leakspok:m:` key encoding lives only in the Valkey adapter (design.md, Architecture & Components); the adapter never decides what is cached — the negative-only policy stays with callers (canonical invariant).
  - Decision context: vendored valkey-go v1.0.75 supports Cacheable HGETALL, multi-field HSET, and ExpireConditionNx (verified in vendor/); canonical invariants from .kepler/specs/rule-matching-cache.md apply unchanged; no conflicting recalled lessons — session memory searched, repository-owned specs govern.
- Notes: {"version":1,"repository":"/Users/caio.cavalcante/workspace/ifood/ai-security/prosus-cyber-xchange/leakspok/.worktrees/hash-map-rule-matching-cache","taskDefinition":"db84820629fdb48769174647a2d0fbe37057546a4c0ef23d2134ca6e6dbc131f","intentCommit":"2269fc9c1b0471ca2e87bf4dce004a58f1239865","inputs":[{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/design.md","blob":"eb9c20a068f02f63698193f547ac91804748332a"},{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/requirements/rule-matching-cache.md","blob":"20354364f1a559502b696060661578b4a67ed471"},{"path":".kepler/specs/rule-matching-cache.md","blob":"f8e4c7a89d6906a563546f23db3ef11e2d97635b"},{"path":"AGENTS.md","blob":"5306df3ceaa1f6629d40d356dc8dda62ecbe4885"}]}
- Steps:
  1. Red — write analyzer/cache/rule_matching_cache_token_test.go covering the acceptance scenarios against the new method names; run `go test ./analyzer/cache/ -run 'TestTokenHash'` and observe the intended failure (compilation error: GetMatches/SaveMatches do not exist on CacheStore), which is the intended Red for a new-API task.
  2. Green — add the two methods to the CacheStore interface; implement them in RuleMatchingCache (token key encoding `leakspok:m:<data>`; GetMatches via DoCache or Do; SaveMatches via DoMulti HSET + EXPIRE NX with the jittered TTL established once, skipping EXPIRE at zero base TTL), NoopRuleMatchingCache, and CacheStoreTracer (add token-tagged helpers TraceCacheGetMatches/TraceCacheSaveMatches in monitoring/cache.go); regenerate the mock with `mockgen -destination=mocks/mock_cache_store.go -package=cachemock github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache CacheStore`; rerun the same command until it passes for the intended reason.
  3. Refactor — extract shared key-encoding or jitter helpers only where they remove real duplication without widening the surface; record the decision if none is needed; rerun the focused tests plus the pre-existing adapter suite (`go test ./analyzer/cache/...`, container-backed — record the Docker environment outcome).
  4. If a phase fails for the wrong reason, Verify fails, or feedback is `changes-requested`, record the correction and repeat Red → Green → Refactor with relevant verification in this task; block for re-plan if the correction exceeds Acceptance or approved intent; Files is optional guidance.
- Verify: `go test ./analyzer/cache/ -run 'TestTokenHash'` (Docker required) and fresh LSP diagnostics over analyzer/cache/store.go, analyzer/cache/valkey.go, analyzer/cache/noop.go, analyzer/cache/tracer.go, monitoring/cache.go → focused tests pass and no LSP errors.
- Commit: `feat(cache): add token-scoped hash storage contract`

### Task 2: Route the serial runner through token-scoped evaluation

- Goal: SerialRulesRunner performs exactly one GetMatches per Process call, consults the returned map per rule in order, evaluates only missing entities whose own exception check passes, coalesces the token's miss evaluation and batched save under a token-level flight when SingleflightEnabled is true — the leader evaluates its own missing set in rule order, persists one negative batch, and shares the results and save error with waiters, while waiters merge the shared results and evaluate residual misses inline, so each caller issues at most one SaveMatches — and returns the first matching rule through a final in-order pass, preserving first-match semantics, exception non-persistence, log-and-continue degradation on backend errors, and the canonical invariants. Needs Task 1's result (the token-scoped contract and regenerated spy mock) to exist because the runner consumes GetMatches/SaveMatches and verification spies on them.
- Depends on: Task 1
- Files: optional guidance
  - Created: none
  - Modified: analyzer/serial_runner.go, analyzer/singleflight_coalescer.go (add the shared tokenFlightKey helper), analyzer/serial_runner_test.go, analyzer/runner_behavior_test.go, analyzer/coverage_gap_test.go
  - Deleted: none
- Acceptance:
  - Satisfies the runner-level requirement scenarios: "Cache Storage Layout" (map consulted; absent entities are misses), "Token-Scoped Cache Reads" (one read per token; backend error logs and treats every entity as a miss), "Batched Negative-Result Writes" (one batched write per token; the positive match is never cached; negatives accumulated before an early positive match are still persisted), and "Coalescing of Concurrent Misses" (a burst shares one evaluation and one save; a cancelled waiter unblocks with its context error; a save failure is shared, logged, and continued past; sequential bursts recompute).
  - Spy-based criteria: a CacheStore mock records exactly one GetMatches and at most one SaveMatches per Process call; a cached hit never invokes that entity's matcher; an excepted entity is never evaluated or persisted; a partially cached token evaluates only absent entities.
  - Quality constraints: single responsibility and dependency inversion — the runner owns evaluation flow and persistence policy, the coalescer owns in-flight sharing, the adapter owns storage mechanics (design.md, Ports & Adapters); the runner touches no concrete IO beyond the CacheStore port.
  - Decision context: approved design Decisions (runner flow, coalescing at token scope, exception checks outside the flight); canonical invariant "positive matches MUST NEVER be cached" enforced in the save path. Requester-accepted consequence: the first positive-matching encounter evaluates the full missing set rather than stopping at the match — anonymization outputs are unchanged and repeat encounters cost one matcher evaluation.
- Notes: {"version":1,"repository":"/Users/caio.cavalcante/workspace/ifood/ai-security/prosus-cyber-xchange/leakspok/.worktrees/hash-map-rule-matching-cache","taskDefinition":"a04e00492274bd7793294c3b601e53099269ccd220cdc5675047372cc3811d73","intentCommit":"2269fc9c1b0471ca2e87bf4dce004a58f1239865","inputs":[{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/design.md","blob":"eb9c20a068f02f63698193f547ac91804748332a"},{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/requirements/rule-matching-cache.md","blob":"20354364f1a559502b696060661578b4a67ed471"},{"path":".kepler/specs/rule-matching-cache.md","blob":"f8e4c7a89d6906a563546f23db3ef11e2d97635b"},{"path":"AGENTS.md","blob":"5306df3ceaa1f6629d40d356dc8dda62ecbe4885"}]}
- Steps:
  1. Red — extend analyzer/serial_runner_test.go with spy-mock tests asserting the new call pattern and behaviors (exactly one GetMatches, at most one SaveMatches, cached hit skips the matcher, excepted entity never evaluated or persisted, early positive match still flushes accumulated negatives, backend read error degrades to full evaluation, token-key coalescing shares one evaluation and save); run `go test ./analyzer/ -run 'TestSerialRulesRunner'` and observe the intended failures (the current runner performs per-pair GetMatch/SaveMatch calls).
  2. Green — restructure SerialRulesRunner.Process: fetch once (on error, log as today and use an empty map); build the ordered rule view separating cached results, excepted misses (own isException check, never persisted), and the missing set deduplicated by entity; when coalescing is enabled and the missing set is non-empty, run coalescer.do(tokenFlightKey(data), leader) where the leader evaluates its missing set in rule order, persists one negative batch via SaveMatches, and returns the results and save error to waiters — waiters merge and evaluate residual misses inline; when coalescing is disabled, evaluate inline; persist the caller's residual negatives with at most one SaveMatches; then run the final in-order pass returning the first rule whose entity result is true. Preserve existing logging and context conventions.
  3. Refactor — extract the shared flight-key helper tokenFlightKey into analyzer/singleflight_coalescer.go so Task 3 can reuse it, without changing concurrent_runner.go behavior; rerun the focused tests plus `go test ./analyzer/ -run 'TestSerialRulesRunner' -race`.
  4. If a phase fails for the wrong reason, Verify fails, or feedback is `changes-requested`, record the correction and repeat Red → Green → Refactor with relevant verification in this task; block for re-plan if the correction exceeds Acceptance or approved intent; Files is optional guidance.
- Verify: `go test ./analyzer/ -run 'TestSerialRulesRunner' -race` and fresh LSP diagnostics over analyzer/serial_runner.go, analyzer/singleflight_coalescer.go, and the affected test files → tests pass and no LSP errors.
- Commit: `feat(analyzer): token-scoped evaluation in the serial runner`

### Task 3: Route the concurrent runner through token-scoped evaluation

- Goal: ConcurrentRulesRunner performs one GetMatches per Process call before submitting workers, submits only missing entities whose exception check passes, keeps first-match cancellation exactly as today (workers bailed by cancellation produce nothing), flushes the negatives collected from completed workers in at most one SaveMatches, and coalesces the token's miss evaluation and batched save under tokenFlightKey when enabled — the leader submits, collects, and saves while waiters merge the shared results and evaluate residual misses inline sequentially. Needs Task 1's contract (GetMatches/SaveMatches and the spy mock) and Task 2's tokenFlightKey helper in the shared coalescer file; it depends on both because it consumes both results.
- Depends on: Task 1, Task 2
- Files: optional guidance
  - Created: none
  - Modified: analyzer/concurrent_runner.go, analyzer/singleflight_coalescer.go (remove singleflightKey; keep tokenFlightKey as Task 2 defined it), analyzer/concurrent_runner_coalescing_test.go, analyzer/runner_behavior_test.go
  - Deleted: none
- Acceptance:
  - Satisfies the same runner-level requirement scenarios as Task 2, plus preserved first-match cancellation: Process returns as soon as a collected result matches, cancelled workers produce no results, and only completed workers' negatives are flushed.
  - Spy-based criteria: exactly one GetMatches per Process; at most one SaveMatches per Process; a cached hit never submits a worker for that entity; a partially cached token submits only absent entities.
  - Quality constraints: coordination constraint — analyzer/singleflight_coalescer.go is shared with Task 2, so Task 3 reuses tokenFlightKey as Task 2 defined it and must not rewrite Task 2's serial flow or evidence; pool usage keeps today's blocking profile (the flight leader submits to the runner pool and waits; waiters hold no pool slots while waiting on the flight channel).
  - Decision context: approved design Decisions (concurrent flow, first-match cancellation unchanged, coalescing at token scope); concurrency safety is verified under -race as in the existing suite.
- Notes: {"version":1,"repository":"/Users/caio.cavalcante/workspace/ifood/ai-security/prosus-cyber-xchange/leakspok/.worktrees/hash-map-rule-matching-cache","taskDefinition":"8aeb14357fcc1838f35ab9412debd6db583c598fec3f74cbbb96303ecea9eb8a","intentCommit":"2269fc9c1b0471ca2e87bf4dce004a58f1239865","inputs":[{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/design.md","blob":"eb9c20a068f02f63698193f547ac91804748332a"},{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/requirements/rule-matching-cache.md","blob":"20354364f1a559502b696060661578b4a67ed471"},{"path":".kepler/specs/rule-matching-cache.md","blob":"f8e4c7a89d6906a563546f23db3ef11e2d97635b"},{"path":"AGENTS.md","blob":"5306df3ceaa1f6629d40d356dc8dda62ecbe4885"}]}
- Steps:
  1. Red — extend the concurrent runner tests with spy-based assertions (exactly one GetMatches per Process, at most one SaveMatches carrying completed workers' negatives, cached hits not submitted, token-key coalescing sharing one evaluation and save); run `go test ./analyzer/ -run 'TestConcurrentRulesRunner'` and observe the intended failures (per-pair calls today).
  2. Green — restructure ConcurrentRulesRunner.Process per the goal's flow: prefetch once and degrade to an empty map on error; compute the missing set after per-rule exception checks; submit one pool task per missing entity with the existing first-match cancellation; collect results; flush completed negatives in at most one SaveMatches; when coalescing is enabled, wrap the evaluate-and-save phase in coalescer.do(tokenFlightKey(data), …) so waiters share the results and save error, evaluating their own residual misses inline; replace singleflightKey usage with tokenFlightKey and remove singleflightKey once its last caller is gone; rerun the same command until green.
  3. Refactor — align result collection and logging with the serial path's conventions where the duplication is pure; rerun the focused tests plus `go test ./analyzer/ -run 'TestConcurrentRulesRunner' -race`.
  4. If a phase fails for the wrong reason, Verify fails, or feedback is `changes-requested`, record the correction and repeat Red → Green → Refactor with relevant verification in this task; block for re-plan if the correction exceeds Acceptance or approved intent; Files is optional guidance.
- Verify: `go test ./analyzer/ -run 'TestConcurrentRulesRunner' -race` and fresh LSP diagnostics over analyzer/concurrent_runner.go, analyzer/singleflight_coalescer.go, and the affected test files → tests pass and no LSP errors.
- Commit: `feat(analyzer): token-scoped evaluation in the concurrent runner`

### Task 4: Remove the legacy per-pair cache contract

- Goal: with Tasks 2 and 3 complete, no caller uses GetMatch/SaveMatch; remove them from CacheStore and every implementer (Valkey adapter, noop, tracer), remove the sentinel error pair ErrCacheNotFound/IsCacheNotFoundError and the legacy entity-tagged monitoring span helpers, remove singleflightKey if still present, delete or rewrite legacy-only test cases (legacy cases in rule_matching_cache_test.go, cache_tracer_test.go, and ttl_jitter_test.go that cover only the removed surface), regenerate the mock, and drop the stale factory.go comment. Depends on Tasks 2 and 3 because a removal must observe zero remaining callers first.
- Depends on: Task 2, Task 3
- Files: optional guidance
  - Created: none
  - Modified: analyzer/cache/store.go, analyzer/cache/valkey.go, analyzer/cache/noop.go, analyzer/cache/tracer.go, monitoring/cache.go, analyzer/cache/mocks/mock_cache_store.go (regenerated), analyzer/factory.go (stale comment), legacy test files
  - Deleted: analyzer/cache/errors.go
- Acceptance:
  - `git grep -n 'GetMatch(\|SaveMatch(\|ErrCacheNotFound\|IsCacheNotFoundError\|TraceCacheGet(\|TraceCacheSave(\|singleflightKey' -- analyzer monitoring` returns no references outside vendor.
  - The token-scoped contract from Task 1 is unchanged: its tests pass byte-for-byte in behavior.
  - Quality constraints: DRY / single source of truth — exactly one storage contract remains; the task is a pure removal with no intended behavior change.
  - Decision context: design.md Code Attribution marks the legacy surface as removed; canonical spec invariants are unaffected; no conflicting recalled lessons.
- Notes: {"version":1,"repository":"/Users/caio.cavalcante/workspace/ifood/ai-security/prosus-cyber-xchange/leakspok/.worktrees/hash-map-rule-matching-cache","taskDefinition":"cf0120bf903d6ce851b047fe682ae61d34252e2174cc47e5e0c1c4a5fd3068ca","intentCommit":"2269fc9c1b0471ca2e87bf4dce004a58f1239865","inputs":[{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/design.md","blob":"eb9c20a068f02f63698193f547ac91804748332a"},{"path":".kepler/specs/rule-matching-cache.md","blob":"f8e4c7a89d6906a563546f23db3ef11e2d97635b"},{"path":"AGENTS.md","blob":"5306df3ceaa1f6629d40d356dc8dda62ecbe4885"}]}
- Steps:
  1. Red — a conventional failing behavioral check is technically impossible for a pure removal (there is no new runtime behavior to observe before the edit), so record the repeatable alternative: run `git grep -n 'GetMatch(\|SaveMatch(\|ErrCacheNotFound\|IsCacheNotFoundError\|TraceCacheGet(\|TraceCacheSave(\|singleflightKey' -- analyzer monitoring` and record the reference count before production edits (the count is greater than zero — the defect is the dead contract surface).
  2. Green — remove the legacy methods, the sentinel error file, the legacy span helpers, singleflightKey, the stale comment, and the legacy-only test cases; regenerate the mock; rerun the grep (count zero) and the focused suites (`go test ./analyzer/cache/... ./analyzer/ -run 'TestTokenHash|TestSerialRulesRunner|TestConcurrentRulesRunner'`) until green.
  3. Refactor — no cleanup beyond the removal is expected; record that decision and rerun the focused suites.
  4. If a phase fails for the wrong reason, Verify fails, or feedback is `changes-requested`, record the correction and repeat Red → Green → Refactor with relevant verification in this task; block for re-plan if the correction exceeds Acceptance or approved intent; Files is optional guidance.
- Verify: the grep reference count is zero; `go test ./analyzer/...` (container-backed tests require Docker — record the environment outcome) and fresh LSP diagnostics over analyzer/ and monitoring/ → no references, tests pass, no LSP errors.
- Commit: `refactor(cache): remove legacy per-pair cache contract`

### Task 5: Untrack the stray coordination artifact

- Goal: `.pi/artifacts/TODO.md` — a stale, malformed coordination-state file swept into tracking by the init commit c47f9aa and classified pass-1-blocking-1 (code-defect) by the whole-branch review — is removed from Git tracking and prevented from re-entering, with no other path affected. The whole-branch review gate blocks on this remediation, which is why it exists.
- Depends on: none
- Files: optional guidance
  - Created: none
  - Modified: .gitignore (append a `.pi/` ignore entry)
  - Deleted: `.pi/artifacts/TODO.md` from the index (the file stays on disk — it is the todo tool's live state)
- Acceptance:
  - `git ls-files .pi` returns no paths. The alternative Red records the count (>0) before the removal: a conventional failing behavioral check is technically impossible for an untracking remediation because there is no runtime behavior to observe.
  - `git check-ignore -q .pi/artifacts/TODO.md` succeeds (the ignore entry prevents re-adding).
  - `git status --short` is clean after the commit and no other path changed.
  - Quality constraints: single responsibility — the task touches only the stray artifact's tracking state (whole-branch review pass-1-blocking-1); zero behavior change.
  - Decision context: whole-branch review Pass 1 (commit ca2abf5) classification and correction instruction "remove from tracking"; kepler-init treats `.pi/` as tool-local state that is never project-tracked.
- Notes: {"version":1,"repository":"/Users/caio.cavalcante/workspace/ifood/ai-security/prosus-cyber-xchange/leakspok/.worktrees/hash-map-rule-matching-cache","taskDefinition":"18277f6d84de0bd0fe73c4bbefb3eb8b2e7831a5cc424e8b6e66b53caf539c58","intentCommit":"4a49fb70e8080bbd486a44745020624b828135e9","inputs":[{"path":"AGENTS.md","blob":"5306df3ceaa1f6629d40d356dc8dda62ecbe4885"}]}
- Steps:
  1. Red — run `git ls-files .pi | wc -l` and record the count (>0) before production edits; record the technical reason (pure untracking remediation, no runtime behavior) for the alternative.
  2. Green — append `.pi/` to .gitignore; `git rm --cached .pi/artifacts/TODO.md`; rerun `git ls-files .pi` (empty) and `git check-ignore -q .pi/artifacts/TODO.md` (succeeds).
  3. Refactor — none needed beyond the removal; record the decision; rerun `git status --short` (clean) and `git show --stat` (no other path changed).
  4. If a phase fails for the wrong reason, Verify fails, or feedback is `changes-requested`, record the correction and repeat Red → Green → Refactor with relevant verification in this task; block for re-plan if the correction exceeds Acceptance or approved intent; Files is optional guidance.
- Verify: `git ls-files .pi` empty, `git check-ignore -q .pi/artifacts/TODO.md` succeeds, `git status --short` clean; LSP diagnostics are not applicable (no Go files changed) — record why.
- Commit: `chore: untrack stray coordination artifact from init commit` — include the task-local progress record in the same commit.

### Task 6: Tag cache spans with a token digest instead of raw token text

- Goal: remediate whole-branch review pass-1-blocking-2 (intent-defect): cache spans tag `cache.token` with a short non-reversible SHA-256 digest prefix of the token — never raw token text — per the revised approved design decision at 4a49fb7. The current implementation tags raw token content (unsanitized PII candidates from the tokenizer) into DataDog APM.
- Depends on: none
- Files: optional guidance
  - Created: none
  - Modified: monitoring/cache.go (digest-prefix computation in TraceCacheGetMatches/TraceCacheSaveMatches), monitoring/tracing_test.go or a new monitoring test asserting the tag contract, analyzer/cache/rule_matching_cache_token_test.go (only if its tracer-delegation tests pin raw tag values)
  - Deleted: none
- Acceptance:
  - The `cache.token` tag value equals the first 12 hex characters of SHA-256(token) and never contains the raw token text (a test asserts both directions).
  - Span granularity is unchanged: one read span and one write span per token-scoped operation (the revised design decision preserves it).
  - `go test ./monitoring/ -count=1` and the Task 1 tracer-delegation tests pass; LSP clean over monitoring/cache.go and affected test files.
  - Quality constraints: DRY — the digest computation lives only in monitoring (design.md, Ports & Adapters: monitoring owns span helpers); the adapter passes the token and knows nothing of the digest (boundary preserved).
  - Decision context: revised approved design decision (commit 4a49fb7, requester "Approve revision" 2026-10-16); whole-branch review pass-1-blocking-2.
- Notes: {"version":1,"repository":"/Users/caio.cavalcante/workspace/ifood/ai-security/prosus-cyber-xchange/leakspok/.worktrees/hash-map-rule-matching-cache","taskDefinition":"ff0d6b4b62ba20c8c4d18e388206021be2608a34939f243d59ddfe9dec7e2c8a","intentCommit":"4a49fb70e8080bbd486a44745020624b828135e9","inputs":[{"path":".kepler/changes/2026-10-16-hash-map-rule-matching-cache/design.md","blob":"f32300c574d00600c256b3a494d3e6a8df07d89f"},{"path":"AGENTS.md","blob":"5306df3ceaa1f6629d40d356dc8dda62ecbe4885"}]}
- Steps:
  1. Red — write a monitoring test asserting the `cache.token` tag equals the 12-hex digest prefix and never the raw token; run `go test ./monitoring/ -run 'TestCacheSpan'` and observe the intended failure (the current implementation tags the raw token).
  2. Green — compute the sha256 hex prefix in monitoring/cache.go for both span helpers; rerun the same command until green.
  3. Refactor — none expected beyond the small change; record the decision; rerun the monitoring suite and the analyzer token tests (tracer delegation).
  4. If a phase fails for the wrong reason, Verify fails, or feedback is `changes-requested`, record the correction and repeat Red → Green → Refactor with relevant verification in this task; block for re-plan if the correction exceeds Acceptance or approved intent; Files is optional guidance.
- Verify: `go test ./monitoring/ -count=1`, `go test ./analyzer/cache/ -run 'TestTokenHash'` (Docker), fresh LSP over monitoring/cache.go and affected test files → pass, no errors.
- Commit: `fix(monitoring): tag cache spans with token digest prefix` — include the task-local progress record in the same commit.

### Task 7: Make the branch's own files lint-clean

- Goal: remediate whole-branch review pass-2-blocking-1 (code-defect): `task lint` fails with 41 branch-introduced findings in the change's own added/rewritten files. The binding design gate "task lint and task test/unit must pass" (plan Overview, deferred to this stage) must hold for the branch's own files. Six pre-existing findings on unchanged constructs are out-of-scope repo debt — do not touch them.
- Depends on: none
- Files: optional guidance
  - Created: none
  - Modified: analyzer/cache/valkey.go (govet shadow rename), analyzer/cache/rule_matching_cache_token_test.go (revive param order; dead nolintlint directives at :332/:363 removed; exhaustive per-line nolint with justification), analyzer/serial_runner_test.go, analyzer/concurrent_runner_coalescing_test.go, analyzer/cache/rule_matching_cache_test.go, analyzer/coverage_gap_test.go, analyzer/serial_runner_coalescing_test.go (exhaustive per-line nolint on the intentionally-partial map[pattern.Entity]bool fixtures), monitoring/cache_test.go (testpackage compliance)
  - Deleted: none
- Acceptance:
  - `task lint` reports ZERO findings in the branch's own files; the six pre-existing out-of-scope findings on unchanged constructs may remain (record them as the documented baseline debt).
  - Behavior is unchanged: `go test ./analyzer/... ./monitoring/ -count=1` passes (Docker available); partial-map fixtures still exercise partial-map semantics (the per-line nolint preserves the intentionally-partial fixtures — do NOT add the missing keys, which would erase absent-entity miss coverage).
  - The production govet shadow (analyzer/cache/valkey.go:138) is fixed by renaming, not suppression.
  - Quality constraints: minimal means per the review's correction instruction — mechanical renames/reorders/deletions and established per-line nolint practice with justification (the repo documents inline nolint usage in AGENTS.md Known Linter Suppressions; the review's correction instruction endorses "established per-line nolint practice"). Any `.golangci.yml` change is FORBIDDEN in this task (AGENTS.md ask-first boundary).
  - Decision context: whole-branch review Pass 2 (commit 24788b1) classification; the exhaustive linter's complaint against intentionally-partial fixtures is a fixture-intent conflict resolved by per-line justification, not fixture restructure.
- Notes: {"version":1,"repository":"/Users/caio.cavalcante/workspace/ifood/ai-security/prosus-cyber-xchange/leakspok/.worktrees/hash-map-rule-matching-cache","taskDefinition":"bb16d7b22a5e47df998b58a5cd526571a20208fb29fa390d1136bd9e63fa39ed","intentCommit":"4a49fb70e8080bbd486a44745020624b828135e9","inputs":[{"path":"AGENTS.md","blob":"5306df3ceaa1f6629d40d356dc8dda62ecbe4885"}]}
- Steps:
  1. Red — run `task lint 2>&1 | tee /tmp/lint-before.txt` and record the exact finding list (the 41 branch-introduced findings are the failing state; the pre-existing six on unchanged constructs are documented debt, not the target).
  2. Green — fix each branch-introduced finding with minimal means: rename the shadowed variable in valkey.go; reorder the helper's context parameter; delete the two dead nolintlint directives; add per-line `//nolint:exhaustive // intentionally-partial fixture: absent entities must remain misses` on the partial map literals (established practice, preserves partial-map coverage); make monitoring/cache_test.go testpackage-compliant (keep unexported access if needed via the behavioral span assertion, or move the digest test accordingly — judge and record). Rerun `task lint` until only the six pre-existing findings remain.
  3. Refactor — none expected; record the decision; rerun `go test ./analyzer/... ./monitoring/ -count=1` (Docker) and `go build ./...`.
  4. If a phase fails for the wrong reason, Verify fails, or feedback is `changes-requested`, record the correction and repeat Red → Green → Refactor with relevant verification in this task; block for re-plan if the correction exceeds Acceptance or approved intent (e.g., if a finding cannot be fixed without a .golangci.yml change, STOP and block — that boundary requires owner approval); Files is optional guidance.
- Verify: `task lint` with zero branch-introduced findings (six documented baseline-debt findings may remain, recorded); `go test ./analyzer/... ./monitoring/ -count=1`; `go build ./...`; fresh LSP over the changed files (no errors).
- Commit: `fix(lint): make the branch's own files lint-clean` — include the task-local progress record in the same commit.

## Done when

- Every active task has worker-recorded Red, Green and Refactor evidence (or a justified alternative Red) in its task log, an integrated original task commit and coordinator-published completion.
- Each task requires fresh committed independent `approved` feedback to become fully gated. Readiness does not grant admission, and root done does not grant clearance. Compatible isolated ready lanes may run concurrently; descendants remain held until prerequisite approval. `changes-requested` returns the same task for correction and another pass.
- Tests related to changed files and affected behavior pass; fresh LSP diagnostics report no errors in the affected scope. Full-suite tests and full builds remain deferred to later workflow stages.
- Whole-branch review is approved with no blocking findings after every active task is fully gated with fresh feedback.
