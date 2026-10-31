#!/usr/bin/env bash
set -euo pipefail
version=$1
checksum=$2
roles=$3
case "$version" in *[!0-9.]*|'') echo 'invalid Collector version' >&2; exit 1 ;; esac
case "$checksum" in *[!a-f0-9]*|'') echo 'invalid Collector checksum' >&2; exit 1 ;; esac
[ "${#checksum}" -eq 64 ] || { echo 'invalid Collector checksum length' >&2; exit 1; }
[ "$(dpkg --print-architecture)" = amd64 ] || { echo 'amd64 only' >&2; exit 1; }

installed=$(dpkg-query -W -f='${Version}' splunk-otel-collector 2>/dev/null || true)
if [ -n "$installed" ] && [ "$installed" != "$version" ]; then
  echo "Collector $installed is installed; review an upgrade to $version separately" >&2
  exit 1
fi

if [ -z "$installed" ]; then
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT
  package="splunk-otel-collector_${version}_amd64.deb"
  release="https://github.com/signalfx/splunk-otel-collector/releases/download/v${version}"
  curl -fsSL "$release/$package" -o "$work/$package"
  printf '%s  %s\n' "$checksum" "$package" > "$work/package.sha256"
  (cd "$work" && sha256sum -c package.sha256)
  export DEBIAN_FRONTEND=noninteractive NEEDRESTART_SUSPEND=1 NEEDRESTART_MODE=l
  apt-get install -y "$work/$package"
  systemctl disable splunk-otel-collector
fi

source_dir=/opt/isucon-observability/config/splunk
install -d -m 755 /etc/otel/collector /etc/systemd/system/splunk-otel-collector.service.d
chown root:root /etc/otel/collector
install -m 644 "$source_dir/collector.yaml" /etc/otel/collector/isucon.yaml
args=(--config=/etc/otel/collector/isucon.yaml)
case " $roles " in
  *' db '*)
    install -m 644 "$source_dir/postgresql.yaml" /etc/otel/collector/postgresql.yaml
    args+=(--config=/etc/otel/collector/postgresql.yaml)
    ;;
esac

SPLUNK_ACCESS_TOKEN=configuration-validation /usr/bin/otelcol validate "${args[@]}"
{
  printf '[Service]\nExecStart=\nExecStart=/usr/bin/otelcol'
  printf ' %s' "${args[@]}"
  printf '\nEnvironment=SPLUNK_MEMORY_TOTAL_MIB=256\nMemoryMax=384M\nCPUAccounting=yes\nMemoryAccounting=yes\nCPUWeight=10\nNice=10\n'
} > /etc/systemd/system/splunk-otel-collector.service.d/20-isucon.conf
if [ ! -f /etc/otel/collector/splunk-otel-collector.conf ]; then
  install -m 600 /dev/null /etc/otel/collector/splunk-otel-collector.conf
fi
systemctl daemon-reload
echo 'Collector configuration validated. Run splunk-token to enter the token and start it.'
