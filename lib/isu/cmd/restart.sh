# shellcheck shell=bash
cmd_restart() {
  parse_force "$@"
  local h hosts
  hosts=$(resolve_hosts ${ARGS[@]+"${ARGS[@]}"}) || return
  for h in $hosts; do
    guard_host "$h" "$FORCE" || return
    restart_services "$h" nginx db app || return
  done
}

# restart_services host [nginx] [db] [systemd] [app]  役割を持たないものは飛ばす
restart_services() {
  local h=$1 what cmds=""
  shift
  for what in "$@"; do
    case "$what" in
      nginx) has_role "$h" nginx && cmds="$cmds sudo nginx -t -q && sudo systemctl reload nginx &&" ;;
      db) has_role "$h" db && cmds="$cmds sudo systemctl restart $ISU_DB_SERVICE &&" ;;
      systemd) cmds="$cmds sudo systemctl daemon-reload &&" ;;
      app)
        local extra
        extra=$(host_attr "$h" app_services)
        # ホスト固有の追加サービスは、追試の再起動後も上がるよう enable してから restart する
        [ -n "$extra" ] && cmds="$cmds sudo systemctl enable -q $extra &&"
        has_role "$h" app && cmds="$cmds sudo systemctl restart $ISU_APP_SERVICES $extra &&" ;;
    esac
  done
  [ -n "$cmds" ] || return 0
  log "restart $h: $*"
  rssh "$h" "$cmds true"
}
