@echo off
chcp 65001 >nul
echo ============================================
echo   Vigo 框架上传到 Gitee 和 GitHub
echo   版本：v2.0.12
echo ============================================
echo.

REM 检查是否已配置 Git
git config user.name >nul 2>&1
if errorlevel 1 (
    echo [配置] 请设置 Git 用户名
    set /p gitname=请输入 Git 用户名：
    git config --global user.name "%gitname%"
    
    set /p gitemail=请输入 Git 邮箱：
    git config --global user.email "%gitemail%"
)

echo [步骤 1/4] 推送到 Gitee...
echo.

REM 检查是否已添加 Gitee 远程仓库
git remote | findstr gitee >nul
if errorlevel 1 (
    set /p gitee_url=请输入 Gitee 仓库地址 (例如：git@gitee.com:yourname/vigo.git):
    git remote add gitee %gitee_url%
)

echo 正在推送到 Gitee...
git push -u gitee master
if errorlevel 1 (
    echo [错误] Gitee 推送失败，请检查网络连接和仓库地址
    pause
    goto :github
)

echo.
echo [成功] Gitee 推送完成！
echo.

:github
echo [步骤 2/4] 推送到 GitHub...
echo.

REM 检查是否已添加 GitHub 远程仓库
git remote | findstr github >nul
if errorlevel 1 (
    set /p github_url=请输入 GitHub 仓库地址 (例如：git@github.com:yourname/vigo.git):
    git remote add github %github_url%
)

echo 正在推送到 GitHub...
git push -u github master
if errorlevel 1 (
    echo [错误] GitHub 推送失败，请检查网络连接和仓库地址
    pause
    goto :end
)

echo.
echo [成功] GitHub 推送完成！
echo.

:end
echo ============================================
echo   上传完成！
echo ============================================
echo.
echo Gitee 地址：https://gitee.com/YOUR_USERNAME/vigo
echo GitHub 地址：https://github.com/YOUR_USERNAME/vigo
echo.
pause
