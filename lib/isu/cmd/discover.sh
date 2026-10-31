# shellcheck shell=bash
cmd_discover() {
  local h out tmp hosts status
  hosts=$(resolve_hosts "$@") || return
  for h in $hosts; do
    mkdir -p "$ISU_ROOT/hosts/$h"
    out="$ISU_ROOT/hosts/$h/discovery.md"
    log "discover $h -> ${out#$ISU_ROOT/}"
    tmp=$(mktemp "$out.XXXXXX") || return
    if rscript "$h" lib/isu/remote/discover.sh "${ISU_DB_NAME:-isureal}" > "$tmp"; then
      mv "$tmp" "$out" || return
    else
      status=$?
      rm -f "$tmp"
      return "$status"
    fi
    if has_role "$h" db; then
      case "$ISU_DB_SERVICE" in
        postgresql*)
          out="$ISU_ROOT/hosts/$h/postgresql-schema.sql"
          tmp=$(mktemp "$out.XXXXXX") || return
          if rscript "$h" lib/isu/remote/postgresql-schema.sh "${ISU_DB_NAME:-isureal}" > "$tmp"; then
            mv "$tmp" "$out" || return
          else
            status=$?
            rm -f "$tmp"
            return "$status"
          fi
          ;;
      esac
    fi
  done
}
