# Job #22627 — Passed, score 29112 (PR #80: per-region posting-window locks)
- Run window: 2026-10-31 15:27:12–15:28:43 JST (06:27:12–06:28:43 UTC). Deployed: main @ 90ee464 on top of the Job #21658 state.
- Grand prix: asia 23 / europe 65 / america 52 (total 140). Asia unusually grew to 27 users by day 5.
- Posts (uri-exact): avg 0.570/0.878/0.855, creations 566 total (126/243/197). 5xx = 0. PG: 61,376 queries / 154.1s.
- Read: #80 shows no regression; its effect is within draw variance, consistent with the reviewer's estimate (table-lock time was ~1.4s per run).

## Merged PRs newly reflected in this run
- PR #80 (merge 90ee464, by komatsu-codex-premium): the users TABLE lock at posting-window start (LOCK TABLE users IN SHARE ROW EXCLUSIVE) is replaced by per-region advisory transaction locks (pg_advisory_xact_lock[_shared], keys (20261031, 7201/7202/7203)). Window start takes its region exclusively; registration takes its region shared; region moves and account deletion take all three shared locks in a fixed order before touching user/window rows. Window starts for other regions and unrelated user updates no longer serialize against a start. All app hosts were switched together (no mixed old/new membership handling).

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78 + #80: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55, #64, #76, #78, #80; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-w.txt)
