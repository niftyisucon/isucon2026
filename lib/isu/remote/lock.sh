#!/usr/bin/env bash
# リモートで root として実行される。usage: lock.sh acquire <who> <purpose> <ttl> | release <who> | status
set -eu
dir=/run/isu.lock
now=$(date +%s)
case "$1" in
  acquire)
    who=$2 purpose=$3 ttl=$4
    if mkdir "$dir" 2>/dev/null; then
      printf '%s\t%s\t%s\n' "$who" "$now" "$purpose" > "$dir/info"
      echo OK
      exit 0
    fi
    IFS=$'\t' read -r owner since what < "$dir/info" || true
    if [ "${owner:-}" = "$who" ]; then
      printf '%s\t%s\t%s\n' "$who" "$now" "$purpose" > "$dir/info"
      echo "OK reentrant"
    elif [ $((now - ${since:-0})) -gt "$ttl" ]; then
      printf '%s\t%s\t%s\n' "$who" "$now" "$purpose" > "$dir/info"
      echo "OK stolen from $owner"
    else
      echo "BUSY $owner ($what, $((now - since))s)"
    fi
    ;;
  release)
    [ -f "$dir/info" ] || exit 0
    IFS=$'\t' read -r owner _ _ < "$dir/info" || true
    [ "${owner:-}" = "$2" ] && rm -rf "$dir"
    exit 0
    ;;
  status)
    if [ -f "$dir/info" ]; then
      IFS=$'\t' read -r owner since what < "$dir/info"
      echo "$owner ($what, $((now - since))s)"
    else
      echo "-"
    fi
    ;;
esac
