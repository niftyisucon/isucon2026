# Issue #41: 低優先インデックスの選定

追加は `follows(followee_id)` のみ。`UNIQUE(follower_id, followee_id)` はフォロワー一覧の `WHERE followee_id=$1` の先頭列にならないため、逆引き用の単列索引を追加する。SQL・応答・データは変更しない。初期構築と既存環境の初期化の両方に `IF NOT EXISTS` で定義する。

## ローカル検証（2026-10-31）

PostgreSQL 16.15。実データや競技サーバーは使用せず、保存済み `hosts/isu4/discovery.md` の規模に合わせた合成データで `ANALYZE` 後に `EXPLAIN (ANALYZE, BUFFERS)` を比較した。時間は単発測定であり、スコア改善の実測ではない。

| 候補 | 合成条件 | 追加前 → 追加後 | 判断 |
| --- | --- | --- | --- |
| follows(followee_id) | 10,020行、2,000 followeeへ均等、対象5行 | Seq Scan → Bitmap Index Scan、64 → 7 buffers、0.450 → 0.051ms | 追加 |
| block_relations(blocked_id) | 258行、OR両方向の対象2行 | Seq Scanのまま、2 buffers | 現時点は見送り |
| posts(posting_window_id, posted_at DESC, id DESC) | 625,239行、360期間中3期間active | 既存posts_posting_window_idのBitmap Scan＋top-N Sortのまま、5,222 buffers | 見送り |

followsは `WHERE followee_id='123' ORDER BY updated_at DESC,id DESC LIMIT 51`、block_relationsは `WHERE blocker_id='123' OR blocked_id='123'`、postsはposting_windowsとのJOINで `ended_at IS NULL ORDER BY posted_at DESC,id DESC LIMIT 51` を使用。postsの検証は期間JOINとソート部分のみで、実フィードのユーザーJOIN・公開範囲条件は含まない。分布、UUIDの幅、キャッシュ状態も実環境と異なるため、見送りは効果がないとの断定ではない。

新規の空DBで `001-schema.sql` → `003-initial-snapshot.sql` → `reset.sql` → `reset.sql` を `ON_ERROR_STOP=1` で適用し、定義と再実行を確認した。実データの復元時間は未検証。

## 競技担当者の適用と計測

マージ・反映はkomatsu-muse担当。ベンチ外にSQLを配布し、既存DBへ以下のDDLだけを適用できる（この索引のために全データをresetする必要はない）。競技中のinitializeでも同じ定義が作成される。

```sql
CREATE INDEX IF NOT EXISTS follows_followee_id ON follows(followee_id);
```

通常のCREATE INDEXは構築中に書き込みを待たせるため、ベンチ中に実行しない。構築成功と `pg_indexes` の定義を確認してから計測する。代表的なfollowers一覧の実クエリについて、追加前後のEXPLAIN、buffers、総時間、索引サイズ、initialize所要時間（60秒以内）、follow作成・解除の時間とスコアを比較する。改善しない場合はこの索引を削除し、2つのSQL定義も戻す。今回は競技DBへの適用・ベンチは未実施。
