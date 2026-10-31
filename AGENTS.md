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
├── config/
│   └── alp.yml                  # ALP の集計設定
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
| `config/` | ALP などの分析ツールの共有設定。`config/alp.yml` で URI の集約方法や表示項目を指定する。`pull` / `deploy` の同期対象外 |
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

## 計測・分析ツールの追加方針

性能調査に必要なツールは、競技ルール・制約を確認したうえで必要に応じて追加します。まず手動で使って効果を確認し、繰り返す操作は CLI にまとめます。

現在の導入・運用状況は次のとおりです。

| ツール | `setup` での処理 | 設定・実行方法 |
|---|---|---|
| ALP | 未導入ならインストール。nginx の LTSV ログを設定 | `config/alp.yml` と後述の手動集計手順を使用 |
| pt-query-digest | 未導入なら percona-toolkit をインストール。MySQL のスローログを設定 | サーバー上で手動実行。専用の共有設定・CLI は未追加 |
| pidstat など | pidstat がなければ sysstat をインストール | サーバー上で手動実行。自動記録・集計は未追加 |
| unzip | 未導入ならインストール | 導入処理用の補助ツール |

追加する内容ごとの配置先は次のとおりです。

| 追加する内容 | 配置先・対応 |
|---|---|
| 全ホスト共通のサーバーツール導入 | `lib/isu/remote/setup.sh`。導入済みならスキップするなど、再実行できる形にする |
| 特定の言語・役割だけで使うツール導入 | 必要なホストだけを対象とする個別手順、または専用コマンド。全ホスト共通の `setup` に無条件で加えない |
| 分析ツールの共有設定 | `config/<ツール名>.<拡張子>`。ファイルが増えたらツール別のサブディレクトリにまとめる |
| ツールの場所やビルド方法など CLI 全体の共有設定 | ルートの `isu.conf` |
| サーバーに反映するサービス・ミドルウェア設定 | `hosts/<host>/` のサーバー上と同じパス。必要なら `ISU_HOST_FILES` に管理対象を追加 |
| 個人設定、取得したログ、一時的な集計結果 | Git 管理外の `.local/` |
| 繰り返し使う操作の CLI 化 | `lib/isu/cmd/<name>.sh` に `cmd_<name>()` を定義。`bin/isu` のヘルプにも使い方を追記 |
| サーバー上で実行する処理 | `lib/isu/remote/<name>.sh`。ローカル側のコマンドから共通関数 `rscript` などで呼び出す |
| 導入条件・使い方・出力先の説明 | この `AGENTS.md` のツール一覧と手順を更新 |

現在の `setup [host...]` は、対象ホストに同じ導入処理を実行します。引数省略時は全ホストが対象で、役割別のツール選択機能はありません。
`config/` に置くだけでは、サーバーへの転送やツールへの読み込みは行われません。実行時に設定パスを指定し、サーバー側で使う場合は転送方法も手順またはコマンドに含めてください。

## ALP の設定と使い方

ALP の集計設定は `config/alp.yml` に置きます。サーバーへ反映する設定は `hosts/`、個人設定や一時的なログ・集計結果は `.local/`、分析ツールの共有設定は `config/` で管理します。CLI 全体の共有設定 `isu.conf` はルートに置いたままにします。

`config/alp.yml` の `matching_groups` に、本戦アプリのルーティング定義を確認して正規表現を追記してください。ID などのパスパラメータを含む URI をまとめ、エンドポイント単位で集計するために使います。例えば `/api/items/123` と `/api/items/456` をまとめる場合は、既存の `matching_groups: []` を次のように置き換えます。

```yaml
matching_groups:
  - ^/api/items/[^/]+$
```

この設定例の URI は説明用です。本戦のパスに合わせて編集し、別の処理を過剰にまとめないようにしてください。ファイル内の ISUCON14 の例も参考用です。冒頭コメントの `isu-recon` は元の運用名であり、このリポジトリには同名のスキルはありません。

初期設定では合計レスポンス時間（`sum`）の降順で Markdown を出力します。設定オプションの詳細は [ALP の公式ドキュメント](https://github.com/tkuchiki/alp) を参照してください。

`./bin/isu setup` はサーバーに ALP を導入し、nginx の LTSV ログを設定します。`config/alp.yml` の作成・編集・サーバーへの転送や集計の実行は行いません。

ローカル PC にも ALP を導入してある場合、リポジトリルートから次の手順でサーバーのログを取得して集計できます。`isu1` は対象の nginx ホストに置き換えてください。

```bash
mkdir -p .local
./bin/isu ssh isu1 'sudo cat /var/log/nginx/access_ltsv.log' > .local/access-isu1.ltsv
alp ltsv --file .local/access-isu1.ltsv --config config/alp.yml > .local/alp-isu1.md
cat .local/alp-isu1.md
```

ログ取得が成功したことを確認してから集計してください。ログパスを変更している場合は取得元も変更します。`matching_groups` を編集したら、同じ取得済みログに対して `alp ltsv` を再実行できます。ログの初期化や計測区間の切り出しはこの手順には含まれないため、取得したログに含まれるリクエストが集計対象になります。

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
