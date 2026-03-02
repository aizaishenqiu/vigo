package controller

import (
	"net/http"
	"vigo/framework/facade"
	"vigo/framework/mvc"
	"strconv"
)

type NacosController struct {
	BaseController
}

// Index 管理页面
func (n *NacosController) Index(c *mvc.Context) {
	c.HTML(http.StatusOK, "nacos/index.html", map[string]interface{}{
		"title": "Nacos 服务管理中心",
	})
}

// Status 获取连接状态
func (n *NacosController) Status(c *mvc.Context) {
	nc := facade.Nacos()
	if nc == nil {
		c.Success(map[string]interface{}{
			"connected": false,
			"config":    nil,
			"error":     "Nacos 客户端未初始化",
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

	c.Success(result)
}

// GetConfig 获取配置
func (n *NacosController) GetConfig(c *mvc.Context) {
	nc := facade.Nacos()
	if nc == nil {
		c.Error(http.StatusServiceUnavailable, "Nacos 客户端未初始化")
		return
	}

	dataId := c.Input("data_id")
	group := c.Input("group")

	if dataId == "" {
		content, err := nc.GetConfig()
		if err != nil {
			c.Error(http.StatusInternalServerError, err.Error())
			return
		}
		c.Success(map[string]interface{}{
			"content": content,
			"data_id": nc.GetConfigInfo()["data_id"],
			"group":   nc.GetConfigInfo()["group"],
		})
		return
	}

	content, err := nc.GetConfigByID(dataId, group)
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success(map[string]interface{}{
		"content": content,
		"data_id": dataId,
		"group":   group,
	})
}

// PublishConfig 发布配置
func (n *NacosController) PublishConfig(c *mvc.Context) {
	dataId := c.Input("data_id")
	group := c.Input("group")
	content := c.Input("content")

	if dataId == "" || content == "" {
		c.Error(http.StatusBadRequest, "data_id 和 content 不能为空")
		return
	}

	nc := facade.Nacos()
	if nc == nil {
		c.Error(http.StatusServiceUnavailable, "Nacos 客户端未初始化")
		return
	}

	if err := nc.PublishConfig(dataId, group, content); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("配置发布成功")
}

// DeleteConfig 删除配置
func (n *NacosController) DeleteConfig(c *mvc.Context) {
	dataId := c.Input("data_id")
	group := c.Input("group")

	if dataId == "" {
		c.Error(http.StatusBadRequest, "data_id 不能为空")
		return
	}

	nc := facade.Nacos()
	if nc == nil {
		c.Error(http.StatusServiceUnavailable, "Nacos 客户端未初始化")
		return
	}

	if err := nc.DeleteConfig(dataId, group); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("配置已删除")
}

// Services 获取服务列表
func (n *NacosController) Services(c *mvc.Context) {
	nc := facade.Nacos()
	if nc == nil || !nc.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "Nacos 未连接")
		return
	}

	page, _ := strconv.Atoi(c.Input("page"))
	pageSize, _ := strconv.Atoi(c.Input("page_size"))

	result, err := nc.ListServices(page, pageSize)
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success(result)
}

// Instances 获取服务实例
func (n *NacosController) Instances(c *mvc.Context) {
	serviceName := c.Input("service")
	if serviceName == "" {
		c.Error(http.StatusBadRequest, "service 名称不能为空")
		return
	}

	nc := facade.Nacos()
	if nc == nil || !nc.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "Nacos 未连接")
		return
	}

	result, err := nc.GetServiceInstances(serviceName)
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success(result)
}

// RegisterService 注册服务实例
func (n *NacosController) RegisterService(c *mvc.Context) {
	ip := c.Input("ip")
	portStr := c.Input("port")
	serviceName := c.Input("service")

	if ip == "" || portStr == "" || serviceName == "" {
		c.Error(http.StatusBadRequest, "ip, port, service 不能为空")
		return
	}

	port, _ := strconv.ParseUint(portStr, 10, 64)

	nc := facade.Nacos()
	if nc == nil || !nc.IsConnected() {
		c.Error(http.StatusServiceUnavailable, "Nacos 未连接")
		return
	}

	if err := nc.RegisterInstance(ip, port, serviceName); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.Success("服务实例注册成功")
}
