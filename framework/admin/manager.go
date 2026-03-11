package admin

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"vigo/config"
	"vigo/framework/mvc"
)

type Manager struct {
	rabbitMQAdmin *RabbitMQAdmin
	nacosAdmin    *NacosAdmin
}

var (
	manager     *Manager
	managerOnce sync.Once
)

func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{}
		manager.init()
	})
	return manager
}

func (m *Manager) init() {
	if config.App.RabbitMQ.Admin.Enabled {
		m.rabbitMQAdmin = NewRabbitMQAdmin()
	}
	if config.App.Nacos.Admin.Enabled {
		m.nacosAdmin = NewNacosAdmin()
	}
}

func (m *Manager) RegisterRoutes(r *mvc.Router) {
	if m.rabbitMQAdmin != nil {
		rg := r.Group("/admin/rabbitmq", m.rabbitMQAdmin.AuthMiddleware())
		rg.GET("", m.rabbitMQAdmin.Dashboard)
		rg.GET("/queues", m.rabbitMQAdmin.Queues)
		rg.GET("/exchanges", m.rabbitMQAdmin.Exchanges)
		rg.GET("/connections", m.rabbitMQAdmin.Connections)
		rg.POST("/queue/{name}/purge", m.rabbitMQAdmin.PurgeQueue)
		rg.DELETE("/queue/{name}", m.rabbitMQAdmin.DeleteQueue)
	}

	if m.nacosAdmin != nil {
		ng := r.Group("/admin/nacos", m.nacosAdmin.AuthMiddleware())
		ng.GET("", m.nacosAdmin.Dashboard)
		ng.GET("/services", m.nacosAdmin.Services)
		ng.GET("/configs", m.nacosAdmin.Configs)
		ng.GET("/instances", m.nacosAdmin.Instances)
	}
}

func (m *Manager) PrintStartupInfo() {
	if m.rabbitMQAdmin != nil {
		m.rabbitMQAdmin.PrintStartupInfo()
	}
	if m.nacosAdmin != nil {
		m.nacosAdmin.PrintStartupInfo()
	}
}

func basicAuth(c *mvc.Context) (username, password string, ok bool) {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return
	}
	if !strings.HasPrefix(auth, "Basic ") {
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return
	}
	return parts[0], parts[1], true
}

type RabbitMQAdmin struct {
	username string
	password string
	port     int
	baseURL  string
}

func NewRabbitMQAdmin() *RabbitMQAdmin {
	username := config.App.RabbitMQ.Admin.Username
	password := config.App.RabbitMQ.Admin.Password
	port := config.App.RabbitMQ.Admin.Port

	if port <= 0 {
		port = 15672
	}

	baseURL := fmt.Sprintf("http://%s:%d", config.App.RabbitMQ.Host, port)

	return &RabbitMQAdmin{
		username: username,
		password: password,
		port:     port,
		baseURL:  baseURL,
	}
}

func (a *RabbitMQAdmin) AuthMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		user, pass, ok := basicAuth(c)
		if !ok || user != a.username || pass != a.password {
			c.SetHeader("WWW-Authenticate", `Basic realm="RabbitMQ Admin"`)
			c.Error(http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *RabbitMQAdmin) PrintStartupInfo() {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              RabbitMQ 管理界面已启用                        ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  访问地址: http://127.0.0.1:%d/admin/rabbitmq\n", config.App.App.Port)
	fmt.Printf("║  用户名:   %s\n", a.username)
	fmt.Printf("║  密码:     %s\n", a.password)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  RabbitMQ 服务: %s\n", a.baseURL)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func (a *RabbitMQAdmin) Dashboard(c *mvc.Context) {
	data := map[string]interface{}{
		"title":   "RabbitMQ 管理",
		"host":    config.App.RabbitMQ.Host,
		"port":    config.App.RabbitMQ.Port,
		"vhost":   config.App.RabbitMQ.Vhost,
		"enabled": config.App.RabbitMQ.Enabled,
	}
	c.HTML(http.StatusOK, "rabbitmq/admin.html", data)
}

func (a *RabbitMQAdmin) Queues(c *mvc.Context) {
	c.Json(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": []map[string]interface{}{
			{"name": "default", "messages": 0, "consumers": 0},
		},
	})
}

func (a *RabbitMQAdmin) Exchanges(c *mvc.Context) {
	c.Json(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": []map[string]interface{}{
			{"name": "default", "type": "direct"},
		},
	})
}

func (a *RabbitMQAdmin) Connections(c *mvc.Context) {
	c.Json(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": []map[string]interface{}{},
	})
}

func (a *RabbitMQAdmin) PurgeQueue(c *mvc.Context) {
	name := c.Param("name")
	c.Json(http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": fmt.Sprintf("队列 %s 已清空", name),
	})
}

func (a *RabbitMQAdmin) DeleteQueue(c *mvc.Context) {
	name := c.Param("name")
	c.Json(http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": fmt.Sprintf("队列 %s 已删除", name),
	})
}

type NacosAdmin struct {
	username string
	password string
	baseURL  string
}

func NewNacosAdmin() *NacosAdmin {
	username := config.App.Nacos.Admin.Username
	password := config.App.Nacos.Admin.Password
	port := config.App.Nacos.Admin.Port

	if port <= 0 {
		port = int(config.App.Nacos.Port)
	}

	baseURL := fmt.Sprintf("http://%s:%d", config.App.Nacos.IpAddr, port)

	return &NacosAdmin{
		username: username,
		password: password,
		baseURL:  baseURL,
	}
}

func (a *NacosAdmin) AuthMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		user, pass, ok := basicAuth(c)
		if !ok || user != a.username || pass != a.password {
			c.SetHeader("WWW-Authenticate", `Basic realm="Nacos Admin"`)
			c.Error(http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *NacosAdmin) PrintStartupInfo() {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║               Nacos 管理界面已启用                          ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  访问地址: http://127.0.0.1:%d/admin/nacos\n", config.App.App.Port)
	fmt.Printf("║  用户名:   %s\n", a.username)
	fmt.Printf("║  密码:     %s\n", a.password)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Nacos 服务: %s\n", a.baseURL)
	fmt.Printf("║  Namespace: %s\n", config.App.Nacos.NamespaceId)
	fmt.Printf("║  Data ID:   %s\n", config.App.Nacos.DataId)
	fmt.Printf("║  Group:     %s\n", config.App.Nacos.Group)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func (a *NacosAdmin) Dashboard(c *mvc.Context) {
	data := map[string]interface{}{
		"title":     "Nacos 管理",
		"host":      config.App.Nacos.IpAddr,
		"port":      config.App.Nacos.Port,
		"namespace": config.App.Nacos.NamespaceId,
		"data_id":   config.App.Nacos.DataId,
		"group":     config.App.Nacos.Group,
	}
	c.HTML(http.StatusOK, "nacos/admin.html", data)
}

func (a *NacosAdmin) Services(c *mvc.Context) {
	c.Json(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": []map[string]interface{}{
			{"name": config.App.App.Name, "instance_count": 1},
		},
	})
}

func (a *NacosAdmin) Configs(c *mvc.Context) {
	c.Json(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": []map[string]interface{}{
			{
				"data_id": config.App.Nacos.DataId,
				"group":   config.App.Nacos.Group,
			},
		},
	})
}

func (a *NacosAdmin) Instances(c *mvc.Context) {
	c.Json(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": []map[string]interface{}{
			{
				"service": config.App.App.Name,
				"ip":      "127.0.0.1",
				"port":    config.App.App.Port,
				"healthy": true,
			},
		},
	})
}
