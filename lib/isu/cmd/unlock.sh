# shellcheck shell=bash
cmd_unlock() {
  [ "$#" -ge 1 ] || die "usage: isu unlock <host>"
  is_host "$1" || die "isu.conf に無いホスト: $1"
  lock_release "$1"
  log "unlock $1"
}
