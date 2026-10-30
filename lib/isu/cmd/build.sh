# shellcheck shell=bash
cmd_build() {
  [ -n "${ISU_BUILD_CMD:-}" ] || { log "ISU_BUILD_CMD 未設定（ビルド不要）"; return 0; }
  log "build: $ISU_BUILD_CMD"
  (cd "$ISU_ROOT" && bash -c "$ISU_BUILD_CMD")
}
