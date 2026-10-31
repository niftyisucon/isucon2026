# Job #9358 — Passed, score 9128 (batch D: PR #23 proxy stage 1)
- Run window: 2026-10-31 12:43:24–12:45:26 JST (03:43:24–03:45:26 UTC)
- Deployed: main @ 7fb1225 (batch C + #23). App binary 28,938,037 bytes on isu1/isu2/isu3; isu1 also runs isureal-europe (:8081) and isureal-america (:8082); PostgreSQL max_connections=200 on isu4.
- Deploy note: the first PG restart failed because the new conf.d file was mode 640 root-owned (PostgreSQL could not read it); fixed to 644 and started via pg_ctlcluster. ~2 min DB downtime, no bench running, no score impact.
- Grand prix entrants: asia 16 / europe 8 / america 5 (europe best so far)
- Proxy effect (isu2/isu3 LTSV, full path): posts eu avg 2.02->0.64s, am 2.07->0.66s; friend-requests eu/am ~1.69->0.16s; login eu/am ~0.96->0.13s. asia posts avg 0.59s (mild contention). 5xx = 0 on all servers.
- Remaining: eu/am GETs still local (feeds eu avg 0.93s, images eu/am ~0.59s) — proxy stage 2 (GETs to asia, SSE local) is the sketched next step in the nginx config comments.
- Log extraction windows: nginx LTSV slices in this dir = 03:43–03:45 UTC each server. PostgreSQL aggregate = 03:43:00–03:46:00 UTC on isu-db.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt
