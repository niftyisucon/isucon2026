# shellcheck shell=bash
cmd_hosts() {
  local h
  printf '%-8s %-10s %-20s %-16s %s\n' HOST OWNER ROLES PRIVATE_IP LOCK
  for h in "${ISU_HOSTS[@]}"; do
    printf '%-8s %-10s %-20s %-16s %s\n' "$h" "$(host_attr "$h" owner)" \
      "$(host_attr "$h" roles)" "$(host_attr "$h" private_ip)" "$(lock_status "$h")"
  done
  echo "本番計測ホスト: ${ISU_PROD_HOST:-未設定} / 自分: $ISU_ME"
}
