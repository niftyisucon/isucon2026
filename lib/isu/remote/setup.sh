#!/usr/bin/env bash
# リモートで root として実行。冪等。usage: setup.sh <alp_version>
set -eu
alp_version=$1
export DEBIAN_FRONTEND=noninteractive NEEDRESTART_SUSPEND=1 NEEDRESTART_MODE=l

need_apt=""
command -v pt-query-digest >/dev/null || need_apt="$need_apt percona-toolkit"
command -v pidstat >/dev/null || need_apt="$need_apt sysstat"
command -v unzip >/dev/null || need_apt="$need_apt unzip"
if [ -n "$need_apt" ]; then
  echo "apt install:$need_apt"
  apt-get update -qq
  # shellcheck disable=SC2086
  apt-get install -y -qq $need_apt >/dev/null 2>&1 || { echo "apt install に失敗しました:$need_apt" >&2; exit 1; }
fi

if ! command -v alp >/dev/null; then
  arch=$(dpkg --print-architecture)
  echo "install alp $alp_version ($arch)"
  curl -fsSL -o /tmp/alp.tar.gz "https://github.com/tkuchiki/alp/releases/download/${alp_version}/alp_linux_${arch}.tar.gz"
  tar -xzf /tmp/alp.tar.gz -C /usr/local/bin alp
  chmod 755 /usr/local/bin/alp
fi

# nginx: LTSV のアクセスログを http コンテキストに追加（conf.d は http 内で include される前提）
if [ -d /etc/nginx/conf.d ]; then
  if ! grep -q 'include /etc/nginx/conf.d/\*.conf' /etc/nginx/nginx.conf; then
    echo "warning: nginx.conf が conf.d を include していません。LTSV ログは手動で設定してください" >&2
  elif [ ! -f /etc/nginx/conf.d/isu-ltsv.conf ]; then
    echo "nginx: enable LTSV access log"
    cat > /etc/nginx/conf.d/isu-ltsv.conf <<'NGINX'
# isu setup が追加。最終提出前に無効化する。
log_format isu_ltsv "time:$time_local\thost:$remote_addr\tforwardedfor:$http_x_forwarded_for"
  "\treq:$request\tstatus:$status\tmethod:$request_method\turi:$request_uri"
  "\tsize:$body_bytes_sent\treferer:$http_referer\tua:$http_user_agent"
  "\treqtime:$request_time\tapptime:$upstream_response_time\tvhost:$host";
access_log /var/log/nginx/access_ltsv.log isu_ltsv;
NGINX
    nginx -t && systemctl reload nginx
  fi
  if nginx -T 2>/dev/null | grep -E '^\s*access_log' | grep -vqE 'access_ltsv|/var/log/nginx/access.log;'; then
    echo "warning: server/location 単位の access_log があります。そこでは LTSV ログが出ない可能性があります" >&2
  fi
fi

# MySQL / MariaDB: スローログを全件出す
for d in /etc/mysql/mysql.conf.d /etc/mysql/mariadb.conf.d; do
  [ -d "$d" ] || continue
  if [ ! -f "$d/99-isu.cnf" ]; then
    echo "mysql: enable slow log ($d)"
    cat > "$d/99-isu.cnf" <<'CNF'
# isu setup が追加。最終提出前に無効化する。
[mysqld]
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 0
CNF
    svc=mysql
    systemctl is-active --quiet mariadb && svc=mariadb
    systemctl restart "$svc"
  fi
  break
done

echo "setup done: $(hostname)"
