# discovery: ip-192-168-4-10 (2026-10-31T01:39:59+00:00)

## マシン

```
Ubuntu 24.04.5 LTS
7.0.0-1014-aws x86_64
cpu: 2  mem: 3.7Gi
Filesystem      Size  Used Avail Use% Mounted on
/dev/root        58G   24G   34G  41% /
/dev/root        58G   24G   34G  41% /
```

## ネットワーク

```
2: ens5    inet 192.168.4.10/24 metric 100 brd 192.168.4.255 scope global dynamic ens5\       valid_lft 3417sec preferred_lft 3417sec
default via 192.168.4.1 dev ens5 proto dhcp src 192.168.4.10 metric 100 
192.168.0.2 via 192.168.4.1 dev ens5 proto dhcp src 192.168.4.10 metric 100 
192.168.4.0/24 dev ens5 proto kernel scope link src 192.168.4.10 metric 100 
192.168.4.1 dev ens5 proto dhcp scope link src 192.168.4.10 metric 100 
```

## 待ち受けポート (ss -ltnp)

```
State  Recv-Q Send-Q Local Address:Port Peer Address:PortProcess                                                
LISTEN 0      200          0.0.0.0:5432      0.0.0.0:*    users:(("postgres",pid=1187,fd=6))                    
LISTEN 0      4096      127.0.0.54:53        0.0.0.0:*    users:(("systemd-resolve",pid=345,fd=17))             
LISTEN 0      4096   127.0.0.53%lo:53        0.0.0.0:*    users:(("systemd-resolve",pid=345,fd=15))             
LISTEN 0      4096         0.0.0.0:22        0.0.0.0:*    users:(("sshd",pid=674,fd=3),("systemd",pid=1,fd=178))
LISTEN 0      200             [::]:5432         [::]:*    users:(("postgres",pid=1187,fd=7))                    
LISTEN 0      4096            [::]:22           [::]:*    users:(("sshd",pid=674,fd=4),("systemd",pid=1,fd=181))
```

## 動いているサービス（標準以外）

```
postgresql@16-main.service
snap.amazon-ssm-agent.amazon-ssm-agent.service
```

## systemd ユニット（/etc/systemd/system 直下の自作ユニット）

```
### /etc/systemd/system/envcheck-baseline.service
User=root
ExecStart=/usr/local/bin/isucon-contest-envcheck --record-baseline
### /etc/systemd/system/envcheck.service
User=root
ExecStart=/usr/local/bin/isucon-contest-envcheck
### /etc/systemd/system/isucon-register.service
ExecStart=/usr/bin/python3 /opt/isucon-registration/register.py
### /etc/systemd/system/isureal-go.service
User=isucon
WorkingDirectory=/home/isucon/webapp/go
EnvironmentFile=/home/isucon/env.sh
ExecStart=/home/isucon/webapp/go/isureal
### /etc/systemd/system/isureal-guard.service
User=isucon
EnvironmentFile=/home/isucon/env.sh
ExecStart=/opt/isureal-guard/bin/isureal-guard
### /etc/systemd/system/isureal-nodejs.service
User=isucon
WorkingDirectory=/home/isucon/webapp/nodejs
EnvironmentFile=/home/isucon/env.sh
ExecStart=/home/isucon/local/node/bin/node src/main.ts
### /etc/systemd/system/isureal-perl.service
User=isucon
WorkingDirectory=/home/isucon/webapp/perl
EnvironmentFile=/home/isucon/env.sh
ExecStart=/home/isucon/local/perl/bin/perl /home/isucon/webapp/perl/local/bin/hypnotoad -f /home/isucon/webapp/perl/script/isureal
### /etc/systemd/system/isureal-php.service
User=isucon
WorkingDirectory=/home/isucon/webapp/php
EnvironmentFile=/home/isucon/env.sh
ExecStart=/home/isucon/local/php/sbin/php-fpm --nodaemonize --force-stderr --fpm-config /home/isucon/webapp/php/php-fpm.conf
### /etc/systemd/system/isureal-python.service
User=isucon
WorkingDirectory=/home/isucon/webapp/python
EnvironmentFile=/home/isucon/env.sh
ExecStart=/home/isucon/webapp/python/.venv/bin/gunicorn
### /etc/systemd/system/isureal-ruby.service
User=isucon
WorkingDirectory=/home/isucon/webapp/ruby
EnvironmentFile=/home/isucon/env.sh
ExecStart=/home/isucon/local/ruby/bin/bundle exec puma -C config/puma.rb
### /etc/systemd/system/isureal-rust.service
User=isucon
WorkingDirectory=/home/isucon/webapp/rust
EnvironmentFile=/home/isucon/env.sh
ExecStart=/home/isucon/webapp/rust/isureal
### /etc/systemd/system/snap.amazon-ssm-agent.amazon-ssm-agent.service
EnvironmentFile=-/etc/environment
ExecStart=/usr/bin/snap run amazon-ssm-agent
WorkingDirectory=/var/snap/amazon-ssm-agent/13349
```

## systemd の実効設定と追加設定パス

```
ExecStart={ path=/usr/sbin/nginx ; argv[]=/usr/sbin/nginx -g daemon on; master_process on; ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }
WorkingDirectory=
User=
Group=
Id=nginx.service
ActiveState=inactive
FragmentPath=/usr/lib/systemd/system/nginx.service
DropInPaths=

ExecStart={ path=/home/isucon/webapp/go/isureal ; argv[]=/home/isucon/webapp/go/isureal ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }
EnvironmentFiles=/home/isucon/env.sh (ignore_errors=no)
WorkingDirectory=/home/isucon/webapp/go
User=isucon
Group=isucon
Id=isureal-go.service
ActiveState=inactive
FragmentPath=/etc/systemd/system/isureal-go.service
DropInPaths=

ExecStart={ path=/opt/isureal-guard/bin/isureal-guard ; argv[]=/opt/isureal-guard/bin/isureal-guard ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }
EnvironmentFiles=/home/isucon/env.sh (ignore_errors=no)
WorkingDirectory=
User=isucon
Group=isucon
Id=isureal-guard.service
ActiveState=inactive
FragmentPath=/etc/systemd/system/isureal-guard.service
DropInPaths=

ExecStart={ path=/usr/bin/pg_ctlcluster ; argv[]=/usr/bin/pg_ctlcluster --skip-systemctl-redirect 16-main start ; ignore_errors=yes ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }
WorkingDirectory=
User=
Group=
Id=postgresql@16-main.service
ActiveState=active
FragmentPath=/usr/lib/systemd/system/postgresql@.service
DropInPaths=
```

## ホームディレクトリ

```
/home/isuadmin:
-rw-r--r-- 1 isuadmin isuadmin  220 Mar 31  2024 .bash_logout
-rw-r--r-- 1 isuadmin isuadmin 3771 Mar 31  2024 .bashrc
-rw-r--r-- 1 isuadmin isuadmin  807 Mar 31  2024 .profile
drwx------ 2 isuadmin isuadmin 4096 Oct 31 01:06 .ssh
/home/isucon:
drwxrwxr-x  3 isucon isucon  4096 Oct 30 11:52 .ansible
-rw-r--r--  1 isucon isucon   220 Mar 31  2024 .bash_logout
-rw-r--r--  1 isucon isucon  3771 Mar 31  2024 .bashrc
drwxrwxr-x  3 isucon isucon  4096 Oct 30 11:59 .bundle
drwxrwxr-x  5 isucon isucon  4096 Oct 31 01:16 .cache
drwxrwxr-x  4 isucon isucon  4096 Oct 30 11:59 .cargo
drwxr-xr-x  2 isucon isucon  4096 Oct 30 12:21 .claude
drwxr-xr-x  2 isucon isucon  4096 Oct 30 12:21 .codex
drwxrwxr-x  5 isucon isucon  4096 Oct 30 12:21 .config
drwxr-xr-x  2 isucon isucon  4096 Oct 30 12:21 .copilot
drwxr-xr-x  2 isucon isucon  4096 Oct 30 12:21 .gemini
drwxrwxr-x  4 isucon isucon  4096 Oct 30 11:54 .local
drwxrwxr-x  4 isucon isucon  4096 Oct 30 12:01 .npm
drwxrwxr-x  5 isucon isucon  4096 Oct 30 12:00 .perl-cpm
drwxr-xr-x  3 isucon isucon  4096 Oct 30 12:21 .pi
-rw-r--r--  1 isucon isucon  1101 Oct 30 11:59 .profile
drwxrwxr-x  6 isucon isucon  4096 Oct 30 11:54 .rustup
drwx------  2 isucon isucon  4096 Oct 31 01:07 .ssh
-rw-r--r--  1 isucon isucon 10940 Oct 30 12:21 AGENTS.md
-rw-r--r--  1 isucon isucon   143 Oct 30 11:51 env.sh
drwxrwxr-x  3 isucon isucon  4096 Oct 30 11:59 go
drwxr-xr-x  8 isucon isucon  4096 Oct 30 11:58 local
drwxr-xr-x 11 isucon isucon  4096 Oct 30 12:21 webapp
/home/ubuntu:
-rw------- 1 ubuntu ubuntu  124 Oct 30 12:48 .Xauthority
drwx------ 3 ubuntu ubuntu 4096 Oct 30 11:51 .ansible
-rw-r--r-- 1 ubuntu ubuntu  220 Mar 31  2024 .bash_logout
-rw-r--r-- 1 ubuntu ubuntu 3771 Mar 31  2024 .bashrc
drwx------ 2 ubuntu ubuntu 4096 Oct 30 11:51 .cache
-rw-r--r-- 1 ubuntu ubuntu  807 Mar 31  2024 .profile
drwx------ 2 ubuntu ubuntu 4096 Oct 30 11:50 .ssh
-rw-r--r-- 1 ubuntu ubuntu    0 Oct 30 11:51 .sudo_as_admin_successful
```

## 環境変数ファイル /home/isucon/env.sh

```
ISUREAL_REGION=asia
ISUREAL_DB_HOST=127.0.0.1
ISUREAL_DB_PORT=5432
ISUREAL_DB_USER=isureal
ISUREAL_DB_PASSWORD=[REDACTED]
ISUREAL_DB_NAME=isureal
```

## webapp

```
/home/isucon/webapp:
12K	/home/isucon/webapp/AGENTS.md
17M	/home/isucon/webapp/go
42M	/home/isucon/webapp/nodejs
14M	/home/isucon/webapp/perl
3.3M	/home/isucon/webapp/php
384K	/home/isucon/webapp/public
75M	/home/isucon/webapp/python
27M	/home/isucon/webapp/ruby
660M	/home/isucon/webapp/rust
44K	/home/isucon/webapp/sql
  sql/001-schema.sql
  sql/003-initial-snapshot.sql
  sql/init.sh
  sql/reset.sql
```

## nginx -T（include を展開した全設定）

```
nginx: the configuration file /etc/nginx/nginx.conf syntax is ok
nginx: configuration file /etc/nginx/nginx.conf test is successful
# configuration file /etc/nginx/nginx.conf:
user www-data;
worker_processes auto;
pid /run/nginx.pid;
error_log /var/log/nginx/error.log;
include /etc/nginx/modules-enabled/*.conf;

events {
	worker_connections 768;
	# multi_accept on;
}

http {

	##
	# Basic Settings
	##

	sendfile on;
	tcp_nopush on;
	types_hash_max_size 2048;
	# server_tokens off;

	# server_names_hash_bucket_size 64;
	# server_name_in_redirect off;

	include /etc/nginx/mime.types;
	default_type application/octet-stream;

	##
	# SSL Settings
	##

	ssl_protocols TLSv1 TLSv1.1 TLSv1.2 TLSv1.3; # Dropping SSLv3, ref: POODLE
	ssl_prefer_server_ciphers on;

	##
	# Logging Settings
	##

	access_log /var/log/nginx/access.log;

	##
	# Gzip Settings
	##

	gzip on;

	# gzip_vary on;
	# gzip_proxied any;
	# gzip_comp_level 6;
	# gzip_buffers 16 8k;
	# gzip_http_version 1.1;
	# gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

	##
	# Virtual Host Configs
	##

	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}


#mail {
#	# See sample authentication script at:
#	# http://wiki.nginx.org/ImapAuthenticateWithApachePhpScript
#
#	# auth_http localhost/auth.php;
#	# pop3_capabilities "TOP" "USER";
#	# imap_capabilities "IMAP4rev1" "UIDPLUS";
#
#	server {
#		listen     localhost:110;
#		protocol   pop3;
#		proxy      on;
#	}
#
#	server {
#		listen     localhost:143;
#		protocol   imap;
#		proxy      on;
#	}
#}

# configuration file /etc/nginx/mime.types:

types {
    text/html                                        html htm shtml;
    text/css                                         css;
    text/xml                                         xml;
    image/gif                                        gif;
    image/jpeg                                       jpeg jpg;
    application/javascript                           js;
    application/atom+xml                             atom;
    application/rss+xml                              rss;

    text/mathml                                      mml;
    text/plain                                       txt;
    text/vnd.sun.j2me.app-descriptor                 jad;
    text/vnd.wap.wml                                 wml;
    text/x-component                                 htc;

    image/avif                                       avif;
    image/png                                        png;
    image/svg+xml                                    svg svgz;
    image/tiff                                       tif tiff;
    image/vnd.wap.wbmp                               wbmp;
    image/webp                                       webp;
    image/x-icon                                     ico;
    image/x-jng                                      jng;
    image/x-ms-bmp                                   bmp;

    font/woff                                        woff;
    font/woff2                                       woff2;

    application/java-archive                         jar war ear;
    application/json                                 json;
    application/mac-binhex40                         hqx;
    application/msword                               doc;
    application/pdf                                  pdf;
    application/postscript                           ps eps ai;
    application/rtf                                  rtf;
    application/vnd.apple.mpegurl                    m3u8;
    application/vnd.google-earth.kml+xml             kml;
    application/vnd.google-earth.kmz                 kmz;
    application/vnd.ms-excel                         xls;
    application/vnd.ms-fontobject                    eot;
    application/vnd.ms-powerpoint                    ppt;
    application/vnd.oasis.opendocument.graphics      odg;
    application/vnd.oasis.opendocument.presentation  odp;
    application/vnd.oasis.opendocument.spreadsheet   ods;
    application/vnd.oasis.opendocument.text          odt;
    application/vnd.openxmlformats-officedocument.presentationml.presentation
                                                     pptx;
    application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
                                                     xlsx;
    application/vnd.openxmlformats-officedocument.wordprocessingml.document
                                                     docx;
    application/vnd.wap.wmlc                         wmlc;
    application/wasm                                 wasm;
    application/x-7z-compressed                      7z;
    application/x-cocoa                              cco;
    application/x-java-archive-diff                  jardiff;
    application/x-java-jnlp-file                     jnlp;
    application/x-makeself                           run;
    application/x-perl                               pl pm;
    application/x-pilot                              prc pdb;
    application/x-rar-compressed                     rar;
    application/x-redhat-package-manager             rpm;
    application/x-sea                                sea;
    application/x-shockwave-flash                    swf;
    application/x-stuffit                            sit;
    application/x-tcl                                tcl tk;
    application/x-x509-ca-cert                       der pem crt;
    application/x-xpinstall                          xpi;
    application/xhtml+xml                            xhtml;
    application/xspf+xml                             xspf;
    application/zip                                  zip;

    application/octet-stream                         bin exe dll;
    application/octet-stream                         deb;
    application/octet-stream                         dmg;
    application/octet-stream                         iso img;
    application/octet-stream                         msi msp msm;

    audio/midi                                       mid midi kar;
    audio/mpeg                                       mp3;
    audio/ogg                                        ogg;
    audio/x-m4a                                      m4a;
    audio/x-realaudio                                ra;

    video/3gpp                                       3gpp 3gp;
    video/mp2t                                       ts;
    video/mp4                                        mp4;
    video/mpeg                                       mpeg mpg;
    video/ogg                                        ogv;
    video/quicktime                                  mov;
    video/webm                                       webm;
    video/x-flv                                      flv;
    video/x-m4v                                      m4v;
    video/x-matroska                                 mkv;
    video/x-mng                                      mng;
    video/x-ms-asf                                   asx asf;
    video/x-ms-wmv                                   wmv;
    video/x-msvideo                                  avi;
}

# configuration file /etc/nginx/sites-enabled/isureal.conf:
server {
  listen 80 default_server;
  server_name _;

  client_max_body_size 32m;
  root /home/isucon/webapp/public;

  location /api/ {
    proxy_set_header Host $host;
    proxy_pass http://127.0.0.1:8080;
  }

  location / {
    try_files $uri /index.html;
  }
}

```

## ミドルウェア

```
active: postgresql
```

## PostgreSQL

```
psql (PostgreSQL) 16.15 (Ubuntu 16.15-0ubuntu0.24.04.1)
Ver Cluster Port Status Owner    Data directory              Log file
16  main    5432 online postgres /var/lib/postgresql/16/main /var/log/postgresql/postgresql-16-main.log
 datname  |  size   
----------+---------
 postgres | 7503 kB
 isureal  | 16 GB
(2 rows)

            name            |            setting             | unit |                   sourcefile                   
----------------------------+--------------------------------+------+------------------------------------------------
 effective_cache_size       | 524288                         | 8kB  | 
 log_directory              | log                            |      | 
 log_filename               | postgresql-%Y-%m-%d_%H%M%S.log |      | 
 log_min_duration_statement | 0                              | ms   | /etc/postgresql/16/main/conf.d/99-isu-log.conf
 logging_collector          | off                            |      | 
 maintenance_work_mem       | 65536                          | kB   | 
 max_connections            | 100                            |      | /etc/postgresql/16/main/postgresql.conf
 max_wal_size               | 1024                           | MB   | /etc/postgresql/16/main/postgresql.conf
 shared_buffers             | 16384                          | 8kB  | /etc/postgresql/16/main/postgresql.conf
 synchronous_commit         | on                             |      | 
 wal_level                  | replica                        |      | 
 work_mem                   | 4096                           | kB   | 
(12 rows)

            name            |                setting                 | unit |       source       |                   sourcefile                   
----------------------------+----------------------------------------+------+--------------------+------------------------------------------------
 DateStyle                  | ISO, MDY                               |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 TimeZone                   | Etc/UTC                                |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 application_name           | psql                                   |      | client             | 
 cluster_name               | 16/main                                |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 default_text_search_config | pg_catalog.english                     |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 dynamic_shared_memory_type | posix                                  |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 external_pid_file          | /var/run/postgresql/16-main.pid        |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 lc_messages                | C.UTF-8                                |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 lc_monetary                | C.UTF-8                                |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 lc_numeric                 | C.UTF-8                                |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 lc_time                    | C.UTF-8                                |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 listen_addresses           | *                                      |      | configuration file | /etc/postgresql/16/main/conf.d/isucon.conf
 log_line_prefix            | %m [%p] %q%u@%d                        |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 log_min_duration_statement | 0                                      | ms   | configuration file | /etc/postgresql/16/main/conf.d/99-isu-log.conf
 log_timezone               | Etc/UTC                                |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 max_connections            | 100                                    |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 max_wal_size               | 1024                                   | MB   | configuration file | /etc/postgresql/16/main/postgresql.conf
 min_wal_size               | 80                                     | MB   | configuration file | /etc/postgresql/16/main/postgresql.conf
 port                       | 5432                                   |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 shared_buffers             | 16384                                  | 8kB  | configuration file | /etc/postgresql/16/main/postgresql.conf
 ssl                        | on                                     |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 ssl_cert_file              | /etc/ssl/certs/ssl-cert-snakeoil.pem   |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 ssl_key_file               | /etc/ssl/private/ssl-cert-snakeoil.key |      | configuration file | /etc/postgresql/16/main/postgresql.conf
 unix_socket_directories    | /var/run/postgresql                    |      | configuration file | /etc/postgresql/16/main/postgresql.conf
(24 rows)

         relname         | n_live_tup | n_dead_tup | seq_scan | idx_scan 
-------------------------+------------+------------+----------+----------
 administrator_sessions  |          1 |          1 |       27 |        1
 administrators          |          1 |          0 |        2 |        1
 block_relations         |        255 |          0 |      558 |      513
 comments                |     546011 |          0 |        6 |     1835
 follows                 |      10036 |          1 |        7 |       79
 friend_requests         |      29090 |         24 |      192 |      434
 friendships             |      25061 |          1 |     1268 |        6
 image_objects           |          0 |          0 |        0 |        0
 image_objects_202409    |          0 |          0 |        5 |     4602
 image_objects_202410    |         30 |          0 |        5 |     4602
 image_objects_202411    |        200 |          0 |        5 |     4602
 image_objects_202412    |        218 |          0 |        5 |     4602
 image_objects_202501    |        214 |          0 |        5 |     4602
 image_objects_202502    |        174 |          0 |        5 |     4602
 image_objects_202503    |        214 |          0 |        5 |     4602
 image_objects_202504    |        198 |          0 |        5 |     4602
 image_objects_202505    |        217 |          0 |        5 |     4602
 image_objects_202506    |        215 |          0 |        5 |     4602
 image_objects_202507    |        209 |          0 |        5 |     4602
 image_objects_202508    |        213 |          0 |        5 |     4602
 image_objects_202509    |        199 |          0 |        5 |     4602
 image_objects_202510    |      19054 |          0 |        5 |     4602
 image_objects_202511    |     150273 |          0 |        5 |     4602
 image_objects_202512    |     153901 |          0 |        5 |     4602
 image_objects_202601    |     154984 |          0 |        5 |     4602
 image_objects_202602    |     141545 |          0 |        5 |     4602
 image_objects_202603    |     154867 |          0 |        5 |     4602
 image_objects_202604    |     150384 |          0 |        5 |     4602
 image_objects_202605    |     155262 |          0 |        5 |     4602
 image_objects_202606    |     149322 |          0 |        5 |     4602
 image_objects_202607    |     153703 |          0 |        5 |     4602
 image_objects_202608    |     154966 |          0 |        5 |     4602
 image_objects_202609    |     150783 |          0 |        5 |     4602
 image_objects_202610    |     135605 |          0 |        6 |     4602
 image_objects_20261028  |        132 |          0 |       27 |     4582
 image_objects_202611    |          0 |          0 |        7 |     4602
 image_objects_202612    |          0 |          0 |        7 |     4602
 image_objects_202701    |          0 |          0 |        7 |     4602
 image_objects_202702    |          0 |          0 |        7 |     4602
 image_objects_202703    |          0 |          0 |        7 |     4602
 image_objects_202704    |          0 |          0 |        7 |     4602
 image_objects_202705    |          0 |          0 |        7 |     4602
 image_objects_202706    |          0 |          0 |        7 |     4602
 image_objects_default   |          0 |          0 |        7 |     4602
 initial_block_relations |        250 |          0 |        1 |         
 initial_data_boundary   |          1 |          0 |        8 |         
 initial_follows         |      10020 |          0 |        1 |         
 initial_friend_requests |      28970 |          0 |        1 |         
 initial_friendships     |      24985 |          0 |        1 |         
 initial_users           |       5000 |          0 |        1 |         
 ip_bans                 |         45 |         29 |      611 |      147
 login_sessions          |        247 |          7 |     2378 |      247
 notifications           |    1854450 |          0 |        6 |    25032
 posting_windows         |       1113 |         15 |     1276 |  2503605
 posts                   |     625239 |         15 |       10 |  1766966
 realmojis               |    1199351 |          2 |      344 |     5150
 users                   |       5043 |          8 |     2165 |  6210859
(57 rows)

                indexrelname                 |    size    | idx_scan 
---------------------------------------------+------------+----------
 administrator_sessions_pkey                 | 16 kB      |        1
 administrators_pkey                         | 16 kB      |        1
 block_relations_blocker_id_blocked_id_key   | 56 kB      |      513
 block_relations_pkey                        | 40 kB      |        0
 comments_pkey                               | 40 MB      |        0
 comments_post_id                            | 31 MB      |     1835
 follows_follower_id_followee_id_key         | 1384 kB    |       79
 follows_pkey                                | 744 kB     |        0
 friend_requests_pkey                        | 2232 kB    |      434
 friendships_pkey                            | 1984 kB    |        0
 friendships_user_low_id_user_high_id_key    | 3384 kB    |        6
 image_objects_202409_pkey                   | 8192 bytes |     4602
 image_objects_202410_pkey                   | 16 kB      |     4602
 image_objects_202411_pkey                   | 40 kB      |     4602
 image_objects_202412_pkey                   | 40 kB      |     4602
 image_objects_202501_pkey                   | 40 kB      |     4602
 image_objects_202502_pkey                   | 40 kB      |     4602
 image_objects_202503_pkey                   | 40 kB      |     4602
 image_objects_202504_pkey                   | 40 kB      |     4602
 image_objects_202505_pkey                   | 40 kB      |     4602
 image_objects_202506_pkey                   | 40 kB      |     4602
 image_objects_202507_pkey                   | 40 kB      |     4602
 image_objects_202508_pkey                   | 40 kB      |     4602
 image_objects_202509_pkey                   | 40 kB      |     4602
 image_objects_202510_pkey                   | 1632 kB    |     4602
 image_objects_202511_pkey                   | 13 MB      |     4602
 image_objects_202512_pkey                   | 13 MB      |     4602
 image_objects_202601_pkey                   | 13 MB      |     4602
 image_objects_202602_pkey                   | 12 MB      |     4602
 image_objects_202603_pkey                   | 13 MB      |     4602
 image_objects_202604_pkey                   | 12 MB      |     4602
 image_objects_202605_pkey                   | 13 MB      |     4602
 image_objects_202606_pkey                   | 12 MB      |     4602
 image_objects_202607_pkey                   | 13 MB      |     4602
 image_objects_202608_pkey                   | 13 MB      |     4602
 image_objects_202609_pkey                   | 13 MB      |     4602
 image_objects_20261028_pkey                 | 16 kB      |     4582
 image_objects_202610_pkey                   | 11 MB      |     4602
 image_objects_202611_pkey                   | 8192 bytes |     4602
 image_objects_202612_pkey                   | 8192 bytes |     4602
 image_objects_202701_pkey                   | 8192 bytes |     4602
 image_objects_202702_pkey                   | 8192 bytes |     4602
 image_objects_202703_pkey                   | 8192 bytes |     4602
 image_objects_202704_pkey                   | 8192 bytes |     4602
 image_objects_202705_pkey                   | 8192 bytes |     4602
 image_objects_202706_pkey                   | 8192 bytes |     4602
 image_objects_default_pkey                  | 8192 bytes |     4602
 ip_bans_pkey                                | 16 kB      |      147
 login_sessions_pkey                         | 40 kB      |      247
 notifications_pkey                          | 137 MB     |        0
 notifications_user_id_posting_window_id_key | 241 MB     |    25032
 posting_windows_one_active_region           | 16 kB      |    10412
 posting_windows_pkey                        | 88 kB      |  2493175
 posting_windows_region_local_date_key       | 72 kB      |       18
 posts_author_id                             | 5536 kB    |     8892
 posts_pkey                                  | 46 MB      |  1749327
 posts_posting_window_id                     | 7376 kB    |     8747
 realmojis_one_active                        | 141 MB     |     1629
 realmojis_pkey                              | 87 MB      |        7
 realmojis_post_id                           | 49 MB      |     3514
 users_pkey                                  | 472 kB     |  6210859
(61 rows)

```

## ログファイル

```
/var/log/nginx:
total 12
drwxr-xr-x  2 root     adm    4096 Oct 30 11:59 .
drwxrwxr-x 13 root     syslog 4096 Oct 31 01:07 ..
-rw-r-----  1 www-data adm       0 Oct 30 11:59 access.log
-rw-r-----  1 www-data adm      80 Oct 30 11:59 error.log

/var/log/postgresql:
total 484
drwxrwxr-t  2 root     postgres   4096 Oct 30 11:59 .
drwxrwxr-x 13 root     syslog     4096 Oct 31 01:07 ..
-rw-r-----  1 postgres adm      479653 Oct 31 01:40 postgresql-16-main.log
```

## 計測ツール

```
alp              -
pt-query-digest  -
dstat            -
pidstat          /usr/bin/pidstat
go               -
git              /usr/bin/git
go
node
perl
php
ruby
uv
```
