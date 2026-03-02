# ==============================================
# Vigo 应用打包脚本 (Windows PowerShell)
# 用于生成完整的 Linux 部署包
# ==============================================

$ErrorActionPreference = "Stop"

# 项目根目录
$ProjectDir = Split-Path -Parent $PSScriptRoot
# 版本号
$Version = "1.0.1"
# 打包目录
$PackageDir = Join-Path $ProjectDir "dist\vigo-$Version-linux-amd64"

# ==============================================
# 颜色输出函数
# ==============================================
function Write-Green {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Red {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Red
}

function Write-Yellow {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Yellow
}

# ==============================================
# 清理旧的打包文件
# ==============================================
Write-Host "清理旧的打包文件..."
$DistDir = Join-Path $ProjectDir "dist"
if (Test-Path $DistDir) {
    Remove-Item -Path $DistDir -Recurse -Force
}
New-Item -ItemType Directory -Path $PackageDir -Force | Out-Null

# ==============================================
# 编译 Linux 版本
# ==============================================
Write-Host "编译 Linux amd64 版本..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$BinaryPath = Join-Path $PackageDir "vigo-linux-amd64"

go build -o $BinaryPath $ProjectDir

if ($LASTEXITCODE -ne 0) {
    Write-Red "编译失败！"
    exit 1
}

Write-Green "编译成功！"

# ==============================================
# 复制配置文件
# ==============================================
Write-Host "复制配置文件..."
$ConfigSource = Join-Path $ProjectDir "config.yaml"
$ConfigDest = Join-Path $PackageDir "config.yaml"
Copy-Item $ConfigSource $ConfigDest

# ==============================================
# 复制部署脚本
# ==============================================
Write-Host "复制部署脚本..."
$DeployDir = $PSScriptRoot
$Scripts = @("start.sh", "stop.sh", "restart.sh", "vigo.service", "部署说明.md")

foreach ($Script in $Scripts) {
    $Source = Join-Path $DeployDir $Script
    $Dest = Join-Path $PackageDir $Script
    Copy-Item $Source $Dest
}

# ==============================================
# 创建必要的目录
# ==============================================
Write-Host "创建目录结构..."
$RuntimeDir = Join-Path $PackageDir "runtime"
$LogDir = Join-Path $RuntimeDir "log"
New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogDir -Force | Out-Null

# ==============================================
# 打包为 zip
# ==============================================
Write-Host "打包为 zip..."
$ZipPath = Join-Path $DistDir "vigo-$Version-linux-amd64.zip"
Compress-Archive -Path $PackageDir -DestinationPath $ZipPath -Force

if ($?) {
    Write-Host ""
    Write-Green "=============================================="
    Write-Green "打包完成！"
    Write-Green "=============================================="
    Write-Host ""
    Write-Host "部署包位置: $ZipPath"
    Write-Host "解压目录: $PackageDir"
    Write-Host ""
    Write-Host "部署步骤:"
    Write-Host "  1. 上传 vigo-$Version-linux-amd64.zip 到服务器"
    Write-Host "  2. 解压: unzip vigo-$Version-linux-amd64.zip"
    Write-Host "  3. 配置环境变量"
    Write-Host "  4. 设置权限: chmod +x *.sh vigo-linux-amd64"
    Write-Host "  5. 启动: ./start.sh"
    Write-Host ""
    Write-Host "详细说明请查看: $(Join-Path $PackageDir '部署说明.md')"
    Write-Host ""
} else {
    Write-Red "打包失败！"
    exit 1
}
