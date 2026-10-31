# shellcheck shell=bash
# 計測ツールとログ設定を入れる。何度実行してもよい。
cmd_setup() {
  local h hosts
  hosts=$(resolve_hosts "$@") || return
  for h in $hosts; do
    log "setup $h"
    rscript "$h" lib/isu/remote/setup.sh "${ISU_ALP_VERSION:-v1.0.21}" "$(host_attr "$h" roles)" "$ISU_DB_SERVICE" || return
  done
}
