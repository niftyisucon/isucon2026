# shellcheck shell=bash
# shellcheck source=build.sh
source "$ISU_LIB/cmd/build.sh"
# shellcheck source=restart.sh
source "$ISU_LIB/cmd/restart.sh"

# ビルド → 管理対象ファイルと webapp を rsync → 変わったものだけ再起動。
cmd_deploy() {
  parse_force "$@"
  local hosts h
  hosts=$(resolve_hosts ${ARGS[@]+"${ARGS[@]}"})
  for h in $hosts; do guard_host "$h" "$FORCE"; done
  cmd_build
  for h in $hosts; do
    lock_acquire "$h" deploy
    deploy_host "$h" || { lock_release "$h"; die "deploy $h 失敗"; }
    lock_release "$h"
  done
}

deploy_host() {
  local h=$1 p ex=() changed out restart=()
  local c_nginx=0 c_db=0 c_systemd=0 c_app=0
  for p in "${ISU_SYNC_EXCLUDES[@]}"; do ex+=(--exclude "$p"); done
  for p in "${ISU_HOST_FILES[@]}"; do
    [ -e "$ISU_ROOT/hosts/$h$p" ] || continue
    local own=()
    case "$p" in /home/*/*) u=${p#/home/}; u=${u%%/*}; own=(--chown "$u:$u") ;; esac
    out=$(isu_rsync -R -i --delete ${own[@]+"${own[@]}"} "${ex[@]}" "$ISU_ROOT/hosts/$h/.$p" "$h:/")
    changed=$(echo "$out" | grep -E '^([<>ch]|\*deleting)' || true)
    [ -n "$changed" ] || continue
    echo "$changed" | sed "s|^|  $h: |" >&2
    case "$p" in
      */nginx*) c_nginx=1 ;;
      */mysql*|*/mariadb*) c_db=1 ;;
      */systemd/*) c_systemd=1; c_app=1 ;;
      *) c_app=1 ;;
    esac
  done
  if has_role "$h" app; then
    local wex=()
    for p in "${ISU_WEBAPP_EXCLUDES[@]}"; do wex+=(--exclude "$p"); done
    out=$(isu_rsync -i --delete --chown "$ISU_APP_USER:$ISU_APP_USER" "${wex[@]}" "$ISU_ROOT/webapp/" "$h:$ISU_WEBAPP_REMOTE/")
    changed=$(echo "$out" | grep -E '^([<>ch]|\*deleting)' || true)
    if [ -n "$changed" ]; then
      echo "$changed" | head -20 | sed "s|^|  $h: |" >&2
      c_app=1
    fi
  fi
  [ "$c_nginx" = 1 ] && restart+=(nginx)
  [ "$c_db" = 1 ] && restart+=(db)
  [ "$c_systemd" = 1 ] && restart+=(systemd)
  [ "$c_app" = 1 ] && restart+=(app)
  if [ "${#restart[@]}" -eq 0 ]; then
    log "deploy $h: 変更なし"
    return 0
  fi
  restart_services "$h" "${restart[@]}"
  rssh "$h" "sleep 1; systemctl is-active $ISU_APP_SERVICES ${ISU_DB_SERVICE} nginx 2>/dev/null | paste -sd' ' -" | sed "s|^|  $h active: |" >&2 || true
  log "deploy $h: ok (${restart[*]})"
}
