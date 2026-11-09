#!/usr/bin/env bash
# scripts/smoke.sh —— 端到端冒烟测试（Linux / macOS / CI 用）
#
# 与 scripts/smoke.ps1 覆盖同一组用例，方便在 Linux 服务器和 GitHub Actions 上直接跑：
#   bash scripts/smoke.sh
#   BASE_URL=http://127.0.0.1:8080 USER_NAME=admin PASSWORD='Admin@123456' bash scripts/smoke.sh

set -uo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
USER_NAME="${USER_NAME:-admin}"
PASSWORD="${PASSWORD:-Admin@123456}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
JAR="$WORK/cookies.txt"
BODY="$WORK/body.txt"
OK=0
FAIL=0

check() {
  local name="$1" cond="$2" detail="${3:-}"
  if [ "$cond" = "true" ]; then
    OK=$((OK + 1))
    printf '  PASS  %s\n' "$name"
  else
    FAIL=$((FAIL + 1))
    printf '  FAIL  %s  %s\n' "$name" "$detail"
  fi
}

# status <method> <path> [curl args...]  → 打印状态码，响应体落在 $BODY
status() {
  local method="$1" path="$2"
  shift 2
  curl -s -o "$BODY" -w '%{http_code}' -X "$method" "$BASE_URL$path" "$@"
}

echo
echo "[1] 公开页面与安全响应头"
CODE=$(status GET /)
check "GET / 返回 200" "$([ "$CODE" = "200" ] && echo true || echo false)" "status=$CODE"
check "首页包含项目名" "$(grep -q 'GoVulnLab' "$BODY" && echo true || echo false)"
HDR="$WORK/home_headers.txt"
curl -s -D "$HDR" -o /dev/null "$BASE_URL/"
check "存在 X-Request-Id" "$(grep -qi 'X-Request-Id' "$HDR" && echo true || echo false)"
check "CSP 使用 nonce" "$(grep -qi 'Content-Security-Policy.*nonce-' "$HDR" && echo true || echo false)"

echo
echo "[2] 登录与会话"
CODE=$(curl -s -o /dev/null -c "$JAR" -w '%{http_code}' -X POST "$BASE_URL/auth/login" \
  -d "username=$USER_NAME" -d "password=$PASSWORD")
check "登录成功返回 302" "$([ "$CODE" = "302" ] && echo true || echo false)" "status=$CODE"
check "下发了会话 Cookie" "$(grep -q 'gvl_session' "$JAR" && echo true || echo false)"
check "Cookie 带 HttpOnly" "$(grep -qi 'HttpOnly' "$JAR" && echo true || echo false)"

CODE=$(status GET /profile -b "$JAR")
check "登录后可访问个人中心" "$([ "$CODE" = "200" ] && echo true || echo false)" "status=$CODE"
CSRF=$(grep -o 'name="csrf_token" value="[^"]*"' "$BODY" | head -n1 | sed 's/.*value="//; s/"$//')
check "页面包含 CSRF Token" "$([ -n "$CSRF" ] && echo true || echo false)"

echo
echo "[3] CSRF 防护"
CODE=$(status POST /orders -b "$JAR" -d 'goodsname=no-token' -d 'amount=1.00')
check "缺少 Token 的写请求被拒绝(403)" "$([ "$CODE" = "403" ] && echo true || echo false)" "status=$CODE"

echo
echo "[4] 下单与金额精度"
CODE=$(status POST /orders -b "$JAR" -d "csrf_token=$CSRF" -d 'goodsname=机械键盘' -d 'amount=99.99')
check "带 Token 下单成功(302)" "$([ "$CODE" = "302" ] && echo true || echo false)" "status=$CODE"

CODE=$(status GET '/api/orders?size=10' -b "$JAR")
check "订单列表 API 返回 200" "$([ "$CODE" = "200" ] && echo true || echo false)" "status=$CODE"
check "金额以分为单位" "$(grep -q '"amount_cent":9999' "$BODY" && echo true || echo false)"

echo
echo "[5] 越权防护（安全实现）"
CODE=$(status GET /api/orders/999999 -b "$JAR")
check "不存在的订单返回 404" "$([ "$CODE" = "404" ] && echo true || echo false)" "status=$CODE"

echo
echo "[6] 管理后台"
CODE=$(status GET /admin/orders -b "$JAR")
check "管理员可访问后台" "$([ "$CODE" = "200" ] && echo true || echo false)" "status=$CODE"

echo
echo "[7] 漏洞靶场"
CODE=$(status GET /lab -b "$JAR")
check "靶场首页可访问" "$([ "$CODE" = "200" ] && echo true || echo false)" "status=$CODE"
CODE=$(status POST /lab/sqli -b "$JAR" --data-urlencode "username=admin' OR '1'='1' -- " -d 'password=x')
check "SQL 注入复现成功" "$(grep -q '注入成功' "$BODY" && echo true || echo false)"
CODE=$(status GET '/lab/idor?id=1' -b "$JAR")
check "越权靶场可读取任意订单" "$(grep -q '读到订单' "$BODY" && echo true || echo false)"
CODE=$(status POST /lab/bruteforce -b "$JAR" -d 'username=labadmin' -d 'password=Lab@123456')
check "暴破靶场命中口令" "$(grep -q '命中口令' "$BODY" && echo true || echo false)"

echo
echo "[8] 注销"
CODE=$(status POST /auth/logout -b "$JAR" -d "csrf_token=$CSRF")
check "注销返回 302" "$([ "$CODE" = "302" ] && echo true || echo false)" "status=$CODE"
CODE=$(status GET /profile -b "$JAR")
check "注销后访问个人中心被重定向" "$([ "$CODE" = "302" ] && echo true || echo false)" "status=$CODE"

echo
printf '结果：%d 通过 / %d 失败\n' "$OK" "$FAIL"
[ "$FAIL" -eq 0 ]
