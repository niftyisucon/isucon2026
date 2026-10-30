# shellcheck shell=bash
cmd_ssh() {
  [ "$#" -ge 1 ] || die "usage: isu ssh <host> [cmd...]"
  local host=$1
  shift
  is_host "$host" || die "isu.conf に無いホスト: $host"
  if [ "$#" -eq 0 ]; then
    exec ssh -F "$SSH_CONFIG" -t "$host"
  fi
  rssh "$host" "$@"
}
