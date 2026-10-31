# Job #6888 — FAIL, score 0 (invalid measurement)
- Run window: 2026-10-31 12:09:17–12:09:39 JST (03:09:17–03:09:39 UTC)
- Deployed code: main @ 4d48eed (batch B) — same code as Job #7091; the code was not the cause.
- Cause: initial-data verification failed: GET /api/me/export returned 502 for initial user club_de_recherche_triple_4141. isu2's nginx could not write its proxy temp file (Permission denied) because /var/lib/nginx was not owned by www-data, while the app returned 200. Fixed by normalizing ownership on all 3 app servers; Job #7091 then passed with the same build.
- Log extraction window: nginx LTSV slices in this dir = 03:09 UTC each server.
- Files: portal.txt (verbatim), isu{1,2,3}-ltsv.txt
