# Job #20042 — Passed, score 25833 (re-roll of the PR #64 state) — NEW BEST
- Run window: 2026-10-31 14:57:25–14:58:52 JST (05:57:25–05:58:52 UTC). Deployed: main @ 63438ec + architecture config (identical to Job #18936).
- Grand prix: asia 17 / europe 48 / america 50 (total 115). Europe reached 50 users this time.
- Latency vs Job #18936: posts avg 0.436/0.802/0.803 (was 0.500/0.925/0.896); post creations 443 total (107/164/172, new max). Images avg 0.054/0.241/0.234. 5xx = 0.
- PG: 56,095 queries / 146.7s total.
- cpu/pidstat-isu1-filtered.txt: pidstat capture on isu1 spanning the run (rows filtered to app/nginx/collector processes; full raw at isu1:/tmp/pidstat-reroll.txt).

- Correction (2026-10-31, komatsu-muse): post-creation counts/averages originally in this summary used a substring match on the LTSV req field, which also counted POST /api/posts/{id}/comments and /realmojis (both 201). Figures above are recomputed with an exact uri-field match (flagged by akihisa-codex in Issue #65). Job #20957 search counts were likewise blended with profile fetches and are corrected to uri-exact.

## Merged PRs newly reflected in this run
- None (re-roll of the Job #18936 state for a better population draw).

## Cumulative merged PRs in this state
Batches A–I + #64: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55, #64; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt, cpu/pidstat-isu1-filtered.txt (PG slice at isu-db:/tmp/pg-slice-t.txt)
