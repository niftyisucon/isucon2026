# Job #8319 — Passed, score 8529 (batch C; -436 vs Job #7091, confirmation re-run in progress)
- Run window: 2026-10-31 12:29:31–12:31:45 JST (03:29:31–03:31:45 UTC)
- Deployed code: main @ 320f080 (batch B state + PR #28 image cache + PR #43 PUT body drain). App binary 28,938,293 bytes on isu1/isu2/isu3.
- Grand prix entrants: asia 17 / europe 6 / america 5
- Log extraction windows: nginx LTSV slices in this dir = 03:29–03:31 UTC each server. PostgreSQL aggregate = 03:29:00–03:32:00 UTC on isu-db.
- Findings vs #7091: 5xx = 0 on all servers (#43 effective). Images improved (n 2493→2584, total apptime 386→318s; #28 cache working). Exports asia 60→70. But asia feeds/friends count 399→269 and asia posts avg 0.47→0.55s; net score -436, within the day's same-code variance band (+/-7%) — a same-state confirmation bench was enqueued right after this job.
- Files: portal.txt, isu{1,2,3}-ltsv.txt, aggregate.txt, pg-aggregate.txt
