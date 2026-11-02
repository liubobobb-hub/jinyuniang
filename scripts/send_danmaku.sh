#!/usr/bin/env bash
# 往本地小鲸鱼推一条弹幕。
# 用法：
#   ./scripts/send_danmaku.sh
#   ./scripts/send_danmaku.sh "递归怎么才能看懂" "晚晚"
# 环境变量：WHALE_HOST（默认 http://127.0.0.1:8787）、WHALE_INGEST_TOKEN

set -euo pipefail

TEXT="${1:-小鲸鱼，帮我讲讲这段代码}"
USER_NAME="${2:-小鱼干}"
HOST="${WHALE_HOST:-http://127.0.0.1:8787}"
TOKEN="${WHALE_INGEST_TOKEN:-}"

json_escape() {
  local s=$1
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  s=${s//$'\n'/\\n}
  s=${s//$'\r'/}
  s=${s//$'\t'/\\t}
  printf '%s' "$s"
}

payload=$(printf '{"user":"%s","text":"%s","source":"script"}' \
  "$(json_escape "$USER_NAME")" \
  "$(json_escape "$TEXT")")

args=(-sS -X POST "${HOST%/}/api/danmaku" \
  -H "Content-Type: application/json; charset=utf-8" \
  --data-binary "$payload")
if [[ -n "$TOKEN" ]]; then
  args+=(-H "X-Ingest-Token: $TOKEN")
fi

curl "${args[@]}"
printf '\n'
