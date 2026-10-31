# Job #12918 — Passed, score 11952 (Batch F: PR #47 + #48; PR #51 conflicted, deferred)
- Run window: 2026-10-31 13:30:42–13:32:11 JST = 04:30:42–04:32:11 UTC (server LTSV hump 04:30–04:32 matches; runtime-measured).
- Deployed: main @ 8b39132 (stage 2 + #45 + #47 + #48). SQL-only batch: reset.sql + 001-schema.sql synced to all hosts.
- Grand prix entrants: asia 16 / europe 8 / america 15.
- Latency vs Job #11956: images improved on all servers (avg isu1 0.186->0.164, isu2 0.320->0.284, isu3 0.322->0.307; p95 down too). Feeds mixed at higher counts. Posts asia 0.84->1.03 under an asia-44 draw; am 1.00->0.85. Friend-requests avg 0.18–0.28s. 5xx = 0.
- PG aggregate (04:30–04:32 UTC): 43,143 queries / 314.3s total — DB total time down for the third consecutive code state (337.8 -> 324.4 -> 314.3).
- Verdict: keep Batch F.
- NOTE: this run's data was initially packaged as "job-12217" from a corrupted browser handoff; Job #12217 does not exist in the portal list (verified 13:35 JST). The package has been moved here; bench-data/job-12217/ is retracted. From now on every bench result is verified against the portal job list before reporting.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-l.txt)
