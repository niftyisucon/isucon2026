# Job #10368 — Passed, score 12455 (proxy stage 2: PR #60)
- Run window: 2026-10-31 12:56:44–12:58:14 JST (03:56:44–03:58:14 UTC)
- Deployed: main @ f912448 (stage 1 + #60). nginx-only change on isu2/isu3: all /api/ to the asia region apps (:8081/:8082), GET /api/notifications exact-match stays local. App binaries unchanged (28,938,037 bytes).
- Grand prix entrants: asia 17 / europe 15 / america 20 (eu/am populations grew mid-run: eu 7->16, am 6->21)
- Stage-2 effect vs Job #9358 (isu2/isu3 LTSV): feeds eu avg 0.841->0.244s (n 101->382), am 0.712->0.233s (n 90->433); images eu 0.595->0.337s (n 226->453), am 0.568->0.334s (n 169->548). 5xx = 0 on all servers.
- New ceiling: everything now concentrates on isu1 + the shared DB. Posts avg rose with volume (eu 0.64->0.82 n=98, am 0.66->1.01 n=110, asia 0.59->0.90) — isu1 CPU and DB CPU are the constraint now; per-query cuts are the next lever.
- Log extraction windows: nginx LTSV slices in this dir = 03:56–03:58 UTC each server. PostgreSQL aggregate = 03:56:00–03:59:00 UTC on isu-db.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt
