#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077
[ -t 0 ] || { echo 'Run splunk-token from an interactive terminal.' >&2; exit 1; }
[ -f /etc/systemd/system/splunk-otel-collector.service.d/20-isucon.conf ] || {
  echo 'Run splunk-setup first.' >&2
  exit 1
}
printf 'Splunk au0 Ingest token (hidden): ' >/dev/tty
IFS= read -r -s splunk_token </dev/tty
printf '\n' >/dev/tty
case "$splunk_token" in
  ''|*[!A-Za-z0-9._=-]*) echo 'Token is empty or contains unexpected characters.' >&2; exit 1 ;;
esac
token_file=$(mktemp /etc/otel/collector/.splunk-token.XXXXXX)
trap 'rm -f "$token_file"; unset splunk_token' EXIT
printf 'SPLUNK_ACCESS_TOKEN=%s\n' "$splunk_token" > "$token_file"
chown root:root "$token_file"
chmod 600 "$token_file"
mv -f "$token_file" /etc/otel/collector/splunk-otel-collector.conf
unset splunk_token
systemctl enable splunk-otel-collector
systemctl restart splunk-otel-collector
systemctl is-active splunk-otel-collector
