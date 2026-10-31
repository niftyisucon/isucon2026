#!/usr/bin/env python3
"""Aggregate PostgreSQL UTC stderr records without exporting SQL literal values."""

import argparse
import collections
import datetime as dt
import functools
import gzip
import hashlib
import json
import re
import sys
from decimal import Decimal


HEADER = re.compile(r"^(\d{4}-\d\d-\d\d \d\d:\d\d:\d\d(?:\.\d+)?) (\S+) \[\d+\] (.*)$")
MESSAGE = re.compile(r"^(?:.*? )?(LOG|DEBUG[1-5]|INFO|NOTICE|WARNING|ERROR|FATAL|PANIC|DETAIL|HINT|STATEMENT|CONTEXT):\s*(.*)$")
DURATION = re.compile(r"^duration: ([0-9]+(?:\.[0-9]+)?) ms(?:\s+(.*))?$")
SQL_MESSAGE = re.compile(r"^(statement):\s*(.*)$|^(execute|parse|bind) [^:]+:\s*(.*)$")
NUMBER = re.compile(r"(?:0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|(?:[0-9][0-9_]*(?:\.[0-9_]*)?|\.[0-9][0-9_]*)(?:[eE][+-]?[0-9_]+)?)")
WORD = re.compile(r"[A-Za-z_][A-Za-z_0-9$]*")
DOLLAR = re.compile(r"\$(?:[A-Za-z_][A-Za-z_0-9]*)?\$")
STRING_PREFIX = re.compile(r"(?i)(?:[ebxn]|u&)?'")
PARAM = re.compile(r"\$[0-9]+")


@functools.lru_cache(maxsize=256)
def canonical_sql(sql):
    """Lex standard_conforming_strings=on SQL; reject rather than emit unknown syntax.

    Strip comments and all literal values, including E strings/dollar bodies.
    Quoted identifiers are also hidden. Never include input text in errors.
    """
    tokens = []
    i = 0
    while i < len(sql):
        if sql[i].isspace():
            i += 1
            continue
        if sql.startswith("--", i):
            end = sql.find("\n", i + 2)
            i = len(sql) if end < 0 else end + 1
            continue
        if sql.startswith("/*", i):
            depth = 1
            i += 2
            while i < len(sql) and depth:
                if sql.startswith("/*", i):
                    depth += 1
                    i += 2
                elif sql.startswith("*/", i):
                    depth -= 1
                    i += 2
                else:
                    i += 1
            if depth:
                raise ValueError("unclosed_comment")
            continue
        prefix = STRING_PREFIX.match(sql, i)
        if prefix:
            escaped = prefix.group().lower() == "e'"
            i += len(prefix.group())
            while i < len(sql):
                if escaped and sql[i] == "\\":
                    i += 2
                elif sql.startswith("''", i):
                    i += 2
                elif sql[i] == "'":
                    i += 1
                    break
                else:
                    i += 1
            else:
                raise ValueError("unclosed_string")
            tokens.append("?")
            continue
        if sql[i] == '"' or sql[i:i + 3].lower() == 'u&"':
            i += 1 if sql[i] == '"' else 3
            while i < len(sql):
                if sql.startswith('""', i):
                    i += 2
                elif sql[i] == '"':
                    i += 1
                    break
                else:
                    i += 1
            else:
                raise ValueError("unclosed_identifier")
            tokens.append('"identifier"')
            continue
        dollar = DOLLAR.match(sql, i)
        if dollar:
            end = sql.find(dollar.group(), dollar.end())
            if end < 0:
                raise ValueError("unclosed_dollar_string")
            i = end + len(dollar.group())
            tokens.append("?")
            continue
        param = PARAM.match(sql, i)
        if param:
            i += len(param.group())
            tokens.append("?")
            continue
        number = NUMBER.match(sql, i)
        if number:
            tokens.append("?")
            i = number.end()
            continue
        word = WORD.match(sql, i)
        if word:
            tokens.append(word.group().lower())
            i = word.end()
            continue
        if sql[i] in "(),.;:+-*/%=<>!~^|&?#[]":
            tokens.append(sql[i])
            i += 1
            continue
        raise ValueError("unsupported_sql_character")
    if not tokens and sql.strip():
        # pgx validates reused connections with a comment-only ping. Its text
        # must not be exported, but it is a known no-op, not a missing query.
        return "<comment-only statement>"
    result = " ".join(tokens).rstrip(" ;")
    # Variable-length bind lists are one query shape, as in pgBadger.
    result = re.sub(r"\bin \( \?(?: , \?)+ \)", "in ( ? )", result)
    if not result or result in ("select", "with", "insert", "update", "delete"):
        raise ValueError("empty_or_incomplete_sql")
    # SQL-looking identifiers alone are not sufficient to classify a record.
    if tokens[0] not in {"select", "with", "insert", "update", "delete", "begin", "commit", "rollback", "start", "end", "savepoint", "release", "set", "reset", "show", "lock", "create", "alter", "drop", "truncate", "do", "copy", "call", "explain", "analyze", "vacuum", "grant", "revoke", "listen", "unlisten", "notify", "prepare", "execute", "deallocate", "discard", "declare", "fetch", "close", "move", "values", "table"}:
        raise ValueError("unsupported_sql_command")
    return result


def records(lines):
    current = None
    for line in lines:
        header = HEADER.match(line)
        if header:
            if current:
                yield current
            message = MESSAGE.match(header[3].rstrip("\n"))
            current = {"stamp": header[1], "timezone": header[2], "level": message[1] if message else "", "body": message[2] if message else "", "continuations": []}
        elif line.startswith("\t") and current:
            current["continuations"].append(line[1:])
        elif line.strip():
            # Do not attach unrecognized records to another PID's query.
            if current:
                yield current
                current = None
            yield {"unrecognized_line": True}
    if current:
        yield current


def summarize(lines, begin=None, end=None, top=50):
    counts = collections.Counter()
    milliseconds = collections.defaultdict(Decimal)
    unknown = collections.Counter()
    unknown_ms = collections.defaultdict(Decimal)
    queries = {}
    buckets = {}
    first = last = None
    recovered = first_line_blank = 0
    unrecognized = outside = 0
    for record in records(lines):
        if record.get("unrecognized_line"):
            unrecognized += 1
            continue
        if record["level"] != "LOG":
            continue
        match = DURATION.match(record["body"])
        if not match:
            continue
        if record["timezone"] not in {"UTC", "GMT", "Etc/UTC", "+00", "+0000", "+00:00"}:
            raise ValueError("only UTC PostgreSQL stderr logs are supported")
        # Inclusive seconds, matching pgBadger --begin/--end filtering.
        second = record["stamp"][:19]
        if (begin and second < begin) or (end and second > end):
            outside += 1
            continue
        duration = Decimal(match[1])
        tail = match[2] or ""
        sql_match = SQL_MESSAGE.match(tail)
        kind = "duration_only"
        sql = ""
        if sql_match:
            kind = sql_match[1] or sql_match[3]
            sql = sql_match[2] if sql_match[1] else sql_match[4]
        counts[kind] += 1
        milliseconds[kind] += duration
        first = min(first or second, second)
        last = max(last or second, second)
        bucket = buckets.setdefault(second[:18] + "0", {"count": 0, "total_ms": Decimal(0)})
        bucket["count"] += 1
        bucket["total_ms"] += duration
        if sql_match and not sql.strip():
            first_line_blank += 1
        if sql_match:
            continued = "".join(record["continuations"])
            if not sql.strip() and continued.strip():
                recovered += 1
            sql += "\n" + continued
            if re.match(r"(?is)^\s*set\s+(?:(?:local|session)\s+)?standard_conforming_strings\b", sql):
                raise ValueError("session string-mode changes are unsupported")
        try:
            if not sql_match:
                raise ValueError("duration_without_sql")
            fingerprint = canonical_sql(sql)
        except ValueError as exc:
            unknown[str(exc)] += 1
            unknown_ms[str(exc)] += duration
            continue
        if re.search(r'(?:^| ; )set (?:local |session )?(?:standard_conforming_strings|"identifier")(?= |$)', fingerprint):
            raise ValueError("session string-mode changes are unsupported")
        row = queries.setdefault(fingerprint, {"count": 0, "total_ms": Decimal(0), "max_ms": Decimal(0), "kinds": collections.Counter(), "kind_ms": collections.defaultdict(Decimal)})
        row["count"] += 1
        row["total_ms"] += duration
        row["max_ms"] = max(row["max_ms"], duration)
        row["kinds"][kind] += 1
        row["kind_ms"][kind] += duration
    total_count = sum(counts.values())
    total_ms = sum(milliseconds.values(), Decimal(0))
    missing_ms = sum(unknown_ms.values(), Decimal(0))
    ordered = sorted(queries.items(), key=lambda item: (-item[1]["total_ms"], item[0]))
    rows = []
    for fingerprint, row in ordered[:top]:
        rows.append({"fingerprint": hashlib.sha256(fingerprint.encode()).hexdigest(), "sql": fingerprint, **row,
                     "avg_ms": row["total_ms"] / row["count"]})
    return {
        "format": "pgfingerprint-v1", "timezone": "UTC", "begin_inclusive": begin, "end_inclusive": end,
        "first_duration": first, "last_duration": last, "outside_interval": outside,
        "total_count": total_count, "total_ms": total_ms,
        "by_kind": {kind: {"count": count, "total_ms": milliseconds[kind]} for kind, count in sorted(counts.items())},
        "first_sql_line_blank": first_line_blank, "recovered_from_continuations": recovered,
        "unattributed_count": sum(unknown.values()), "unattributed_ms": missing_ms,
        "unattributed_percent": missing_ms * 100 / total_ms if total_ms else Decimal(0),
        "unattributed_reasons": {reason: {"count": count, "total_ms": unknown_ms[reason]} for reason, count in sorted(unknown.items())},
        "unrecognized_lines": unrecognized, "unique_fingerprints": len(queries),
        "fingerprint_count": sum(row["count"] for row in queries.values()),
        "fingerprint_total_ms": sum((row["total_ms"] for row in queries.values()), Decimal(0)),
        "top_queries": rows, "buckets_10s": dict(sorted(buckets.items())),
        "complete": bool(total_count) and not unknown and not unrecognized,
    }


def timestamp(value):
    try:
        return dt.datetime.strptime(value.removesuffix("Z").replace("T", " "), "%Y-%m-%d %H:%M:%S").strftime("%Y-%m-%d %H:%M:%S")
    except ValueError:
        raise argparse.ArgumentTypeError("use YYYY-MM-DD HH:MM:SS in UTC") from None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="existing PostgreSQL UTC stderr log (.gz also supported)")
    parser.add_argument("--begin", type=timestamp)
    parser.add_argument("--end", type=timestamp)
    parser.add_argument("--top", type=int, default=50)
    args = parser.parse_args()
    if args.begin and args.end and args.begin > args.end:
        parser.error("begin must not follow end")
    if args.top < 1:
        parser.error("top must be positive")
    opener = gzip.open if args.input.endswith(".gz") else open
    try:
        with opener(args.input, "rt", encoding="utf-8", errors="strict") as source:
            result = summarize(source, args.begin, args.end, args.top)
    except (OSError, UnicodeError, ValueError):
        # Exception strings from parsers/I/O may contain input data; never echo them.
        print("pgfingerprint: input could not be read as supported UTC stderr records", file=sys.stderr)
        return 2
    json.dump(result, sys.stdout, ensure_ascii=True, indent=2, default=lambda value: float(value) if isinstance(value, Decimal) else value)
    print()
    if not result["complete"]:
        print("pgfingerprint: incomplete attribution; inspect counts/reasons before using this report", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
