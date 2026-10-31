# ISUCON 2026 本戦

## 目的・作業方針

このリポジトリは ISUCON 2026 本戦のベンチマークコンテスト用です。
主目的は、競技ルールと要求仕様を満たしたうえでレスポンスを高速化し、スループットとベンチマークスコアを向上させることです。
一般的なコード品質・抽象化・保守性の向上よりも、競技時間内に性能改善を実現することを優先してください。
性能改善に直接つながらないリファクタリングや過剰な設計・テスト整備は避け、変更の検証は正当性と性能への影響を確認するために必要な範囲で行ってください。

## 重要事項

- **後から取り込む競技ルール・レギュレーション・制約・要求仕様を必ず確認し、遵守してください。** 性能やスコアの向上を理由に違反してはいけません。
- ルールや制約に更新・追加があった場合は、その内容を以後の調査・実装・サーバー操作に反映してください。不明な点は推測で回避せず確認してください。
- テンプレート由来の設定は本戦の構成を保証しません。実サーバーを調査し、`isu.conf` の `ChangeMe`、サービス名、パス、採用言語を実環境に合わせて変更してください。

## Claude Code / Codex の共有設定

- 共通の作業指示はこの `AGENTS.md` を編集します。`CLAUDE.md` は `AGENTS.md` への相対シンボリックリンクで、同じ内容を参照します。
- Codex の標準パスに合わせ、共有スキルの実体は `.agents/skills/<skill-name>/SKILL.md` に置きます（`.agent/` ではなく `.agents/`）。
- Claude Code 用の `.claude/skills` は `../.agents/skills` への相対シンボリックリンクです。スキルを追加・編集すると両方から参照できます。現在は置き場のみで、スキルは未追加です。
- `.claude/` と `.agents/` 全体はリンクせず、ツール固有の設定はそれぞれの仕様に従って管理します。Claude 専用の設定や機能は、共有スキルでも Codex に適用されるとは限りません。

```text
AGENTS.md                    # 共通指示の実体
CLAUDE.md -> AGENTS.md
.agents/
└── skills/                  # 共有スキルの実体
.claude/
└── skills -> ../.agents/skills
```

## CLI の準備

### Discord の状況共有

作業前に `docs/エージェント連携ガイド.md` を読み、その運用ルールに従います。
Windows では `pwsh -File ./tools/agent-log/alog.ps1 read` で直近の投稿を確認し、
`pwsh -File ./tools/agent-log/alog.ps1 post '「着手」対象と作業内容'` などで節目を共有します。
当日最初は「参加」、ベンチ前後は「ベンチ予約」「ベンチ結果」を投稿します。
読み取り未設定の場合は競合作業の有無を人間に確認します。
認証情報は Git 管理外の `tools/agent-log/.env` に置きます。
詳細は `docs/Discord-ローカルセットアップ.md` を参照してください。

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

## 取り込み後のディレクトリ構成と役割

個人設定を準備し、`discover` → `pull` を実行した後の構成例です。
`contest-go.service` と `webapp/` 内の名前は例であり、実際の構成は `isu.conf` とサーバー上のファイルによって決まります。

```text
isucon2026/
├── AGENTS.md
├── CLAUDE.md -> AGENTS.md
├── .agents/
│   └── skills/
├── .claude/
│   └── skills -> ../.agents/skills
├── README.md
├── isu.conf
├── .gitignore
├── .local/
│   ├── ssh_config
│   └── isu.local.conf
├── bin/
│   └── isu
├── lib/isu/
│   ├── common.sh
│   ├── cmd/
│   └── remote/
├── examples/
│   ├── ssh_config
│   └── isu.local.conf
├── hosts/
│   ├── isu1/
│   │   ├── discovery.md
│   │   ├── etc/
│   │   │   ├── nginx/
│   │   │   ├── mysql/
│   │   │   └── systemd/system/contest-go.service
│   │   └── home/isucon/env.sh
│   ├── isu2/                    # isu1 と同様にホスト別の設定を配置
│   └── isu3/
└── webapp/                      # 共通のアプリ一式
    ├── go/
    ├── sql/
    └── public/
```

| パス | 役割 |
|---|---|
| `.agents/skills/` | Claude Code / Codex で共有するスキルの実体 |
| `.claude/` | Claude Code 用の領域。`skills` は `.agents/skills/` を参照するリンク |
| `.local/` | 手動で準備する個人用の接続情報・操作者設定。Git 管理外で、`pull` は作成しない |
| `bin/` | CLI の実行入口 `isu` を置く |
| `lib/isu/` | CLI の共通処理を置く。`cmd/` はローカルで動く各サブコマンド、`remote/` は SSH 経由でサーバー上で実行するスクリプト |
| `examples/` | `.local/` にコピーして使う個人設定のひな形 |
| `hosts/` | ホストごとの調査結果と管理対象のサーバー設定を置く |
| `hosts/<host>/etc/` | そのホストから取り込んだ nginx・DB・systemd などの設定。サーバーの `/etc/` 以下のパス構造を保持する |
| `hosts/<host>/home/` | 管理対象に指定したホーム配下のファイル。例: `/home/isucon/env.sh` |
| `webapp/` | 全 app ホストへの反映に使う共通のアプリ。`go/` は実装、`sql/` は SQL、`public/` は静的ファイルの配置例であり、CLI が固定で作るものではない |

- `discovery.md` は `discover` が作成します。`pull` だけでは生成されません。
- `pull` は `ISU_HOST_FILES` に指定された、サーバーに存在するファイル・ディレクトリを取り込みます。存在しなければ警告してスキップします。現在の `ChangeMe.service` は実際のサービス名へ変更してください。
- 設定はホスト別です。例えば `isu2:/etc/nginx/nginx.conf` は `hosts/isu2/etc/nginx/nginx.conf` に入ります。
- アプリは、指定された対象ホストのうち最初に `app` の役割を持つホストから一度だけ取得します。現在の設定で引数なしなら `isu1`、`pull isu2` なら `isu2` です。アプリの取得対象は `ISU_WEBAPP_REMOTE` で指定します。
- アプリの中身は直接 `webapp/` に入り、`webapp/webapp/` にはなりません。`ISU_SYNC_EXCLUDES`・`ISU_WEBAPP_EXCLUDES`・`ISU_WEBAPP_PULL_EXCLUDES` による除外も適用されます。
- `deploy isu2` は `hosts/isu2/` 内の管理対象設定を反映し、`isu2` が `app` の役割を持つ場合は共通の `webapp/` も反映します。

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
