# Job #21658 — Passed, score 27235 (PR #78: ranked feed detail fetch capped at the page)
- Run window: 2026-10-31 15:15:42–15:17:12 JST (06:15:42–06:17:12 UTC). Deployed: main @ 4443417 on top of the Job #20957 state.
- Grand prix: asia 15 / europe 59 / america 50 (total 124). America grew slowly this draw (day5: 51 vs 77 in Job #20957) — the score gap vs 29335 is mostly population draw, not a regression signal.
- Feed latency (all feed kinds; ranked/FoF proxy): avg 0.146/0.349/0.342, p95 0.507/0.832/0.810 — slightly BETTER than Job #20957 (avg 0.152/0.395/0.393, p95 0.558/0.979/0.952) at comparable counts, so #78 reads neutral-to-positive on its target metric.
- Posts: avg 0.395/0.695/0.725, creations 690 total. 5xx = 0. PG: 57,642 queries / 152.3s.

## Merged PRs newly reflected in this run
- PR #78 (merge 4443417, by komatsu-codex-premium): ranked (friends-of-friends) feed now scores all eligible candidates but materializes post/author details ONLY for the returned page (at most 51), inside a READ ONLY REPEATABLE READ transaction so ranking and detail fetch see one consistent snapshot. Ordering, cursor and hasMore semantics unchanged (verified against the old implementation across full cursor pages on PG16).

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55, #64, #76, #78; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-v.txt)
