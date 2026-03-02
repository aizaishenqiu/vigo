#!/bin/bash
# ==============================================
# Vigo 应用停止脚本
# 适用于 CentOS 7.x 及以上版本
# ==============================================

# 获取脚本所在目录
APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR"

# 应用名称
APP_NAME="vigo"
# PID 文件路径
PID_FILE="$APP_DIR/runtime/$APP_NAME.pid"

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
# 停止应用
# ==============================================
stop_app() {
    if [ ! -f "$PID_FILE" ]; then
        echo_yellow "PID 文件不存在，应用可能未运行"
        return 1
    fi

    local pid=$(cat "$PID_FILE" 2>/dev/null)
    if [ -z "$pid" ]; then
        echo_yellow "PID 文件为空，应用可能未运行"
        rm -f "$PID_FILE"
        return 1
    fi

    # 检查进程是否存在
    if ! kill -0 "$pid" 2>/dev/null; then
        echo_yellow "进程 $pid 不存在，清理 PID 文件"
        rm -f "$PID_FILE"
        return 1
    fi

    echo "正在停止 $APP_NAME (PID: $pid) ..."

    # 优雅停止：发送 SIGTERM 信号
    kill -15 "$pid"

    # 等待进程结束
    local count=0
    local max_wait=30
    while kill -0 "$pid" 2>/dev/null; do
        sleep 1
        count=$((count + 1))
        echo -n "."
        if [ $count -ge $max_wait ]; then
            echo ""
            echo_yellow "进程未响应，强制杀死..."
            kill -9 "$pid" 2>/dev/null
            sleep 2
            break
        fi
    done

    # 清理 PID 文件
    rm -f "$PID_FILE"

    # 再次确认进程是否已停止
    if kill -0 "$pid" 2>/dev/null; then
        echo_red "停止失败，进程仍在运行"
        return 1
    else
        echo ""
        echo_green "应用已停止"
        return 0
    fi
}

# ==============================================
# 主程序
# ==============================================
stop_app
