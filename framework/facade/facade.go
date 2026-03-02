package facade

import (
	"vigo/framework/container"
	"vigo/framework/nacos"
	"vigo/framework/rabbitmq"
	"vigo/framework/redis"
)

// Facade 门面模式
// 在 Go 中，门面模式主要用于提供一个简洁的、全局可访问的接口来访问容器中的服务。
// 它可以极大地简化代码，让开发者不需要在每个地方都注入容器或手动从容器获取实例。

// Redis 获取 Redis 客户端门面
func Redis() *redis.Client {
	instance := container.App().Make("redis")
	if instance == nil {
		return nil
	}
	return instance.(*redis.Client)
}

// RabbitMQ 获取 RabbitMQ 客户端门面
func RabbitMQ() *rabbitmq.Client {
	instance := container.App().Make("rabbitmq")
	if instance == nil {
		return nil
	}
	return instance.(*rabbitmq.Client)
}

// Nacos 获取 Nacos 客户端门面
func Nacos() *nacos.Client {
	instance := container.App().Make("nacos")
	if instance == nil {
		return nil
	}
	return instance.(*nacos.Client)
}

// Config 获取配置项
func Config() interface{} {
	return container.App().Make("config")
}

// App 获取应用核心容器
func App() *container.Container {
	return container.App()
}
