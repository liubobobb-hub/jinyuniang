# 往本地小鲸鱼推一条弹幕。
# 用法：
#   powershell -ExecutionPolicy Bypass -File .\scripts\send_danmaku.ps1
#   powershell -ExecutionPolicy Bypass -File .\scripts\send_danmaku.ps1 -Text "递归怎么才能看懂" -User "晚晚"
# 环境变量：WHALE_HOST（默认 http://127.0.0.1:8787）、WHALE_INGEST_TOKEN

param(
  [string]$Text = "小鲸鱼，帮我讲讲这段代码",
  [string]$User = "小鱼干",
  [string]$HostUrl = $(if ($env:WHALE_HOST) { $env:WHALE_HOST } else { "http://127.0.0.1:8787" })
)

$headers = @{}
if ($env:WHALE_INGEST_TOKEN) {
  $headers["X-Ingest-Token"] = $env:WHALE_INGEST_TOKEN
}

$body = @{ user = $User; text = $Text; source = "script" } | ConvertTo-Json -Compress
$uri = $HostUrl.TrimEnd("/") + "/api/danmaku"
$bytes = [System.Text.Encoding]::UTF8.GetBytes($body)

Invoke-RestMethod -Uri $uri -Method Post -ContentType "application/json; charset=utf-8" -Headers $headers -Body $bytes
