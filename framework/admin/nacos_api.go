package admin

import (
	"strconv"
	"vigo/framework/facade"
	"vigo/framework/mvc"
)

// nacosStatus 获取连接状态
func nacosStatus(c *mvc.Context) {
	nc := facade.Nacos()
	if nc == nil {
		c.Json(200, map[string]interface{}{
			"code": 0,
			"msg":  "success",
			"data": map[string]interface{}{
				"connected": false,
				"config":    nil,
				"error":     "Nacos 客户端未初始化",
			},
		})
		return
	}

	healthy := nc.CheckHealth()
	result := map[string]interface{}{
		"connected": healthy,
		"config":    nc.GetConfigInfo(),
	}

	if healthy {
		if namespaces, err := nc.ListNamespaces(); err == nil {
			result["namespaces"] = namespaces
		}
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": result,
	})
}

// nacosConfig 获取配置
func nacosConfig(c *mvc.Context) {
	nc := facade.Nacos()
	if nc == nil {
		c.Json(503, map[string]interface{}{
			"code": 503,
			"msg":  "Nacos 客户端未初始化",
		})
		return
	}

	dataId := c.Request.URL.Query().Get("data_id")
	group := c.Request.URL.Query().Get("group")

	if dataId == "" {
		// 如果没有 dataId，返回当前应用的配置
		content, err := nc.GetConfig()
		if err != nil {
			c.Json(500, map[string]interface{}{
				"code": 500,
				"msg":  err.Error(),
			})
			return
		}
		c.Json(200, map[string]interface{}{
			"code": 0,
			"msg":  "success",
			"data": map[string]interface{}{
				"content": content,
				"data_id": nc.GetConfigInfo()["data_id"],
				"group":   nc.GetConfigInfo()["group"],
			},
		})
		return
	}

	// 获取指定配置
	content, err := nc.GetConfigByID(dataId, group)
	if err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"content": content,
			"data_id": dataId,
			"group":   group,
		},
	})
}

// nacosConfigPublish 发布配置
func nacosConfigPublish(c *mvc.Context) {
	dataId := c.Input("data_id")
	group := c.Input("group")
	content := c.Input("content")

	if dataId == "" || content == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "data_id 和 content 不能为空",
		})
		return
	}

	nc := facade.Nacos()
	if nc == nil {
		c.Json(503, map[string]interface{}{
			"code": 503,
			"msg":  "Nacos 客户端未初始化",
		})
		return
	}

	if err := nc.PublishConfig(dataId, group, content); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "配置发布成功",
	})
}

// nacosConfigDelete 删除配置
func nacosConfigDeleteV2(c *mvc.Context) {
	dataId := c.Input("data_id")
	group := c.Input("group")

	if dataId == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "data_id 不能为空",
		})
		return
	}

	nc := facade.Nacos()
	if nc == nil {
		c.Json(503, map[string]interface{}{
			"code": 503,
			"msg":  "Nacos 客户端未初始化",
		})
		return
	}

	if err := nc.DeleteConfig(dataId, group); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "配置已删除",
	})
}

// nacosServices 获取服务列表
func nacosServices(c *mvc.Context) {
	nc := facade.Nacos()
	if nc == nil || !nc.IsConnected() {
		c.Json(503, map[string]interface{}{
			"code": 503,
			"msg":  "Nacos 未连接",
		})
		return
	}

	page, _ := strconv.Atoi(c.Request.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(c.Request.URL.Query().Get("page_size"))

	result, err := nc.ListServices(page, pageSize)
	if err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": result,
	})
}

// nacosInstances 获取服务实例
func nacosInstances(c *mvc.Context) {
	serviceName := c.Request.URL.Query().Get("service")
	if serviceName == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "service 名称不能为空",
		})
		return
	}

	nc := facade.Nacos()
	if nc == nil || !nc.IsConnected() {
		c.Json(503, map[string]interface{}{
			"code": 503,
			"msg":  "Nacos 未连接",
		})
		return
	}

	result, err := nc.GetServiceInstances(serviceName)
	if err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": result,
	})
}

// nacosRegisterService 注册服务实例
func nacosRegisterService(c *mvc.Context) {
	ip := c.Input("ip")
	portStr := c.Input("port")
	serviceName := c.Input("service")

	if ip == "" || portStr == "" || serviceName == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "ip, port, service 不能为空",
		})
		return
	}

	port, _ := strconv.ParseUint(portStr, 10, 64)

	nc := facade.Nacos()
	if nc == nil || !nc.IsConnected() {
		c.Json(503, map[string]interface{}{
			"code": 503,
			"msg":  "Nacos 未连接",
		})
		return
	}

	if err := nc.RegisterInstance(ip, port, serviceName); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}
	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "服务实例注册成功",
	})
}
