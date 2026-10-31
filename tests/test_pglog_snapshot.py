import gzip
import importlib.util
import io
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "lib/isu/remote/postgresql-log.sh"
spec = importlib.util.spec_from_file_location("pgfingerprint", ROOT / "lib/isu/pgfingerprint.py")
pg = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pg)


def record(second, sql="SELECT 1"):
    return f"2026-10-31 06:08:{second:02d}.125 UTC [101] private@db LOG:  duration: 1 ms  execute stmt: {sql}\n"


class PostgreSQLSnapshotTests(unittest.TestCase):
    def capture(self, content, mib="1", begin="", end=""):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "postgresql.log"
            path.write_bytes(content.encode())
            result = subprocess.run(["bash", str(SCRIPT), "tail", str(path), "unused", mib, begin, end], capture_output=True)
            self.assertEqual(path.read_bytes(), content.encode(), "source must remain unchanged")
            return result

    def test_multiline_sql_detail_and_final_record(self):
        sql = record(1, "") + "\tSELECT id FROM users\n\tWHERE id='PRIVATE_VALUE'\n"
        detail = "2026-10-31 06:08:01.126 UTC [101] private@db DETAIL:  parameters: $1='PRIVATE_PARAMETER'\n"
        kept = record(0) + sql + detail + record(2)
        result = self.capture(kept + record(3), begin="2026-10-31 06:08:01", end="2026-10-31 06:08:01")
        self.assertEqual(result.returncode, 0, result.stderr)
        decoded = gzip.decompress(result.stdout).decode()
        self.assertEqual(decoded, kept)
        summary = pg.summarize(io.StringIO(decoded), "2026-10-31 06:08:01", "2026-10-31 06:08:01")
        self.assertTrue(summary["complete"])
        self.assertEqual(summary["recovered_from_continuations"], 1)
        self.assertEqual(summary["total_count"], 1)
        self.assertNotIn(b"PRIVATE", result.stderr)
        self.assertIn(b"last_record_omitted=true", result.stderr)

    def test_start_inside_old_record_drops_orphan_continuations(self):
        old = record(0, "") + ("\tSELECT 'old'\n" * 100000)
        kept = record(1, "") + "\tSELECT 'new'\n\tFROM users\n" + record(2)
        result = self.capture(old + kept + record(3))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(gzip.decompress(result.stdout).decode(), kept)
        self.assertNotIn(b"start_byte=0\n", result.stderr)

    def test_missing_begin_fails(self):
        result = self.capture(record(1) + record(2) + record(3), begin="2026-10-31 06:08:00")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"does not cover begin", result.stderr)

    def test_end_requires_next_second_in_retained_records(self):
        result = self.capture(record(0) + record(1) + record(2), end="2026-10-31 06:08:01")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"full end second", result.stderr)

    def test_empty_wrong_timezone_and_single_record_fail(self):
        for text in ["", record(0), (record(0) + record(1)).replace(" UTC ", " JST ")]:
            with self.subTest(text=text[:30]):
                result = self.capture(text)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(b"no complete UTC stderr records", result.stderr)

    def test_partial_last_line_is_omitted(self):
        kept = record(0, "") + "\tSELECT 1\n"
        result = self.capture(kept + record(1, "") + "\tSELECT 'unfinished")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(gzip.decompress(result.stdout).decode(), kept)

    def test_invalid_size_and_oversized_record_fail(self):
        for mib in ["", "0", "-1", "4097", "1.5", "01"]:
            with self.subTest(mib=mib):
                result = self.capture(record(0) + record(1), mib=mib)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(b"tail-mib", result.stderr)
        result = self.capture(record(0, "") + "\t" + "x" * (16 * 1024 * 1024) + "\n" + record(1), mib="32")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"16 MiB safety limit", result.stderr)

    def test_open_descriptor_survives_rename_and_excludes_appends(self):
        self.capture_during_mutation(truncate=False)

    def test_truncate_during_snapshot_fails(self):
        self.capture_during_mutation(truncate=True)

    def capture_during_mutation(self, truncate):
        # A record larger than the stdout pipe blocks compression after the
        # file is opened/sized, while many records are still unread.
        sql = "SELECT '" + os.urandom(128 * 1024).hex() + "'"
        parts = [record(i, sql) for i in range(40)]
        content = "".join(parts).encode()
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "postgresql.log"
            path.write_bytes(content)
            with subprocess.Popen(["bash", str(SCRIPT), "tail", str(path), "unused", "32", "", ""], stdout=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=0) as process:
                first = process.stdout.read(1)
                self.assertEqual(first, b"\x1f")
                if truncate:
                    path.write_bytes(b"")
                else:
                    rotated = path.with_suffix(".old")
                    path.rename(rotated)
                    path.write_text(record(50, "SELECT 'replacement'"))
                    with rotated.open("ab") as stream:
                        stream.write(record(51, "SELECT 'appended'").encode())
                output, diagnostic = process.communicate(timeout=15)
            if truncate:
                self.assertNotEqual(process.returncode, 0)
                self.assertIn(b"truncated", diagnostic)
            else:
                self.assertEqual(process.returncode, 0, diagnostic)
                self.assertEqual(gzip.decompress(first + output), "".join(parts[:-1]).encode())

    def test_cli_removes_failed_snapshot_before_analysis(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "analyzer.pl").write_text('die "analyzer must not run";')
            harness = r'''
set -euo pipefail
ISU_HOSTS=(isu4)
ISU_LOCAL_DIR=$1
ISU_PGBADGER_BIN=$1/analyzer.pl
ISU_PG_LOG_PATH=unused
ISU_DB_NAME=unused
has_role() { return 0; }
is_host() { return 0; }
die() { echo "$*" >&2; exit 1; }
log() { :; }
warn() { echo "$*" >&2; }
rscript() {
  if [ "$3" = info ]; then printf '%%m [%%p] %%q%%u@%%d \nEtc/UTC\n0\nstderr\n'; return; fi
  [ "$3" = tail ] && [ "$6" = 1 ] && [ "$7" = '2026-10-31 06:08:00' ] || exit 90
  printf 'incomplete gzip'
  echo 'snapshot does not cover begin' >&2
  return 1
}
source "$2"
cmd_pglog isu4 --tail-mib 1 --begin '2026-10-31 06:08:00' --output-dir "$1/out"
'''
            result = subprocess.run(["bash", "-c", harness, "test", directory, str(ROOT / "lib/isu/cmd/pglog.sh")], capture_output=True)
            self.assertEqual(result.returncode, 1, result.stderr)
            self.assertNotIn(b"analyzer must not run", result.stderr)
            self.assertFalse((root / "out/postgresql.log.gz").exists())
            self.assertFalse((root / "out/postgresql.log.gz.tmp").exists())
            self.assertIn("does not cover begin", (root / "out/snapshot-info.txt").read_text())


if __name__ == "__main__":
    unittest.main()
