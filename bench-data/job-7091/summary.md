# Job #7091 — Passed, score 8965
- Run window: 2026-10-31 12:12:23–12:14:39 JST (03:12:23–03:14:39 UTC)
- Deployed code: main @ 4d48eed (after batch B). App binary 28,928,105 bytes on isu1/isu2/isu3.
- Merged PRs reflected (cumulative): #1, #2, #3, #4 (logging), #5, #6, #7 (indexes), #8 (tracing infra), #10, #14, #18. Batch B = #10 + #14 + #18 on top of the Job #5003 state (7918).
- Grand prix entrants: asia 17 / europe 5 / america 5
- Log extraction windows: nginx LTSV slices in this dir = 03:12–03:14 UTC each server. PostgreSQL aggregate (pg-aggregate.txt) = 03:12:00–03:15:00 UTC on isu-db; raw slice reproducible from /var/log/postgresql/postgresql-16-main.log by timestamp, or ./bin/isu pglog isu4 --begin/--end.
- Files: portal.txt (portal page verbatim: summary/result/conclusion/stdout), isu{1,2,3}-ltsv.txt, aggregate.txt (nginx per-endpoint), pg-aggregate.txt
- Key moves vs #5003: posts avg asia 0.86→0.47s, eu 4.49→2.04s, am 3.95→2.13s; images out of the top apptime ranks; PG: LOCK TABLE users 62 exec / 38.8s total is the next contention source.
