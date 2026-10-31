#!/usr/bin/env bash
# リモートで root として実行。冪等。usage: setup.sh <alp_version> <roles> <db_service>
set -eu
alp_version=$1
roles=${2:-nginx db}
db_service=${3:-mysql}
has_role() { case " $roles " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }
export DEBIAN_FRONTEND=noninteractive NEEDRESTART_SUSPEND=1 NEEDRESTART_MODE=l

need_apt=""
if has_role db; then
  case "$db_service" in
    mysql|mariadb) command -v pt-query-digest >/dev/null || need_apt="$need_apt percona-toolkit" ;;
  esac
fi
command -v pidstat >/dev/null || need_apt="$need_apt sysstat"
command -v unzip >/dev/null || need_apt="$need_apt unzip"
if [ -n "$need_apt" ]; then
  echo "apt install:$need_apt"
  apt-get update -qq
  # shellcheck disable=SC2086
  apt-get install -y -qq $need_apt >/dev/null 2>&1 || { echo "apt install に失敗しました:$need_apt" >&2; exit 1; }
fi

if has_role nginx && ! command -v alp >/dev/null; then
  arch=$(dpkg --print-architecture)
  echo "install alp $alp_version ($arch)"
  curl -fsSL -o /tmp/alp.tar.gz "https://github.com/tkuchiki/alp/releases/download/${alp_version}/alp_linux_${arch}.tar.gz"
  tar -xzf /tmp/alp.tar.gz -C /usr/local/bin alp
  chmod 755 /usr/local/bin/alp
fi

# nginx: LTSV のアクセスログを http コンテキストに追加（conf.d は http 内で include される前提）
if has_role nginx && [ -d /etc/nginx/conf.d ]; then
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
    nginx -t
    systemctl reload nginx
  fi
  if nginx -T 2>/dev/null | grep -E '^\s*access_log' | grep -vqE 'access_ltsv|/var/log/nginx/access.log;'; then
    echo "warning: server/location 単位の access_log があります。そこでは LTSV ログが出ない可能性があります" >&2
  fi
fi

# PostgreSQL: 指定クラスタのクエリ時間を記録。reload で反映でき、再起動は不要。
if has_role db; then
case "$db_service" in
  postgresql@*-*)
    cluster=${db_service#postgresql@}
    pg_conf_dir="/etc/postgresql/${cluster%%-*}/${cluster#*-}"
    [ -d "$pg_conf_dir/conf.d" ] || { echo "PostgreSQL conf.d がありません: $pg_conf_dir" >&2; exit 1; }
    if [ ! -f "$pg_conf_dir/conf.d/99-isu-log.conf" ]; then
      echo "postgresql: enable query duration log"
      cat > "$pg_conf_dir/conf.d/99-isu-log.conf" <<'PGCONF'
# isu setup が追加。最終提出前に無効化する。
log_min_duration_statement = 0
log_parameter_max_length = 0
PGCONF
      chown postgres:postgres "$pg_conf_dir/conf.d/99-isu-log.conf"
      systemctl reload "$db_service"
    fi
    ;;
  postgresql)
    echo "PostgreSQL は postgresql@16-main などのクラスタサービス名を指定してください" >&2
    exit 1
    ;;
  mysql|mariadb)
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
    ;;
  *) echo "未対応のDBサービス: $db_service" >&2; exit 1 ;;
esac
fi

echo "setup done: $(hostname)"
