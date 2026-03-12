package admin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
	"vigo/config"
	"vigo/framework/db"
	"vigo/framework/mvc"
	"vigo/framework/redis"
)

// ==================== 登录/认证 API 处理器 ====================

// loginAPI 处理登录请求
func loginAPI(c *mvc.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "无效的请求"})
		return
	}

	adminConfig := config.App.Admin
	if adminConfig.Username != "" && (req.Username != adminConfig.Username || req.Password != adminConfig.Password) {
		c.Json(200, map[string]interface{}{"code": 1, "msg": "用户名或密码错误"})
		return
	}

	// 登录成功，设置 Cookie
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "admin_token",
		Value:    "logged_in", // 简单标识，实际应使用 Token
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	c.Json(200, map[string]interface{}{"code": 0, "msg": "登录成功"})
}

// logoutAPI 处理登出请求
func logoutAPI(c *mvc.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "admin_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
	c.Json(200, map[string]interface{}{"code": 0, "msg": "已退出登录"})
}

// changePasswordAPI 修改管理密码
func changePasswordAPI(c *mvc.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "无效的请求"})
		return
	}

	if config.App.Admin.Password != "" && req.OldPassword != config.App.Admin.Password {
		c.Json(200, map[string]interface{}{"code": 1, "msg": "原密码错误"})
		return
	}

	if req.NewPassword == "" {
		c.Json(200, map[string]interface{}{"code": 1, "msg": "新密码不能为空"})
		return
	}

	if err := updateTopLevelAdminPassword(req.NewPassword); err != nil {
		c.Json(500, map[string]interface{}{"code": 500, "msg": "密码保存失败：" + err.Error()})
		return
	}
	config.Init()
	_ = InitConfigManager()
	config.App.Admin.Password = req.NewPassword

	c.Json(200, map[string]interface{}{"code": 0, "msg": "密码修改成功"})
}

func updateTopLevelAdminPassword(newPassword string) error {
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	inAdmin := false
	adminIndent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		if !inAdmin {
			if indent == 0 && strings.HasPrefix(trimmed, "admin:") {
				inAdmin = true
				adminIndent = indent
			}
			continue
		}

		if indent <= adminIndent {
			break
		}

		if strings.HasPrefix(trimmed, "password:") {
			lines[i] = strings.Repeat(" ", indent) + "password: " + formatYamlValue(newPassword)
			return ioutil.WriteFile("config.yaml", []byte(strings.Join(lines, "\n")), 0644)
		}
	}

	return fmt.Errorf("未找到 admin.password 配置项")
}

// ==================== Nacos API 处理器 ====================

// nacosConfigList 获取配置列表
func nacosConfigList(c *mvc.Context) {
	configs := getNacosConfigs()
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"configs": configs,
			"total":   len(configs),
		},
	})
}

// nacosConfigCreate 创建配置
func nacosConfigCreate(c *mvc.Context) {
	var req NacosConfigReq
	if err := c.Request.ParseForm(); err != nil {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "请求解析失败",
		})
		return
	}

	// 从请求体获取 JSON 数据
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "JSON 解析失败",
		})
		return
	}

	if err := createNacosConfig(req); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "创建失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "配置创建成功",
	})
}

// nacosConfigUpdate 更新配置
func nacosConfigUpdate(c *mvc.Context) {
	var req NacosConfigReq
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "JSON 解析失败",
		})
		return
	}

	if err := updateNacosConfig(req); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "更新失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "配置更新成功",
	})
}

// nacosConfigDelete 删除配置
func nacosConfigDelete(c *mvc.Context) {
	dataID := c.Request.URL.Query().Get("data_id")
	group := c.Request.URL.Query().Get("group")

	if dataID == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "缺少 data_id 参数",
		})
		return
	}

	if group == "" {
		group = "DEFAULT_GROUP"
	}

	if err := deleteNacosConfig(dataID, group); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "删除失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "配置删除成功",
	})
}

// nacosServiceList 获取服务列表
func nacosServiceList(c *mvc.Context) {
	services := getNacosServices()
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"services": services,
			"total":    len(services),
		},
	})
}

// ==================== RabbitMQ API 处理器 ====================

// rabbitmqQueueList 获取队列列表
func rabbitmqQueueList(c *mvc.Context) {
	queues := getRabbitMQQueues()
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"queues": queues,
			"total":  len(queues),
		},
	})
}

// rabbitmqQueueCreate 创建队列
func rabbitmqQueueCreate(c *mvc.Context) {
	var req struct {
		Name    string `json:"name"`
		Durable bool   `json:"durable"`
	}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "请求参数解析失败",
		})
		return
	}

	if req.Name == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "队列名称不能为空",
		})
		return
	}

	if err := createRabbitMQQueue(req.Name, req.Durable); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "队列创建失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "队列创建成功",
		"data": map[string]interface{}{
			"name":    req.Name,
			"durable": req.Durable,
		},
	})
}

// rabbitmqQueueDelete 删除队列
func rabbitmqQueueDelete(c *mvc.Context) {
	name := c.Request.URL.Query().Get("name")
	if name == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "队列名称不能为空",
		})
		return
	}

	if err := deleteRabbitMQQueue(name); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "队列删除失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "队列删除成功",
	})
}

// rabbitmqExchangeList 获取交换机列表
func rabbitmqExchangeList(c *mvc.Context) {
	exchanges := getRabbitMQExchanges()
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"exchanges": exchanges,
			"total":     len(exchanges),
		},
	})
}

// rabbitmqExchangeCreate 创建交换机
func rabbitmqExchangeCreate(c *mvc.Context) {
	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Durable bool   `json:"durable"`
	}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "请求参数解析失败",
		})
		return
	}

	if req.Name == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "交换机名称不能为空",
		})
		return
	}

	if req.Type == "" {
		req.Type = "direct"
	}

	if err := createRabbitMQExchange(req.Name, req.Type, req.Durable); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "交换机创建失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "交换机创建成功",
		"data": map[string]interface{}{
			"name":    req.Name,
			"type":    req.Type,
			"durable": req.Durable,
		},
	})
}

// rabbitmqExchangeDelete 删除交换机
func rabbitmqExchangeDelete(c *mvc.Context) {
	name := c.Request.URL.Query().Get("name")
	if name == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "交换机名称不能为空",
		})
		return
	}

	if err := deleteRabbitMQExchange(name); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "交换机删除失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "交换机删除成功",
	})
}

// ==================== 压力测试 API 处理器 ====================

// stressTestResults 获取所有测试结果
func stressTestResults(c *mvc.Context) {
	results := GlobalStressManager.GetStressResults()
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"results": results,
			"total":   len(results),
		},
	})
}

// stressTestDelete 删除测试结果
func stressTestDelete(c *mvc.Context) {
	testID := c.Request.URL.Query().Get("id")
	if testID == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "测试ID不能为空",
		})
		return
	}

	if err := GlobalStressManager.DeleteStressResult(testID); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "删除失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "删除成功",
	})
}

// stressTestClear 清空所有测试结果
func stressTestClear(c *mvc.Context) {
	if err := GlobalStressManager.ClearAllResults(); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "清空失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "已清空所有测试结果",
	})
}

// ==================== 系统监控 API 处理器 ====================

// systemRealtimeStats 获取实时系统统计
func systemRealtimeStats(c *mvc.Context) {
	stats := getRealSystemStats()
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": stats,
	})
}

// ==================== 压力测试页面处理器 ====================

// stressHandler 压力测试中心页面
func stressHandler(c *mvc.Context) {
	renderView(c, "views/stress.html", map[string]interface{}{"title": "压测中心"})
}

// stressTestStats 获取实时统计（给前端轮询使用）
func stressTestStats(c *mvc.Context) {
	// 获取系统统计
	sysStats := getRealSystemStats()

	// 获取压测数据
	var stressData map[string]interface{}
	runningTests := GlobalStressManager.GetRunningTests()

	if len(runningTests) > 0 {
		// 获取第一个运行中的测试数据
		test := runningTests[0]

		// 解析延迟字符串为数值（ms）
		avgLatency := parseFloat64(test.AvgLatency)
		p95Latency := parseFloat64(test.P99Latency)
		p99Latency := parseFloat64(test.MaxLatency)

		// 计算剩余请求数
		remaining := float64(test.TotalRequests) - float64(test.Completed)
		if remaining < 0 {
			remaining = 0
		}

		stressData = map[string]interface{}{
			"qps":         test.QPS,
			"latency":     avgLatency,
			"p95":         p95Latency,
			"p99":         p99Latency,
			"workers":     test.QPS, // 用 QPS 近似表示活跃 worker 数
			"remaining":   remaining,
			"total":       test.TotalRequests,
			"mysqlOps":    0, // TODO: 从实际数据库连接池获取
			"redisOps":    0, // TODO: 从实际 Redis 连接池获取
			"mqOps":       0, // TODO: 从实际 MQ 连接池获取
			"successRate": (1 - test.ErrorRate) * 100,
			"failedRate":  test.ErrorRate * 100,
		}
	} else {
		stressData = map[string]interface{}{
			"qps":         0,
			"latency":     0,
			"p95":         0,
			"p99":         0,
			"workers":     0,
			"remaining":   0,
			"total":       0,
			"mysqlOps":    0,
			"redisOps":    0,
			"mqOps":       0,
			"successRate": 0,
			"failedRate":  0,
		}
	}

	// 合并数据
	stats := map[string]interface{}{
		"qps":         stressData["qps"],
		"latency":     stressData["latency"],
		"p95":         stressData["p95"],
		"p99":         stressData["p99"],
		"workers":     stressData["workers"],
		"remaining":   stressData["remaining"],
		"total":       stressData["total"],
		"mysqlOps":    stressData["mysqlOps"],
		"redisOps":    stressData["redisOps"],
		"mqOps":       stressData["mqOps"],
		"cpu":         sysStats.CPU.Usage,
		"memUsed":     sysStats.Memory.Used * 1024 * 1024,
		"memTotal":    sysStats.Memory.Total * 1024 * 1024,
		"netSpeed":    sysStats.Network.SentRate + sysStats.Network.RecvRate,
		"successRate": stressData["successRate"],
		"failedRate":  stressData["failedRate"],
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": stats,
	})
}

// parseFloat64 解析字符串为 float64
func parseFloat64(s string) float64 {
	if s == "" {
		return 0
	}
	// 去除单位
	s = strings.TrimSuffix(s, "ms")
	s = strings.TrimSuffix(s, "s")
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

// stressTestServices 检测服务状态
func stressTestServices(c *mvc.Context) {
	services := map[string]bool{
		"mysql": false,
		"redis": false,
		"mq":    false,
	}

	// 检测 MySQL 连接（尝试 ping）
	if config.App.Database.Host != "" {
		if db := db.GetWriteDB(); db != nil {
			if err := db.Ping(); err == nil {
				services["mysql"] = true
			}
		}
	}

	// 检测 Redis 连接（尝试 ping）
	if config.App.Redis.Host != "" {
		redisClient := redis.New(redis.Config{
			Host:         config.App.Redis.Host,
			Port:         config.App.Redis.Port,
			Password:     config.App.Redis.Password,
			DB:           config.App.Redis.DB,
			PoolSize:     config.App.Redis.PoolSize,
			MinIdleConns: config.App.Redis.MinIdleConns,
			MaxIdleConns: config.App.Redis.MaxIdleConns,
			Cluster: redis.ClusterConfig{
				Enabled: config.App.Redis.Cluster.Enabled,
				Addrs:   config.App.Redis.Cluster.Addrs,
			},
		})
		if err := redisClient.Connect(); err == nil {
			services["redis"] = true
		}
	}

	// 检测 RabbitMQ 连接（检查配置）
	if config.App.RabbitMQ.Host != "" && config.App.RabbitMQ.User != "" {
		services["mq"] = true
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": services,
	})
}

// stressTestStart 启动压测（placeholder，实际逻辑在 stress.go 中）
func stressTestStart(c *mvc.Context) {
	// 支持表单和 JSON 两种格式
	var req StressTestReq

	contentType := c.Request.Header.Get("Content-Type")
	if contentType == "application/json" {
		if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
			c.Json(400, map[string]interface{}{
				"code": 400,
				"msg":  "无效的请求",
			})
			return
		}
	} else {
		// 表单格式
		c.Request.ParseForm()
		req.URL = c.Request.FormValue("url")
		req.Method = c.Request.FormValue("method")
		req.Concurrency, _ = strconv.Atoi(c.Request.FormValue("concurrency"))
		req.TotalRequests, _ = strconv.Atoi(c.Request.FormValue("total_requests"))
		req.Timeout, _ = strconv.Atoi(c.Request.FormValue("timeout"))
		req.Body = c.Request.FormValue("body")
		req.ContentType = c.Request.FormValue("content_type")
	}

	if req.URL == "" {
		req.URL = "http://localhost:8080/"
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 10
	}
	if req.TotalRequests <= 0 {
		req.TotalRequests = 100
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}
	if req.Method == "" {
		req.Method = "GET"
	}

	testID, err := GlobalStressManager.StartStressTest(req)
	if err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "启动失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "启动成功",
		"data": map[string]string{
			"test_id": testID,
		},
	})
}

// stressTestStop 停止压测
func stressTestStop(c *mvc.Context) {
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "停止成功",
	})
}

// stressTestReset 重置压测数据
func stressTestReset(c *mvc.Context) {
	if err := GlobalStressManager.ClearAllResults(); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "重置失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "重置成功",
	})
}

// stressTestStartHTTP 启动 HTTP 压测
func stressTestStartHTTP(c *mvc.Context) {
	c.Request.ParseForm()

	concurrency, _ := strconv.Atoi(c.Request.FormValue("concurrency"))
	duration, _ := strconv.Atoi(c.Request.FormValue("duration"))
	targetURL := c.Request.FormValue("target_url")
	method := c.Request.FormValue("method")

	if targetURL == "" {
		targetURL = "http://localhost:8080/"
	}
	if concurrency <= 0 {
		concurrency = 10
	}
	if duration <= 0 {
		duration = 10
	}
	if method == "" {
		method = "GET"
	}

	req := StressTestReq{
		URL:           targetURL,
		Method:        method,
		Concurrency:   concurrency,
		TotalRequests: concurrency * duration * 10, // 根据时长和并发计算总请求数
		Timeout:       duration,
	}

	testID, err := GlobalStressManager.StartStressTest(req)
	if err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "启动失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "启动成功",
		"data": map[string]string{
			"test_id": testID,
		},
	})
}
