# Job #20957 — Passed, score 29335 (PR #76: persisted search normalization columns) — NEW BEST
- Run window: 2026-10-31 15:08:03–15:09:34 JST (06:08:03–06:09:34 UTC). Deployed: main @ 74e131b on top of the Job #20042 state.
- Grand prix: asia 16 / europe 61 / america 76 (total 153; europe reached 62 users, america 77 during day 5).
- Latency: posts avg 0.449/0.859/0.833, creations 521 total (102/186/233, new max); user search avg 0.159/0.327/0.361 (n=44/186/196, uri-exact); images avg 0.043/0.247/0.256. 5xx = 0.
- PG: 61,934 queries / 152.4s total at the highest activity level yet.
- Deploy rehearsal (reviewer conditions): applied outside a bench; all three app hosts restarted SIMULTANEOUSLY — no panics, advisory-lock serialization worked, all 5,131 users' search columns backfilled at startup; search endpoint verified (200, literal %_ handling).

- Correction (2026-10-31, komatsu-muse): post-creation counts/averages originally in this summary used a substring match on the LTSV req field, which also counted POST /api/posts/{id}/comments and /realmojis (both 201). Figures above are recomputed with an exact uri-field match (flagged by akihisa-codex in Issue #65). Job #20957 search counts were likewise blended with profile fetches and are corrected to uri-exact.

## Merged PRs newly reflected in this run
- PR #76 (merge 74e131b, by komatsu-codex-premium): user search normalization persisted. users gains search_username/search_display_name (NFC + Unicode case-folded, same normalization as Go). Register and display-name changes maintain the columns in the same transaction; a trigger NULLs them if an old binary updates names, so stale rows fall back to the exact Go predicate. Search now matches prepared rows IN SQL with strpos (literal-safe for %/_/backslash), applies cursor/active/block filters and takes at most 51 candidates; unprepared (NULL) rows are unioned separately so they never consume the 51 slots. prepareUserSearch runs at process startup and after initialize: pg_advisory_xact_lock serializes the DDL across simultaneous region starts, columns/triggers are idempotent, backfill proceeds in 256-row batches guarded by original-name equality. reset.sql/001-schema.sql carry the columns; reset NULLs them for restored initial users and initialize re-prepares.

## Cumulative merged PRs in this state
Batches A–I + #64 + #76: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55, #64, #76; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-u.txt)
