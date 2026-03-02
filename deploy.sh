#!/bin/bash
# Linux 一键部署脚本
set -e

APP_NAME="vigo"
APP_DIR="${APP_DIR:-/opt/vigo}"
LOG_DIR="$APP_DIR/runtime/log"

echo "=== 开始部署 $APP_NAME ==="

mkdir -p "$APP_DIR" "$LOG_DIR"

# 备份旧版本
if [ -f "$APP_DIR/$APP_NAME" ]; then
    cp "$APP_DIR/$APP_NAME" "$APP_DIR/${APP_NAME}.bak"
fi

# 复制新文件
cp -f "$APP_NAME" "$APP_DIR/" 2>/dev/null || cp -f ./vigo "$APP_DIR/" 2>/dev/null || true
cp -f config.yaml "$APP_DIR/" 2>/dev/null || true
[ -d app/view ] && cp -r app/view "$APP_DIR/app/" 2>/dev/null || mkdir -p "$APP_DIR/app" && cp -r app/view "$APP_DIR/app/" 2>/dev/null || true
[ -d public ] && cp -r public "$APP_DIR/" 2>/dev/null || true

chmod +x "$APP_DIR/$APP_NAME" 2>/dev/null || true

# 停止旧服务
if pgrep -f "$APP_NAME" > /dev/null; then
    echo "停止旧服务..."
    pkill -f "$APP_NAME" || true
    sleep 2
fi

# 启动新服务
echo "启动新服务..."
cd "$APP_DIR"
nohup ./$APP_NAME > "$LOG_DIR/app.log" 2>&1 &

echo "=== 部署完成 ==="
echo "查看日志: tail -f $LOG_DIR/app.log"
