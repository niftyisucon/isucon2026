# shellcheck shell=bash
# Existing files only: no SSH, SQL execution, config changes, or log deletion.
cmd_pgfingerprint() {
  command -v python3 >/dev/null || die "Python 3.9以上が必要です"
  python3 "$ISU_LIB/pgfingerprint.py" "$@"
}
