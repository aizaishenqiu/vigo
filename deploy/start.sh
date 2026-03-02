#!/bin/bash
# ==============================================
# Vigo 应用启动脚本
# 适用于 CentOS 7.x 及以上版本
# ==============================================

# 获取脚本所在目录
APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR"

# 应用名称
APP_NAME="vigo"
# 可执行文件路径
BINARY_FILE="$APP_DIR/vigo-linux-amd64"
# PID 文件路径
PID_FILE="$APP_DIR/runtime/$APP_NAME.pid"
# 日志文件路径
LOG_FILE="$APP_DIR/runtime/app.log"

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
# 检查进程是否运行
# ==============================================
check_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            return 0
        else
            rm -f "$PID_FILE"
        fi
    fi
    return 1
}

# ==============================================
# 启动应用
# ==============================================
start_app() {
    if check_running; then
        echo_yellow "应用已在运行，PID: $(cat $PID_FILE)"
        return 1
    fi

    # 创建必要的目录
    mkdir -p "$APP_DIR/runtime"
    mkdir -p "$APP_DIR/runtime/log"

    # 检查可执行文件是否存在
    if [ ! -f "$BINARY_FILE" ]; then
        echo_red "错误: 可执行文件不存在: $BINARY_FILE"
        return 1
    fi

    # 检查配置文件是否存在
    if [ ! -f "$APP_DIR/config.yaml" ]; then
        echo_red "错误: 配置文件不存在: $APP_DIR/config.yaml"
        return 1
    fi

    # 赋予执行权限
    chmod +x "$BINARY_FILE"

    echo "正在启动 $APP_NAME ..."
    
    # 后台启动应用
    nohup "$BINARY_FILE" > "$LOG_FILE" 2>&1 &
    local pid=$!

    # 等待进程启动
    sleep 2

    # 检查进程是否启动成功
    if kill -0 "$pid" 2>/dev/null; then
        echo "$pid" > "$PID_FILE"
        echo_green "应用启动成功！"
        echo_green "PID: $pid"
        echo_green "日志文件: $LOG_FILE"
        echo ""
        echo "查看日志: tail -f $LOG_FILE"
        echo "停止应用: $APP_DIR/stop.sh"
        return 0
    else
        echo_red "应用启动失败，请查看日志: $LOG_FILE"
        tail -20 "$LOG_FILE" 2>/dev/null
        return 1
    fi
}

# ==============================================
# 主程序
# ==============================================
start_app
