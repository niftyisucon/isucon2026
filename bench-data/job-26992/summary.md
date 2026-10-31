# Job #26992 — Passed, score 34749 (PRs #86 + #90)
- Run window: 2026-10-31 16:13:58–16:15:30 JST (07:13:58–07:15:30 UTC). Deployed: main @ 83710a5 on top of the Job #26419 state.
- Grand prix: asia 16 / europe 47 / america 69 — asia population ballooned (92 users) but scored few GP actions; overseas GP counts were low this draw.
- Posts (uri-exact): avg 1.032/0.951/0.826, creations 707 total (296/187/224). 5xx = 0, deduction 0, no critical errors — the PR #86 first-bench check (no login 401 failures) passes; the 401s in the LTSV are the expected unauthenticated probes on isu1.
- PG: 71,019 queries / 239.0s.

## Merged PRs newly reflected in this run
- PR #86 (merge e21d6c0, by komatsu-codex-premium): session lookups are served from an in-memory cache in the isu1 core process only (edges keep the DB path; PostgreSQL remains the source of truth). Cache misses fall back to the SELECT and populate; invalidation runs after commit attempts on login/logout/deletion/region-change, and initialize wipes it. Removes one pool acquisition per authenticated request (session.lookup averaged ~88ms under load, mostly pool wait). Rollback: ISUREAL_SESSION_CACHE=false.
- PR #90 (merge 83710a5, by komatsu-codex-premium): Memories export image JOIN prunes image_objects partitions via the existing image_locations locator (same pattern as PR #45); unregistered images and old schemas fall back to the previous full search. No DDL.

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78 + #80 + #77 + #79 + #81 + #21 + #83 + #85 + #91 + #86 + #90 (see Job #26419 summary; plus #86 #90).
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-cc.txt)
