package admin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"vigo/config"
	"vigo/framework/mvc"
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
