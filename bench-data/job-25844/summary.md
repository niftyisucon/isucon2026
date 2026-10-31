# Job #25844 — Passed, score 37136 (PRs #83 + #85; diagnostics drop-in removed)
- Run window: 2026-10-31 16:01:34–16:03:01 JST (07:01:34–07:03:01 UTC). Deployed: main @ 535a772 on top of the Job #24740 state; isu1 diagnostics drop-in REMOVED before this run (clean measurement).
- Grand prix: asia 16 / europe 78 / america 92 (total 186).
- Posts (uri-exact): avg 0.523/0.897/1.014, creations 657 total (98/248/311). Search avg 0.079/0.325/0.348 (n=38/239/254). 5xx = 0. PG: 77,709 queries / 235.8s.

## Merged PRs newly reflected in this run
- PR #83 (merge 9c80f8d, by komatsu-codex-premium): friends-of-friends scoring aggregates relationships/reactions IN SQL when candidates >= 256 (per-post scores only cross the wire); small candidate sets keep the previous path. Same scoring formula, connection cap 30 and reaction dedup; SET LOCAL plan_cache_mode=force_custom_plan inside the read transaction only.
- PR #85 (merge 535a772, by komatsu-codex-premium): user search pushes the predicate down (candidates CTE NOT MATERIALIZED), expresses literal substring match as escaped LIKE so the pg_trgm GIN index on the #76 normalized columns can serve it (strpos fallback for trigram-less terms); NULL-fallback, block/active filters, ordering and cursor unchanged. Index migration webapp/sql/search-index.sql was applied separately on isu-db before this run (pg_trgm extension + users_search_trgm GIN (fastupdate=off) + users_search_legacy partial B-tree; verified present).

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78 + #80 + #77 + #79 + #81 + #21 + #83 + #85 (see Job #24339 summary; plus #83 #85). No PRs remain open.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-aa.txt)
