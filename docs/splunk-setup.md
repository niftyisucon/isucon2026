# 4台にSplunk監視を導入する

サーバー上のビルド・反映・再起動、マージ、ベンチEnqueueは `komatsu-muse` が一元担当します。他のエージェント・人間は直接実行せず、`agent-log` で依頼してください。下記のサーバー変更コマンドは担当者向けの手順です。人間は秘密値の入力とSplunk画面確認を担当者と調整して行います。ローカルの実装・設定変更はPRとして提出し、レビューは `michito-claudecode` に依頼します。

停止指示や未完了のベンチ予約がある間は、導入・トークン更新・ビルド・反映・再起動を行いません。再開には、人間による全台のスクリプト停止確認と `komatsu-muse` の合意が必要です。

旧 `splunk-tracing` は廃止しました。旧版 `330992a` のコマンドはGoバイナリの反映とアプリ再起動を含むため、実行しないでください。修正版は引数や `-f` にかかわらず、サーバー操作をせずエラーで終了します。
既存のSplunk Observability Cloud（realm `au0`）に、4台のホストメトリクス、Goアプリ3台のHTTP・SQLトレース、isu4のPostgreSQL統計を送ります。

Collectorの初回導入は、担当者がisu1で表示を確認してから残りへ展開します。各コードブロックは1コマンドです。コマンドはリポジトリルートで実行します。

## 作業の前提を確認する

- 開発端末にBash、SSH、GNU rsync 3系が必要です。GoアプリのビルドはGo 1.26.5以降を備えた、競技4台以外の開発端末で行います。
- サーバーにはUbuntu 24.04 amd64、パスワード不要のsudo、GitHubへのHTTPS接続が必要です。
- SplunkのAPMが利用可能であることは人間に確認済みです。Ingestトークンの値は人間が保持します。
- `isu4` は共有PostgreSQLサーバーで、従来のSSH名 `isu-db` と同じ接続先です。
- `-f` はowner・mainのチェックを省略します。ベンチとの調整や作業停止指示は省略できません。ロックも省略しません。
- アプリの反映は `komatsu-muse` が通常のデプロイ手順で行い、対象3台のコミットとトレース設定を揃えます。Splunk導入コマンドはGoバイナリを反映しません。

安全修正が含まれる最新mainを取得します。未マージの旧PRブランチは使用しません。

```bash
git fetch origin
```

```bash
git switch main
```

```bash
git merge --ff-only origin/main
```

```bash
git status --short
```

意図しない変更がないことを確認します。個人用のSSH設定を指定します。

```bash
export ISU_SSH_CONFIG="$HOME/.ssh/config"
```

SSH設定に `isu4` がなければ、`isu-db` と同じIP・ユーザー・鍵が適用されるように追加します。共通の `Host` グループにも `isu4` を含めます。秘密鍵の内容はコピーしません。

```bash
./bin/isu ssh isu4 'hostname'
```

`ip-192-168-4-10` が返ることを確認します。Discordの最新状況を読み、未完了の作業やベンチ予約と衝突しないことを確認します。

```bash
bash tools/agent-log/alog.sh read
```

```bash
bash tools/agent-log/alog.sh post '「着手」komatsu-muse担当の作業時間内にSplunk Collectorを導入します。まずisu1で確認し、その後isu2・isu3・isu4に展開します。'
```

## isu1にCollectorを入れる

```bash
./bin/isu splunk-setup isu1 -f
```

公式のCollector 0.164.0パッケージを、`isu.conf` に固定したSHA-256と照合してインストールします。異なるバージョンが導入済みの場合は停止します。
初回はCollectorを起動せず、設定の検証まで行います。nginx、Goアプリ、PostgreSQLは再起動しません。既存Collectorに対する再実行では設定を更新しますが、再起動は次のトークン入力時です。

人間が対話入力でIngestトークンを投入します。値をコマンド引数、Git、チャット、agent-logに書かないでください。

```bash
./bin/isu splunk-token isu1 -f
```

入力中の文字は表示されません。トークンはroot所有・0600の `/etc/otel/collector/splunk-otel-collector.conf` に保存され、Collectorを起動します。成功時は `active` が返ります。

```bash
./bin/isu ssh isu1 'curl --fail --silent --show-error http://127.0.0.1:13133/'
```

[Splunk](https://app.au0.signalfx.com) のInfrastructureで `host.name=ip-192-168-1-10` を探します。CPU、メモリ、ロード、ディスクI/O、ネットワークが更新されることを確認します。health checkの成功だけではSplunkへの送信成功は確認できません。

## 残り3台にCollectorを入れる

```bash
./bin/isu splunk-setup isu2 isu3 isu4 -f
```

isu4の監視用DBロールを作ります。このDB変更も `komatsu-muse` に依頼します。アプリ用ユーザーのパスワードや `pg_hba.conf` は変更しません。

```bash
./bin/isu lock isu4 'Splunk PostgreSQL監視ロールの作成'
```

```bash
./bin/isu ssh isu4 'sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -f /opt/isucon-observability/config/splunk/postgresql-role.sql'
```

```bash
./bin/isu ssh isu4 'sudo -u splunk-otel-collector psql -h /var/run/postgresql -U splunk-otel-collector -d isureal -c "SELECT current_user"'
```

`splunk-otel-collector` が返ることを確認します。OSユーザーとDBロールの同名peer認証を使用します。Collector設定内の `unused-peer-auth` はreceiverが要求する非秘密の固定値で、peer認証では使われません。

```bash
./bin/isu unlock isu4
```

失敗した場合もロックを解放し、原因を確認してから再実行します。ロール作成SQLは再実行できます。

```bash
./bin/isu splunk-token isu2 -f
```

```bash
./bin/isu splunk-token isu3 -f
```

```bash
./bin/isu splunk-token isu4 -f
```

Infrastructureに4ホストが表示されることを確認します。isu4では `postgresql.backends`、`postgresql.commits`、`postgresql.db_size` などのDBメトリクスも確認します。DBロールに与える権限はLOGIN、`pg_monitor`、`isureal` へのCONNECTです。アプリテーブルの書き込み権限は与えません。

## Goトレースの設定をPRで準備する

ここからの設定編集も、競技4台以外の開発端末で行います。通常の反映対象には、3台の `30-isucon-tracing.conf` が含まれます。現在の運用報告と同じ、トレース有効・1%サンプリングを管理対象にしています。無効化が必要な場合も、このファイルをPRで変更します。

最新mainを含むことを確認します。この確認が失敗した場合は更新を担当者と調整し、古いブランチを反映しません。

```bash
git fetch origin
```

```bash
git merge-base --is-ancestor origin/main HEAD
```

設定変更用のブランチを作ります。

```bash
git switch -c ops/enable-go-tracing
```

```bash
cp config/splunk/tracing.conf hosts/isu1/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
```

```bash
cp config/splunk/tracing.conf hosts/isu2/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
```

```bash
cp config/splunk/tracing.conf hosts/isu3/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
```

```bash
git add -- hosts/isu{1,2,3}/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
```

```bash
git diff --cached
```

```bash
git commit -m '3台のGoトレースを有効にする'
```

```bash
git push -u origin HEAD
```

GitHubでmain向けPRを作成し、レビューを受けます。この段階ではサーバーの設定・バイナリ・プロセスは変更されません。

反映は、人間が全台のスクリプト停止を明示確認し、`komatsu-muse` が作業時間を承認してから行います。`komatsu-muse` が最新の改善を含む同一コミットを3台へ通常のデプロイ手順で反映し、バイナリとトレース設定を確認します。ベンチ中には反映しません。ホストロックの取得成功だけでは、ベンチが停止している証明にはなりません。

## 反映後にGoトレースを確認する

`komatsu-muse` から反映完了の共有を受けてから、確認用リクエストを送ります。

```bash
./bin/isu ssh isu1 'curl --silent --show-error --output /dev/null --write-out "%{http_code}\n" -H "traceparent: 00-1234567890abcdef1234567890abcdef-1234567890abcdef-01" http://127.0.0.1:8080/api/me'
```

未認証のため `401` が返ることを確認します。この確認リクエストだけは `traceparent` のsampledフラグで記録対象にします。Splunk APMでは環境 `isucon2026`、サービス `isureal`、属性 `isureal.region=asia` を探します。確認用Trace IDは `1234567890abcdef1234567890abcdef` です。
通常の画面操作や予約済みベンチの通信からSQLを含むトレースも到着することを確認します。通常のサンプリングは1%なので、単発リクエストでは記録されない場合があります。

HTTPスパンは `GET /api/users/:userId` のようなルート名です。配下のPostgreSQLスパンでSQLの所要時間と回数を確認します。
SQLは本文のSHA-256先頭16桁で識別します。同じSQL文字列は同じ値になります。PostgreSQLの `queryid` とは別の識別子です。
URL内の実ID、クエリ文字列、HTTPヘッダー、本文、SQL本文・引数、DBエラー詳細はトレースに含めません。

isu2は `isureal.region=europe`、isu3は `isureal.region=america` で区別します。

## ベンチで監視の負荷を確認する

```bash
bash tools/agent-log/alog.sh read
```

`komatsu-muse` が、人間による全台スクリプト停止確認と3台のコード・設定の一致を確認してから予約します。実行中の操作がある、または状態が不明な間は再開しません。

```bash
bash tools/agent-log/alog.sh post '「ベンチ予約」Splunk監視・Goトレース有効化後の正当性とスコアをポータルで確認します。'
```

[ベンチの手順](bench-howto.md)に従ってポータルから実行します。同じコード・負荷条件でトレース無効時と有効時を比較し、スコア、エラー率、CollectorのCPU・メモリ、送信失敗・欠落、Splunk上の表示を確認します。監視によるスコアへの影響はPR作成時点で未計測です。

```bash
./bin/isu ssh isu1 'systemctl show splunk-otel-collector -p MemoryCurrent -p CPUUsageNSec'
```

```bash
./bin/isu ssh isu1 'curl --fail --silent --show-error http://127.0.0.1:8888/metrics'
```

内部メトリクスの `otelcol_exporter_send_failed_*`、`otelcol_exporter_enqueue_failed_*`、`otelcol_processor_refused_*` が増え続けていないことを確認します。カウンターが存在しない場合は0回とは断定せず、Splunk側への到着も確認します。
結果の値を置き換えて投稿します。

```bash
bash tools/agent-log/alog.sh post '「ベンチ結果」score=<スコア> / Splunk監視あり・Goトレース1%。エラー=<結果>、Collector負荷=<結果>。'
```

```bash
bash tools/agent-log/alog.sh post '「完了」4台のホスト・PostgreSQL統計とGo HTTP/SQLトレースの導入・表示確認を完了しました。'
```

## トレースを無効にする

有効化と同じく、最新mainから設定変更PRを作成します。設定ファイルの削除ではなく、明示的な `false` へ置き換えます。ローカルファイルを削除するだけでは、通常のdeployが同期をスキップし、サーバー側に有効設定が残るためです。

```bash
cp config/splunk/tracing-disabled.conf hosts/isu1/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
```

```bash
cp config/splunk/tracing-disabled.conf hosts/isu2/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
```

```bash
cp config/splunk/tracing-disabled.conf hosts/isu3/etc/systemd/system/isureal-go.service.d/30-isucon-tracing.conf
```

有効化と同様に差分を確認してPRを作成し、`komatsu-muse` が作業時間を調整して通常のデプロイで反映します。ホスト・DBメトリクスは継続します。
ホスト監視も止める場合は `komatsu-muse` に依頼します。担当者が実行するisu1の例です。

```bash
./bin/isu lock isu1 'Splunk Collectorの停止'
```

```bash
./bin/isu ssh isu1 'sudo systemctl disable --now splunk-otel-collector'
```

```bash
./bin/isu unlock isu1
```

## asia1 の CPU と DB プール待ちを切り分ける

実機の採取・反映・ベンチは `komatsu-muse` が担当します。新しい診断口は標準では無効です。
採取する場合は `config/architecture/isu1/50-diagnostics.conf` を共有プロセスの
`isureal-go.service.d/50-diagnostics.conf` として通常の反映手順へ組み込みます。
既存の `40-architecture.conf` と鍵・画像ディレクトリの設定を保持してください。
旧リージョン別プロセスに同じ診断ポートを追加しません。

`ISUREAL_DIAGNOSTICS_ADDR=127.0.0.1:6060` でループバック上にだけ待ち受けます。
ワイルドカード・非ループバックIP・ホスト名は起動時に拒否します。
公開API/nginxへのルート追加はありません。提供するのは次の2つだけです。

- `GET /debug/stats`: PID・時刻・稼働秒数、共用 `sql.DB.Stats()`、GC/割当/実行系の runtime metrics、画像URL署名・JPEG検証キャッシュの件数とhit/miss。JPEG検証は保持バイト数・大きい入力のbypass件数も返します（[評価手順](performance/issue-88-jpeg-validation-cache.md)）。
- `GET /debug/pprof/profile?seconds=30`: 1〜30秒のCPUプロファイル。cmdline・heap・goroutine dump・実行トレースは提供しません。

環境変数、DSN、SQL本文、画像ID、利用者ID、HTTPヘッダー・本文は診断JSONへ含めません。
`WaitCount` と `WaitDuration` はプロセス全体の累積値です。後者のJSON単位は**ナノ秒**です。
APIトレースの外に見える時間をCPU時間とみなさず、負荷前後でこの値の増分を採ります。

担当者が予約したベンチの開始前から終了後まで、端末Aで1秒間隔の統計を取得します。
以下の `job-NEW` を対象Job番号に置き換え、保存ディレクトリを作成してから使います。

```bash
mkdir -p .local/profiles/job-NEW
```

```bash
./bin/isu ssh isu1 'for i in $(seq 1 120); do curl --fail --silent --show-error --max-time 2 http://127.0.0.1:6060/debug/stats || exit; sleep 1; done' > .local/profiles/job-NEW/runtime.jsonl
```

負荷が立ち上がった時間帯に、端末BからCPUを1回採取します。同時に複数のCPUプロファイルを開始しません。
採取開始・終了UTC、Job、反映commit、PID、人数、採取に伴う負荷も記録します。

```bash
./bin/isu ssh isu1 'curl --fail --silent --show-error --max-time 35 "http://127.0.0.1:6060/debug/pprof/profile?seconds=30"' > .local/profiles/job-NEW/cpu.pprof
```

解析はローカルで行います。

```bash
python3 tools/analyze-runtime-stats.py .local/profiles/job-NEW/runtime.jsonl
```

```bash
go tool pprof -top .local/profiles/job-NEW/cpu.pprof
```

プールの `InUse=MaxOpenConnections` と待ちカウンターの増加が続くならプール待ちを疑います。
待ち時間の増分は全接続の合計なので、経過時間を超えることがあります。
WaitCountは待ち開始時、WaitDurationは待ち終了時に更新されるため、短い区間の比をリクエスト平均待ち時間とは呼びません。
PIDが変わった区間やカウンターが戻った区間は解析ツールが拒否します。
GC CPUはGo runtimeの推定CPU秒で、ホスト全体のbusy率と直接同一視しません。

APMには `session.lookup`（SQL呼び出し前のプール取得を含む）、`post.bind`、`image.validate`、`image.persist` を追加しています。
`session.lookup` と配下のSQLとの差にはプール待ちのほかGoの実行待ちやScan等も含まれます。
エッジからcoreへtraceparentを伝播するため、海外経路も同じtraceで追えます。
通知SSEの長いHTTPスパンは接続保持時間で、通知の配信遅延ではありません。

画像URLの再利用は、現在の秘密鍵・画像ID・期限秒の完全一致だけを対象に最大16,384件を保持します。
認可・可視性・画像GETのDB確認は再利用しません。期限を丸めず署名の1,000回反復・出力形式も保持します。
初期化/通知epoch変更時にキャッシュを空にし、hit/miss累積カウンターは採取期間の差分を取れるよう保持します。
ローカルの反復ベンチでは100種類のURLを繰り返し、10,000呼び出しごとに期限秒を変えて、
非キャッシュ約81µs/opに対しキャッシュ約0.86µs/opでした。実機のhit率・CPU削減率・スコア効果は未計測です。
全呼び出しがmissになる別試験では約83〜85→84〜88µs/op、1,058→約1,377 B/opでした。
再利用の少ない負荷には割当コストが増えるため、実機のhit率・GCも併せて比較します。
署名再利用だけを無効にして比較する場合は `ISUREAL_IMAGE_URL_CACHE=false` をプロセス起動前に設定します。
診断口・APM・プール上限30はそのままで比較でき、設定反映と再起動もMuseが担当します。

認証のセッション確認は、`ISUREAL_LISTEN_REGIONS` で複数リージョンを受ける asia1 の共有 core でだけメモリを使います。
isu2・isu3 のSSEなど他のプロセスは、これまでどおり毎回PostgreSQLを確認します。
命中数は `/debug/stats` の `session_cache` で確認できます。
DB確認へ戻して比較する場合は `ISUREAL_SESSION_CACHE=false` をプロセス起動前に設定します。

参照: [DBStats](https://pkg.go.dev/database/sql#DBStats)、[runtime metrics](https://pkg.go.dev/runtime/metrics)、[CPU profile](https://pkg.go.dev/net/http/pprof#Profile)。

## 設定と確認範囲

メトリクスは10秒間隔です。Collectorはメモリlimiter 192 MiB、systemd上限384 MiB、低いCPU優先度で動かします。Goの送信キューは2048スパンで、満杯時はリクエスト処理を待たせず破棄します。サンプル外のエラーや遅いリクエストが必ず残る構成ではありません。

PostgreSQLは統計を読み取ります。`pg_stat_statements` の有効化、DB再起動、SQLサンプルの送信は行いません。GoのCPUを消費する関数の特定には、別途pprofを使います。
OTLP受信、health check、内部メトリクスはすべて `127.0.0.1` で待ち受けます。公開ポートは追加しません。

旧コマンドを実行してもGo・SSH・rsyncを呼ばないことは、次のローカル回帰テストで確認します。

```bash
bash tests/splunk-tracing.sh
```

Goのテスト・静的解析、実際のpgxクエリの結果と親スパンとの関連付け、Collector 0.164.0による設定検証、PostgreSQL 16でのpeer認証・メトリクス収集はローカルで確認済みです。人間が共有した画面で4ホストのメトリクス到着も確認しました。PostgreSQL指標は `0 MTS` で到着未確認、Go APMへの到着も未確認です。ベンチ中のアプリ反映が発生した計測については、ベンチ担当が条件を揃えて再確認します。

配布元と設定仕様は、[Collector 0.164.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.164.0)、[PostgreSQL receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.164.0/receiver/postgresqlreceiver)、[OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go)を参照してください。
