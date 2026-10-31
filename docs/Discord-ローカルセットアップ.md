# Discord のローカルセットアップ（Windows / PowerShell 7）

運用ルールは「エージェント連携ガイド.md」を参照してください。
tools/agent-log/ に共通の alog.sh と Windows 用の alog.ps1 をまとめています。両方とも同じフォルダの .env を読み込みます。

1. tools/agent-log/.env.example を tools/agent-log/.env にコピーします。
2. ALOG_WEBHOOK_URL を設定します。読み取りには共通 Bot の ALOG_BOT_TOKEN も必要です。
3. status で表示される ChannelId を ALOG_CHANNEL_ID に設定します。

```powershell
pwsh -File ./tools/agent-log/alog.ps1 status
pwsh -File ./tools/agent-log/alog.ps1 post '「参加」Codex が参加しました。'
pwsh -File ./tools/agent-log/alog.ps1 read
pwsh -File ./tools/agent-log/alog.ps1 read -Limit 50
```

tools/agent-log/.env は Git 管理外です。認証情報は表示・投稿・コミットしないでください。
同名の環境変数がある場合はファイルの値より優先します。
投稿者名は Webhook の設定をそのまま使い、メンション通知は無効にします。
投稿は wait=true でメッセージ ID を確認します。失敗時は自動再送しません。

Webhook だけではチャンネル履歴を読めません。Bot には View Channel と
Read Message History の権限、および MESSAGE CONTENT INTENT が必要です。
Bot 未設定の間は他のエージェントとの重複確認ができないため、競技作業開始前に人間へ確認してください。
