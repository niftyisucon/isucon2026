# Job #16824 — Passed, score 20826 (Batch I step 3: PRs #52 + #55) — NEW BEST
- Run window: 2026-10-31 14:19:10–14:20:45 JST (05:19:10–05:20:45 UTC). Deployed: main @ 892a119.
- Grand prix: asia 16 / europe 32 / america 30 (total 78). Saved POST /api/posts 201 responses total 380 (107/148/125); the former 524/highest-yet claim is withdrawn. Representative metrics are reproduced below; saved-log 5xx = 0. The former user-search averages are withdrawn because their source/conditions are unconfirmed.
- PG aggregate in pg-aggregate.txt.
- Batch I sequence (#49 -> #56 -> #52+#55) scores: 17766 -> 19809 -> 20826 (draws vary; these scores alone do not establish a throughput trend).

## Merged PRs newly reflected in this run
- PR #52 (merge c5318d2): ranked (friends-of-friends) feed — the eligibility predicate moves into SQL: author NOT IN blocked, NOT IN friends, IN friends-of-friends, visibility IN ('public','friends_of_friends'), applied BEFORE ranking/pagination so scoring sees the true candidate set instead of filtering in Go after fetch. Ships an exhaustive SQL-vs-Go equivalence test over relationship/visibility combinations.
- PR #55 (merge 892a119): user search — the candidate scan now selects only id/username/display_name and excludes blocked users in SQL; Unicode NFC + case-fold matching stays in Go (normalizeSearchText), collects at most 51 matching IDs, and only then loads full user rows (previously every active user's full row was transferred and filtered in Go). Response shape unchanged.

## Cumulative merged PRs in this state
Batches A–I: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56, #52 #55; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-r.txt)


## Saved-log metrics (Issue #65 correction)

Use the [Job #16256 extraction conditions and checking procedure](../job-16256/summary.md#saved-log-metrics-issue-65-correction):
all saved rows, exact method and status, query-stripped path (prefix for feeds/images),
arithmetic mean of numeric `apptime` in seconds. Counts include checks and are not portal score counts.

<!-- representative-api-metrics -->
| Host | API (method / path / status) | n | avg apptime (seconds) |
|---|---|---:|---:|
| isu1 | POST /api/posts / 201 | 107 | 0.999 |
| isu1 | GET /api/feeds/* / 200 | 828 | 0.140 |
| isu1 | GET /api/images/* / 200 | 1789 | 0.048 |
| isu2 | POST /api/posts / 201 | 148 | 2.411 |
| isu2 | GET /api/feeds/* / 200 | 613 | 0.284 |
| isu2 | GET /api/images/* / 200 | 829 | 0.284 |
| isu3 | POST /api/posts / 201 | 125 | 2.353 |
| isu3 | GET /api/feeds/* / 200 | 646 | 0.221 |
| isu3 | GET /api/images/* / 200 | 770 | 0.229 |
<!-- /representative-api-metrics -->

```sh
python3 tools/bench/check-summary.py bench-data/job-16824 --check
```

- The table reproduces all three aggregate POST /api/posts rows. The former summary's post counts
  154 / 187 / 183 exceed all saved POST /api/posts rows (107 / 148 / 125, all status 201).
  A narrower time/status filter on these files cannot produce the former counts.
- Saved files contain 6822 / 3569 / 3305 rows, with UTC timestamp ranges on 2026-10-31 of
  05:19:15–05:21:20 / 05:19:15–05:21:17 / 05:19:16–05:21:18.
  Post response ranges are 05:19:29–05:20:23 / 05:19:30–05:20:27 / 05:19:33–05:20:23.
- `portal.txt` records start 14:19:10.643 and finish 14:20:45.021 (summary identifies JST),
  i.e. 05:19:10.643–05:20:45.021 UTC. All saved post responses are before this finish;
  removing trailing responses leaves the post metrics unchanged. After 05:20:45 UTC, only
  GET /api/notifications appears: isu1 2 status 200 + 18 status 499, isu2 32 status 499,
  isu3 30 status 499. No clock offset is inferred.
- Feed/image means in the former summary reproduce. The former search means 0.139 / 0.319 / 0.265
  do not: all saved GET /api/users status 200 rows (query removed) give n=63 / 110 / 87,
  mean apptime 0.234 / 0.385 / 0.345 seconds. No alternate search condition is documented,
  so the former search claim is withdrawn rather than used as comparison evidence.
- Provenance: `db1f939` added summary/portal/aggregate, `c525a1d` and `f03ece4` added LTSV.
  `portal.txt` is explicitly an excerpt; complete source nginx logs, extraction command and full portal
  stdout are not preserved in this package. Passed / score 20826 and the recorded deployment are retained;
  this correction did not independently re-verify the portal. The origin of the former counts/search means,
  source-slice completeness, and whether this shares Job #16256's generating cause remain unconfirmed.
  The existing source inquiry to Muse remains pending; no duplicate inquiry or alternate job is inferred.
