# Job #16256 — Passed, score 19809 (Batch I step 2: PR #56 alone, SSE aggregation) — NEW BEST
- Run window: 2026-10-31 14:12:31–14:14:02 JST (05:12:31–05:14:02 UTC). Deployed: main @ b7d273d.
- Grand prix: asia 20 / europe 30 / america 21 (total 71). Asia population 18->22 is recorded in portal.txt. The previous cross-run throughput claim is withdrawn pending comparable extraction conditions.
- Saved-log metrics: see the reproducible table below; 5xx = 0 across all saved rows. These are nginx upstream response times, not portal/client latency or scored request counts.
- PG aggregate in pg-aggregate.txt.

## Merged PRs newly reflected in this run
- PR #56 (merge b7d273d): notification SSE polling aggregated per process. Before: every SSE connection ran its own 500ms DB query (users LEFT JOIN LATERAL notifications). After: one poller goroutine per process snapshots all live subscriptions and runs ONE query per 500ms tick for all of them (jsonb_to_recordset + WITH ORDINALITY, same per-user LATERAL semantics), delivering each subscriber its own notification ID; region mismatch or DB error closes the subscription exactly as before; initialize resets all subscriptions. Handler behavior (one event, then close) unchanged.

## Cumulative merged PRs in this state
Batches A–I: #2 #6 #3, #10 #14 #18, #28 #43, #23, #60, #45, #47 #48, #51 #26 #27, #46 #53 #54 #57, #49, #56; plus #1 #4 #7 #9 #8 #15. (PR #50 closed unmerged.)
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt (PG slice at isu-db:/tmp/pg-slice-q.txt)


## Saved-log metrics (Issue #65 correction)

All rows of each `isu{1,2,3}-ltsv.txt` are included; no additional time filter.
Match `method`, query-stripped `uri`, and `status` exactly as shown (the `*` suffix means prefix matching).
Average is the arithmetic mean of numeric `apptime` in seconds, rounded to three decimals.
The tracked nginx format maps `apptime` to `$upstream_response_time`, `reqtime` to `$request_time`,
and `time` to `$time_local` (second precision). Do not mix these fields or treat these counts as portal score counts.

<!-- representative-api-metrics -->
| Host | API (method / path / status) | n | avg apptime (seconds) |
|---|---|---:|---:|
| isu1 | POST /api/posts / 201 | 113 | 1.470 |
| isu1 | GET /api/feeds/* / 200 | 1087 | 0.152 |
| isu1 | GET /api/images/* / 200 | 1965 | 0.059 |
| isu2 | POST /api/posts / 201 | 136 | 2.304 |
| isu2 | GET /api/feeds/* / 200 | 673 | 0.242 |
| isu2 | GET /api/images/* / 200 | 863 | 0.241 |
| isu3 | POST /api/posts / 201 | 99 | 1.510 |
| isu3 | GET /api/feeds/* / 200 | 616 | 0.182 |
| isu3 | GET /api/images/* / 200 | 732 | 0.196 |
<!-- /representative-api-metrics -->

Reproduce and check the table plus the three `aggregate.txt` POST /api/posts rows from the repository root:

```sh
python3 tools/bench/check-summary.py bench-data/job-16256 --check
```

### Evidence and boundaries

- `portal.txt` is a saved summary/excerpt, not a full portal capture: it explicitly says
  `Stdout (excerpts; full text in portal detail)`. It records start 14:12:31.695 and finish
  14:14:02.332 (the summary identifies these as JST), i.e. 05:12:31.695–05:14:02.332 UTC.
  Passed / score 19809 and the recorded deployment are retained; no independent portal re-verification was performed for this correction.
- Saved files have 7325 / 3570 / 2956 rows. Their `time` ranges are respectively
  05:12:36–05:14:38, 05:12:36–05:14:36, 05:12:36–05:14:38 on 2026-10-31 UTC.
  They include initialization, checks and trailing responses; they are not a load-phase-only slice.
- POST /api/posts has only status 201 in the saved files. Its response timestamps range
  05:12:48–05:13:43 / 05:12:49–05:13:45 / 05:12:53–05:13:44 UTC.
  Keeping only responses before the recorded job finish gives exactly the same post counts and means.
  All rows after 05:14:02 UTC are GET /api/notifications: isu1 has 11 status 200 + 10 status 499,
  isu2 has 30 status 499, isu3 has 21 status 499. Trailing SSE responses therefore do not explain the post mismatch.
- The former summary reported posts 192 / 162 / 143, mean 0.976 / 1.996 / 1.119 seconds.
  Those counts exceed **all** saved POST /api/posts rows, so a narrower time window or status filter
  on these files cannot reproduce them. The aggregate already agrees with the saved files; it needs no numeric change.
  Feed counts/means and image means in the former summary do reproduce from the conditions above.
- Provenance: commit `cc3fdc9` added summary/portal/aggregate; `4a888bb` and `5018e00` added the saved LTSV.
  No original extraction command, complete source nginx log, or full portal stdout is preserved in this package.
  The exact origin of the former post values and completeness of the original slice remain unconfirmed;
  source confirmation was requested from the benchmark operator in Discord. No clock offset or alternate job is inferred.

For future packages, generate the representative table with the same command (replace the job path),
place it between the markers, and run `--check` before citing the summary. A different time window,
status, method, path or timing field must be stated separately with its source and extraction command;
the all-saved-rows checker must not silently be reused for that slice. Preserve full portal output and
original extraction boundaries when available. This check covers these representative APIs only.
