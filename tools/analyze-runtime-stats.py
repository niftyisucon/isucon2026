#!/usr/bin/env python3
"""Summarize same-process /debug/stats JSONL; no network or server operations."""
import argparse
import json
from pathlib import Path


def summarize(rows):
    if len(rows) < 2:
        raise ValueError("at least two snapshots are required")
    first, last = rows[0], rows[-1]
    if len({row["pid"] for row in rows}) != 1:
        raise ValueError("PID changed; split captures at the process restart")
    if any(b["uptime_seconds"] <= a["uptime_seconds"] for a, b in zip(rows, rows[1:])):
        raise ValueError("snapshots must have increasing process uptime")

    def delta(section, name):
        values = [row[section][name] for row in rows]
        if any(b < a for a, b in zip(values, values[1:])):
            raise ValueError(f"counter reset: {section}.{name}")
        return values[-1] - values[0]

    result = {
        "pid": first["pid"],
        "first_time": first["time"],
        "last_time": last["time"],
        "samples": len(rows),
        "elapsed_seconds": last["uptime_seconds"] - first["uptime_seconds"],
        "pool_max_open": sorted({row["db"]["MaxOpenConnections"] for row in rows}),
        "pool_peak_in_use": max(row["db"]["InUse"] for row in rows),
        "pool_peak_open": max(row["db"]["OpenConnections"] for row in rows),
        "pool_wait_count_delta": delta("db", "WaitCount"),
        "pool_wait_seconds_delta": delta("db", "WaitDuration") / 1e9,
        "gc_cpu_seconds_delta": delta("runtime", "/cpu/classes/gc/total:cpu-seconds"),
        "allocated_mib_delta": delta("runtime", "/gc/heap/allocs:bytes") / 1048576,
        "gc_cycles_delta": delta("runtime", "/gc/cycles/total:gc-cycles"),
        "url_cache_hits_delta": delta("image_url_cache", "hits"),
        "url_cache_misses_delta": delta("image_url_cache", "misses"),
        "url_cache_peak_entries": max(row["image_url_cache"]["entries"] for row in rows),
    }
    attempts = result["url_cache_hits_delta"] + result["url_cache_misses_delta"]
    result["url_cache_hit_fraction"] = result["url_cache_hits_delta"] / attempts if attempts else None
    if all("jpeg_validation_cache" in row for row in rows):
        section = "jpeg_validation_cache"
        for name in ("hits", "misses", "bypasses"):
            result[f"jpeg_cache_{name}_delta"] = delta(section, name)
        for name in ("entries", "bytes"):
            result[f"jpeg_cache_peak_{name}"] = max(row[section][name] for row in rows)
        attempts = result["jpeg_cache_hits_delta"] + result["jpeg_cache_misses_delta"] + result["jpeg_cache_bypasses_delta"]
        result["jpeg_cache_hit_fraction"] = result["jpeg_cache_hits_delta"] / attempts if attempts else None
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("jsonl", type=Path)
    args = parser.parse_args()
    try:
        rows = [json.loads(line) for line in args.jsonl.read_text().splitlines() if line.strip()]
        print(json.dumps(summarize(rows), ensure_ascii=False, indent=2))
    except (ValueError, KeyError, TypeError) as error:
        parser.exit(1, f"invalid capture: {error}\n")


if __name__ == "__main__":
    main()
