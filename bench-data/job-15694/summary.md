# Job #15694 — Passed, score 17766 (Batch I step 1: PR #49 alone, shared_buffers)
- Run window: 2026-10-31 14:05:27–14:06:57 JST (05:05:27–05:06:57 UTC). Deployed: main @ cba7484 (code identical to Batch H state; only the isu4 PostgreSQL config changed).
- Grand prix: asia 17 / europe 31 / america 30 (total 78; Job #14902 had 79 and 19032). Latency is flat-to-slightly-better vs #14902 (feeds eu 0.240->0.194, am 0.232->0.213; images comparable; posts mixed by region with different per-region counts). No regression; no dramatic gain either — shared_buffers 1GB kept as a principled setting for the dedicated DB host.
- 5xx = 0 on all servers.
- cpu/pidstat-isu1-filtered.txt: pidstat rows filtered to app/nginx/collector processes (full raw capture remains at isu1:/tmp/pidstat-batchI1.txt) -u -r -w -p ALL 1s capture on isu1 spanning the run (Issue #63 request).

## Merged PRs newly reflected in this run
- PR #49 (merge cba7484): PostgreSQL memory config for isu4 — new conf.d/90-isu-memory.conf sets `shared_buffers = 1GB` (was 128MB, ~27% of the host's 3.7GiB) and `effective_cache_size = 2GB` (was 4GB). work_mem stays 4MB; durability settings untouched. Required a PostgreSQL restart; verified live: shared_buffers=131072 (8kB), effective_cache_size=262144, pending_restart=false, pg_file_settings errors=0.

## Cumulative merged PRs in this state
Batches A–I: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged — duplicated #57.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt, cpu/pidstat-isu1.txt (PG slice at isu-db:/tmp/pg-slice-p.txt)
