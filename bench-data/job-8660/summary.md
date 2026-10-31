# Job #8660 — Passed, score 11283 (same code as Job #8319: variance confirmation run)
- Run window: 2026-10-31 12:34:05–12:36:14 JST (03:34:05–03:36:14 UTC)
- Deployed code: main @ 320f080 (batch C = batch B + #28 + #43). Identical state to Job #8319 (8529).
- Scenario anomaly: asia users grew 18 -> 19 -> 28 -> 40 -> 61 across days (previous runs stayed at 18); grand prix entrants asia 0 / europe 5 / america 4, yet the score is the day's best. Same-code spread is now 8529 / 8965(batch B) / 11283 — scenario population draw dominates single-run deltas. Judge changes by per-endpoint latency and counts, not score alone.
- Log extraction windows: nginx LTSV slices in this dir = 03:34–03:36 UTC each server. PostgreSQL aggregate = 03:34:00–03:37:00 UTC on isu-db.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt
