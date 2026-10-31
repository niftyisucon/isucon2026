# shellcheck shell=bash
cmd_splunk-token() {
  parse_force "$@"
  [ "${#ARGS[@]}" -eq 1 ] || die "usage: isu splunk-token <host> [-f]"
  local h=${ARGS[0]}
  is_host "$h" || die "isu.conf に無いホスト: $h"
  guard_host "$h" "$FORCE" || return
  lock_acquire "$h" splunk-token || return
  local result=0
  ssh "${SSH_OPTS[@]}" -t "$h" 'sudo bash /opt/isucon-observability/lib/isu/remote/splunk-token.sh' || result=$?
  lock_release "$h" || return
  return "$result"
}
