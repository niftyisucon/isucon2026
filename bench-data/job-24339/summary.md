# Job #24339 — Passed, score 41096 (PR #21 deployed; PR #81 is tooling only) — NEW BEST
- Run window: 2026-10-31 15:45:50–15:47:18 JST (06:45:50–06:47:18 UTC). Deployed: main @ a7d6ce5 on top of the Job #23625 state.
- Grand prix: asia 11 / europe 145 / america 104 (total 260; europe reached 147 users).
- Posts (uri-exact): avg 0.459/1.308/1.027 under heavy load, creations 811 total (71/422/318). 5xx = 0. PG: 81,124 queries / 227.9s.

## Merged PRs newly reflected in this run
- PR #21 (merge a7d6ce5): posting-window start replaces the post-INSERT re-SELECT with RETURNING, trimming a DB round trip while the (now per-region advisory) lock is held. Lock order and notification creation unchanged.
- PR #81 (merge 8adf1ac, by komatsu-codex-premium): tooling only — adds ./bin/isu pgfingerprint, an anonymized PostgreSQL log aggregator that keeps continuation lines (Issue #73). No application change.

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78 + #80 + #77 + #79 + #81 + #21: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55, #64, #76, #78, #80, #77, #79, #81, #21; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged. No PRs remain open.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-y.txt)
