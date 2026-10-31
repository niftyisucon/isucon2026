# ISUCON 2026 本戦

ISUCON 2026 本戦の競技用リポジトリです。
競技ルール・制約・要求仕様を遵守し、レスポンスの高速化、スループットとベンチマークスコアの向上を目指します。
サーバーの構成調査・セットアップ・設定とアプリの取り込み・デプロイには `./bin/isu` を使います。

## はじめに

必要なものは Bash、SSH、GNU rsync 3 系（macOS は `brew install rsync`）、Git、アプリのビルドに必要なツールチェーンです。
サーバーではパスワードなしの sudo が必要です。setup は Debian/Ubuntu 系、サービス操作は systemd を想定しています。

```bash
mkdir -p .local
cp examples/ssh_config .local/ssh_config
cp examples/isu.local.conf .local/isu.local.conf
```

`.local/ssh_config` に各ホストの接続先と鍵、`.local/isu.local.conf` に自分の名前（`ISU_ME`）を設定します。
`isu.conf` のホスト名・owner をそれぞれ一致させてください。`.local/` は Git 管理対象外です。

## 初動からデプロイまで

```bash
./bin/isu help
./bin/isu hosts
./bin/isu discover          # hosts/<host>/discovery.md に構成を出力
```

現在の `isu.conf` は、isu1=asia1、isu2=europe1、isu3=america1、isu4=asia2（共有 PostgreSQL 16）と、Go 実装に対応しています。ルーターは操作対象外です。
owner は担当確定まで `unassigned` なので、反映操作の前に担当者を設定してください。採用言語を変える場合は除外設定とビルド方法も変更します。

```bash
./bin/isu setup             # 役割別に計測ツール導入、nginx LTSV・PostgreSQLクエリ時間ログ設定
./bin/isu pull              # サーバー設定とアプリを取り込む
./bin/isu deploy isu1       # ビルド → rsync → 変更に応じてサービス再起動
```

`discover`、`setup`、`pull`、`deploy`、`restart` はホスト名を省略すると全ホストを対象にします。
`pull` は設定を `hosts/<host>/<サーバーの絶対パス>` に、最初の app ホストのアプリを `webapp/` に取り込みます。
`pull` と `deploy` は rsync の `--delete` を使うため、同期先の対象ファイルは同期元に合わせて削除もされます。除外設定は `isu.conf` で指定します。

`discover` は実効サービス設定・DB統計を含む調査結果と、DB の実スキーマ（`hosts/isu4/postgresql-schema.sql`）も収集します。実スキーマは調査用で、deploy の反映対象には含みません。
`setup` はパッケージ導入とログ設定を行い、nginx と PostgreSQL の reload を実行します（MySQL / MariaDB 構成ではDB restart）。
最終提出前のログ無効化は手動で行ってください。

`deploy` は owner と本番ホストのチェックを行い、ホスト単位のロックを取ります。
`ISU_PROD_HOST` に指定したホストは、Git の `main` ブランチかつ追跡対象に未コミット変更がない状態から反映します。
`-f` / `--force` で owner・本番チェックを省略できます。

コマンドの詳細は `./bin/isu help`、競技中の作業方針とディレクトリ構成は [AGENTS.md](AGENTS.md) を参照してください。

## 計測・分析ツール

PostgreSQLログは次のCLIで取得し、ローカルのpgBadgerでHTML・TEXTに集計できます。

```bash
./bin/isu pgbadger-setup    # ローカルに固定版を導入（Perl・curlが必要）
./bin/isu pglog isu4        # DBログ取得 → ローカル集計
```

時刻範囲指定、取得済みログの再利用、レポートの見方は [PostgreSQLログの取得・pgBadger集計](docs/pgbadger.md) を参照してください。
期間指定は解析範囲だけを絞ります。ベンチ直後に1回取得し、以後の再集計は `--input-dir` で同じログを再利用してください（転送し直しません）。

分析ツールの共有設定は `config/` に置きます。ALP の設定は `config/alp.yml` です。
アプリ3台のログは `/var/log/nginx/access_ltsv.log`、DBのクエリ時間ログは `/var/log/postgresql/postgresql-16-main.log` に出力します。
リポジトリルートから、設定を標準入力で渡してサーバー上のALPで集計できます（`isu2`、`isu3` も同様）。

```bash
./bin/isu ssh isu1 'sudo alp ltsv --file /var/log/nginx/access_ltsv.log --config /dev/stdin' < config/alp.yml
./bin/isu ssh isu4 'sudo -u postgres psql -X -d isureal -c "SHOW log_min_duration_statement;"'
```

現在のツール一覧、追加時の配置先と CLI 化の方針は [AGENTS.md の計測・分析ツールの追加方針](AGENTS.md#計測分析ツールの追加方針)、ALP の実行手順は [ALP の設定と使い方](AGENTS.md#alp-の設定と使い方) を参照してください。
