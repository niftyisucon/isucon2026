#!/usr/bin/env bash
# alog.sh - ISUCON2026 agent log helper (Discord #agent-log)
# Usage:
#   alog.sh post "message"
#   alog.sh read [limit]   (default 20)
# Config: the .env file next to this script (see env example)
#   ALOG_WEBHOOK_URL  this agent's webhook URL (for post)
#   ALOG_BOT_TOKEN    shared bot token (for read)
#   ALOG_CHANNEL_ID   channel id (for read)
set -euo pipefail
cd "$(dirname "$0")"
ENV_FILE="./.env"
if [ -f "$ENV_FILE" ]; then set -a; . "$ENV_FILE"; set +a; fi
cmd="${1:-}"
if [ $# -gt 0 ]; then shift; fi
case "$cmd" in
post)
msg="${1:?usage: alog.sh post message}"
: "${ALOG_WEBHOOK_URL:?ALOG_WEBHOOK_URL is not set}"
ALOG_MSG="$msg" python3 -c 'import json,os,urllib.request; data=json.dumps({"content":os.environ["ALOG_MSG"]}).encode(); req=urllib.request.Request(os.environ["ALOG_WEBHOOK_URL"],data=data,headers={"Content-Type":"application/json","User-Agent":"DiscordBot (https://isucon2026.example, 1.0)"}); urllib.request.urlopen(req).read(); print("posted")'
;;
read)
limit="${1:-20}"
: "${ALOG_BOT_TOKEN:?ALOG_BOT_TOKEN is not set}"
: "${ALOG_CHANNEL_ID:?ALOG_CHANNEL_ID is not set}"
ALOG_LIMIT="$limit" python3 -c 'import json,os,urllib.request; req=urllib.request.Request("https://discord.com/api/v10/channels/"+os.environ["ALOG_CHANNEL_ID"]+"/messages?limit="+os.environ["ALOG_LIMIT"],headers={"Authorization":"Bot "+os.environ["ALOG_BOT_TOKEN"],"User-Agent":"DiscordBot (https://isucon2026.example, 1.0)"}); msgs=json.load(urllib.request.urlopen(req)); [print("["+m["timestamp"]+"] "+m["author"].get("username","?")+": "+m["content"].replace("\n"," / ")) for m in reversed(msgs)]'
;;
*)
echo "usage: alog.sh post \"message\" | alog.sh read [limit]" >&2
exit 1
;;
esac
