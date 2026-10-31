# Job #14902 — Passed, score 19032 (Batch H) — CURRENT BEST, list-verified + server-log verified
- Run window: 2026-10-31 13:55:42–13:57:13 JST (04:55:42–04:57:13 UTC). Deployed: main @ c28f8a5.
- Populations: eu 7->33, am 6->31; grand prix asia 17 / europe 32 / america 30 (total 79 — the biggest GP field yet).
- Latency (server LTSV 04:55:30–04:57:14 UTC): posts avg 0.801/1.603/1.622 (n=139/159/146); feeds avg 0.107/0.240/0.232; images avg 0.036/0.251/0.237 (n=2317/862/775); register avg 0.30–0.60s (no LOCK TABLE stall); friend-request POSTs avg 0.141/0.359/0.328 at n=290/248/236; RealMoji POST avg 0.163/0.483/0.557. 5xx = 0 on all servers.
- PG: see pg-aggregate.txt (window 04:55:30–04:57:14 UTC).
- Note: an earlier phantom report ("Job #14796, 14218") for this code state was retracted — see bench-data/job-14796/RETRACTED.md. This job (#14902) is the only real run after the Batch H deploy.

## Merged PRs newly reflected in this run (Batch H)
- PR #46 (merge e8959fc): register no longer does `LOCK TABLE users` + EXISTS pre-check. users gets `CREATE UNIQUE INDEX users_username_unique ON users(username)` (001-schema.sql + reset.sql), and register uses `INSERT ... ON CONFLICT (username) DO NOTHING RETURNING ...`, mapping the no-row result to the same `409 username_taken`. Also removes the follow-up getUser re-read. (Issue #29: the table lock had cost 38.8s total and stalled user-row selects.)
- PR #53 (merge dfa5a70): in createPost, the image_objects INSERT (with a fresh created_at) moves BEFORE the user-row lock inside the same transaction, so the heavy blob write no longer happens while holding the lock. Image created_at is now insert-time instead of the post's postedAt (the #45 locator trigger reads NEW.created_at, so partition lookup stays consistent).
- PR #54 (merge 8088c54): RealMoji replace — the separate getPost read and beginRelationshipTx are replaced by beginRealMojiTx, a single CTE query that fetches the post author and locks both user rows at once; canViewPost then runs inside the tx. Same 403 semantics.
- PR #57 (merge c28f8a5): relationship write paths consolidated — comment creation locks both users in ONE ordered `FOR NO KEY UPDATE` select with a row-count check; friend-request creation replaces isBlocked + full friend-list load with ONE query of two EXISTS (blocked either way / already friends); follow/block paths in relationship.go/relationship_api.go get the same treatment (follow 6->2 SQL, block 7->2).

## Cumulative merged PRs in this state
Batches A–H: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57; plus #1 #4 #7 #9 #8 #15.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-oa.txt)
