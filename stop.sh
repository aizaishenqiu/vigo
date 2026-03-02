#!/bin/bash
# 服务停止脚本
APP_NAME="vigo"
if pgrep -f "$APP_NAME" > /dev/null; then
    pkill -f "$APP_NAME"
    echo "已停止 $APP_NAME"
else
    echo "$APP_NAME 未在运行"
fi
