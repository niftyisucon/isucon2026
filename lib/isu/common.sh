# shellcheck shell=bash
# bin/isu から source される共通処理。

# .local/ は git 管理外なので worktree には無い。無ければメインの checkout のものを使う。
find_local_dir() {
  local common
  if [ -d "$ISU_ROOT/.local" ]; then
    echo "$ISU_ROOT/.local"
    return
  fi
  common=$(git -C "$ISU_ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)
  if [ -n "$common" ] && [ -d "$(dirname "$common")/.local" ]; then
    echo "$(dirname "$common")/.local"
    return
  fi
  echo "$ISU_ROOT/.local"
}

load_config() {
  [ -f "$ISU_ROOT/isu.conf" ] || die "isu.conf がありません"
  # shellcheck source=/dev/null
  source "$ISU_ROOT/isu.conf"
  ISU_LOCAL_DIR=$(find_local_dir)
  if [ -f "$ISU_LOCAL_DIR/isu.local.conf" ]; then
    # shellcheck source=/dev/null
    source "$ISU_LOCAL_DIR/isu.local.conf"
  fi
  SSH_CONFIG=${ISU_SSH_CONFIG:-$ISU_LOCAL_DIR/ssh_config}
  SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=10 -o ServerAliveInterval=15)
  [ -f "$SSH_CONFIG" ] && SSH_OPTS=(-F "$SSH_CONFIG" "${SSH_OPTS[@]}")
  : "${ISU_ME:=$(whoami)}"
}

die() { echo "isu: $*" >&2; exit 1; }
log() { echo "==> $*" >&2; }
warn() { echo "isu: warning: $*" >&2; }

# ホストの属性を読む: host_attr isu1 roles
host_attr() {
  local var="${1}_${2}"
  echo "${!var:-}"
}

is_host() {
  local h
  for h in "${ISU_HOSTS[@]}"; do [ "$h" = "$1" ] && return 0; done
  return 1
}

has_role() {
  case " $(host_attr "$1" roles) " in *" $2 "*) return 0 ;; esac
  return 1
}

# 引数のホスト一覧を検証して返す。空なら全ホスト。
resolve_hosts() {
  local h
  if [ "$#" -eq 0 ]; then
    printf '%s\n' "${ISU_HOSTS[@]}"
    return
  fi
  for h in "$@"; do
    is_host "$h" || die "isu.conf に無いホスト: $h"
    echo "$h"
  done
}

# ロックや記録に使う識別子。担当者名と現在の短縮コミット番号で区別する。
agent_id() {
  local commit
  commit=$(git -C "$ISU_ROOT" rev-parse --short HEAD 2>/dev/null) || commit=no-commit
  echo "${ISU_ME}:$commit"
}

rssh() {
  local host=$1
  shift
  ssh "${SSH_OPTS[@]}" "$host" "$@"
}

# ローカルのスクリプトをリモートで root として実行する: rscript host lib/isu/remote/x.sh args...
rscript() {
  local host=$1 script=$2
  shift 2
  local args="" a
  for a in "$@"; do args="$args $(printf '%q' "$a")"; done
  ssh "${SSH_OPTS[@]}" "$host" "sudo bash -s --$args" < "$ISU_ROOT/$script"
}

# macOS 標準の openrsync は -R や --itemize-changes が使えないので GNU rsync 3 系を探す。
find_rsync() {
  local r
  for r in "${ISU_RSYNC:-}" /opt/homebrew/bin/rsync /usr/local/bin/rsync /usr/bin/rsync; do
    [ -n "$r" ] && [ -x "$r" ] || continue
    # pipefail 下で grep -q が先に閉じると SIGPIPE で失敗扱いになるので、一度変数に受ける
    case "$("$r" --version 2>/dev/null)" in "rsync  version 3"*) echo "$r"; return ;; esac
  done
  die "GNU rsync 3 系が必要です（macOS: brew install rsync）"
}

# isu_rsync [opts...] src dest  リモート側は sudo で読み書きする
isu_rsync() {
  [ -n "${RSYNC_BIN:-}" ] || RSYNC_BIN=$(find_rsync)
  "$RSYNC_BIN" -rlptcz --rsync-path="sudo rsync" -e "ssh ${SSH_OPTS[*]}" "$@"
}

# 持ち主チェック: 他人のホストや本番ホストへの変更系操作を止める。
guard_host() {
  local host=$1 force=$2 owner
  [ "$force" = 1 ] && return 0
  owner=$(host_attr "$host" owner)
  if [ -n "$owner" ] && [ "$owner" != "$ISU_ME" ]; then
    die "$host の持ち主は $owner です（自分: $ISU_ME）。必要なら -f"
  fi
  if [ "$host" = "${ISU_PROD_HOST:-}" ]; then
    local branch
    branch=$(git -C "$ISU_ROOT" rev-parse --abbrev-ref HEAD)
    [ "$branch" = main ] || die "$host は本番計測ホストです。main 以外（$branch）からは反映できません。必要なら -f"
    git -C "$ISU_ROOT" diff --quiet HEAD -- || die "$host は本番計測ホストです。未コミットの変更があります。必要なら -f"
  fi
}

# 共通オプション -f を取り除き、FORCE と残りの引数 ARGS を設定する。
parse_force() {
  FORCE=0
  ARGS=()
  local a
  for a in "$@"; do
    case "$a" in
      -f|--force) FORCE=1 ;;
      *) ARGS+=("$a") ;;
    esac
  done
}

# ロックは各ホストの /run/isu.lock（再起動で消える）。TTL を過ぎたものは奪える。
ISU_LOCK_TTL=${ISU_LOCK_TTL:-900}
ISU_LOCK_WAIT=${ISU_LOCK_WAIT:-600}

lock_acquire() {
  local host=$1 purpose=${2:-work} me waited=0 out
  me=$(agent_id)
  while :; do
    out=$(rscript "$host" lib/isu/remote/lock.sh acquire "$me" "$purpose" "$ISU_LOCK_TTL")
    case "$out" in
      OK*) [ "$out" = "OK reentrant" ] || log "lock $host ($purpose)"; return 0 ;;
      BUSY*)
        if [ "$waited" -ge "$ISU_LOCK_WAIT" ]; then
          die "$host のロック待ちがタイムアウト: ${out#BUSY }"
        fi
        [ "$waited" -eq 0 ] && log "$host はロック中: ${out#BUSY } 待機します"
        sleep 10
        waited=$((waited + 10))
        ;;
      *) die "lock 失敗: $out" ;;
    esac
  done
}

lock_release() {
  rscript "$1" lib/isu/remote/lock.sh release "$(agent_id)" >/dev/null
}

lock_status() {
  rscript "$1" lib/isu/remote/lock.sh status 2>/dev/null || echo "?"
}

load_config
