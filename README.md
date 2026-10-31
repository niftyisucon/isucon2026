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

調査結果を見て `isu.conf` の `ChangeMe`、役割、管理対象パス、採用する言語、ビルド方法を埋めてください。
ひな形の言語除外設定は Go 用なので、採用言語に合わせて変更します。

```bash
./bin/isu setup             # 計測ツール導入、nginx LTSV ログ・MySQL スローログ設定
./bin/isu pull              # サーバー設定とアプリを取り込む
./bin/isu deploy isu1       # ビルド → rsync → 変更に応じてサービス再起動
```

`discover`、`setup`、`pull`、`deploy`、`restart` はホスト名を省略すると全ホストを対象にします。
`pull` は設定を `hosts/<host>/<サーバーの絶対パス>` に、最初の app ホストのアプリを `webapp/` に取り込みます。
`pull` と `deploy` は rsync の `--delete` を使うため、同期先の対象ファイルは同期元に合わせて削除もされます。除外設定は `isu.conf` で指定します。

`setup` はパッケージ導入とログ設定を行い、nginx の reload と DB の restart を実行します。
最終提出前のログ無効化は手動で行ってください。

`deploy` は owner と本番ホストのチェックを行い、ホスト単位のロックを取ります。
`ISU_PROD_HOST` に指定したホストは、Git の `main` ブランチかつ追跡対象に未コミット変更がない状態から反映します。
`-f` / `--force` で owner・本番チェックを省略できます。

コマンドの詳細は `./bin/isu help`、競技中の作業方針とディレクトリ構成は [AGENTS.md](AGENTS.md) を参照してください。

## 計測・分析ツール

分析ツールの共有設定は `config/` に置きます。ALP の設定は `config/alp.yml` です。
現在のツール一覧、追加時の配置先と CLI 化の方針は [AGENTS.md の計測・分析ツールの追加方針](AGENTS.md#計測分析ツールの追加方針)、ALP の実行手順は [ALP の設定と使い方](AGENTS.md#alp-の設定と使い方) を参照してください。
