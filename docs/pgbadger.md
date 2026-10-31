# PostgreSQLログの取得・pgBadger集計

DBのクエリ別実行回数、合計時間、平均時間などをpgBadgerで確認します。
ログ取得も集計もリポジトリルートから `./bin/isu` を使います。集計処理はローカルで行い、競技DB上では実行しません。

## 準備

ローカルに Bash、Perl、curl、gzip、SSH が必要です。macOSでは既存のPerlを使えます。
WindowsではWSLなど、これらと `./bin/isu` が使える環境で実行してください。

```bash
./bin/isu pgbadger-setup
```

公式pgBadger v13.2の固定コミットを取得し、SHA256検証とPerl構文検査後に `.local/tools/pgbadger/pgbadger` へ配置します。システム全体へのインストールはしません。
固定バージョン・コミット・SHA256は `isu.conf` にあります。独自の導入先を使う場合は、`.local/isu.local.conf` に `ISU_PGBADGER_BIN=/絶対パス/pgbadger` を指定します。

DB側は既存の `./bin/isu setup isu4` でクエリ時間ログを有効にしておきます。現環境では実施済みです。
設定は `log_min_duration_statement = 0`、出力先は `/var/log/postgresql/postgresql-16-main.log` です。

## 取得から集計まで

```bash
./bin/isu pglog isu4
```

DBホストを省略すると `isu.conf` で最初に `db` の役割を持つホストを選びます。
DBのログprefix、ログ時刻のタイムゾーン、duration閾値、出力形式を読み取り、現在のログをgzipでローカルへ取得してHTMLとTEXTを生成します。
取得開始時のログサイズまでを対象にし、取得中にrenameされても開いたファイルを読み続けます。取得中のtruncateを検出した場合は失敗として再取得を求めます。

出力は `.local/pgbadger/isu4-<取得日時UTC>.<一意文字列>/` です。

| ファイル | 内容 |
|---|---|
| `postgresql.log.gz` | 取得したログ |
| `log-settings.txt` | 取得時のprefix・タイムゾーン・閾値・形式（順に4行） |
| `source.txt` | 取得元ホスト・ログパス・取得完了日時UTC |
| `snapshot-info.txt` | 取得時の診断。容量指定時は取得境界と保持レコード数も記録（SQL・bind値なし） |
| `report.html` | ブラウザーで確認する集計レポート |
| `report.txt` | ターミナルやエージェントで読む集計レポート |

コマンド末尾にHTML・TEXT・入力ディレクトリの絶対パスを表示します。`report.html` をブラウザーで開いてください。
重いSQLを合計時間順、実行回数順、平均時間順に確認できます。TEXTにも同じ集計の主要項目が出ます（上位50件）。

## ベンチの時間帯に絞る

```bash
./bin/isu pglog isu4 \
  --begin '2026-10-31 01:45:00' \
  --end '2026-10-31 01:48:00'
```

`--begin` / `--end` は **ログに記録されたタイムゾーン** の日時です。現在の `log_timezone` は `Etc/UTC` なので、上の例は日本時間10:45〜10:48です。時刻変換は自動では行いません。
指定範囲の解析はローカルで行い、転送するログ自体は現在のファイル全体です。ベンチ予約・実行・ログ初期化はこのコマンドでは行いません。

**`--begin` / `--end` を指定しても、SSHで転送するログ量は減りません。** 全クエリ記録中は時間経過でログが増えるため、ベンチ終了後に対象時間帯を指定して1回取得し、以後の期間変更・再解析には次節の `--input-dir` を使ってください。同じログを毎回取得し直す必要はありません。
終了直後に取得すれば、後から増えた別作業のログを余分に転送せずに済みますが、それ以前に蓄積したログは引き続き転送対象です。

### ログが大きい場合：直近の容量を指定して取得

```bash
./bin/isu pglog isu4 --tail-mib 128 \
  --begin '2026-10-31 07:01:34' --end '2026-10-31 07:03:01'
```

`--tail-mib N` は取得開始時のファイルサイズを固定し、末尾N MiBだけを読みます（1〜4096）。このオプションのみ、**DBホスト側のPython 3**と、現環境のUTC stderr prefixを必要とします。SQL本文・bind値を含むgzipは従来どおり所有者限定のローカル出力先へ保存します。サーバーへのファイル作成や設定変更はありません。

- 先頭の途中レコードを捨て、次の時刻・PIDヘッダーからSQL継続行とDETAILをそのまま保持します。日付行のgrepを使いません。
- 末尾1レコードは追記途中か判別できないため、常に除外します。16 MiB超の単一レコードはメモリ使用を抑えるため取得失敗にします。
- `--begin` より前の保持レコードと、`--end` の**次の秒以降**の保持レコードを必要とします。足りなければ終了コード1とし、途中gzipを削除して集計しません。開始不足は容量を増やすかローテーション済みログを別途確認、終了不足は終了後のログが揃ってから再取得してください。
- `snapshot-info.txt` に最初・最後の保持レコード時刻、読み始めたbyte位置、元ファイルサイズを記録します。時刻を指定しない場合は直近の部分取得であり、ベンチ全体を含む保証はありません。
- renameローテーション後も同じファイル記述子を読みます。短い読み取りや終了時サイズの縮小は失敗にします。truncate後に元サイズまで増えた場合や元ログ自体の欠落を検出するものではありません。
- 区間の境界検査はログが時刻順に記録される前提です。ローテーション済みファイルは含めず、解析器の `complete` と同様に全イベントの収集を証明するものではありません。

取得後は `--input-dir` または `pgfingerprint --input .../postgresql.log.gz` でローカル再解析できます。`--tail-mib` と `--input-dir` は併用しません。既定の全量取得は変更しません。

## 取得済みログを再集計する（SSH不要）

```bash
./bin/isu pglog isu4 \
  --input-dir .local/pgbadger/取得済みディレクトリ \
  --begin '2026-10-31 01:45:00' \
  --end '2026-10-31 01:48:00' \
  --output-dir .local/pgbadger/job-1557
```

`--input-dir` 内のログと取得時の設定を使うため、サーバーへは接続しません。
出力先には新しいディレクトリを指定します。既存レポートは上書きしません。
pgBadgerのPIDファイル・作業ファイルも出力先ごとに分離するため、異なる出力先への集計は同時に実行できます。
ログ取得後に解析だけが失敗した場合も、取得済みディレクトリを `--input-dir` に指定して再試行できます。

## 注意点と確認方法

- このCLIはstderr形式の非圧縮PostgreSQLログを取得する構成です。元ログパスは `isu.conf` の `ISU_PG_LOG_PATH` で変更できます。
- 自動取得するのは現在の1ファイルだけです。ローテーション済みファイルは含めません。対象期間が残っているうちに取得してください。
- クエリが0件のレポートは正常完了扱いにせず、終了コード1で知らせます。時刻範囲、durationログの有効化、ログ形式を確認してください。
- `log_min_duration_statement` が0より大きい場合は、その閾値を超えたクエリのみが集計対象です。`-1` はdurationログ無効です。
- 生ログやレポートにはSQLの値や画像データ等が含まれ得ます。`.local/` はGit管理外のまま使い、チーム外へ公開しないでください。生成ファイルは所有者のみ読み書きできる権限で作成します。
- ログ取得・解析ではサービス再起動、ログ削除、DBデータ変更は行いません。

公式のオプション・レポート仕様は [pgBadgerドキュメント](https://pgbadger.darold.net/documentation.html)、固定版は [v13.2](https://github.com/darold/pgbadger/releases/tag/v13.2) を参照してください。

## 空パターンを残さないSQL指紋集計（Issue #73）

`pgfingerprint` はPython 3.9以上の標準ライブラリのみで動き、**取得済みファイルだけ**を読みます。SSH・DB接続・設定変更・ログ削除はしません。既存のpgBadgerと独立して、durationイベントの総数/総時間とSQLへ帰属できた量を照合するための補助です。

```bash
mkdir -p .local/pg-analysis
umask 077
./bin/isu pgfingerprint \
  --input .local/pg-analysis/pg-slice-u.txt \
  --begin '2026-10-31 06:08:03' --end '2026-10-31 06:09:34' \
  > .local/pg-analysis/job-20957.json
```

入力は現環境の `%m [%p] %q%u@%d ` prefix、UTC、英語メッセージのstderrログです。通常ファイルとgzip（`.gz`）に対応します。`standard_conforming_strings=on` を前提とし、その設定変更を含む入力は拒否します。上記区間はJob全体であり、負荷フェーズのみとは断定しません。`--begin/--end` は秒単位・両端を含み、指定秒の小数部分も含みます。省略するとファイル全体を対象にします。

**元ログはSQLの継続行を残してください。** 日付で始まる行だけをgrepしたファイルは、`execute ...:` の次行以降にあるSQLを失っています。このツールでも失われたSQLは復元できません。元ログ全体をローカルへ取得し、期間選択はツールに任せるか、各ログレコードの継続行を保持した既存sliceを使います。運用中サーバーでの解析や新たな採取は反映担当と調整し、ローカル再解析を優先します。

JSONの主要項目:

| 項目 | 意味 |
|---|---|
| `total_count/total_ms` | 対象durationイベントの件数・合計経過時間。parse/bindも含み、HTTP件数・CPU秒ではない |
| `by_kind` | statement、execute、parse、bind、SQLなしdurationを分離。pgBadgerのqueriesは主にstatement+execute等で、parse/bindは件数に含まれない |
| `first_sql_line_blank/recovered_from_continuations` | 先頭行SQLが空だった件数と、継続行から復元できた件数 |
| `fingerprint_count/fingerprint_total_ms` | SQL指紋へ帰属した全イベント（上位N件だけの合計ではない） |
| `unattributed_*` | SQL欠落・不完全/未対応構文の件数、時間、理由。値を推測して他のSQLへ割り当てない |
| `unrecognized_lines` | 対応prefix/継続行として扱えなかった非空行。形式不一致や切り出し不備を確認する |
| `top_queries` | 合計時間降順のSQL指紋・件数・avg/max・kind別時間。既定50件、`--top N`で変更 |
| `buckets_10s` | 10秒ごとのduration件数/合計。初期化・負荷・後処理との照合用 |
| `complete` | 取得した対象イベントを全て帰属できたか。ログ自体の取りこぼしや、非空SQLの途中切れまで証明する値ではない |

SQL文字列、E/B/X/N/U文字列、ドル引用、数値、bind変数、コメントを除き、引用された識別子も隠します。ユーザー名・DB名・PID・パラメータDETAIL・エラー本文は出力しません。引用識別子を隠すため、その識別子だけが違うSQLは同じ指紋になります。未対応/閉じていないSQLは生テキストを出さず未帰属にします。pgxのコメントだけのpingは、内容を捨てて専用の指紋へ帰属させます。非引用識別子はテーブル/列の判別用に残るため、**出力は公開前に確認**してください。生ログやpgBadgerレポートのサンプルSQLをGit/Discordへ貼る代用にはしません。

未帰属・未認識行・対象0件の場合もJSONは出しますが、終了コード2になります。`complete=false` を無視して「SQL全部を分析済み」と扱わないでください。時刻/文字コード/ファイル自体が非対応なら集計を出さず終了コード2です。

pgBadgerとの比較では、`total_ms`をprepare+bind+executeの合計と照合し、件数は`by_kind`を分けます。ローカルPG16/pgxで実ログを生成した検証では、14 durationイベント（parse 2、bind 4、execute 4、statement 4）とpgBadgerの8 queriesを、この段階の違いで説明できました。先頭SQLが空の11イベントを全て復元し、合計時間は両者で一致しました。SQL継続行を削ったログは未帰属として失敗します。

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests -p 'test_pgfingerprint.py' -v
```
