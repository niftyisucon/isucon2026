# Job #13886 — Passed, score 18454 (Batch G; list-verified) — NEW BEST
- Run window: 2026-10-31 13:42:52–13:44:24 JST (04:42:52–04:44:24 UTC). Portal list and detail agree.
- Deployed: main @ b5d34b7.
- Populations exploded: asia 18→20, europe 7→26, america 6→30 (region "user moved" events appeared). Grand prix: asia 18 / europe 25 / america 29 (summary line; per-region lines 19/25/30).
- Latency vs Job #12918: images isu1 avg 0.164->0.046 p95 0.447->0.163 (n 1263->1835); eu 0.284->0.214, am 0.307->0.229. Feeds all better (0.185->0.134 / 0.183->0.191 / 0.220->0.234 at much higher counts). Posts avg rose (1.13/1.13/1.59) because post volume roughly doubled — the write path is the new bottleneck. 5xx = 0.
- PG aggregate (04:42–04:44 UTC): 60,241 queries / 221.3s total, avg 3.67ms — vs Job #12918's 43,143 / 314.3s / ~7.3ms. 40% more queries in 30% less DB time: per-query cost halved (Batch G's log-suppression + pool reuse signature).
- Verdict: keep Batch G. The throughput unlock let eu/am populations grow instead of stalling, which is where the score jump came from.

## Merged PRs newly reflected in this run (Batch G)
- PR #51 (merge 76809af): adds reverse-lookup index `follows_followee_id ON follows(followee_id)` to webapp/sql/001-schema.sql and reset.sql (IF NOT EXISTS). Follower-list queries could not use the UNIQUE(follower_id, followee_id) index for followee lookups; this makes them index scans.
- PR #26 (merge 3a4c8d1): database pool tuning in webapp/go/server.go — new `configureDatabasePool`: MaxOpenConns stays 30, MaxIdleConns raised from the default 2 to 10 (plus an integration test, skipped without a test DSN). Cuts connection re-creation (measured 3,710 connections in 5 min during Job #5003, most used <=5 times).
- PR #27 (merge b5d34b7): PostgreSQL log suppression — adds `log_parameter_max_length = 0` to hosts/isu4 conf.d/99-isu-log.conf and the setup.sh generator. Statements and durations are still logged, but parameter values (88.7% of log volume, incl. image blob hex) are no longer written, relieving isu-db CPU/IO on every query. Deployed with a config reload; verified live via SHOW log_parameter_max_length = 0.

## Cumulative merged PRs in this state
Batches A–G: #2 #6 #3 (A), #10 #14 #18 (B), #28 #43 (C), #23 (D, proxy stage 1), #60 (stage 2), #45, #47 #48 (F), #51 #26 #27 (G); plus #1 #4 #7 #9 #8 #15 from the morning.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-m.txt)
