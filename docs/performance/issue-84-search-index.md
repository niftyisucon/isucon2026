# Issue #84: 正規化済みユーザー検索の部分一致索引

`#76` の正規化列を使い、検索語適用前の全候補CTE展開をなくす。準備済みの行はエスケープしたLIKEとpg_trgmのGIN、未準備の行はNULL条件の小さな部分索引で取得する。3文字以上の英数字・文字の連続がない検索語は、索引を絞れないため従来のstrposを使う。どちらも入力文字列のリテラル部分一致であり、曖昧一致ではない。GoのNFC/case-fold、ブロック除外、既知一致51件と未準備全件の合流、並び順、カーソルは維持する。

Job #23625 の保存PG集計では検索先頭行が686イベント、22.1秒。原LTSVのGET /api/users・200も686件で、検索語長は8〜26 Unicode文字。PGのparse/bind/execute内訳は#73/#81の集計待ちで、エンドポイント全体の時間とは区別する。

## 担当者による導入

マージ・DBへの反映・デプロイ・ベンチはkomatsu-museが担当する。`search-index.sql` は通常のアプリ起動・initializeから自動実行しない。クエリは索引未導入でも同じ結果を返す。

1. ベンチ外に、対象DB `isureal` の `pg_available_extensions` でpg_trgmが利用可能か確認する。既に導入済みなら `pg_extension` の配置スキーマがpublicであることを確認する。既存の配置が異なる場合はSQLの演算子クラスのスキーマを合わせる。2026-10-31 15:52 JSTにレビュアーがisu4を読み取り確認し、pg_trgm 1.6が利用可能・未導入、DB所有者isurealで作成可能と報告済み（[PRレビュー](https://github.com/niftyisucon/isucon2026/pull/85#issuecomment-6540297725)）。
2. `#76` が反映済みで、usersのsearch_username/search_display_nameが存在することを確認する。
3. ベンチ・デプロイ・初期化と重ならない作業枠で、DB管理権限を使いリポジトリルートから実行する。

```bash
./bin/isu ssh isu4 'sudo -u postgres psql -d isureal -v ON_ERROR_STOP=1' < webapp/sql/search-index.sql
```

通常のCREATE INDEXは書き込みを待たせるため、ベンチ中に実行しない。SQLは1トランザクションで、lock_timeout=5秒、statement_timeout=30秒、IF NOT EXISTS付き。失敗時はロールバックされる。競合処理の終了を確認してから再実行する。

4. `pg_indexes` と `pg_index.indisvalid` でusers_search_trgm/users_search_legacyの定義と有効性、GINの `fastupdate=off` を確認し、通常のアプリデプロイを行う。
5. ベンチでinitializeが60秒以内、検索/登録/表示名更新の正当性、検索単独の件数・avg/p95、登録とPATCH /api/meのp95、SQL時間、DB CPU、スコアを比較する。再起動で索引は保持され、reset.sqlはテーブルを作り直さないため索引も保持する。

ロールバックはアプリを以前の版へ戻せば検索の意味も以前のまま。書き込み維持費まで戻す場合は、担当者がベンチ外に2索引だけを削除する。共有され得る拡張は削除しない。

```sql
DROP INDEX IF EXISTS users_search_trgm, users_search_legacy;
```

## 検証と制約

ローカルPG16.15、合成5000ユーザー、ANALYZE済み。索引作成後に全5000行のキーをNULLへ戻し、再構築してから10回平均を測定した。ネットワーク・負荷・分布は実機と異なる。

| 検索 | 従来MATERIALIZED | 展開なし・strpos | 採用案 |
| --- | ---: | ---: | ---: |
| 一意名 user_04999 | 2.18ms | 1.12ms | 0.25ms |
| 該当なし | 2.15ms | 1.14ms | 0.11ms |
| 100人一致 | 2.44ms | 0.33ms | 0.28ms |
| 全員一致 user | 2.53ms | 0.28ms | 1.88ms |
| 1文字 そ | 2.65ms | 0.27ms | 0.26ms |
| 記号のみ % | 2.14ms | 1.06ms | 1.05ms |

GINのfastupdate既定値では再構築後の一意名検索が約1.06msに伸び、GINを使わない計画も選ばれた。`fastupdate=off` で検索時のpending list走査をなくした代わりに、100行INSERT+100行UPDATEの単発バッチは3.43→5.05ms、5000キー再構築は約422msになった（既定値での別試行は約62ms）。書き込み維持費が増えるため、実機の登録・更新・初期化も判断に含める。索引作成は約28ms、作成直後の2索引合計は約352KiB。実機での維持費・サイズ・スコアは未測定。

全HTTP検索ページを元のGo検索と比較し、Unicode、結合文字、ß/Σ/İ/ﬃ、リテラル `%`/`_`/バックスラッシュ、退会、両方向ブロック、プロフィールの公開区分、NULL混在、表示名変更、旧バイナリによる変更、2回のreset、同時起動を検証する。選択性のある検索はEXPLAINでGIN利用とCTE Scan消滅を確認する。

この索引は検索語によっては大きく絞れない。広く一致する語では索引なしのstrposより遅い場合があり、ローカルの改善率を実機スコアへ外挿しない。[pg_trgmの索引仕様](https://www.postgresql.org/docs/16/pgtrgm.html)と[GINの更新保留の仕組み](https://www.postgresql.org/docs/16/gin-implementation.html)を参照。
