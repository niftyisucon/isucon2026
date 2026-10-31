# ベンチマークの回し方

ポータルから負荷走行（ベンチマーク）を実行する手順。どのエージェントが回しても同じになるよう、実際の画面操作ベースで書く。

## 手順

1. ポータル <https://portal.isucon.net/contestant> に GitHub アカウントでサインインする
2. 上部ナビゲーションの「ベンチマーク」を開く（Benchmark Jobs: <https://portal.isucon.net/contestant/benchmark_jobs>）
3. **「Enqueue」を1回だけ押す**
- 「Enqueue を押すと計測を開始します。」と表示されている
- 対象サーバーの選択や設定入力はない（ルーター経由で asia1 / europe1 / america1 / asia2 の全リージョンが対象）。確認ダイアログも出ない
- 押すと即 Job 詳細ページに遷移し、RUNNING になる
4. Job 詳細ページで「Reload」を押しながら Status が **FINISHED** になるまで待つ
- 初回実績（Job #256）は Enqueue から約2分20秒で完了
- 流れは 初期化（`POST /api/initialize`）→ 整合性チェック → 負荷 → 最終チェック
5. 結果を確認する：Status / Result（Passed か FAIL か）/ Score / Score Breakdown（base と deduction）/ Stdout
- 初期化や検証に失敗すると FAIL でスコア0

## ルール・注意

- **#agent-log でを宣言してから Enqueue する**（ベンチはチームで同時に1本だけ）
- WAITING / RUNNING の間は新しい負荷走行をリクエストできない
- 結果が出たら #agent-log にでスコアと Job の URL を共有する
- Job 詳細ページ下部の **「Re-Enqueue」はもう1回実行するボタン**。確認なしで再実行になるので、結果を見たいだけのときは押さない
- 「インスタンスの登録が完了していません」と表示されて Enqueue が押せないときは、サーバー起動後に画面をリロードすると、サーバーリストが RUNNING になって Enqueue が有効になる
- **最終スコアは、競技中に最後に実行した負荷走行の結果**。最後の走行が FAIL だと受賞対象外になるため、終盤のベンチは慎重に

## HTTP リクエストだけで回せるか

現時点では未確認。Enqueue はサインイン済みセッション前提の画面操作で、ログインなしの HTTP リクエストで実行する方法は確認できていない（要調査）。
