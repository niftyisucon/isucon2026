# Job #26419 — Passed, score 47155 (PR #91 solo: image URL expiry bucketed to 60s) — NEW BEST
- Run window: 2026-10-31 16:07:51–16:09:24 JST (07:07:51–07:09:24 UTC). Deployed: main @ 6a1dc1b on top of the Job #25844 state.
- Grand prix: asia 15 / europe 160 / america 110 (total 285; europe reached 161 users).
- Posts (uri-exact): avg 0.450/1.417/0.984, creations 885 total (85/462/338). 5xx = 0; deduction 0 and empty messages — NO image verification errors, so PR #91 is ADOPTED (the reviewer's solo-bench condition).
- PG: 91,428 queries / 251.2s — the highest throughput yet.

## Merged PRs newly reflected in this run
- PR #91 (merge 6a1dc1b, by komatsu-codex-premium): signed image URL expiry is rounded DOWN to 60-second buckets (lifetime stays >23h59m and <=24h; URL format, HMAC-SHA256 x1000 signing and all GET-side verification unchanged). Same image+key within a minute now shares one signature, so the PR #79 signing cache hits far more often (signing was ~40% of isu1 CPU in the Job #24740 profile). Rollback: ISUREAL_IMAGE_URL_EXPIRY_BUCKET_SECONDS=1.

## Cumulative merged PRs in this state
Batches A–I + #64 + #76 + #78 + #80 + #77 + #79 + #81 + #21 + #83 + #85 + #91 (see Job #25844 summary; plus #91).
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-bb.txt)
