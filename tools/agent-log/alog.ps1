#requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory, Position = 0)]
    [ValidateSet('post', 'read', 'status')][string]$Command,
    [Parameter(Position = 1)][string]$Message,
    [ValidateRange(1, 100)][int]$Limit = 20
)

$ErrorActionPreference = 'Stop'
$configPath = Join-Path $PSScriptRoot '.env'
$config = @{}
if (Test-Path -LiteralPath $configPath) {
    foreach ($line in Get-Content -LiteralPath $configPath) {
        if ($line -match '^\s*(ALOG_[A-Z_]+)=(.*)$') {
            $config[$Matches[1]] = $Matches[2].Trim()
        }
    }
}
foreach ($key in @('ALOG_WEBHOOK_URL', 'ALOG_BOT_TOKEN', 'ALOG_CHANNEL_ID')) {
    $value = [Environment]::GetEnvironmentVariable($key)
    if ($value) { $config[$key] = $value }
}

function Invoke-Discord {
    param([string]$Uri, [string]$Method = 'Get', [hashtable]$Headers = @{}, [string]$Body)
    $request = @{ Uri = $Uri; Method = $Method; Headers = $Headers; TimeoutSec = 30 }
    if ($Body) {
        $request.ContentType = 'application/json; charset=utf-8'
        $request.Body = [Text.Encoding]::UTF8.GetBytes($Body)
    }
    try { Invoke-RestMethod @request }
    catch {
        # Do not expose request URLs, tokens, or response bodies in errors.
        $status = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { 'network error' }
        throw "Discord request failed ($status). Check local credentials/permissions. No automatic retry was made."
    }
}

try {
    if ($Command -eq 'read') {
        if (!$config.ALOG_BOT_TOKEN -or !$config.ALOG_CHANNEL_ID) {
            throw 'Reading requires ALOG_BOT_TOKEN and ALOG_CHANNEL_ID in tools/agent-log/.env.'
        }
        if ($config.ALOG_CHANNEL_ID -notmatch '^\d+$') { throw 'Invalid ALOG_CHANNEL_ID.' }
        $messages = @(Invoke-Discord -Uri "https://discord.com/api/v10/channels/$($config.ALOG_CHANNEL_ID)/messages?limit=$Limit" -Headers @{ Authorization = "Bot $($config.ALOG_BOT_TOKEN)" })
        [array]::Reverse($messages)
        foreach ($entry in $messages) {
            "[$($entry.timestamp)] $($entry.author.username): $($entry.content)"
        }
        exit 0
    }

    $webhook = $config.ALOG_WEBHOOK_URL
    if (!$webhook -or $webhook -notmatch '^https://discord\.com/api(?:/v\d+)?/webhooks/\d+/[A-Za-z0-9_-]+$') {
        throw 'Set a valid ALOG_WEBHOOK_URL in tools/agent-log/.env.'
    }
    if ($Command -eq 'status') {
        $info = Invoke-Discord -Uri $webhook
        [pscustomobject]@{
            WebhookName = $info.name
            ChannelId = $info.channel_id
            HistoryConfigured = [bool]($config.ALOG_BOT_TOKEN -and $config.ALOG_CHANNEL_ID)
        }
        exit 0
    }
    if ([string]::IsNullOrWhiteSpace($Message) -or $Message.Length -gt 2000) {
        throw 'Message must contain 1 to 2000 characters.'
    }
    $payload = @{ content = $Message; allowed_mentions = @{ parse = @() } } | ConvertTo-Json -Depth 4 -Compress
    $sent = Invoke-Discord -Uri "${webhook}?wait=true" -Method Post -Body $payload
    if (!$sent.id) { throw 'Discord did not return a message ID; check delivery before retrying.' }
    "Posted: message=$($sent.id) channel=$($sent.channel_id)"
}
catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
