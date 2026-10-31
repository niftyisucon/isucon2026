import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("pgfingerprint", ROOT / "lib/isu/pgfingerprint.py")
pg = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pg)


def line(ms, sql, kind="execute stmtcache_x", second="03", pid=101):
    return f"2026-10-31 06:08:{second}.125 UTC [{pid}] private_user@private_db LOG:  duration: {ms} ms  {kind}: {sql}\n"


class PostgreSQLFingerprintTests(unittest.TestCase):
    def test_literals_comments_and_identifiers_are_not_exported(self):
        query = r'''SELECT 'SECRET_PLAIN', E'SECRET_ESCAPE\'still private',
        U&'SECRET_UNICODE', B'10101', X'CAFE', N'SECRET_NATIONAL',
        'it''s SECRET_DOUBLED', $$SECRET_DOLLAR$$, $body$SECRET_TAGGED$body$,
        "SECRET_IDENTIFIER", U&"SECRET_UNICODE_ID", 0xCAFE, 0o755, 0b101,
        1_234.56e-7, .001, $1 FROM users /* SECRET_COMMENT /* nested */ rest */
        WHERE id IN ($2,$3,$4) -- SECRET_LINE_COMMENT
        AND region='SECRET_REGION' '''
        result = pg.canonical_sql(query)
        self.assertNotIn("secret", result)
        self.assertNotIn("private", result)
        self.assertNotIn("cafe", result)
        self.assertNotIn("234", result)
        self.assertIn("from users", result)
        self.assertIn("in ( ? )", result)
        self.assertEqual(pg.canonical_sql("-- SECRET_PING"), "<comment-only statement>")
        self.assertEqual(pg.canonical_sql("SELECT 'a\\', 'PRIVATE_NEXT'"), "select ? , ?")
        for query in ["SELECT 'SECRET", 'SELECT "SECRET', "SELECT $tag$SECRET", "SELECT /* SECRET", "SELECT"]:
            with self.assertRaises(ValueError) as error:
                pg.canonical_sql(query)
            self.assertNotIn("SECRET", str(error.exception))

    def test_multiline_and_protocol_accounting(self):
        text = line("1", "SELECT $1", "parse stmtcache_x")
        text += line("2", "SELECT $1", "bind stmtcache_x")
        text += line("4", "") + "\tSELECT id FROM users\n\tWHERE id=$1\n"
        text += "2026-10-31 06:08:03.126 UTC [101] private@db DETAIL:  parameters: $1='SECRET_PARAMETER'\n"
        text += "2026-10-31 06:08:03.126 UTC [101] private@db ERROR:  duration: 123 ms  statement: SELECT SECRET_ERROR_BODY\n"
        text += line("1", "SELECT 1", "statement", pid=102)
        text += line("2", "commit", "statement")
        text += "2026-10-31 06:08:03.127 UTC [103] private@db LOG:  duration: 2.000 ms\n"
        text += line("2", "", pid=104)
        text += line("1", "SELECT 2", "statement", pid=104)
        result = pg.summarize(io.StringIO(text))
        self.assertEqual(result["total_count"], 8)
        self.assertEqual(result["total_ms"], 15)
        self.assertEqual(result["fingerprint_count"], 6)
        self.assertEqual(result["fingerprint_total_ms"], 11)
        self.assertEqual(result["unattributed_count"], 2)
        self.assertEqual(result["unattributed_ms"], 4)
        self.assertEqual(result["first_sql_line_blank"], 2)
        self.assertEqual(result["recovered_from_continuations"], 1)
        self.assertEqual(result["by_kind"]["parse"]["total_ms"], 1)
        self.assertEqual(result["by_kind"]["bind"]["total_ms"], 2)
        self.assertIn("2026-10-31 06:08:00", result["buckets_10s"])
        self.assertFalse(result["complete"])
        self.assertNotIn("SECRET_PARAMETER", str(result))
        self.assertNotIn("secret_error_body", str(result))
        self.assertNotIn("private_user", str(result))

    def test_interval_keeps_sql_continuations_and_includes_end_second(self):
        text = line("1", "SELECT 1", second="02")
        text += line("5", "", second="03") + "\tSELECT id\n\tFROM users WHERE id=$1\n"
        text += line("6", "SELECT 2", second="04")
        result = pg.summarize(io.StringIO(text), "2026-10-31 06:08:03", "2026-10-31 06:08:03")
        self.assertEqual(result["total_ms"], 5)
        self.assertEqual(result["total_count"], 1)
        self.assertEqual(result["outside_interval"], 2)
        self.assertIn("from users", result["top_queries"][0]["sql"])
        self.assertTrue(result["complete"])

    def test_missing_continuations_and_orphans_fail_closed(self):
        result = pg.summarize(io.StringIO("\torphan sql SECRET\n" + line("3", "")))
        self.assertEqual(result["unrecognized_lines"], 1)
        self.assertFalse(result["complete"])
        self.assertEqual(result["top_queries"], [])
        self.assertNotIn("SECRET", str(result))
        with self.assertRaises(ValueError):
            pg.summarize(io.StringIO(line("1", "SELECT 1").replace(" UTC ", " JST ")))
        with self.assertRaises(ValueError):
            pg.summarize(io.StringIO(line("1", "SET standard_conforming_strings=off", "statement")))
        self.assertFalse(pg.summarize(io.StringIO(""))["complete"])

    def test_cli_outputs_no_literals_and_nonzero_on_missing_sql(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "pg.log"
            path.write_text(line("3", "SELECT 'SECRET_CLI'", "statement"))
            command = [str(ROOT / "bin/isu"), "pgfingerprint", "--input", str(path)]
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(json.loads(result.stdout)["complete"])
            self.assertNotIn("SECRET_CLI", result.stdout + result.stderr)
            path.write_text(line("3", ""))
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 2)
            self.assertFalse(json.loads(result.stdout)["complete"])


if __name__ == "__main__":
    unittest.main()
