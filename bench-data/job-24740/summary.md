# Job #24740 — Passed, score 30560 (same code as Job #24339; diagnostics capture run)
- Run window: 2026-10-31 15:49:49–15:51:23 JST (06:49:49–06:51:23 UTC). Deployed: main @ a7d6ce5 (identical to Job #24339) + isu1 diagnostics drop-in enabled.
- Grand prix: asia 63 / europe 46 / america 26 — an asia-heavy draw (asia reached 118 users; x1 multiplier), hence the lower score than Job #24339 on identical code.
- Posts (uri-exact): avg 1.374/1.366/1.271, creations 744 total (378/209/157). 5xx = 0. PG: 74,039 queries / 145.6s.
- Diagnostics (diagnostics/): DB pool InUse hit the 30 cap, WaitCount +31,162 during the run, cumulative WaitDuration ~5,840s — the shared pool is a live bottleneck at this load. Image URL cache hits 25,698 / misses 46,330.

## Merged PRs newly reflected in this run
- None (re-roll of the Job #24339 state; only the isu1 diagnostics drop-in was added, app code unchanged).

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78 + #80 + #77 + #79 + #81 + #21 (see Job #24339 summary for the full list).
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt, diagnostics/ (PG slice at isu-db:/tmp/pg-slice-z.txt)
