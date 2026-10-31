# shellcheck shell=bash
# shellcheck source=build.sh
source "$ISU_LIB/cmd/build.sh"

cmd_splunk-tracing() {
  parse_force "$@"
  [ "${#ARGS[@]}" -eq 2 ] || die "usage: isu splunk-tracing <enable|disable> <host> [-f]"
  local mode=${ARGS[0]} h=${ARGS[1]} result=0
  case "$mode" in enable|disable) ;; *) die "mode は enable または disable" ;; esac
  is_host "$h" || die "isu.conf に無いホスト: $h"
  has_role "$h" app || die "$h は app ホストではありません"
  guard_host "$h" "$FORCE" || return
  if [ "$mode" = enable ]; then cmd_build || return; fi
  lock_acquire "$h" splunk-tracing || return
  splunk_tracing_host "$mode" "$h" || result=$?
  lock_release "$h" || return
  return "$result"
}

splunk_tracing_host() {
  local mode=$1 h=$2
  local dropin=/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
  if [ "$mode" = enable ]; then
    rssh "$h" 'curl --max-time 3 -fsS http://127.0.0.1:13133/ >/dev/null' || return
    rssh "$h" 'sudo install -d -m 755 /etc/systemd/system/isureal-go.service.d' || return
    isu_rsync --chown "$ISU_APP_USER:$ISU_APP_USER" "$ISU_ROOT/webapp/go/isureal" "$h:$ISU_WEBAPP_REMOTE/go/isureal" || return
    isu_rsync --chown root:root "$ISU_ROOT/config/splunk/tracing.conf" "$h:$dropin" || return
  else
    rssh "$h" "sudo rm -f $dropin" || return
  fi
  rssh "$h" 'sudo systemctl daemon-reload && sudo systemctl restart isureal-go && systemctl is-active isureal-go'
}
