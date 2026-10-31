#!/usr/bin/env bash
# 読み取り専用。info は設定、snapshot は取得開始時のログをgzipで標準出力へ。
set -euo pipefail
mode=$1
path=$2
db=$3
case "$mode" in
  info)
    sudo -u postgres psql -XAt -d "$db" \
      -c 'SHOW log_line_prefix' -c 'SHOW log_timezone' \
      -c 'SHOW log_min_duration_statement' -c 'SHOW log_destination'
    ;;
  snapshot)
    # ファイル記述子を固定し、取得中のrenameローテーションでも同じログを読む。
    exec 3< "$path"
    size=$(stat -Lc %s /proc/self/fd/3)
    head -c "$size" <&3 | gzip -1
    [ "$(stat -Lc %s /proc/self/fd/3)" -ge "$size" ] || {
      echo "取得中にログがtruncateされました。再取得してください" >&2
      exit 1
    }
    ;;
  tail)
    # Python はこのオプションだけで必要。stdout はgzip、stderrは値を含まない採取情報。
    python3 - "$path" "${4:-}" "${5:-}" "${6:-}" <<'PY'
import datetime
import gzip
import os
import re
import sys

def capture():
    path, mib, begin, end = sys.argv[1:]
    if not re.fullmatch(r"[1-9][0-9]{0,3}", mib) or int(mib) > 4096:
        raise ValueError("tail-mib must be between 1 and 4096")
    # pgfingerprint と同じUTC stderr prefix。SQL本文・bind値はstderrへ出さない。
    header = re.compile(rb"^(\d{4}-\d\d-\d\d \d\d:\d\d:\d\d(?:\.\d+)?) UTC \[\d+\] ")
    def timestamp(value):
        return datetime.datetime.fromisoformat(value)
    begin_at = timestamp(begin) if begin else None
    end_after = timestamp(end) + datetime.timedelta(seconds=1) if end else None
    first = last = None
    count = 0
    record = []
    record_bytes = 0
    record_time = None
    max_record = 16 * 1024 * 1024
    with open(path, "rb") as source, gzip.GzipFile(fileobj=sys.stdout.buffer, mode="wb", compresslevel=1, mtime=0) as output:
        size = os.fstat(source.fileno()).st_size
        start = max(0, size - int(mib) * 1024 * 1024)
        source.seek(start)
        remaining = size - start
        if start:
            # 開始位置が行境界でも最初の行を捨て、次の完全なheaderから開始する。
            skipped = source.readline(min(remaining, max_record + 1))
            remaining -= len(skipped)
            if not skipped.endswith(b"\n"):
                raise ValueError("leading line exceeds record limit or snapshot ends mid-line")
        while remaining:
            line = source.readline(min(remaining, max_record + 1))
            if not line:
                raise ValueError("log was truncated during capture")
            remaining -= len(line)
            match = header.match(line)
            if match:
                if record:
                    for part in record:
                        output.write(part)
                    first = first or record_time
                    last = record_time
                    count += 1
                record = []
                record_bytes = 0
                record_time = match.group(1).decode("ascii")
            elif not record:
                # 開始位置がSQL継続行やDETAILの途中なら次のheaderまで捨てる。
                continue
            record.append(line)
            record_bytes += len(line)
            if record_bytes > max_record:
                raise ValueError("log record exceeds 16 MiB safety limit")
        # EOFのrecordは、次のheaderがなく完全性を確認できないので常に捨てる。
        if os.fstat(source.fileno()).st_size < size:
            raise ValueError("log was truncated during capture")
        print(f"snapshot_bytes={size}\nstart_byte={start}\nrecords={count}\nfirst_record_utc={first}\nlast_record_utc={last}\nlast_record_omitted=true", file=sys.stderr)
        if not count:
            raise ValueError("no complete UTC stderr records in snapshot")
        if begin_at and timestamp(first) > begin_at:
            raise ValueError("snapshot does not cover begin; increase --tail-mib or read rotated logs")
        if end_after and timestamp(last) < end_after:
            raise ValueError("snapshot does not cover the full end second; capture after end or read rotated logs")

try:
    capture()
except (OSError, ValueError) as error:
    # OSError本文にはpathなどが含まれ得るため詳細を出さない。
    print("snapshot failed: " + (str(error) if isinstance(error, ValueError) else type(error).__name__), file=sys.stderr)
    sys.exit(1)
PY
    ;;
  *) echo "unknown mode: $mode" >&2; exit 1 ;;
esac
