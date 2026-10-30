# shellcheck shell=bash
cmd_lock() {
  [ "$#" -ge 1 ] || die "usage: isu lock <host> [purpose]"
  is_host "$1" || die "isu.conf に無いホスト: $1"
  lock_acquire "$1" "${2:-manual}"
}
