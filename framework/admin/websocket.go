package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"vigo/framework/mvc"
	"vigo/framework/websocket"
)

// ==================== WebSocket 管理器 ====================

// WSManager WebSocket 管理器
type WSManager struct {
	hub *websocket.Hub
}

// WSMessage WebSocket 消息结构
type WSMessage struct {
	Type      string      `json:"type"`      // 消息类型：system_stats, nacos_update, rabbitmq_update, stress_progress, health_status
	Action    string      `json:"action"`    // 操作类型：subscribe, unsubscribe, update, delete, create
	Channel   string      `json:"channel"`   // 频道：system, nacos, rabbitmq, stress, health
	Data      interface{} `json:"data"`      // 数据内容
	Timestamp int64       `json:"timestamp"` // 时间戳
}

// GlobalWSManager 全局 WebSocket 管理器
var GlobalWSManager *WSManager

// InitWSManager 初始化 WebSocket 管理器
func InitWSManager() {
	hub := websocket.NewHub()

	GlobalWSManager = &WSManager{
		hub: hub,
	}

	// 启动 Hub
	go hub.Run()

	// 启动数据推送协程
	go pushSystemStats(hub)
	go pushHealthStatus(hub)

	log.Printf("[WS] WebSocket 管理器已初始化")
}

// pushSystemStats 推送系统统计（每 2 秒）
func pushSystemStats(hub *websocket.Hub) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := getRealSystemStats()
		hub.BroadcastToChannel("system", "system_stats", stats)
	}
}

// pushHealthStatus 推送健康状态（每 5 秒）
func pushHealthStatus(hub *websocket.Hub) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		health := getRealHealthStatus()
		hub.BroadcastToChannel("health", "health_status", health)
	}
}

// BroadcastToChannel 广播消息到指定频道
func (m *WSManager) BroadcastToChannel(channel string, message WSMessage) {
	message.Timestamp = time.Now().UnixNano() / 1e6

	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	m.hub.BroadcastToChannel(channel, message.Type, json.RawMessage(data))
}

// GetClientCount 获取客户端数量
func (m *WSManager) GetClientCount() int {
	return m.hub.ClientCount()
}

// WSHandler WebSocket 连接处理
func WSHandler(c *mvc.Context) {
	// 直接使用 c.Writer，它已经是 StatusWriter，实现了 Hijacker 接口
	websocket.Handler(GlobalWSManager.hub, c.Writer, c.Request)
}

// getChannelData 获取频道数据
func getChannelData(channel string) interface{} {
	switch channel {
	case "system":
		return getRealSystemStats()
	case "health":
		return getRealHealthStatus()
	default:
		return nil
	}
}

// handleWSUpdate 处理 WebSocket 更新
func handleWSUpdate(msg WSMessage) {
	switch msg.Channel {
	case "stress":
		// 处理压力测试命令
		if msg.Action == "start" {
			var req StressTestReq
			data, _ := json.Marshal(msg.Data)
			json.Unmarshal(data, &req)
			// 创建测试进度
			progress := NewStressProgress(req)
			// 生成测试 ID
			testID := fmt.Sprintf("stress_%d", time.Now().UnixNano()/1e6)
			GlobalStressManager.runStressTest(testID, req, progress)
		}
	}
}
