# shellcheck shell=bash
cmd_pgbadger-setup() {
  [ "$#" = 0 ] || die "usage: isu pgbadger-setup"
  command -v perl >/dev/null || die "ローカルに Perl が必要です"
  command -v curl >/dev/null || die "ローカルに curl が必要です"
  local dir tmp digest
  dir="$ISU_LOCAL_DIR/tools/pgbadger"
  mkdir -p "$dir" || return
  tmp=$(mktemp "$dir/download.XXXXXX") || return
  if ! curl -fSL --retry 2 --connect-timeout 10 --max-time 120 \
    "https://raw.githubusercontent.com/darold/pgbadger/$ISU_PGBADGER_COMMIT/pgbadger" -o "$tmp"; then
    rm -f "$tmp"
    return 1
  fi
  if command -v sha256sum >/dev/null; then
    digest=$(sha256sum "$tmp") || return
  else
    digest=$(shasum -a 256 "$tmp") || return
  fi
  if [ "${digest%% *}" != "$ISU_PGBADGER_SHA256" ]; then
    rm -f "$tmp"
    die "pgBadger のSHA256が一致しません（導入中止）"
  fi
  if ! perl -c "$tmp"; then rm -f "$tmp"; return 1; fi
  mv "$tmp" "$dir/pgbadger" || return
  perl "$dir/pgbadger" --version
}
