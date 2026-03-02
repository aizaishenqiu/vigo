#!/bin/bash
# 服务重启脚本
APP_NAME="vigo"
APP_DIR="${APP_DIR:-/opt/vigo}"
LOG_DIR="$APP_DIR/runtime/log"

if pgrep -f "$APP_NAME" > /dev/null; then
    echo "停止 $APP_NAME..."
    pkill -f "$APP_NAME" || true
    sleep 2
fi

echo "启动 $APP_NAME..."
cd "$APP_DIR"
nohup ./$APP_NAME >> "$LOG_DIR/app.log" 2>&1 &
echo "已启动，日志: tail -f $LOG_DIR/app.log"
