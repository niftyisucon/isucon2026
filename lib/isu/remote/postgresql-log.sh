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
  *) echo "unknown mode: $mode" >&2; exit 1 ;;
esac
