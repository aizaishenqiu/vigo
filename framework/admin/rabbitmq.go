package admin

import (
	"log"
	"time"
	"vigo/framework/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQQueue RabbitMQ 队列信息
type RabbitMQQueue struct {
	Name      string `json:"name"`
	Messages  int    `json:"messages"`
	Consumers int    `json:"consumers"`
	Status    string `json:"status"`
	Ready     int    `json:"ready"`   // 就绪消息数
	Unacked   int    `json:"unacked"` // 未确认消息数
	Memory    int64  `json:"memory"`  // 内存使用
}

// RabbitMQExchange RabbitMQ 交换机信息
type RabbitMQExchange struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // direct, topic, fanout, headers
	Bindings   int    `json:"bindings"`
	Status     string `json:"status"`
	Durable    bool   `json:"durable"`
	AutoDelete bool   `json:"auto_delete"`
	Internal   bool   `json:"internal"`
}

// RabbitMQBinding RabbitMQ 绑定信息
type RabbitMQBinding struct {
	Queue      string                 `json:"queue"`
	Exchange   string                 `json:"exchange"`
	RoutingKey string                 `json:"routing_key"`
	Arguments  map[string]interface{} `json:"arguments"`
}

// RabbitMQConnection RabbitMQ 连接信息
type RabbitMQConnection struct {
	Name          string `json:"name"`
	State         string `json:"state"`
	Channels      int    `json:"channels"`
	ClientHost    string `json:"client_host"`
	ClientProduct string `json:"client_product"`
}

// getRabbitMQQueues 获取 RabbitMQ 队列列表
func getRabbitMQQueues() []RabbitMQQueue {
	queues := make([]RabbitMQQueue, 0)

	if rabbitmqClient == nil {
		log.Printf("[RabbitMQ] 客户端未设置")
		return queues
	}

	log.Printf("[RabbitMQ] 开始获取队列列表，客户端类型: %T", rabbitmqClient)

	// 类型断言获取客户端实例
	client, ok := rabbitmqClient.(*rabbitmq.Client)
	if !ok {
		log.Printf("[RabbitMQ] 客户端类型断言失败，实际类型: %T", rabbitmqClient)
		return queues
	}

	// 调用 Management API 获取队列列表
	queueList, err := client.ListQueues()
	if err != nil {
		log.Printf("[RabbitMQ] 获取队列列表失败：%v", err)
		return queues
	}

	log.Printf("[RabbitMQ] 成功获取 %d 个队列", len(queueList))

	// 转换为前端需要的格式
	for _, q := range queueList {
		name, _ := q["name"].(string)
		messages, _ := q["messages"].(float64)
		consumers, _ := q["consumers"].(float64)
		state, _ := q["state"].(string)

		log.Printf("[RabbitMQ] 队列: %s, 消息: %d, 消费者: %d, 状态: %s", name, int(messages), int(consumers), state)

		queues = append(queues, RabbitMQQueue{
			Name:      name,
			Messages:  int(messages),
			Consumers: int(consumers),
			Status:    state,
		})
	}

	return queues
}

// getRabbitMQExchanges 获取 RabbitMQ 交换机列表
func getRabbitMQExchanges() []RabbitMQExchange {
	exchanges := make([]RabbitMQExchange, 0)

	if rabbitmqClient == nil {
		return exchanges
	}

	client, ok := rabbitmqClient.(*rabbitmq.Client)
	if !ok {
		return exchanges
	}

	exchangeList, err := client.ListExchanges()
	if err != nil {
		return exchanges
	}

	for _, e := range exchangeList {
		name, _ := e["name"].(string)
		exType, _ := e["type"].(string)
		durable, _ := e["durable"].(bool)
		autoDelete, _ := e["auto_delete"].(bool)

		exchanges = append(exchanges, RabbitMQExchange{
			Name:       name,
			Type:       exType,
			Durable:    durable,
			AutoDelete: autoDelete,
			Status:     "up",
		})
	}

	return exchanges
}

// createRabbitMQQueue 创建 RabbitMQ 队列
func createRabbitMQQueue(name string, durable bool) error {
	if rabbitmqClient == nil {
		return nil
	}

	client, ok := rabbitmqClient.(*rabbitmq.Client)
	if !ok {
		return nil
	}

	err := client.DeclareQueue(name, durable)
	if err != nil {
		return err
	}

	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("rabbitmq", WSMessage{
			Type:    "rabbitmq_update",
			Channel: "rabbitmq",
			Action:  "create",
			Data: map[string]interface{}{
				"type": "queue",
				"name": name,
			},
		})
	}

	return nil
}

// deleteRabbitMQQueue 删除 RabbitMQ 队列
func deleteRabbitMQQueue(name string) error {
	if rabbitmqClient == nil {
		return nil
	}

	client, ok := rabbitmqClient.(*rabbitmq.Client)
	if !ok {
		return nil
	}

	err := client.DeleteQueue(name)
	if err != nil {
		return err
	}

	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("rabbitmq", WSMessage{
			Type:    "rabbitmq_update",
			Channel: "rabbitmq",
			Action:  "delete",
			Data: map[string]interface{}{
				"type": "queue",
				"name": name,
			},
		})
	}

	return nil
}

// purgeRabbitMQQueue 清空 RabbitMQ 队列
func purgeRabbitMQQueue(name string) error {
	if rabbitmqChannel == nil {
		return nil
	}

	_, err := rabbitmqChannel.QueuePurge(name, false)
	if err != nil {
		return err
	}

	return nil
}

// createRabbitMQExchange 创建 RabbitMQ 交换机
func createRabbitMQExchange(name, exType string, durable bool) error {
	if rabbitmqClient == nil {
		return nil
	}

	client, ok := rabbitmqClient.(*rabbitmq.Client)
	if !ok {
		return nil
	}

	err := client.DeclareExchange(name, exType, durable)
	if err != nil {
		return err
	}

	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("rabbitmq", WSMessage{
			Type:    "rabbitmq_update",
			Channel: "rabbitmq",
			Action:  "create",
			Data: map[string]interface{}{
				"type": "exchange",
				"name": name,
			},
		})
	}

	return nil
}

// deleteRabbitMQExchange 删除 RabbitMQ 交换机
func deleteRabbitMQExchange(name string) error {
	if rabbitmqClient == nil {
		return nil
	}

	client, ok := rabbitmqClient.(*rabbitmq.Client)
	if !ok {
		return nil
	}

	err := client.DeleteExchange(name)
	if err != nil {
		log.Printf("[RabbitMQ] 删除交换机失败：%v", err)
		return err
	}

	log.Printf("[RabbitMQ] 交换机已删除：%s", name)

	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("rabbitmq", WSMessage{
			Type:    "rabbitmq_update",
			Channel: "rabbitmq",
			Action:  "delete",
			Data: map[string]interface{}{
				"type": "exchange",
				"name": name,
			},
		})
	}

	return nil
}

// bindRabbitMQQueue 绑定队列到交换机
func bindRabbitMQQueue(queueName, exchangeName, routingKey string) error {
	if rabbitmqChannel == nil {
		return nil
	}

	err := rabbitmqChannel.QueueBind(
		queueName,    // 队列名称
		routingKey,   // 路由键
		exchangeName, // 交换机名称
		false,        // 是否等待
		nil,          // 参数
	)

	if err != nil {
		log.Printf("[RabbitMQ] 绑定队列失败：%v", err)
		return err
	}

	log.Printf("[RabbitMQ] 队列已绑定：%s -> %s (%s)", queueName, exchangeName, routingKey)

	return nil
}

// unbindRabbitMQQueue 解绑队列
func unbindRabbitMQQueue(queueName, exchangeName, routingKey string) error {
	if rabbitmqChannel == nil {
		return nil
	}

	err := rabbitmqChannel.QueueUnbind(
		queueName,    // 队列名称
		routingKey,   // 路由键
		exchangeName, // 交换机名称
		nil,          // 参数
	)

	if err != nil {
		log.Printf("[RabbitMQ] 解绑队列失败：%v", err)
		return err
	}

	log.Printf("[RabbitMQ] 队列已解绑：%s <- %s (%s)", queueName, exchangeName, routingKey)

	return nil
}

// getRabbitMQConnections 获取 RabbitMQ 连接列表
func getRabbitMQConnections() []RabbitMQConnection {
	connections := make([]RabbitMQConnection, 0)

	// 需要通过 Management API 获取
	// 这里返回空列表

	return connections
}

// handleRabbitMQUpdate 处理 RabbitMQ WebSocket 更新
func handleRabbitMQUpdate(msg WSMessage) {
	switch msg.Action {
	case "create_queue":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			name := getString(data, "name")
			durable := getBool(data, "durable")
			createRabbitMQQueue(name, durable)
		}
	case "delete_queue":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			name := getString(data, "name")
			deleteRabbitMQQueue(name)
		}
	case "create_exchange":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			name := getString(data, "name")
			exType := getString(data, "type")
			durable := getBool(data, "durable")
			createRabbitMQExchange(name, exType, durable)
		}
	case "delete_exchange":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			name := getString(data, "name")
			deleteRabbitMQExchange(name)
		}
	case "bind_queue":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			queue := getString(data, "queue")
			exchange := getString(data, "exchange")
			routingKey := getString(data, "routing_key")
			bindRabbitMQQueue(queue, exchange, routingKey)
		}
	}
}

// 辅助函数
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// 全局 RabbitMQ 通道引用
var rabbitmqChannel *amqp.Channel
var rabbitmqClient interface{} // RabbitMQ 客户端

// SetRabbitMQChannel 设置 RabbitMQ 通道
func SetRabbitMQChannel(ch *amqp.Channel) {
	rabbitmqChannel = ch
	log.Printf("[RabbitMQ] 通道已设置")
}

// SetRabbitMQClient 设置 RabbitMQ 客户端
func SetRabbitMQClient(client interface{}) {
	rabbitmqClient = client
	log.Printf("[RabbitMQ] 客户端已设置")
}

// 启动 RabbitMQ 监控协程
func startRabbitMQMonitor() {
	if rabbitmqChannel == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// 定期检查队列和交换机状态
			queues := getRabbitMQQueues()
			exchanges := getRabbitMQExchanges()

			if GlobalWSManager != nil {
				GlobalWSManager.BroadcastToChannel("rabbitmq", WSMessage{
					Type:    "rabbitmq_update",
					Channel: "rabbitmq",
					Data: map[string]interface{}{
						"queues":    queues,
						"exchanges": exchanges,
					},
				})
			}
		}
	}()
}
