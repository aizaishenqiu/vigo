# Vigo 框架上传脚本 - PowerShell 版本
# 版本：v2.0.12

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Vigo 框架上传到 Gitee 和 GitHub" -ForegroundColor Cyan
Write-Host "  版本：v2.0.12" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

# 检查 Git 配置
$gitName = git config user.name
if ([string]::IsNullOrEmpty($gitName)) {
    Write-Host "[配置] 未设置 Git 用户名" -ForegroundColor Yellow
    $gitName = Read-Host "请输入 Git 用户名"
    git config --global user.name $gitName
    
    $gitEmail = Read-Host "请输入 Git 邮箱"
    git config --global user.email $gitEmail
}

Write-Host "[信息] Git 用户：$gitName" -ForegroundColor Green
Write-Host ""

# 获取仓库地址
$giteeRemote = git remote | Select-String "gitee"
if ([string]::IsNullOrEmpty($giteeRemote)) {
    $giteeUrl = Read-Host "请输入 Gitee 仓库地址 (例如：git@gitee.com:yourname/vigo.git)"
    git remote add gitee $giteeUrl
    Write-Host "[成功] 已添加 Gitee 远程仓库" -ForegroundColor Green
} else {
    Write-Host "[信息] Gitee 远程仓库已存在" -ForegroundColor Green
}

$githubRemote = git remote | Select-String "github"
if ([string]::IsNullOrEmpty($githubRemote)) {
    $githubUrl = Read-Host "请输入 GitHub 仓库地址 (例如：git@github.com:yourname/vigo.git)"
    git remote add github $githubUrl
    Write-Host "[成功] 已添加 GitHub 远程仓库" -ForegroundColor Green
} else {
    Write-Host "[信息] GitHub 远程仓库已存在" -ForegroundColor Green
}

Write-Host ""
Write-Host "[步骤 1/2] 推送到 Gitee..." -ForegroundColor Cyan

try {
    git push -u gitee master
    Write-Host "[成功] Gitee 推送完成！" -ForegroundColor Green
} catch {
    Write-Host "[错误] Gitee 推送失败，请检查网络连接和仓库地址" -ForegroundColor Red
}

Write-Host ""
Write-Host "[步骤 2/2] 推送到 GitHub..." -ForegroundColor Cyan

try {
    git push -u github master
    Write-Host "[成功] GitHub 推送完成！" -ForegroundColor Green
} catch {
    Write-Host "[错误] GitHub 推送失败，请检查网络连接和仓库地址" -ForegroundColor Red
}

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  上传完成！" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "提示：" -ForegroundColor Yellow
Write-Host "1. 首次推送需要使用 SSH 密钥或 HTTPS 认证" -ForegroundColor White
Write-Host "2. 请确保已在 Gitee/GitHub 创建仓库并配置 SSH 密钥" -ForegroundColor White
Write-Host "3. 创建新仓库后，运行此脚本即可自动推送" -ForegroundColor White
Write-Host ""
Write-Host "Gitee 地址：https://gitee.com/YOUR_USERNAME/vigo" -ForegroundColor White
Write-Host "GitHub 地址：https://github.com/YOUR_USERNAME/vigo" -ForegroundColor White
Write-Host ""
