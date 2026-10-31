# Job #16824 — Passed, score 20826 (Batch I step 3: PRs #52 + #55) — NEW BEST
- Run window: 2026-10-31 14:19:10–14:20:45 JST (05:19:10–05:20:45 UTC). Deployed: main @ 892a119.
- Grand prix: asia 16 / europe 32 / america 30 (total 78). Post creations total 524 (154/187/183) — the highest yet. Feeds avg 0.140/0.284/0.221; user search avg 0.139/0.319/0.265; images avg 0.048/0.284/0.229. 5xx = 0.
- PG aggregate in pg-aggregate.txt.
- Batch I sequence (#49 -> #56 -> #52+#55) scores: 17766 -> 19809 -> 20826 (draws vary, throughput trend consistent).

## Merged PRs newly reflected in this run
- PR #52 (merge c5318d2): ranked (friends-of-friends) feed — the eligibility predicate moves into SQL: author NOT IN blocked, NOT IN friends, IN friends-of-friends, visibility IN ('public','friends_of_friends'), applied BEFORE ranking/pagination so scoring sees the true candidate set instead of filtering in Go after fetch. Ships an exhaustive SQL-vs-Go equivalence test over relationship/visibility combinations.
- PR #55 (merge 892a119): user search — the candidate scan now selects only id/username/display_name and excludes blocked users in SQL; Unicode NFC + case-fold matching stays in Go (normalizeSearchText), collects at most 51 matching IDs, and only then loads full user rows (previously every active user's full row was transferred and filtered in Go). Response shape unchanged.

## Cumulative merged PRs in this state
Batches A–I: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-r.txt)
