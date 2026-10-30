# shellcheck shell=bash
cmd_discover() {
  local h out
  for h in $(resolve_hosts "$@"); do
    mkdir -p "$ISU_ROOT/hosts/$h"
    out="$ISU_ROOT/hosts/$h/discovery.md"
    log "discover $h -> ${out#$ISU_ROOT/}"
    rscript "$h" lib/isu/remote/discover.sh > "$out"
  done
}
