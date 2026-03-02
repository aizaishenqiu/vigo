#!/bin/bash
# ==============================================
# Vigo 应用打包脚本
# 用于生成完整的部署包
# ==============================================

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# 版本号
VERSION="1.0.1"
# 打包目录
PACKAGE_DIR="$PROJECT_DIR/dist/vigo-$VERSION-linux-amd64"

# ==============================================
# 颜色输出函数
# ==============================================
echo_green() {
    echo -e "\033[32m$1\033[0m"
}

echo_red() {
    echo -e "\033[31m$1\033[0m"
}

echo_yellow() {
    echo -e "\033[33m$1\033[0m"
}

# ==============================================
# 清理旧的打包文件
# ==============================================
echo "清理旧的打包文件..."
rm -rf "$PROJECT_DIR/dist"
mkdir -p "$PACKAGE_DIR"

# ==============================================
# 编译 Linux 版本
# ==============================================
echo "编译 Linux amd64 版本..."
export GOOS=linux
export GOARCH=amd64
go build -o "$PACKAGE_DIR/vigo-linux-amd64" .

if [ $? -ne 0 ]; then
    echo_red "编译失败！"
    exit 1
fi

echo_green "编译成功！"

# ==============================================
# 复制配置文件
# ==============================================
echo "复制配置文件..."
cp "$PROJECT_DIR/config.yaml" "$PACKAGE_DIR/"

# ==============================================
# 复制部署脚本
# ==============================================
echo "复制部署脚本..."
cp "$SCRIPT_DIR/start.sh" "$PACKAGE_DIR/"
cp "$SCRIPT_DIR/stop.sh" "$PACKAGE_DIR/"
cp "$SCRIPT_DIR/restart.sh" "$PACKAGE_DIR/"
cp "$SCRIPT_DIR/vigo.service" "$PACKAGE_DIR/"
cp "$SCRIPT_DIR/部署说明.md" "$PACKAGE_DIR/"

# ==============================================
# 创建必要的目录
# ==============================================
echo "创建目录结构..."
mkdir -p "$PACKAGE_DIR/runtime"
mkdir -p "$PACKAGE_DIR/runtime/log"

# ==============================================
# 设置权限
# ==============================================
echo "设置文件权限..."
chmod +x "$PACKAGE_DIR/vigo-linux-amd64"
chmod +x "$PACKAGE_DIR/start.sh"
chmod +x "$PACKAGE_DIR/stop.sh"
chmod +x "$PACKAGE_DIR/restart.sh"

# ==============================================
# 打包为 tar.gz
# ==============================================
echo "打包为 tar.gz..."
cd "$PROJECT_DIR/dist"
tar -zcvf "vigo-$VERSION-linux-amd64.tar.gz" "vigo-$VERSION-linux-amd64"

if [ $? -eq 0 ]; then
    echo ""
    echo_green "=============================================="
    echo_green "打包完成！"
    echo_green "=============================================="
    echo ""
    echo "部署包位置: $PROJECT_DIR/dist/vigo-$VERSION-linux-amd64.tar.gz"
    echo "解压目录: $PACKAGE_DIR"
    echo ""
    echo "部署步骤:"
    echo "  1. 上传 vigo-$VERSION-linux-amd64.tar.gz 到服务器"
    echo "  2. 解压: tar -zxvf vigo-$VERSION-linux-amd64.tar.gz"
    echo "  3. 配置环境变量"
    echo "  4. 启动: ./start.sh"
    echo ""
    echo "详细说明请查看: $PACKAGE_DIR/部署说明.md"
    echo ""
else
    echo_red "打包失败！"
    exit 1
fi
