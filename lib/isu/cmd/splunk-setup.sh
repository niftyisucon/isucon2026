# shellcheck shell=bash
cmd_splunk-setup() {
  parse_force "$@"
  local hosts h
  hosts=$(resolve_hosts ${ARGS[@]+"${ARGS[@]}"}) || return
  for h in $hosts; do guard_host "$h" "$FORCE" || return; done
  for h in $hosts; do
    lock_acquire "$h" splunk-setup || return
    splunk_setup_host "$h" || { lock_release "$h"; return 1; }
    lock_release "$h" || return
  done
}

splunk_setup_host() {
  local h=$1
  log "splunk-setup $h (Collector ${ISU_SPLUNK_OTEL_VERSION})"
  rssh "$h" 'sudo install -d -m 755 /opt/isucon-observability' || return
  tar -C "$ISU_ROOT" -cf - config/splunk lib/isu/remote/splunk-token.sh |
    rssh "$h" 'sudo tar -xf - -C /opt/isucon-observability' || return
  rscript "$h" lib/isu/remote/splunk-setup.sh "$ISU_SPLUNK_OTEL_VERSION" "$ISU_SPLUNK_OTEL_SHA256" "$(host_attr "$h" roles)"
}
