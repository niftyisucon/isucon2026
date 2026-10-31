# 4台にSplunk監視を導入する

この手順はレビュー後に人間が実行します。PR作成時点では競技サーバーに未反映です。
既存のSplunk Observability Cloud（realm `au0`）に、4台のホストメトリクス、Goアプリ3台のHTTP・SQLトレース、isu4のPostgreSQL統計を送ります。

最初はisu1だけで導入と表示を確認し、その後isu2・isu3・isu4へ展開します。各コードブロックは1コマンドです。コマンドはリポジトリルートで実行します。

## 作業の前提を確認する

- ローカルにBash、SSH、GNU rsync 3系、Go 1.26.5以降が必要です。
- サーバーにはUbuntu 24.04 amd64、パスワード不要のsudo、GitHubへのHTTPS接続が必要です。
- SplunkのAPMが利用可能であることは人間に確認済みです。Ingestトークンの値は人間が保持します。
- `isu4` は共有PostgreSQLサーバーで、従来のSSH名 `isu-db` と同じ接続先です。
- `-f` は現在のowner未割当とPRブランチでの確認に対応する指定です。owner・mainのチェックを省略するため、対象ホストを人間が確認して使います。ロックは省略しません。
- `splunk-tracing enable` は、このcheckoutからビルドしたGoバイナリを反映します。他PRを反映済みのサーバーで試す場合は、その変更を含むブランチで実行します。

ブランチを取得します。

```bash
git fetch origin
```

```bash
git switch codex/splunk-observability
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
bash tools/agent-log/alog.sh post '「着手」人間の操作でSplunk監視を導入します。まずisu1で確認し、その後isu2・isu3・isu4に展開します。Goトレース有効化時は対象アプリを再起動します。'
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

isu4の監視用DBロールを作ります。この部分は人間が実行するDB変更です。アプリ用ユーザーのパスワードや `pg_hba.conf` は変更しません。

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

## isu1でGoトレースを有効にする

```bash
./bin/isu splunk-tracing enable isu1 -f
```

このコマンドはローカルビルド後、Goバイナリとトレース用systemd drop-inだけを転送し、`isureal-go` だけを再起動します。既存のnginx・PostgreSQL設定や他PRで追加されたログ設定を同期しません。Collectorのhealth checkに失敗した場合はバイナリの転送前に停止します。

```bash
./bin/isu ssh isu1 'curl --silent --show-error --output /dev/null --write-out "%{http_code}\n" -H "traceparent: 00-1234567890abcdef1234567890abcdef-1234567890abcdef-01" http://127.0.0.1:8080/api/me'
```

未認証のため `401` が返ることを確認します。この確認リクエストだけは `traceparent` のsampledフラグで記録対象にします。Splunk APMでは環境 `isucon2026`、サービス `isureal`、属性 `isureal.region=asia` を探します。確認用Trace IDは `1234567890abcdef1234567890abcdef` です。
通常の画面操作や予約済みベンチの通信からSQLを含むトレースも到着することを確認します。通常のサンプリングは1%なので、単発リクエストでは記録されない場合があります。

HTTPスパンは `GET /api/users/:userId` のようなルート名です。配下のPostgreSQLスパンでSQLの所要時間と回数を確認します。
SQLは本文のSHA-256先頭16桁で識別します。同じSQL文字列は同じ値になります。PostgreSQLの `queryid` とは別の識別子です。
URL内の実ID、クエリ文字列、HTTPヘッダー、本文、SQL本文・引数、DBエラー詳細はトレースに含めません。

```bash
./bin/isu splunk-tracing enable isu2 -f
```

```bash
./bin/isu splunk-tracing enable isu3 -f
```

isu2は `isureal.region=europe`、isu3は `isureal.region=america` で区別します。

## ベンチで監視の負荷を確認する

```bash
bash tools/agent-log/alog.sh read
```

未完了の予約がなければ、人間がベンチを予約します。

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

```bash
./bin/isu splunk-tracing disable isu1 -f
```

```bash
./bin/isu splunk-tracing disable isu2 -f
```

```bash
./bin/isu splunk-tracing disable isu3 -f
```

トレース用drop-inを削除して対象Goアプリを再起動します。Goコードはトレース無効の経路に戻り、ホスト・DBメトリクスは継続します。
ホスト監視も止める場合は、対象ホストのロックを取得し、人間がCollectorを停止します。isu1の例です。

```bash
./bin/isu lock isu1 'Splunk Collectorの停止'
```

```bash
./bin/isu ssh isu1 'sudo systemctl disable --now splunk-otel-collector'
```

```bash
./bin/isu unlock isu1
```

## 設定と確認範囲

メトリクスは10秒間隔です。Collectorはメモリlimiter 192 MiB、systemd上限384 MiB、低いCPU優先度で動かします。Goの送信キューは2048スパンで、満杯時はリクエスト処理を待たせず破棄します。サンプル外のエラーや遅いリクエストが必ず残る構成ではありません。

PostgreSQLは統計を読み取ります。`pg_stat_statements` の有効化、DB再起動、SQLサンプルの送信は行いません。GoのCPUを消費する関数の特定には、別途pprofを使います。
OTLP受信、health check、内部メトリクスはすべて `127.0.0.1` で待ち受けます。公開ポートは追加しません。

ローカルでGoのテスト・静的解析、実際のpgxクエリの結果と親スパンとの関連付け、Collector 0.164.0による設定検証、PostgreSQL 16でのpeer認証・メトリクス収集を確認済みです。実サーバーでのインストール、トークン投入、Splunkへの到着、競技ベンチは人間の確認が必要です。

配布元と設定仕様は、[Collector 0.164.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.164.0)、[PostgreSQL receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.164.0/receiver/postgresqlreceiver)、[OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go)を参照してください。
