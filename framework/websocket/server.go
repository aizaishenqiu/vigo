package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ==================== 可配置的 WebSocket 参数 ====================

// Config WebSocket 配置
type Config struct {
	WriteWait      time.Duration // 写入超时（默认 10s）
	PongWait       time.Duration // 等待 Pong 超时（默认 60s）
	PingPeriod     time.Duration // Ping 间隔（默认 pongWait * 9/10）
	MaxMessageSize int64         // 最大消息尺寸（默认 4096）
	MaxConnections int           // 最大连接数（默认 0=不限）
	AllowedOrigins []string      // 允许的源（空=全部允许）
}

// DefaultConfig 默认 WebSocket 配置
func DefaultConfig() Config {
	return Config{
		WriteWait:      10 * time.Second,
		PongWait:       60 * time.Second,
		PingPeriod:     54 * time.Second,
		MaxMessageSize: 4096,
		MaxConnections: 0,
	}
}

var globalConfig = DefaultConfig()

// SetConfig 设置全局 WebSocket 配置
func SetConfig(cfg Config) {
	if cfg.WriteWait > 0 {
		globalConfig.WriteWait = cfg.WriteWait
	}
	if cfg.PongWait > 0 {
		globalConfig.PongWait = cfg.PongWait
		if cfg.PingPeriod <= 0 {
			globalConfig.PingPeriod = (cfg.PongWait * 9) / 10
		}
	}
	if cfg.PingPeriod > 0 {
		globalConfig.PingPeriod = cfg.PingPeriod
	}
	if cfg.MaxMessageSize > 0 {
		globalConfig.MaxMessageSize = cfg.MaxMessageSize
	}
	if cfg.MaxConnections > 0 {
		globalConfig.MaxConnections = cfg.MaxConnections
	}
	if len(cfg.AllowedOrigins) > 0 {
		globalConfig.AllowedOrigins = cfg.AllowedOrigins
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		if len(globalConfig.AllowedOrigins) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		for _, o := range globalConfig.AllowedOrigins {
			if o == "*" || o == origin {
				return true
			}
		}
		return false
	},
}

// ==================== 通信协议 ====================

// Message 客户端/服务端通信消息格式
type Message struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ==================== Client ====================

// Client 表示一个 WebSocket 连接
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	channels map[string]bool
	mu       sync.RWMutex
	userID   string // 认证后的用户 ID（可选）
}

// UserID 获取认证后的用户 ID
func (c *Client) UserID() string {
	return c.userID
}

// readPump 持续读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(globalConfig.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(globalConfig.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(globalConfig.PongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[WS] 读取错误: %v", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			c.mu.Lock()
			c.channels[msg.Channel] = true
			c.mu.Unlock()
		case "unsubscribe":
			c.mu.Lock()
			delete(c.channels, msg.Channel)
			c.mu.Unlock()
		case "command":
			c.hub.Command <- CommandMessage{
				Channel: msg.Channel,
				Data:    msg.Data,
				Client:  c,
			}
		}
	}
}

// writePump 持续向客户端写数据 + 心跳
func (c *Client) writePump() {
	ticker := time.NewTicker(globalConfig.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(globalConfig.WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(globalConfig.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// IsSubscribed 检查客户端是否订阅了某频道
func (c *Client) IsSubscribed(channel string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channels[channel]
}

// ==================== CommandMessage ====================

type CommandMessage struct {
	Channel string
	Data    json.RawMessage
	Client  *Client
}

// ==================== Hub ====================

// AuthFunc WebSocket 认证函数类型
// 参数: HTTP 请求
// 返回: 用户 ID（空字符串表示匿名/未认证）, 是否允许连接, 错误信息
type AuthFunc func(r *http.Request) (userID string, allowed bool, err error)

// Hub 连接管理中心
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan ChannelMessage
	Command    chan CommandMessage
	mu         sync.RWMutex

	commandHandlers map[string]func(data json.RawMessage, reply func(interface{}))
	handlerMu       sync.RWMutex

	authFunc AuthFunc // 可选的认证函数
}

// ChannelMessage 频道消息
type ChannelMessage struct {
	Channel string
	Data    []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:         make(map[*Client]bool),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		broadcast:       make(chan ChannelMessage, 256),
		Command:         make(chan CommandMessage, 64),
		commandHandlers: make(map[string]func(data json.RawMessage, reply func(interface{}))),
	}
}

// SetAuth 设置 WebSocket 认证函数
func (h *Hub) SetAuth(fn AuthFunc) {
	h.authFunc = fn
}

// Run 启动 Hub 事件循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WS] 客户端连接 (在线: %d)", h.ClientCount())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WS] 客户端断开 (在线: %d)", h.ClientCount())

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if client.IsSubscribed(msg.Channel) {
					select {
					case client.send <- msg.Data:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
			h.mu.RUnlock()

		case cmd := <-h.Command:
			h.handlerMu.RLock()
			handler, ok := h.commandHandlers[cmd.Channel]
			h.handlerMu.RUnlock()
			if ok {
				reply := func(data interface{}) {
					resp, _ := json.Marshal(Message{
						Type:    "response",
						Channel: cmd.Channel,
						Data:    mustMarshal(data),
					})
					select {
					case cmd.Client.send <- resp:
					default:
					}
				}
				go handler(cmd.Data, reply)
			}
		}
	}
}

// BroadcastToChannel 向指定频道广播
func (h *Hub) BroadcastToChannel(channel string, msgType string, data interface{}) {
	payload, err := json.Marshal(Message{
		Type:    msgType,
		Channel: channel,
		Data:    mustMarshal(data),
	})
	if err != nil {
		return
	}
	select {
	case h.broadcast <- ChannelMessage{Channel: channel, Data: payload}:
	default:
	}
}

// OnCommand 注册频道指令处理器
func (h *Hub) OnCommand(channel string, handler func(data json.RawMessage, reply func(interface{}))) {
	h.handlerMu.Lock()
	defer h.handlerMu.Unlock()
	h.commandHandlers[channel] = handler
}

// ClientCount 返回当前连接数
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HasSubscribers 检查频道是否有订阅者
func (h *Hub) HasSubscribers(channel string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.IsSubscribed(channel) {
			return true
		}
	}
	return false
}

// ==================== HTTP Handler ====================

// Handler WebSocket 升级处理器（支持认证和连接限制）
func Handler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// 连接数限制
	if globalConfig.MaxConnections > 0 && hub.ClientCount() >= globalConfig.MaxConnections {
		http.Error(w, "连接数已达上限", http.StatusServiceUnavailable)
		return
	}

	// 认证检查
	var userID string
	if hub.authFunc != nil {
		uid, allowed, err := hub.authFunc(r)
		if !allowed {
			errMsg := "WebSocket 认证失败"
			if err != nil {
				errMsg = err.Error()
			}
			http.Error(w, errMsg, http.StatusUnauthorized)
			return
		}
		userID = uid
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] 升级失败: %v", err)
		return
	}

	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		channels: make(map[string]bool),
		userID:   userID,
	}

	// 自动订阅 URL 查询参数中指定的频道
	if ch := r.URL.Query().Get("channel"); ch != "" {
		for _, c := range strings.Split(ch, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				client.channels[c] = true
			}
		}
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

// ==================== 工具函数 ====================

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return data
}
