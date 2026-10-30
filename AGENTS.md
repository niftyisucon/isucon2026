# ISUCON 16 本戦

## 目的・作業方針

このリポジトリは ISUCON 16 本戦のベンチマークコンテスト用です。
主目的は、競技ルールと要求仕様を満たしたうえでレスポンスを高速化し、スループットとベンチマークスコアを向上させることです。
一般的なコード品質・抽象化・保守性の向上よりも、競技時間内に性能改善を実現することを優先してください。
性能改善に直接つながらないリファクタリングや過剰な設計・テスト整備は避け、変更の検証は正当性と性能への影響を確認するために必要な範囲で行ってください。

## 重要事項

- **後から取り込む競技ルール・レギュレーション・制約・要求仕様を必ず確認し、遵守してください。** 性能やスコアの向上を理由に違反してはいけません。
- ルールや制約に更新・追加があった場合は、その内容を以後の調査・実装・サーバー操作に反映してください。不明な点は推測で回避せず確認してください。
- テンプレート由来の設定は本戦の構成を保証しません。実サーバーを調査し、`isu.conf` の `ChangeMe`、サービス名、パス、採用言語を実環境に合わせて変更してください。

## CLI の準備

サーバー操作にはリポジトリルートから `./bin/isu` を使います。
必要なものは Bash、SSH、GNU rsync 3 系、Git、アプリのビルド用ツールチェーンです。macOS では GNU rsync を `brew install rsync` で導入します。
サーバー側はパスワードなしの sudo が必要で、setup は Debian/Ubuntu 系、サービス操作は systemd を想定しています。

```bash
mkdir -p .local
cp examples/ssh_config .local/ssh_config
cp examples/isu.local.conf .local/isu.local.conf
```

- `.local/ssh_config`: 接続先・ユーザー・鍵を設定。Host 名を `isu.conf` の `ISU_HOSTS` と一致させます。
- `.local/isu.local.conf`: `ISU_ME` を自分の名前に変更。`isu.conf` の各ホストの owner と対応させます。
- `isu.conf`: チーム共有のホスト構成、同期対象、サービス、ビルド方法を設定します。
- `.local/` は Git 管理対象外です。worktree に個人設定がなければ、メイン checkout の `.local/` が使われます。

## コマンドの使い方

| コマンド | 用途 |
|---|---|
| `./bin/isu help` | 利用できるコマンドを表示。`-h` / `--help` も可 |
| `./bin/isu hosts` | ホストの owner・役割・IP・ロック状態を確認 |
| `./bin/isu whoami` | 操作者の識別子（`担当者名:短縮コミット番号`）を確認。Git未初期化・未コミットなら `担当者名:no-commit` |
| `./bin/isu ssh isu1` | 指定ホストへ対話 SSH |
| `./bin/isu ssh isu1 'systemctl status nginx'` | 指定ホストでコマンドを実行 |
| `./bin/isu discover [host...]` | 構成を調査し `hosts/<host>/discovery.md` に保存 |
| `./bin/isu setup [host...]` | 計測ツール導入、nginx LTSV ログ・MySQL スローログを設定（冪等） |
| `./bin/isu pull [host...]` | サーバー設定を `hosts/<host>/`、最初の app ホストのアプリを `webapp/` に取り込む |
| `./bin/isu build` | `ISU_BUILD_CMD` でローカルビルド。未設定ならスキップ |
| `./bin/isu deploy [host...] [-f]` | ビルド → rsync → 変更に応じたサービス再起動 |
| `./bin/isu restart [host...] [-f]` | ホストの役割に応じて nginx reload・DB / app restart |
| `./bin/isu lock isu1 '作業内容'` | ホストのロックを取得。使用中なら待機 |
| `./bin/isu unlock isu1` | 自分のロックを解放 |

`[host...]` は省略すると全ホスト、指定するとそのホストだけを対象にします（例: `./bin/isu deploy isu2 isu3`）。
ベンチ実行・分析・スコア管理・Docker による手元検証のコマンドは、この CLI には含めていません。

## 初動の流れと操作上の注意

1. 個人設定と `isu.conf` のホスト設定を記入し、`hosts` で接続を確認します。
2. `discover` の結果と競技ルール・制約を確認し、`isu.conf` の管理対象パス・言語除外設定・サービス・ビルド方法を確定します。
3. `setup` で計測環境を準備し、`pull` で設定とアプリを取り込みます。
4. 取り込んだ内容を編集し、`deploy <host>` で反映します。

- `setup` はサーバーを変更し、nginx の reload と DB の restart を実行します。最終提出前の計測ログの無効化は手動で行います。
- `pull` / `deploy` は rsync の `--delete` を使います。同期先の対象ファイルは削除もされるため、同期方向と除外設定を確認してください。
- `deploy` はホスト単位のロックを取得し、owner をチェックします。`restart` も owner をチェックします。
- `ISU_PROD_HOST` への `deploy` / `restart` は、Git の `main` ブランチかつ追跡対象に未コミット変更がない状態が必要です。Git 未初期化なら運用前に初期化・コミットしてください。
- `deploy` / `restart` の `-f` / `--force` は owner・本番チェックを省略します。競技ルールや制約を省略するものではありません。

詳細は [README.md](README.md) と `./bin/isu help` を参照してください。
