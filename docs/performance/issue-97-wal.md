# WAL サイズとチェックポイントの比較

Issue #97 の候補として `max_wal_size` を 1GB から 4GB に変更する。
WAL の増加によるチェックポイントが減るかを単独で評価する。
`checkpoint_timeout` は5分のままなので、時間によるチェックポイントは減らない。
`fsync`、`synchronous_commit`、`full_page_writes` は変更しない。

isu4 の `pg_stat_bgwriter` は 2026-10-30 11:59 UTC のリセット以降、
10-31 07:23 UTC 時点で timed 72 回、requested 39 回だった。
これは累計であり、ベンチ中に WAL が原因で発生した回数や性能への影響は不明。
同時刻の空き領域は約23GBだった。`max_wal_size` は上限保証ではなく、
長時間の処理やアーカイブなどの状況で WAL は設定値を超え得る。

## 適用と比較（komatsu-muse 担当）

1. ベンチ外に、同じアプリ構成で変更前のベンチと下記の統計を記録する。
2. マージ済み main から `./bin/isu deploy isu4` を実行する。CLI は DB を再起動する。
3. 次の SQL で設定値、上書き、エラーを確認する。

```sql
SELECT name, setting, unit, sourcefile, pending_restart
FROM pg_settings WHERE name IN
  ('max_wal_size', 'checkpoint_timeout', 'fsync', 'synchronous_commit', 'full_page_writes');
SELECT sourcefile, sourceline, name, setting, error
FROM pg_file_settings WHERE error IS NOT NULL;
SELECT now(), stats_reset, checkpoints_timed, checkpoints_req,
       checkpoint_write_time, checkpoint_sync_time, buffers_checkpoint
FROM pg_stat_bgwriter;
```

`max_wal_size` が 4096 MB で sourcefile が `91-isu-wal.conf`、
`pending_restart` が false、設定エラーがないことを確認する。
後続の設定ファイルや `postgresql.auto.conf` が上書きした場合は、適用済みと扱わない。

4. 同じアプリ構成でベンチし、Passed、初期化時間、スコア、地域別人数、
   DB の CPU・I/O wait、ディスク空き容量を比較する。統計はベンチ前後の差を使い、
   `stats_reset` が変化した場合は比較しない。再起動後のキャッシュ状態にも注意する。
   要求チェックポイントには明示的な CHECKPOINT なども含まれる。
5. 効果が確認できない場合や空き容量が不足する場合は、設定ファイルを削除する PR を反映し、
   `max_wal_size` が従来の 1024 MB に戻ったことを確認する。WAL ファイルは手動削除しない。

競技サーバーへの適用とベンチは未実施。スコア改善は未確認。
