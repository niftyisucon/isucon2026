# shellcheck shell=bash
# サーバーの管理対象ファイルをリポジトリへ取り込む。
# /etc などは hosts/<host>/ 以下に絶対パスのまま、webapp は最初の app ホストから webapp/ へ。
cmd_pull() {
  local h p hosts status ex=() webapp_done=0
  hosts=$(resolve_hosts "$@") || return
  for p in "${ISU_SYNC_EXCLUDES[@]}"; do ex+=(--exclude "$p"); done
  for h in $hosts; do
    mkdir -p "$ISU_ROOT/hosts/$h"
    for p in "${ISU_HOST_FILES[@]}"; do
      if rssh "$h" "sudo test -e '$p'"; then
        :
      else
        status=$?
        if [ "$status" = 1 ]; then
          warn "$h:$p がありません（スキップ）"
          continue
        fi
        warn "$h:$p の確認に失敗（終了コード ${status}）"
        return "$status"
      fi
      log "pull $h:$p"
      isu_rsync -R --delete "${ex[@]}" "$h:/.$p" "$ISU_ROOT/hosts/$h/" || return
    done
    if [ "$webapp_done" = 0 ] && has_role "$h" app; then
      local wex=()
      for p in "${ISU_WEBAPP_EXCLUDES[@]}" "${ISU_WEBAPP_PULL_EXCLUDES[@]}"; do wex+=(--exclude "$p"); done
      log "pull $h:$ISU_WEBAPP_REMOTE -> webapp/"
      # .gitkeep はリポジトリ側のものなので --delete で消さない
      isu_rsync --delete --exclude=/.gitkeep "${wex[@]}" "$h:$ISU_WEBAPP_REMOTE/" "$ISU_ROOT/webapp/" || return
      webapp_done=1
    fi
  done
  if git -C "$ISU_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$ISU_ROOT" status --short -- hosts webapp | head -20
  else
    log "pull 完了（Git 未初期化のため変更一覧の表示をスキップ）"
  fi
}
