# Job #16256 — Passed, score 19809 (Batch I step 2: PR #56 alone, SSE aggregation) — NEW BEST
- Run window: 2026-10-31 14:12:31–14:14:02 JST (05:12:31–05:14:02 UTC). Deployed: main @ b7d273d.
- Grand prix: asia 20 / europe 30 / america 21 (total 71 — SMALLER than #14902's 79, yet the score is higher: throughput per user rose. Asia's population finally grew mid-run (18->22), and post/feed/image counts are up across servers.)
- Latency: posts n=192/162/143 avg 0.976/1.996/1.119; feeds n=1087/673/616 avg 0.152/0.242/0.182; images avg 0.059/0.241/0.196. 5xx = 0.
- PG aggregate in pg-aggregate.txt.

## Merged PRs newly reflected in this run
- PR #56 (merge b7d273d): notification SSE polling aggregated per process. Before: every SSE connection ran its own 500ms DB query (users LEFT JOIN LATERAL notifications). After: one poller goroutine per process snapshots all live subscriptions and runs ONE query per 500ms tick for all of them (jsonb_to_recordset + WITH ORDINALITY, same per-user LATERAL semantics), delivering each subscriber its own notification ID; region mismatch or DB error closes the subscription exactly as before; initialize resets all subscriptions. Handler behavior (one event, then close) unchanged.

## Cumulative merged PRs in this state
Batches A–I: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-q.txt)
