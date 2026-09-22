---
artifact: progress
change: 2026-09-22-cache-singleflight-ttl-jitter
status: pending
updated: 2026-09-22
decision: accepted
tasks:
  - id: 1
    title: "Add TTL jitter to the rule-matching cache"
    status: done
    progress: tasks/task-1/progress.md
  - id: 2
    title: "Map TTLJitterPercentage through the analyzer factory"
    status: pending
    progress: tasks/task-2/progress.md
  - id: 3
    title: "Pass RunnerOptions to the serial rules runner (breaking)"
    status: pending
    progress: tasks/task-3/progress.md
  - id: 4
    title: "Coalesce serial-runner cache misses with singleflight"
    status: pending
    progress: tasks/task-4/progress.md
  - id: 5
    title: "Coalesce concurrent-runner cache misses with singleflight"
    status: pending
    progress: tasks/task-5/progress.md
---

# Progress: Cache Singleflight Coalescing and TTL Jitter

| Task | Status | Progress |
|---|---|---|
| Task 1: Add TTL jitter to the rule-matching cache | done | [details](tasks/task-1/progress.md) |
| Task 2: Map TTLJitterPercentage through the analyzer factory | pending | [details](tasks/task-2/progress.md) |
| Task 3: Pass RunnerOptions to the serial rules runner (breaking) | pending | [details](tasks/task-3/progress.md) |
| Task 4: Coalesce serial-runner cache misses with singleflight | pending | [details](tasks/task-4/progress.md) |
| Task 5: Coalesce concurrent-runner cache misses with singleflight | pending | [details](tasks/task-5/progress.md) |
