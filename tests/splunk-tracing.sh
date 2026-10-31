#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
scratch=$(mktemp -d /tmp/isu-splunk-cli-test.XXXXXX)
trap 'rm -rf "$scratch"' EXIT
mkdir "$scratch/bin"
for tool in go ssh rsync; do
  cat > "$scratch/bin/$tool" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "${0##*/}" >> "$SPLUNK_TEST_CALLS"
exit 99
MOCK
  chmod 755 "$scratch/bin/$tool"
done
export PATH="$scratch/bin:$PATH"
export SPLUNK_TEST_CALLS="$scratch/calls"
export ISU_RSYNC="$scratch/bin/rsync"
export ISU_SSH_CONFIG=/dev/null
cd "$root"
for mode in enable disable; do
  : > "$SPLUNK_TEST_CALLS"
  if ./bin/isu splunk-tracing "$mode" isu1 -f > "$scratch/output" 2>&1; then
    echo "FAIL: retired splunk-tracing $mode must fail" >&2
    exit 1
  fi
  if [ -s "$SPLUNK_TEST_CALLS" ]; then
    printf 'FAIL: splunk-tracing %s attempted external operations: ' "$mode" >&2
    tr '\n' ' ' < "$SPLUNK_TEST_CALLS" >&2
    printf '\n' >&2
    exit 1
  fi
done
echo 'PASS: retired tracing commands cannot build, connect, copy, or restart'
