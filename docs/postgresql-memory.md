# PostgreSQL メモリ設定（Issue #33）

`hosts/isu4/etc/postgresql/16/main/conf.d/90-isu-memory.conf` で、
`shared_buffers` を 128MB → 1GB、`effective_cache_size` を 4GB → 2GB にする。
`postgresql.conf` の `include_dir = 'conf.d'` と既存の `/etc/postgresql` 同期で反映される。

保存済み `hosts/isu4/discovery.md` の DB ホストは 3.7GiB・2 vCPU。
1GB は RAM の約27%で、PostgreSQL 16 公式の専用DB向けの開始目安（25%）に近い。
残り約2.7GiBを OS・バックエンド・作業メモリ・監視プロセス等で共有する。
2GB は共有バッファと OS キャッシュを合わせたプランナー向けの見積もりで、
追加で2GBを確保する設定ではない。

- [shared_buffers / work_mem の公式説明](https://www.postgresql.org/docs/16/runtime-config-resource.html#RUNTIME-CONFIG-RESOURCE-MEMORY)
- [effective_cache_size の公式説明](https://www.postgresql.org/docs/16/runtime-config-query.html#GUC-EFFECTIVE-CACHE-SIZE)

Issue 記載の累計キャッシュヒット率83.2%は計測区間の差分ではなく、
OS キャッシュの命中も表さない。メモリ容量不足の証拠としては扱わない。
共有バッファの増量で頻出ページの再読み込みを減らす狙いだが、スコア改善は未実測。
画像全体は RAM より大きく、全件をキャッシュすることはできない。

`work_mem` は各演算・接続で消費が重なるため既定4MBを維持する。
`random_page_cost` は実行計画への影響を別途比較するため変更しない。
`synchronous_commit`・`fsync`・`full_page_writes` は変更せず、永続化を維持する。
ログ設定は既存の99-isu-log.confとその担当PRで管理する。

## 適用と比較（komatsu-muse 担当、ベンチ外のみ）

1. 同じアプリ構成で変更前のベンチとDB統計を記録する。他PRと分けて比較する。
2. マージ済み main から owner 等の通常条件を満たして `./bin/isu deploy isu4` を実行する。
   `/etc/postgresql` の変更により CLI が DB を再起動する。reload だけでは反映されない。
3. `./bin/isu ssh isu4` 経由で `sudo -u postgres psql -d isureal` を開き、以下を確認する。

```sql
SELECT name, setting, unit, sourcefile, pending_restart
FROM pg_settings
WHERE name IN ('shared_buffers', 'effective_cache_size', 'work_mem',
               'random_page_cost', 'synchronous_commit', 'fsync', 'full_page_writes');
SELECT sourcefile, sourceline, name, setting, error
FROM pg_file_settings WHERE error IS NOT NULL;
```

期待値は shared_buffers=131072（8kB）、effective_cache_size=262144（8kB）、
work_mem=4096（kB）、random_page_cost=4、永続化3項目=on。
メモリ2項目の sourcefile が90-isu-memory.conf、pending_restart=false、設定エラー0件を確認する。
`postgresql.auto.conf` や後続ファイルに上書きがあれば、反映済みと扱わず担当者が調整する。

4. 同じアプリ構成でベンチを実行し、初期化60秒以内・整合性検証の成功、スコア、
   DB CPU、availableメモリ・swap・OOM、ディスクI/O、チェックポイントを比較する。
   再起動直後は共有キャッシュが冷えるため、必要に応じて同条件で再計測する。
5. 統計は各ベンチ前後の以下の値の差を取る（共有統計のリセットは不要）。

```sql
SELECT now(), stats_reset, blks_hit, blks_read, temp_files, temp_bytes
FROM pg_stat_database WHERE datname = 'isureal';
SELECT now(), checkpoints_timed, checkpoints_req, buffers_checkpoint,
       buffers_clean, buffers_backend FROM pg_stat_bgwriter;
```

`Δblks_hit / (Δblks_hit + Δblks_read)` で区間の命中率を比較する。
分母0・stats_reset変化時は比較しない。readはOSキャッシュ経由も含むため実ディスクI/Oと併せる。
checkpoint増加やメモリ逼迫、再現するスコア低下があれば、この設定ファイルを削除するPRを反映し
DBを再起動して元の128MB / 4GBへ戻す。他PRのログ設定は保持する。
