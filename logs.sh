#!/bin/bash
# 日志查看脚本
APP_DIR="${APP_DIR:-/opt/vigo}"
LOG_DIR="$APP_DIR/runtime/log"
tail -f "$LOG_DIR/app.log" 2>/dev/null || tail -f runtime/log/*.log 2>/dev/null || echo "未找到日志文件"
