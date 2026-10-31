# bench-data — ベンチマーク計測パッケージ
komatsu-muse がベンチのたびに、この形式で repo に置く（形式は人間の指示で固定）。

## job-<番号>/ の中身
- `summary.md` — スコア、実行時間帯（JST/UTC）、デプロイ済みコミット、反映済み PR、ログ抽出時間帯
- `portal.txt` — ポータルのジョブ詳細ページの原文（Summary / Result / Conclusion / Stdout）
- `isu{1,2,3}-ltsv.txt` — 各アプリサーバーの nginx LTSV を実行時間帯で切り出した生データ
- `aggregate.txt` — nginx ログのエンドポイント別集計（件数・平均・p95・apptime 合計）
- `pg-aggregate.txt` — PostgreSQL ログの同時間帯集計（クエリ総数・総時間・パターン別上位・最遅クエリ）。生ログは巨大なため集計のみ。精密な内訳は `./bin/isu pglog isu4`（pgBadger）を summary.md 記載の時間帯で実行すれば再現できる

各エージェントは要約だけでなく一次データで分析してよい。

## Convention (added 2026-10-31, human instruction)
- Every package's summary.md MUST include a "Merged PRs newly reflected in this run" section describing, per PR, what the change actually does (files/mechanism), plus the cumulative merged-PR list for the deployed state.
- Every bench result MUST be verified against the portal Benchmark Jobs LIST (newest job ID + score) before it is reported or packaged. (Two corrupted browser handoffs fabricated jobs #11604 and #12217; both are retracted in their directories.)
