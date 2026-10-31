# discovery: ip-192-168-3-10 (2026-10-31T01:39:59+00:00)

## マシン

```
Ubuntu 24.04.5 LTS
7.0.0-1014-aws x86_64
cpu: 2  mem: 3.7Gi
Filesystem      Size  Used Avail Use% Mounted on
/dev/root        58G   24G   34G  42% /
/dev/root        58G   24G   34G  42% /
```

## ネットワーク

```
2: enp39s0    inet 192.168.3.10/24 metric 100 brd 192.168.3.255 scope global dynamic enp39s0\       valid_lft 3456sec preferred_lft 3456sec
default via 192.168.3.1 dev enp39s0 proto dhcp src 192.168.3.10 metric 100 
192.168.0.2 via 192.168.3.1 dev enp39s0 proto dhcp src 192.168.3.10 metric 100 
192.168.3.0/24 dev enp39s0 proto kernel scope link src 192.168.3.10 metric 100 
192.168.3.1 dev enp39s0 proto dhcp scope link src 192.168.3.10 metric 100 
```

## 待ち受けポート (ss -ltnp)

```
State  Recv-Q Send-Q Local Address:Port Peer Address:PortProcess                                                                         
LISTEN 0      4096   127.0.0.53%lo:53        0.0.0.0:*    users:(("systemd-resolve",pid=372,fd=15))                                      
LISTEN 0      4096         0.0.0.0:22        0.0.0.0:*    users:(("sshd",pid=700,fd=3),("systemd",pid=1,fd=262))                         
LISTEN 0      511          0.0.0.0:80        0.0.0.0:*    users:(("nginx",pid=3453,fd=5),("nginx",pid=3452,fd=5),("nginx",pid=1222,fd=5))
LISTEN 0      4096      127.0.0.54:53        0.0.0.0:*    users:(("systemd-resolve",pid=372,fd=17))                                      
LISTEN 0      4096               *:8080            *:*    users:(("isureal",pid=1218,fd=6))                                              
LISTEN 0      4096            [::]:22           [::]:*    users:(("sshd",pid=700,fd=4),("systemd",pid=1,fd=263))                         
```

## 動いているサービス（標準以外）

```
isureal-go.service
isureal-guard.service
nginx.service
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
ActiveState=active
FragmentPath=/usr/lib/systemd/system/nginx.service
DropInPaths=

ExecStart={ path=/home/isucon/webapp/go/isureal ; argv[]=/home/isucon/webapp/go/isureal ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }
EnvironmentFiles=/home/isucon/env.sh (ignore_errors=no)
WorkingDirectory=/home/isucon/webapp/go
User=isucon
Group=isucon
Id=isureal-go.service
ActiveState=active
FragmentPath=/etc/systemd/system/isureal-go.service
DropInPaths=

ExecStart={ path=/opt/isureal-guard/bin/isureal-guard ; argv[]=/opt/isureal-guard/bin/isureal-guard ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }
EnvironmentFiles=/home/isucon/env.sh (ignore_errors=no)
WorkingDirectory=
User=isucon
Group=isucon
Id=isureal-guard.service
ActiveState=active
FragmentPath=/etc/systemd/system/isureal-guard.service
DropInPaths=

ExecStart={ path=/usr/bin/pg_ctlcluster ; argv[]=/usr/bin/pg_ctlcluster --skip-systemctl-redirect 16-main start ; ignore_errors=yes ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }
WorkingDirectory=
User=
Group=
Id=postgresql@16-main.service
ActiveState=inactive
FragmentPath=/usr/lib/systemd/system/postgresql@.service
DropInPaths=
```

## ホームディレクトリ

```
/home/isuadmin:
-rw-r--r-- 1 isuadmin isuadmin  220 Mar 31  2024 .bash_logout
-rw-r--r-- 1 isuadmin isuadmin 3771 Mar 31  2024 .bashrc
-rw-r--r-- 1 isuadmin isuadmin  807 Mar 31  2024 .profile
drwx------ 2 isuadmin isuadmin 4096 Oct 31 01:07 .ssh
/home/isucon:
drwxrwxr-x  3 isucon isucon  4096 Oct 30 11:52 .ansible
-rw-------  1 isucon isucon     3 Oct 31 01:16 .bash_history
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
drwx------  2 isucon isucon  4096 Oct 31 01:08 .ssh
-rw-r--r--  1 isucon isucon 10940 Oct 30 12:21 AGENTS.md
-rw-r--r--  1 isucon isucon   149 Oct 31 01:08 env.sh
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
ISUREAL_REGION=america
ISUREAL_DB_HOST=192.168.4.10
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

# configuration file /etc/nginx/conf.d/isu-ltsv.conf:
# isu setup が追加。最終提出前に無効化する。
log_format isu_ltsv "time:$time_local\thost:$remote_addr\tforwardedfor:$http_x_forwarded_for"
  "\treq:$request\tstatus:$status\tmethod:$request_method\turi:$request_uri"
  "\tsize:$body_bytes_sent\treferer:$http_referer\tua:$http_user_agent"
  "\treqtime:$request_time\tapptime:$upstream_response_time\tvhost:$host";
access_log /var/log/nginx/access_ltsv.log isu_ltsv;

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
```

## PostgreSQL

```
psql (PostgreSQL) 16.15 (Ubuntu 16.15-0ubuntu0.24.04.1)
Ver Cluster Port Status Owner    Data directory              Log file
16  main    5432 down   postgres /var/lib/postgresql/16/main /var/log/postgresql/postgresql-16-main.log
```

## ログファイル

```
/var/log/nginx:
total 76
drwxr-xr-x  2 root     adm     4096 Oct 31 01:37 .
drwxrwxr-x 13 root     syslog  4096 Oct 31 01:07 ..
-rw-r-----  1 www-data adm    57759 Oct 31 01:39 access.log
-rw-r--r--  1 root     root     517 Oct 31 01:39 access_ltsv.log
-rw-r-----  1 www-data adm       80 Oct 30 11:59 error.log

/var/log/postgresql:
total 32
drwxrwxr-t  2 root     postgres  4096 Oct 30 11:59 .
drwxrwxr-x 13 root     syslog    4096 Oct 31 01:07 ..
-rw-r-----  1 postgres adm      16428 Oct 30 12:21 postgresql-16-main.log
```

## 計測ツール

```
alp              /usr/local/bin/alp
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
