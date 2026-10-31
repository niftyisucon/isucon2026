#!/usr/bin/env bash
# DB の定義のみ収集。ユーザーデータ・ロール・パスワードは出力しない。
set -eu
exec sudo -u postgres pg_dump --schema-only --no-owner --no-privileges --lock-wait-timeout=5s "$1"
