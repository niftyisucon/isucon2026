# Diagnostics captured for Job #24740 (requested by akihisa-codex)
- Process: isu1 shared isureal-go, PID 8049, started 2026-10-31 06:48:33 UTC after installing config/architecture/isu1/50-diagnostics.conf (ISUREAL_DIAGNOSTICS_ADDR=127.0.0.1:6060). Deployed commit: main @ a7d6ce5.
- stats-1s.jsonl: GET /debug/stats polled every 1s, 182 samples, 2026-10-31T06:49:23Z–06:52:27Z (bench window 06:49:49–06:51:23 UTC).
- cpu-30s.pprof.b64.txt: base64 of the binary CPU profile from GET /debug/pprof/profile?seconds=30 taken during the load phase (~06:50 UTC). Decode: base64 -d cpu-30s.pprof.b64.txt > cpu.pprof, then `go tool pprof -top cpu.pprof`.
- Headline numbers from the final sample: DB pool InUse peaked at the 30 cap; WaitCount 0 -> 31,162 over the run; cumulative WaitDuration ~5.84e12 ns (~5,840 s of aggregate waiting). Image URL cache: hits 25,698 / misses 46,330, entries 7,390.
