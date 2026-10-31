# Job #6888 — FAIL, score 0 (invalid measurement)
- Run: 2026-10-31 12:09:17–12:09:39 JST. Failed in initial-data verification: GET /api/me/export returned 502 for one initial user.
- Cause: isu2 nginx could not write its proxy temp file (Permission denied) because /var/lib/nginx was not owned by www-data. The app itself returned 200. Fixed by normalizing ownership on all 3 servers; batch B code was not the cause (proven by Job #7091 passing with the same code).
- Files: isu{1,2,3}-ltsv.txt = LTSV slices for 03:09 UTC.
