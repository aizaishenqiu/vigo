package rabbitmq

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Config RabbitMQ 连接配置
type Config struct {
	Host           string
	Port           int
	User           string
	Password       string
	Vhost          string
	ConnTimeout    int // 秒
	Heartbeat      int // 秒
	ReconnectDelay int // 秒
	MaxRetries     int // 0=无限重试
}

// Client RabbitMQ 客户端（支持自动重连）
type Client struct {
	config Config
	dsn    string
	conn   *amqp.Connection
	mu     sync.RWMutex

	// 重连控制
	reconnecting bool
	closeChan    chan struct{}
	connected    bool
}

// New 创建 RabbitMQ 客户端实例（不立即连接）
func New(cfg Config) *Client {
	if cfg.Vhost == "" {
		cfg.Vhost = "/"
	}
	if cfg.ConnTimeout <= 0 {
		cfg.ConnTimeout = 5
	}
	if cfg.Heartbeat <= 0 {
		cfg.Heartbeat = 10
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = 3
	}

	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Vhost)

	return &Client{
		config:    cfg,
		dsn:       dsn,
		closeChan: make(chan struct{}),
	}
}

// NewFromDSN 使用 DSN 创建客户端（兼容旧API）
func NewFromDSN(dsn string) *Client {
	return &Client{
		dsn:       dsn,
		closeChan: make(chan struct{}),
		config: Config{
			ConnTimeout:    5,
			Heartbeat:      10,
			ReconnectDelay: 3,
		},
	}
}

// Connect 建立连接（带超时和重试）
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected && c.conn != nil && !c.conn.IsClosed() {
		return nil
	}

	err := c.dial()
	if err != nil {
		return fmt.Errorf("RabbitMQ 连接失败: %v", err)
	}

	c.connected = true

	// 启动连接监控（自动重连）
	go c.watchConnection()

	return nil
}

// dial 实际连接操作
func (c *Client) dial() error {
	amqpConfig := amqp.Config{
		Heartbeat: time.Duration(c.config.Heartbeat) * time.Second,
		Locale:    "en_US",
	}

	// 带超时的连接
	done := make(chan error, 1)
	go func() {
		conn, err := amqp.DialConfig(c.dsn, amqpConfig)
		if err != nil {
			done <- err
			return
		}
		c.conn = conn
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(time.Duration(c.config.ConnTimeout) * time.Second):
		return fmt.Errorf("连接超时（%d秒）", c.config.ConnTimeout)
	}
}

// watchConnection 监控连接状态，断线自动重连
func (c *Client) watchConnection() {
	for {
		select {
		case <-c.closeChan:
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			time.Sleep(time.Duration(c.config.ReconnectDelay) * time.Second)
			continue
		}

		// 等待连接关闭通知
		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case <-c.closeChan:
			return
		case amqpErr, ok := <-notifyClose:
			if !ok {
				return
			}

			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()

			if amqpErr != nil {
				log.Printf("[RabbitMQ] 连接断开: %v，准备重连...", amqpErr)
			} else {
				log.Printf("[RabbitMQ] 连接断开，准备重连...")
			}

			c.reconnect()
		}
	}
}

// reconnect 执行重连
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	retries := 0
	for {
		select {
		case <-c.closeChan:
			return
		default:
		}

		retries++
		if c.config.MaxRetries > 0 && retries > c.config.MaxRetries {
			log.Printf("[RabbitMQ] 已达最大重试次数(%d)，停止重连", c.config.MaxRetries)
			return
		}

		log.Printf("[RabbitMQ] 尝试重连 (第 %d 次)...", retries)

		c.mu.Lock()
		err := c.dial()
		if err == nil {
			c.connected = true
			c.mu.Unlock()
			log.Printf("[RabbitMQ] 重连成功!")
			return
		}
		c.mu.Unlock()

		log.Printf("[RabbitMQ] 重连失败: %v", err)

		// 指数退避，最大30秒
		delay := time.Duration(c.config.ReconnectDelay) * time.Second * time.Duration(retries)
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		time.Sleep(delay)
	}
}

// Publish 发布消息
func (c *Client) Publish(exchange, routingKey string, body []byte) error {
	c.mu.RLock()
	conn := c.conn
	isConnected := c.connected
	c.mu.RUnlock()

	if !isConnected || conn == nil || conn.IsClosed() {
		log.Printf("[RabbitMQ Mock] Publish to %s/%s: %s", exchange, routingKey, string(body))
		return fmt.Errorf("RabbitMQ 未连接")
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("创建Channel失败: %v", err)
	}
	defer ch.Close()

	return ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

// PublishToQueue 发布消息到指定队列（自动声明队列）
func (c *Client) PublishToQueue(queue string, body []byte) error {
	c.mu.RLock()
	conn := c.conn
	isConnected := c.connected
	c.mu.RUnlock()

	if !isConnected || conn == nil || conn.IsClosed() {
		return fmt.Errorf("RabbitMQ 未连接")
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("创建Channel失败: %v", err)
	}
	defer ch.Close()

	// 声明队列
	_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("声明队列失败: %v", err)
	}

	return ch.Publish("", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
		Timestamp:    time.Now(),
	})
}

// Consume 消费消息
func (c *Client) Consume(queue string, handler func([]byte)) error {
	c.mu.RLock()
	conn := c.conn
	isConnected := c.connected
	c.mu.RUnlock()

	if !isConnected || conn == nil || conn.IsClosed() {
		log.Printf("[RabbitMQ Mock] Start consuming queue: %s", queue)
		go func() {
			msg := []byte(`{"msg": "hello rabbit (mock)"}`)
			fmt.Printf("[RabbitMQ Mock] Received: %s\n", string(msg))
			handler(msg)
		}()
		return nil
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("创建Channel失败: %v", err)
	}

	// 声明队列
	_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("声明队列失败: %v", err)
	}

	// 设置 QoS
	if err := ch.Qos(10, 0, false); err != nil {
		return fmt.Errorf("设置QoS失败: %v", err)
	}

	msgs, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("消费队列失败: %v", err)
	}

	go func() {
		for d := range msgs {
			handler(d.Body)
			d.Ack(false)
		}
	}()

	return nil
}

// GetDSN 获取连接地址
func (c *Client) GetDSN() string {
	return c.dsn
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil && !c.conn.IsClosed()
}

// GetStatus 获取状态信息
func (c *Client) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := "down"
	if c.connected && c.conn != nil && !c.conn.IsClosed() {
		status = "up"
	}
	return map[string]interface{}{
		"status":       status,
		"dsn":          c.dsn,
		"reconnecting": c.reconnecting,
	}
}

// GetConfig 获取当前配置（隐藏密码）
func (c *Client) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"host":            c.config.Host,
		"port":            c.config.Port,
		"user":            c.config.User,
		"vhost":           c.config.Vhost,
		"conn_timeout":    c.config.ConnTimeout,
		"heartbeat":       c.config.Heartbeat,
		"reconnect_delay": c.config.ReconnectDelay,
		"max_retries":     c.config.MaxRetries,
	}
}

// GetChannel 获取一个临时 Channel（调用方负责关闭）
func (c *Client) GetChannel() (*amqp.Channel, error) {
	c.mu.RLock()
	conn := c.conn
	isConnected := c.connected
	c.mu.RUnlock()

	if !isConnected || conn == nil || conn.IsClosed() {
		return nil, fmt.Errorf("RabbitMQ 未连接")
	}
	return conn.Channel()
}

// ListQueues 获取真实队列列表
func (c *Client) ListQueues() ([]map[string]interface{}, error) {
	ch, err := c.GetChannel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	// AMQP 协议不支持列出所有队列，需要通过 Management HTTP API
	// 这里提供一个基于 HTTP API 的实现
	return c.listQueuesHTTP()
}

// managementAPI 调用 RabbitMQ Management HTTP API
func (c *Client) managementAPI(method, path string) ([]byte, error) {
	url := fmt.Sprintf("http://%s:%d%s", c.config.Host, 15672, path)
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.config.User, c.config.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("management api 不可用：%v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return nil, fmt.Errorf("management api 返回 %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// listQueuesHTTP 通过 Management HTTP API 获取队列列表
func (c *Client) listQueuesHTTP() ([]map[string]interface{}, error) {
	body, err := c.managementAPI("GET", "/api/queues")
	if err != nil {
		return nil, err
	}

	var queues []map[string]interface{}
	if err := json.Unmarshal(body, &queues); err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, q := range queues {
		result = append(result, map[string]interface{}{
			"name":      q["name"],
			"messages":  q["messages"],
			"consumers": q["consumers"],
			"state":     q["state"],
			"durable":   q["durable"],
		})
	}
	return result, nil
}

// ListExchanges 获取交换机列表
func (c *Client) ListExchanges() ([]map[string]interface{}, error) {
	body, err := c.managementAPI("GET", "/api/exchanges")
	if err != nil {
		return nil, err
	}

	var exchanges []map[string]interface{}
	if err := json.Unmarshal(body, &exchanges); err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, e := range exchanges {
		result = append(result, map[string]interface{}{
			"name":        e["name"],
			"type":        e["type"],
			"durable":     e["durable"],
			"auto_delete": e["auto_delete"],
		})
	}
	return result, nil
}

// GetOverview 获取 RabbitMQ 概览信息
func (c *Client) GetOverview() (map[string]interface{}, error) {
	body, err := c.managementAPI("GET", "/api/overview")
	if err != nil {
		return nil, err
	}
	var overview map[string]interface{}
	if err := json.Unmarshal(body, &overview); err != nil {
		return nil, err
	}
	return overview, nil
}

// DeclareQueue 声明队列
func (c *Client) DeclareQueue(name string, durable bool) error {
	ch, err := c.GetChannel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(name, durable, false, false, false, nil)
	return err
}

// DeleteQueue 删除队列
func (c *Client) DeleteQueue(name string) error {
	ch, err := c.GetChannel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDelete(name, false, false, false)
	return err
}

// PurgeQueue 清空队列消息
func (c *Client) PurgeQueue(name string) (int, error) {
	ch, err := c.GetChannel()
	if err != nil {
		return 0, err
	}
	defer ch.Close()

	return ch.QueuePurge(name, false)
}

// DeclareExchange 声明交换机
func (c *Client) DeclareExchange(name, kind string, durable bool) error {
	ch, err := c.GetChannel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.ExchangeDeclare(name, kind, durable, false, false, false, nil)
}

// DeleteExchange 删除交换机
func (c *Client) DeleteExchange(name string) error {
	ch, err := c.GetChannel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.ExchangeDelete(name, false, false)
}

// GetQueueInfo 获取单个队列信息
func (c *Client) GetQueueInfo(name string) (map[string]interface{}, error) {
	ch, err := c.GetChannel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	q, err := ch.QueueDeclarePassive(name, false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"name":      q.Name,
		"messages":  q.Messages,
		"consumers": q.Consumers,
	}, nil
}

// Close 优雅关闭连接
func (c *Client) Close() error {
	close(c.closeChan)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn.Close()
	}
	return nil
}
