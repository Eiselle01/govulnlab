# scripts/smoke.ps1 —— 端到端冒烟测试
#
# 覆盖：首页 → 登录 → 会话 → CSRF 正/反例 → 建单 → JSON API → 订单详情 →
#       越权返回 404 → 管理后台 → 靶场注入 → 注销 → 注销后访问被拦截
#
# 用法：
#   pwsh -File scripts/smoke.ps1
#   pwsh -File scripts/smoke.ps1 -BaseUrl http://127.0.0.1:8080 -User admin -Pass 'Admin@123456'

param(
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$User = "admin",
    [string]$Pass = "Admin@123456"
)

$ErrorActionPreference = "Stop"
$script:okCount = 0
$script:failCount = 0

function Check([string]$name, [bool]$ok, [string]$detail = "") {
    if ($ok) {
        $script:okCount++
        Write-Host "  PASS  $name" -ForegroundColor Green
    } else {
        $script:failCount++
        Write-Host "  FAIL  $name  $detail" -ForegroundColor Red
    }
}

# 用 HttpClient 而不是 Invoke-WebRequest：可以精确控制是否跟随重定向，
# 才能断言「登录成功返回 302 且带 Set-Cookie」这种细节。
$handler = [System.Net.Http.HttpClientHandler]::new()
$handler.CookieContainer = [System.Net.CookieContainer]::new()
$handler.AllowAutoRedirect = $false
$client = [System.Net.Http.HttpClient]::new($handler)
$client.Timeout = [TimeSpan]::FromSeconds(10)

function Send([string]$method, [string]$path, $form) {
    $url = "$BaseUrl$path"
    $req = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::$method, $url)
    if ($null -ne $form) {
        $pairs = [System.Collections.Generic.List[System.Collections.Generic.KeyValuePair[string, string]]]::new()
        foreach ($k in $form.Keys) {
            $pairs.Add([System.Collections.Generic.KeyValuePair[string, string]]::new([string]$k, [string]$form[$k]))
        }
        $req.Content = [System.Net.Http.FormUrlEncodedContent]::new($pairs)
    }
    $resp = $client.SendAsync($req).GetAwaiter().GetResult()
    $body = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    return [pscustomobject]@{ Status = [int]$resp.StatusCode; Body = $body; Headers = $resp.Headers; Resp = $resp }
}

Write-Host "`n[1] 公开页面与安全响应头" -ForegroundColor Cyan
$rHome = Send "Get" "/" $null
Check "GET / 返回 200" ($rHome.Status -eq 200) "status=$($rHome.Status)"
Check "首页包含项目名" ($rHome.Body -like "*GoVulnLab*")
Check "存在 X-Request-Id" ($null -ne $rHome.Resp.Headers.GetValues("X-Request-Id"))
Check "CSP 使用 nonce" (($rHome.Resp.Headers.GetValues("Content-Security-Policy") -join "") -like "*nonce-*")
Check "X-Frame-Options=DENY" ((($rHome.Resp.Headers.GetValues("X-Frame-Options") -join "") -eq "DENY"))
$rHome.Resp.Dispose()

Write-Host "`n[2] 登录与会话" -ForegroundColor Cyan
$rLogin = Send "Post" "/auth/login" @{ username = $User; password = $Pass }
Check "登录成功返回 302" ($rLogin.Status -eq 302) "status=$($rLogin.Status)"
Check "跳转到 /profile" ((($rLogin.Resp.Headers.Location).ToString()) -eq "/profile")
Check "下发会话 Cookie" ($handler.CookieContainer.Count -ge 1)
$rLogin.Resp.Dispose()

$rProfile = Send "Get" "/profile" $null
Check "登录后可访问个人中心" ($rProfile.Status -eq 200) "status=$($rProfile.Status)"
$csrf = [regex]::Match($rProfile.Body, 'name="csrf_token" value="([^"]+)"').Groups[1].Value
Check "页面包含 CSRF Token" ($csrf.Length -ge 20)
$rProfile.Resp.Dispose()

Write-Host "`n[3] CSRF 防护" -ForegroundColor Cyan
$rNoCsrf = Send "Post" "/orders" @{ goodsname = "无 token 下单"; amount = "1.00" }
Check "缺少 CSRF Token 被拒绝(403)" ($rNoCsrf.Status -eq 403) "status=$($rNoCsrf.Status)"
$rNoCsrf.Resp.Dispose()

Write-Host "`n[4] 下单与金额精度" -ForegroundColor Cyan
$rCreate = Send "Post" "/orders" @{ csrf_token = $csrf; goodsname = "机械键盘"; amount = "99.99" }
Check "带 Token 下单成功(302)" ($rCreate.Status -eq 302) "status=$($rCreate.Status)"
$rCreate.Resp.Dispose()

$rList = Send "Get" "/api/orders?size=10" $null
Check "订单列表 API 返回 200" ($rList.Status -eq 200) "status=$($rList.Status)"
$json = $rList.Body | ConvertFrom-Json
Check "订单条数 >= 1" ($json.data.items.Count -ge 1)
Check "金额以分为单位且无浮点误差" (($json.data.items[0].amount_cent % 100) -eq 99) "amount_cent=$($json.data.items[0].amount_cent)"
$firstId = $json.data.items[0].id
$rList.Resp.Dispose()

$rDetail = Send "Get" "/orders/$firstId" $null
Check "订单详情页可访问" ($rDetail.Status -eq 200) "status=$($rDetail.Status)"
$rDetail.Resp.Dispose()

Write-Host "`n[5] 越权防护（安全实现）" -ForegroundColor Cyan
$rApi404 = Send "Get" "/api/orders/999999" $null
Check "不存在的订单返回 404 而非 500" ($rApi404.Status -eq 404) "status=$($rApi404.Status)"
$rApi404.Resp.Dispose()

Write-Host "`n[6] 管理后台" -ForegroundColor Cyan
$rAdmin = Send "Get" "/admin/orders" $null
Check "管理员可访问后台" ($rAdmin.Status -eq 200) "status=$($rAdmin.Status)"
Check "后台页面含全部订单表头" ($rAdmin.Body -like "*下单人*")
$rAdmin.Resp.Dispose()

Write-Host "`n[7] 漏洞靶场" -ForegroundColor Cyan
$rLab = Send "Get" "/lab" $null
Check "靶场首页可访问" ($rLab.Status -eq 200) "status=$($rLab.Status)"
$rLab.Resp.Dispose()

$rInject = Send "Post" "/lab/sqli" @{ username = "admin' OR '1'='1' -- "; password = "x" }
Check "SQL 注入复现成功" ($rInject.Body -like "*注入成功*")
$rInject.Resp.Dispose()

$rIdor = Send "Get" "/lab/idor?id=1" $null
Check "越权靶场可读取任意订单" ($rIdor.Status -eq 200 -and $rIdor.Body -like "*读到订单*")
$rIdor.Resp.Dispose()

$rBrute = Send "Post" "/lab/bruteforce" @{ username = "labadmin"; password = "Lab@123456" }
Check "暴破靶场命中口令" ($rBrute.Body -like "*命中口令*")
$rBrute.Resp.Dispose()

$rBadId = Send "Get" "/lab/idor?id=abc" $null
Check "靶场参数校验仍在（400）" ($rBadId.Status -eq 400) "status=$($rBadId.Status)"
$rBadId.Resp.Dispose()

Write-Host "`n[8] 注销" -ForegroundColor Cyan
$rLogout = Send "Post" "/auth/logout" @{ csrf_token = $csrf }
Check "注销返回 302" ($rLogout.Status -eq 302) "status=$($rLogout.Status)"
$rLogout.Resp.Dispose()

$rAfter = Send "Get" "/profile" $null
Check "注销后访问个人中心被重定向" ($rAfter.Status -eq 302) "status=$($rAfter.Status)"
$rAfter.Resp.Dispose()

Write-Host "`n结果：$script:okCount 通过 / $script:failCount 失败" -ForegroundColor $(if ($script:failCount -eq 0) { "Green" } else { "Red" })
if ($script:failCount -gt 0) { exit 1 }
