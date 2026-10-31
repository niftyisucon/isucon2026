#!/usr/bin/env bash
# リモートで root として実行。構成情報を Markdown で標準出力へ出す。読み取りのみ。
set -u
db_name=${1:-isureal}
sec() { printf '\n## %s\n\n```\n' "$1"; }
end() { printf '```\n'; }
run() { "$@" 2>&1 | head -n "${LIMIT:-80}"; }

echo "# discovery: $(hostname) ($(date -Is))"

sec "マシン"
run lsb_release -ds; uname -mr
echo "cpu: $(nproc)  mem: $(free -h | awk '/Mem:/{print $2}')"
run df -h / /home 2>/dev/null
end

sec "ネットワーク"
run ip -4 -o addr show scope global
run ip -4 route
end

sec "待ち受けポート (ss -ltnp)"
run ss -ltnp
end

sec "動いているサービス（標準以外）"
systemctl list-units --type=service --state=running --no-pager --no-legend \
  | awk '{print $1}' | grep -vE '^(systemd-|dbus|cron|ssh|snapd|rsyslog|polkit|udisks|unattended|getty|serial-getty|multipathd|ModemManager|networkd|chrony|irqbalance|cloud-|user@|amazon-ssm|acpid|atd|packagekit|fwupd|uuidd|apport|thermald|upower)'
end

sec "systemd ユニット（/etc/systemd/system 直下の自作ユニット）"
for f in /etc/systemd/system/*.service; do
  [ -f "$f" ] && [ ! -L "$f" ] || continue
  echo "### $f"
  grep -E '^(ExecStart|WorkingDirectory|EnvironmentFile|User)=' "$f"
done
end

sec "systemd の実効設定と追加設定パス"
run systemctl show nginx.service isureal-go.service isureal-guard.service postgresql@16-main.service -p Id -p ActiveState -p FragmentPath -p DropInPaths -p EnvironmentFiles -p WorkingDirectory -p ExecStart -p User -p Group
end

sec "ホームディレクトリ"
for d in /home/*; do echo "$d:"; ls -la "$d" 2>/dev/null | tail -n +4; done
end

for envf in /home/*/env.sh /home/*/env /home/*/.env; do
  [ -f "$envf" ] || continue
  sec "環境変数ファイル $envf"
  # 調査結果は共有するため、接続先などを残し認証情報の値は記録しない。
  awk 'toupper($0) ~ /^[[:space:]]*(EXPORT[[:space:]]+)?[A-Z0-9_]*(PASSWORD|SECRET|TOKEN|KEY|CREDENTIAL)[A-Z0-9_]*=/ { sub(/=.*/, "=[REDACTED]") } { print }' "$envf"
  end
done

sec "webapp"
for d in /home/*/webapp; do
  [ -d "$d" ] || continue
  echo "$d:"
  du -sh "$d"/* 2>/dev/null
  ls "$d"/sql 2>/dev/null | sed 's/^/  sql\//'
done
end

if command -v nginx >/dev/null; then
  sec "nginx -T（include を展開した全設定）"
  nginx -T 2>&1
  end
fi

for svc in mysql mariadb postgresql redis-server memcached; do
  systemctl is-active --quiet "$svc" 2>/dev/null && echo "active: $svc"
done | { sec "ミドルウェア"; cat; end; }

if command -v mysql >/dev/null; then
  sec "MySQL"
  run mysql --version
  ls /etc/mysql/*.d 2>/dev/null
  mysql -NBe "SELECT table_schema, table_name, table_rows, ROUND((data_length+index_length)/1024/1024,1) AS mb FROM information_schema.tables WHERE table_schema NOT IN ('mysql','sys','information_schema','performance_schema') ORDER BY data_length+index_length DESC" 2>&1 | head -40
  mysql -NBe "SHOW VARIABLES WHERE Variable_name IN ('innodb_buffer_pool_size','max_connections','slow_query_log','slow_query_log_file','long_query_time','innodb_flush_log_at_trx_commit','disable_log_bin','log_bin')" 2>&1
  end
fi

if command -v psql >/dev/null; then
  sec "PostgreSQL"
  run psql --version
  command -v pg_lsclusters >/dev/null && run pg_lsclusters
  if [ -S /var/run/postgresql/.s.PGSQL.5432 ]; then
    run sudo -u postgres psql -X -d postgres -c "SELECT datname, pg_size_pretty(pg_database_size(oid)) AS size FROM pg_database WHERE NOT datistemplate;"
    run sudo -u postgres psql -X -d postgres -c "SELECT name, setting, unit, sourcefile FROM pg_settings WHERE name IN ('shared_buffers','work_mem','maintenance_work_mem','effective_cache_size','max_connections','wal_level','max_wal_size','synchronous_commit','log_min_duration_statement','logging_collector','log_directory','log_filename');"
    LIMIT=200 run sudo -u postgres psql -X -d "$db_name" -c "SELECT name, setting, unit, source, sourcefile FROM pg_settings WHERE source NOT IN ('default','override') ORDER BY name;"
    LIMIT=200 run sudo -u postgres psql -X -d "$db_name" -c "SELECT relname, n_live_tup, n_dead_tup, seq_scan, idx_scan FROM pg_stat_user_tables ORDER BY relname;"
    LIMIT=200 run sudo -u postgres psql -X -d "$db_name" -c "SELECT indexrelname, pg_size_pretty(pg_relation_size(indexrelid)) AS size, idx_scan FROM pg_stat_user_indexes ORDER BY indexrelname;"
  fi
  end
fi

sec "ログファイル"
ls -la /var/log/nginx /var/log/mysql /var/log/postgresql 2>/dev/null
end

sec "計測ツール"
for t in alp pt-query-digest dstat pidstat go git; do printf '%-16s %s\n' "$t" "$(command -v "$t" || echo -)"; done
ls /home/*/local 2>/dev/null
end
