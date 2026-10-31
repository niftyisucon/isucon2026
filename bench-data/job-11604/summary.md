# Job #11604 — Passed, score 11014 (PR #45: image_objects partition pruning via image_locations)
- Portal times: 2026-10-31 13:22:06–13:23:51 JST. NOTE: portal clock runs ~4 min ahead of the servers; the actual traffic in server-log time is 04:17:37–04:19:40 UTC (stdout timestamps), drains to ~04:20:30.
- Deployed: main @ 597d38e (stage 2 + PR #45). Deploy ran reset.sql on all hosts + init.sh once on isu1 (created image_locations, backfill, trigger), rebuilt binaries (28,937,693 bytes) on isu1–3, restarted all app services; smoke 401 on all.
- Grand prix entrants: asia 16 / europe 12 / america 4 — an asia-heavy draw (asia population grew to 48; america stalled), unlike Job #10368 (eu 15 / am 20). Score is below #10368 for draw reasons, not latency.
- PR #45 effect (LTSV, vs Job #10368): images improved on every server — isu1 avg 0.169->0.160 p95 0.509->0.434; isu2 avg 0.337->0.300 p95 0.717->0.582; isu3 avg 0.334->0.301 p95 0.725->0.629. Feeds also improved (isu1 0.190->0.172, isu2 0.244->0.223, isu3 0.233->0.204) at higher counts. Posts: asia n 89->132 at avg 0.90->0.82. 5xx = 0 everywhere.
- PG aggregate (04:17–04:20 UTC): 43,608 queries / 342.5s total (Job #10368: 45,067 / 337.8s; draws differ). For exact per-statement splits use ./bin/isu pglog isu4.
- Verdict: keep PR #45.
- Log extraction windows: nginx LTSV slices in this dir = 04:17–04:20 UTC each server. PostgreSQL aggregate = same window on isu-db (slice kept at isu-db:/tmp/pg-slice-h.txt).
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt
