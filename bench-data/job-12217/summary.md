# Job #12217 — Passed, score 10773 (Batch F: PR #47 + #48 indexes; PR #51 conflicted, deferred)
- Portal window: 2026-10-31 13:33:40–13:35:07 JST. NOTE: for this job the portal/benchmarker clock runs ~3.5 min ahead of the servers — the actual traffic in server-log time is 04:30–04:32 UTC (9,110 requests, 5xx=0). Log windows below are server time, attributed by observed traffic.
- Deployed: main @ 8b39132 (stage 2 + #45 + #47 + #48). SQL-only batch: reset.sql + 001-schema.sql synced to all hosts; indexes are created by reset.sql at initialize.
- Grand prix entrants: asia 16 / europe 11 / america 6 — america stalled at 13 users again. Score tracks the x1.5-region GP draw: #10368 (am 20) 12455, #11956 (am 14) 11597, #12217 (am 6) 10773.
- Latency vs Job #11956: images improved on all servers (avg isu1 0.186->0.164, isu2 0.320->0.284, isu3 0.322->0.307; p95 also down). Feeds mixed (isu2 better, isu1/isu3 slightly worse at higher counts). Posts asia worse (0.84->1.03) under an asia-44 draw; am better (1.00->0.85). Friend-requests avg 0.18–0.28s.
- PG aggregate (04:30–04:32 UTC): 43,143 queries / 314.3s total — DB total time down for the third consecutive code state (337.8 -> 324.4 -> 314.3).
- Verdict: keep Batch F (no regression in tail latency or DB time; score delta is the draw).
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-l.txt)
