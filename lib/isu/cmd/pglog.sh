# shellcheck shell=bash
# ログはローカルへ取得し、解析もローカルのPerlで行う。
cmd_pglog() {
  local host="" begin="" end="" input="" output="" h bin prefix timezone threshold destination
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --begin|--end|--input-dir|--output-dir)
        [ "$#" -ge 2 ] || die "$1 の値が必要です"
        case "$1" in
          --begin) begin=$2 ;; --end) end=$2 ;;
          --input-dir) input=$2 ;; --output-dir) output=$2 ;;
        esac
        shift 2 ;;
      -*) die "unknown option: $1" ;;
      *) [ -z "$host" ] || die "DBホストは1台だけ指定してください"; host=$1; shift ;;
    esac
  done
  if [ -z "$host" ]; then
    for h in "${ISU_HOSTS[@]}"; do
      if has_role "$h" db; then host=$h; break; fi
    done
  fi
  is_host "$host" && has_role "$host" db || die "DB役割のホストを指定してください"
  command -v perl >/dev/null || die "ローカルに Perl が必要です"
  command -v gzip >/dev/null || die "ローカルに gzip が必要です"
  bin=${ISU_PGBADGER_BIN:-$ISU_LOCAL_DIR/tools/pgbadger/pgbadger}
  [ -f "$bin" ] || die "pgBadger がありません。./bin/isu pgbadger-setup を実行してください"
  if [ -n "$input" ]; then
    input=$(cd "$input" && pwd) || return
    [ -s "$input/postgresql.log.gz" ] && [ -s "$input/log-settings.txt" ] || die "取得済みログと設定がありません: $input"
  fi
  umask 077
  if [ -z "$output" ]; then
    mkdir -p "$ISU_LOCAL_DIR/pgbadger" || return
    output=$(mktemp -d "$ISU_LOCAL_DIR/pgbadger/$host-$(date -u +%Y%m%dT%H%M%SZ).XXXXXX") || return
  else
    mkdir -p "$output" || return
  fi
  output=$(cd "$output" && pwd) || return
  [ ! -e "$output/report.html" ] && [ ! -e "$output/report.txt" ] || die "レポートが既にあります。別の出力先を指定してください"
  if [ -z "$input" ]; then
    [ ! -e "$output/postgresql.log.gz" ] && [ ! -e "$output/log-settings.txt" ] || die "取得済みデータがあります。--input-dir で再利用するか別の出力先を指定してください"
    log "PostgreSQLログ取得: $host:$ISU_PG_LOG_PATH -> $output"
    rscript "$host" lib/isu/remote/postgresql-log.sh info "$ISU_PG_LOG_PATH" "$ISU_DB_NAME" > "$output/log-settings.txt" || return
    if rscript "$host" lib/isu/remote/postgresql-log.sh snapshot "$ISU_PG_LOG_PATH" "$ISU_DB_NAME" > "$output/postgresql.log.gz.tmp"; then
      mv "$output/postgresql.log.gz.tmp" "$output/postgresql.log.gz" || return
    else
      rm -f "$output/postgresql.log.gz.tmp"
      return 1
    fi
    input=$output
    printf 'host=%s\nlog=%s\ncaptured_at_utc=%s\n' "$host" "$ISU_PG_LOG_PATH" "$(date -u +%FT%TZ)" > "$output/source.txt"
  fi
  {
    IFS= read -r prefix
    IFS= read -r timezone
    IFS= read -r threshold
    IFS= read -r destination
  } < "$input/log-settings.txt" || die "取得したログ設定が不完全です"
  [ "$destination" = stderr ] || die "stderr形式専用です（取得設定: ${destination}）"
  [ "$threshold" != -1 ] || warn "log_min_duration_statement=-1: durationログが無効です"
  local args=()
  [ -z "$begin" ] || args+=(--begin "$begin")
  [ -z "$end" ] || args+=(--end "$end")
  log "pgBadger 集計（ログの時刻は ${timezone}、duration閾値 ${threshold}ms）"
  # -o はファイル名、-O は絶対ディレクトリ。pgBadgerの複数形式出力に対応。
  perl "$bin" --quiet --format stderr --prefix "$prefix" --top 50 \
    --pid-dir "$output" --tempdir "$output" \
    --outdir "$output" --outfile report.html --outfile report.txt \
    ${args[@]+"${args[@]}"} "$input/postgresql.log.gz" || return
  [ -s "$output/report.html" ] && [ -s "$output/report.txt" ] || die "レポートの生成に失敗しました"
  grep -E '^(Log start from|Number of unique normalized queries:|Number of queries:|Total query duration:)' "$output/report.txt" || true
  printf 'HTML: %s\nTEXT: %s\nINPUT: %s\n' "$output/report.html" "$output/report.txt" "$input"
  grep -Eq '^Number of queries: [1-9]' "$output/report.txt" || {
    warn "解析対象のクエリが0件です。時刻範囲・ログ設定を確認してください"
    return 1
  }
}
