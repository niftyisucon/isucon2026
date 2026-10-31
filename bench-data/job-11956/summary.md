# Job #11956 — Passed, score 11597 (PR #45 verification run; the ONLY bench on PR #45 code)
- Run window: 2026-10-31 13:18:20–13:19:44 JST (04:18:20–04:19:44 UTC). Deployed: main @ 597d38e (stage 2 + PR #45).
- Grand prix entrants: asia 17 / europe 13 / america 14 (per-region lines; the stdout summary line prints "アジア 0 人" — a benchmarker-side inconsistency, verbatim in portal.txt).
- PR #45 effect vs Job #10368 (stage 2, pre-#45): images p95 improved on all servers (isu1 0.509->0.462, isu2 0.717->0.589, isu3 0.725->0.654); images avg isu2 0.337->0.320, isu3 0.334->0.322, isu1 0.169->0.186 (asia GP-heavy draw). Feeds avg improved on all (0.190->0.172 / 0.244->0.226 / 0.233->0.204). 5xx = 0.
- PG aggregate (04:18:30–04:19:59 UTC): 39,612 queries / 324.4s total (Job #10368: 45,067 / 337.8s).
- Verdict: keep PR #45 (no errors; tail latency and DB time down).
- IMPORTANT: an earlier bench-data package "job-11604" was built from a corrupted browser handoff; Job #11604 does not exist in the portal (404). That package is retracted — see bench-data/job-11604/RETRACTED.md. This job (11956) is the sole post-#45 bench and the team's latest official result.
- Log extraction windows: nginx LTSV slices in this dir = 04:18:35–04:19:59 UTC each server. PG slice kept at isu-db:/tmp/pg-slice-i.txt.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt
