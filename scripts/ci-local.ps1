# ci-local.ps1 —— 在本地复刻 .github/workflows/ci.yml 的检查步骤。
#
# 用途：GitHub Actions 跑在云端，本地看不到过程。这个脚本把同样的 5 个检查
# 在你电脑上按顺序跑一遍，告诉你每一步是什么、过没过。
#
# 用法：pwsh -NoProfile -File .\scripts\ci-local.ps1

$ErrorActionPreference = "Continue"
$root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $root

$fail = 0
function Step($no, $total, $title) {
    Write-Host ""
    Write-Host "[$no/$total] $title" -ForegroundColor Cyan
}

function Result($ok, $detail) {
    if ($ok) {
        Write-Host "      PASS $detail" -ForegroundColor Green
    } else {
        Write-Host "      FAIL $detail" -ForegroundColor Red
        $script:fail++
    }
}

Write-Host "== 本地 CI 预演（与 GitHub Actions 同一组检查）==" -ForegroundColor Yellow

# 1. 依赖清单是否干净：go.mod / go.sum 应与代码一致，不能有没提交的改动
Step 1 5 "依赖清单检查（go mod tidy 后是否还有差异）"
go mod tidy | Out-Null
$diff = git diff --name-only -- go.mod go.sum
Result ([string]::IsNullOrWhiteSpace($diff)) $(if ($diff) { "go.mod/go.sum 有未提交的改动：$diff" } else { "go.mod / go.sum 干净" })

# 2. 格式检查：gofmt 认为需要重排的文件列表必须为空
Step 2 5 "格式检查（gofmt）"
$unformatted = gofmt -l cmd internal migrations web
Result ([string]::IsNullOrWhiteSpace($unformatted)) $(if ($unformatted) { "以下文件未格式化：`n$unformatted" } else { "所有 Go 文件格式规范" })

# 3. 静态检查：go vet 能发现 printf 参数不匹配、锁复制一类的明显错误
Step 3 5 "静态检查（go vet）"
$vetOut = go vet ./... 2>&1 | Out-String
Result ($LASTEXITCODE -eq 0) $(if ($LASTEXITCODE -eq 0) { "go vet 无告警" } else { $vetOut.Trim() })

# 4. 单元测试：-race 检测并发读写冲突（这正是并发 bug 最常见的样子）
#    注意：Windows 上 Go 的 race 运行库偶尔起不来（exit status 0xc0000139），
#    那是本机运行时问题，不是代码问题。此时降级为普通 go test，并明确标注。
Step 4 5 "单元测试（go test；优先带 -race 竞态检测）"
$testOut = go test -race -count=1 ./... 2>&1 | Out-String
$raceBroken = $testOut -match "0xc0000139"
$okCount = ([regex]::Matches($testOut, "(?m)^ok\s")).Count
if ($raceBroken) {
    Write-Host "      提示：本机 race 运行库启动失败（0xc0000139），降级为不带 -race 运行。" -ForegroundColor Yellow
    Write-Host "      CI 在 Linux 上会正常带 -race，所以这一步不影响线上结果。" -ForegroundColor DarkGray
    $testOut = go test -count=1 ./... 2>&1 | Out-String
    $okCount = ([regex]::Matches($testOut, "(?m)^ok\s")).Count
    Result ($LASTEXITCODE -eq 0) $(if ($LASTEXITCODE -eq 0) { "$okCount 个包通过（无竞态检测，仅本机降级）" } else { $testOut.Trim() })
} else {
    Result ($LASTEXITCODE -eq 0) $(if ($LASTEXITCODE -eq 0) { "$okCount 个包全部通过（含竞态检测）" } else { $testOut.Trim() })
}

# 5. 构建：能不能真的产出可执行文件（这一步失败=线上起不来）
Step 5 5 "构建二进制（go build）"
$buildOut = go build -trimpath -o bin\ci-check.exe ./cmd/govulnlab 2>&1 | Out-String
$exeOk = Test-Path -LiteralPath "bin\ci-check.exe"
Result ($exeOk) $(if ($exeOk) { "构建成功：bin\ci-check.exe" } else { $buildOut.Trim() })
if ($exeOk) { Remove-Item -LiteralPath "bin\ci-check.exe" -Force }

Write-Host ""
if ($fail -eq 0) {
    Write-Host "== 结果：5/5 通过 —— 这一版推上去，GitHub 上会是一个绿勾 ==" -ForegroundColor Green
    Write-Host "   CI 的价值就在这：在别人看到之前，先把坏版本拦下来。" -ForegroundColor DarkGray
    exit 0
}
Write-Host "== 结果：$fail 项失败 —— 推上去会是红叉，先修再推 ==" -ForegroundColor Red
exit 1
