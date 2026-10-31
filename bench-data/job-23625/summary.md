# Job #23625 — Passed, score 40590 (PRs #77 + #79) — NEW BEST by a wide margin
- Run window: 2026-10-31 15:38:26–15:39:58 JST (06:38:26–06:39:58 UTC). Deployed: main @ 39e6c15 on top of the Job #22627 state.
- Grand prix: asia 10 / europe 103 / america 119 (total 232). Overseas populations roughly doubled vs any previous run (europe reached 103 users, america 120 during day 5).
- Posts (uri-exact): avg 0.478/1.000/1.046 under the much heavier load, creations 742 total (87/302/353). Images avg 0.046/0.291/0.316. 5xx = 0.
- PG: 82,053 queries / 238.9s total — the highest load yet; the stack is now saturated at a far higher activity level, and the score jump is population-compounding (overseas regions count x1.5).

## Merged PRs newly reflected in this run
- PR #77 (merge b70c903, by komatsu-codex-premium): Memories interaction loading now bounds reactions and comments PER POST to the newest 50 (after excluding deleted/replaced rows and inactive reactors) via LATERAL subqueries — still one query per kind for the whole page.
- PR #79 (merge 39e6c15, by michito-codex): (1) image signed-URL generation memoizes the pure HMAC signing step (cache key = SHA-256 of the user's image key + image ID + expiry second; 16 shards x 1024 entries; every caller still loads the current user/key and checks visibility, and image GET still verifies current DB state; disable with ISUREAL_IMAGE_URL_CACHE=false). (2) Diagnostics (default off, loopback only): runtime stats endpoint support, DB pool stats / GC / 30s CPU pprof capture hooks, extra APM spans (session lookup, edge-to-core propagation). The optional isu1 diagnostics drop-in (config/architecture/isu1/50-diagnostics.conf) was NOT installed for this run.

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78 + #80 + #77 + #79: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55, #64, #76, #78, #80, #77, #79; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged; PR #21 still open.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-x.txt)
