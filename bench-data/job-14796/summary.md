# Job #14796 — Passed, score 14218 (Batch H; list-verified)
- Run window: 2026-10-31 13:56:26–13:58:16 JST (04:56:26–04:58:16 UTC). Portal list and detail agree.
- Deployed: main @ c28f8a5.
- Grand prix: asia 20 / europe 25 / america 22 (total 67; Job #13886 was 72 with america 29). Score is below #13886 mainly on the america draw; targeted endpoints show no regression (below).
- Latency vs Job #13886: register avg 0.30–0.60s (n=73, no LOCK TABLE stall signature). Friend-request POSTs avg 0.141/0.359/0.328 at high counts (290/248/236). RealMoji POST avg 0.163/0.483/0.557. Posts avg mixed (asia 1.134->0.801; eu 1.131->1.603; am 1.587->1.622, p95 3.3–3.5s on eu/am — isu1 write contention remains). Feeds/images comparable or better; images isu1 avg 0.040. 5xx = 0.
- PG aggregate in pg-aggregate.txt.
- Verdict: keep Batch H (no regression in its target paths; score delta tracks the GP draw).

## Merged PRs newly reflected in this run (Batch H)
- PR #46 (merge e8959fc): register no longer does `LOCK TABLE users` + EXISTS pre-check. users gets `CREATE UNIQUE INDEX users_username_unique ON users(username)` (001-schema.sql + reset.sql), and register uses `INSERT ... ON CONFLICT (username) DO NOTHING RETURNING ...`, mapping the no-row result to the same `409 username_taken`. Also removes the follow-up getUser re-read. (Issue #29: the table lock had cost 38.8s total and stalled user-row selects.)
- PR #53 (merge dfa5a70): in createPost, the image_objects INSERT (with a fresh created_at) moves BEFORE the user-row lock inside the same transaction, so the heavy blob write no longer happens while holding the lock. Image created_at is now insert-time instead of the post's postedAt (the #45 locator trigger reads NEW.created_at, so partition lookup stays consistent).
- PR #54 (merge 8088c54): RealMoji replace — the separate getPost read and beginRelationshipTx are replaced by beginRealMojiTx, a single CTE query that fetches the post author and locks both user rows at once; canViewPost then runs inside the tx. Same 403 semantics.
- PR #57 (merge c28f8a5): relationship write paths consolidated — comment creation locks both users in ONE ordered `FOR NO KEY UPDATE` select with a row-count check; friend-request creation replaces isBlocked + full friend-list load with ONE query of two EXISTS (blocked either way / already friends); follow/block paths in relationship.go/relationship_api.go get the same treatment (follow 6->2 SQL, block 7->2).

## Cumulative merged PRs in this state
Batches A–H: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57; plus #1 #4 #7 #9 #8 #15.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-n.txt)
