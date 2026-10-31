# 共有coreのDBプール容量比較（Issue #75）

`ISUREAL_CORE_DB_MAX_OPEN` で共有coreのMaxOpenConnectionsを **30 / 45 / 60** から選べます。未設定は従来の30です。idle上限は10のままにして、一度の比較で変える条件を容量だけに絞ります。

適用対象は `ISUREAL_EDGE_UPSTREAM` が空、かつ `ISUREAL_LISTEN_REGIONS` が複数リージョンを指定するプロセスです。edgeと旧構成の単一リージョンプロセスは、環境変数が渡されても30のままです。対象coreに上記以外の値を指定すると起動時にエラーになります。無制限設定はありません。

## 根拠と比較の順序

Job #24740の共有coreでは、92サンプル中40点でInUse=30、WaitCount +31,162、WaitDuration +5,839.6秒でした。後者は並列待ちの累積で、壁時計時間ではありません。同じベンチで署名/JPEGのCPU消費も大きく、プール容量だけが原因とは確定していません。

#91の署名改善後はJob #26419でPassed/47,155点との担当者報告がありますが、同区間のプール統計は未取得です。#86、#94等の処理削減でも状況が変わり得ます。まず採用するアプリの版を固定し、その版で30の基準値を取ってから45、必要なら60を比較します。既定値は自動で増やしません。

## 接続予算

現行の1共有core＋2 edge、各プロセス1 LISTEN接続という構成での上限見積もりです。LISTENはsql.DBプールとは別です。

| 共有coreの上限 | edge 2台の上限 | LISTEN 3本 | アプリ全体の上限見積もり |
|---:|---:|---:|---:|
| 30 | 60 | 3 | 93 |
| 45 | 60 | 3 | 108 |
| 60 | 60 | 3 | 123 |

isu4の既知のmax_connectionsは200です。監視・管理・予約接続等は上表に含みません。旧regionalサービスが残っている場合やプロセス数が違う場合は再計算します。接続数に余裕があっても、DB CPU、availableメモリ、swap、I/O、work_memを同時に見る必要があります。work_memはクエリの演算ごとに重複し、接続あたり一定量の予約ではありません。DB設定・永続化設定は変更しません。

## 実機比較（komatsu-muse担当、ベンチ外で設定反映）

1. 対象バイナリ・コミット・PIDと、1 shared core＋2 edgeの構成を確認します。#25844ではコードの検索SQLと保存ログのSQLに食い違いがあり、checkoutのHEADだけを反映証明にしません。
2. 同じコードで30の基準ベンチを取り、coreの `/debug/stats` を前後または1秒ごとに取得します。診断を使うなら30/45/60の両条件で同じ診断設定にします。環境変数全体・認証情報を出力する必要はありません。
3. `config/architecture/isu1/60-core-pool.conf` は **45の任意適用テンプレート** です。`config/` は通常deployの自動同期対象ではありません。Museが既存architecture設定と同じ管理された配布手順で、共有coreサービスの `/etc/systemd/system/isureal-go.service.d/60-core-pool.conf` に適用し、daemon-reloadとcore再起動を行います。edgeの再起動や設定変更は不要です。
4. 反映後、診断JSONの `db.MaxOpenConnections=45` と新PIDを確認します。環境ファイルに同名変数がある場合は設定が上書きされる可能性があるため、ファイル内容だけで有効値を判断しません。
5. 同じアプリ版で単独ベンチを行い、Passed、スコア、人数/GP、対象API平均・p95、DBStats差分、core/DB CPU・DBメモリ・I/Oを記録します。60を試す場合も同様です。地域偏り・人数・CPUプロファイル採取の有無が違う走行のスコア差を容量の効果だけに帰属させません。

確認用の読み取り例（診断listenerを有効にしている場合）:

```bash
./bin/isu ssh isu1 'curl --fail --silent http://127.0.0.1:6060/debug/stats'
```

記録する値は `pid`, `time`, `db.MaxOpenConnections`, `db.OpenConnections`, `db.InUse`, `db.Idle`, `db.WaitCount`, `db.WaitDuration` です。待機カウンタは同じPID内で前後差分を取り、再起動をまたぐ差を計算しません。

## 戻し方

同じdrop-inで `ISUREAL_CORE_DB_MAX_OPEN=30` にして、ベンチ外でdaemon-reload・core再起動します。テンプレートのローカルファイルを消しただけでは実サーバーの設定は消えません。診断JSONで新PIDとMaxOpenConnections=30を確認します。データ移行やキャッシュ形式の変更はありません。

## ローカル検証

設定選択・無効値拒否のテストに加え、一時PostgreSQL16 DBで実バイナリを起動して、3リージョン共有coreの未設定30 → 45 → 60 → 明示30を `/debug/stats` で確認しました。各coreの3ポートが起動し、認証なしAPIは401のままです。60の設定を渡したedgeと旧単一リージョンプロセスは30でした。各プロセスはSIGTERMで正常終了しました。

これは設定反映とロールバックの検証であり、容量増加の実機性能を証明する負荷試験ではありません。
