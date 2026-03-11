package controller

import (
	"net/http"
	"strconv"
	"vigo/framework/facade"
	"vigo/framework/mvc"
)

type RabbitMQController struct {
	BaseController
}

// Index 管理页面
func (r *RabbitMQController) Index(c *mvc.Context) {
	c.HTML(http.StatusOK, "rabbitmq/index.html", map[string]interface{}{
		"title": "RabbitMQ 管理中心",
	})
}

// Status 获取连接状态 + 概览
func (r *RabbitMQController) Status(c *mvc.Context) {
	mq := facade.RabbitMQ()
	if mq == nil {
		c.Success(map[string]interface{}{
			"connected": false,
			"config":    nil,
			"error":     "RabbitMQ 客户端未初始化",
		})
		return
	}

	result := map[string]interface{}{
		"connected": mq.IsConnected(),
		"config":    mq.GetConfig(),
		"status":    mq.GetStatus(),
	}

	if mq.IsConnected() {
		if overview, err := mq.GetOverview(); err == nil {
			result["version"] = overview["rabbitmq_version"]
			result["erlang_version"] = overview["erlang_version"]
			result["cluster_name"] = overview["cluster_name"]
			if objTotals, ok := overview["object_totals"].(map[string]interface{}); ok {
				result["totals"] = objTotals
			}
			if msgStats, ok := overview["message_stats"].(map[string]interface{}); ok {
				result["message_stats"] = msgStats
			}
		}
	}

	c.Success(result)
}

// Queues 获取队列列表
func (r *RabbitMQController) Queues(c *mvc.Context) {
	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		// 返回友好的错误信息
		c.Success(map[string]interface{}{
			"connected":   false,
			"queues":      []map[string]interface{}{},
			"message":     "RabbitMQ 未连接，请检查配置文件中的 RabbitMQ 设置并启动 RabbitMQ 服务",
			"config_hint": "请在 config.yaml 中配置 rabbitmq.host, rabbitmq.port, rabbitmq.user, rabbitmq.password",
		})
		return
	}

	queues, err := mq.ListQueues()
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success(queues)
}

// CreateQueue 创建队列
func (r *RabbitMQController) CreateQueue(c *mvc.Context) {
	name := c.Input("name")
	if name == "" {
		c.Error(http.StatusBadRequest, "队列名称不能为空")
		return
	}
	durable, _ := strconv.ParseBool(c.Input("durable"))

	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "RabbitMQ 未连接")
		return
	}

	if err := mq.DeclareQueue(name, durable); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("队列 " + name + " 创建成功")
}

// DeleteQueue 删除队列
func (r *RabbitMQController) DeleteQueue(c *mvc.Context) {
	name := c.Input("name")
	if name == "" {
		c.Error(http.StatusBadRequest, "队列名称不能为空")
		return
	}

	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "RabbitMQ 未连接")
		return
	}

	if err := mq.DeleteQueue(name); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("队列 " + name + " 已删除")
}

// PurgeQueue 清空队列
func (r *RabbitMQController) PurgeQueue(c *mvc.Context) {
	name := c.Input("name")
	if name == "" {
		c.Error(http.StatusBadRequest, "队列名称不能为空")
		return
	}

	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "RabbitMQ 未连接")
		return
	}

	count, err := mq.PurgeQueue(name)
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success(map[string]interface{}{
		"message": "队列 " + name + " 已清空",
		"purged":  count,
	})
}

// Exchanges 获取交换机列表
func (r *RabbitMQController) Exchanges(c *mvc.Context) {
	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		// 返回友好的错误信息
		c.Success(map[string]interface{}{
			"connected":   false,
			"exchanges":   []map[string]interface{}{},
			"message":     "RabbitMQ 未连接，请检查配置文件中的 RabbitMQ 设置并启动 RabbitMQ 服务",
			"config_hint": "请在 config.yaml 中配置 rabbitmq.host, rabbitmq.port, rabbitmq.user, rabbitmq.password",
		})
		return
	}

	exchanges, err := mq.ListExchanges()
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success(exchanges)
}

// CreateExchange 创建交换机
func (r *RabbitMQController) CreateExchange(c *mvc.Context) {
	name := c.Input("name")
	kind := c.Input("type")
	if name == "" {
		c.Error(http.StatusBadRequest, "交换机名称不能为空")
		return
	}
	if kind == "" {
		kind = "direct"
	}
	durable, _ := strconv.ParseBool(c.Input("durable"))

	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "RabbitMQ 未连接")
		return
	}

	if err := mq.DeclareExchange(name, kind, durable); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("交换机 " + name + " 创建成功")
}

// DeleteExchange 删除交换机
func (r *RabbitMQController) DeleteExchange(c *mvc.Context) {
	name := c.Input("name")
	if name == "" {
		c.Error(http.StatusBadRequest, "交换机名称不能为空")
		return
	}

	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "RabbitMQ 未连接")
		return
	}

	if err := mq.DeleteExchange(name); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("交换机 " + name + " 已删除")
}

// Publish 发布测试消息
func (r *RabbitMQController) Publish(c *mvc.Context) {
	queue := c.Input("queue")
	exchange := c.Input("exchange")
	routingKey := c.Input("routing_key")
	message := c.Input("message")

	if message == "" {
		c.Error(http.StatusBadRequest, "消息内容不能为空")
		return
	}

	mq := facade.RabbitMQ()
	if mq == nil || !mq.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "RabbitMQ 未连接")
		return
	}

	var err error
	if queue != "" {
		err = mq.PublishToQueue(queue, []byte(message))
	} else {
		if routingKey == "" {
			routingKey = "#"
		}
		err = mq.Publish(exchange, routingKey, []byte(message))
	}

	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("消息发送成功")
}
