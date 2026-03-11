package admin

import (
	"log"

	"vigo/framework/nacos"
)

// NacosConfigItem Nacos 配置项
type NacosConfigItem struct {
	DataID     string `json:"data_id"`
	Group      string `json:"group"`
	Content    string `json:"content"`
	Md5        string `json:"md5"`
	UpdateTime string `json:"update_time"`
	Format     string `json:"format"` // yaml, json, properties
}

// NacosServiceItem Nacos 服务项
type NacosServiceItem struct {
	ServiceName string   `json:"service_name"`
	HealthCount int      `json:"health_count"`
	TotalCount  int      `json:"total_count"`
	Status      string   `json:"status"`
	Clusters    []string `json:"clusters"`
}

// NacosConfigReq Nacos 配置请求
type NacosConfigReq struct {
	DataID  string `json:"data_id"`
	Group   string `json:"group"`
	Content string `json:"content"`
	Format  string `json:"format"`
}

// getNacosConfigs 获取 Nacos 配置列表
func getNacosConfigs() []NacosConfigItem {
	configs := make([]NacosConfigItem, 0)

	if nacosClient == nil {
		// Nacos 未配置，返回空数组，前端会显示配置引导
		return configs
	}

	// 检查 Nacos 是否连接
	if !nacosClient.IsConnected() {
		log.Printf("[Nacos] 客户端未连接")
		return configs
	}

	// 获取配置的 DataID 和 Group（从配置文件中读取）
	configInfo := nacosClient.GetConfigInfo()
	dataID, _ := configInfo["data_id"].(string)
	group, _ := configInfo["group"].(string)

	if dataID == "" {
		return configs
	}

	// 获取配置内容
	content, err := nacosClient.GetConfig()
	if err != nil {
		log.Printf("[Nacos] 获取配置失败：%v", err)
		return configs
	}

	// 构建配置项
	configs = append(configs, NacosConfigItem{
		DataID:     dataID,
		Group:      group,
		Content:    content,
		UpdateTime: "-",
		Format:     "yaml",
	})

	return configs
}

// createNacosConfig 创建 Nacos 配置
func createNacosConfig(req NacosConfigReq) error {
	if nacosClient == nil {
		return nil
	}

	// 使用 Nacos 客户端发布配置
	err := nacosClient.PublishConfig(req.DataID, req.Group, req.Content)
	if err != nil {
		log.Printf("[Nacos] 创建配置失败：%v", err)
		return err
	}

	log.Printf("[Nacos] 配置已创建：%s/%s", req.DataID, req.Group)

	// 通知所有订阅者
	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("nacos", WSMessage{
			Type:    "nacos_update",
			Channel: "nacos",
			Action:  "create",
			Data:    req,
		})
	}

	return nil
}

// updateNacosConfig 更新 Nacos 配置
func updateNacosConfig(req NacosConfigReq) error {
	if nacosClient == nil {
		return nil
	}

	// 使用 Nacos 客户端发布配置
	err := nacosClient.PublishConfig(req.DataID, req.Group, req.Content)
	if err != nil {
		log.Printf("[Nacos] 更新配置失败：%v", err)
		return err
	}

	log.Printf("[Nacos] 配置已更新：%s/%s", req.DataID, req.Group)

	// 通知所有订阅者
	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("nacos", WSMessage{
			Type:    "nacos_update",
			Channel: "nacos",
			Action:  "update",
			Data:    req,
		})
	}

	return nil
}

// deleteNacosConfig 删除 Nacos 配置
func deleteNacosConfig(dataID, group string) error {
	if nacosClient == nil {
		return nil
	}

	err := nacosClient.DeleteConfig(dataID, group)
	if err != nil {
		log.Printf("[Nacos] 删除配置失败：%v", err)
		return err
	}

	log.Printf("[Nacos] 配置已删除：%s/%s", dataID, group)

	// 通知所有订阅者
	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("nacos", WSMessage{
			Type:    "nacos_update",
			Channel: "nacos",
			Action:  "delete",
			Data: map[string]string{
				"data_id": dataID,
				"group":   group,
			},
		})
	}

	return nil
}

// getNacosServices 获取 Nacos 服务列表
func getNacosServices() []NacosServiceItem {
	services := make([]NacosServiceItem, 0)

	if nacosClient == nil {
		// Nacos 未配置，返回空数组，前端会显示配置引导
		return services
	}

	// 检查 Nacos 是否连接
	if !nacosClient.IsConnected() {
		log.Printf("[Nacos] 客户端未连接")
		return services
	}

	// 调用 Nacos API 获取服务列表
	result, err := nacosClient.ListServices(1, 100)
	if err != nil {
		log.Printf("[Nacos] 获取服务列表失败：%v", err)
		return services
	}

	// 解析服务列表
	dom, ok := result["dom"]
	if !ok {
		return services
	}

	serviceList, ok := dom.([]interface{})
	if !ok {
		return services
	}

	// 遍历服务列表
	for _, svc := range serviceList {
		serviceMap, ok := svc.(map[string]interface{})
		if !ok {
			continue
		}

		serviceName, _ := serviceMap["name"].(string)
		clusterMap, _ := serviceMap["clusters"].([]interface{})

		clusters := make([]string, 0)
		for _, c := range clusterMap {
			if clusterName, ok := c.(string); ok {
				clusters = append(clusters, clusterName)
			}
		}

		// 获取服务实例
		instancesResult, err := nacosClient.GetServiceInstances(serviceName)
		healthCount := 0
		totalCount := 0
		if err == nil {
			hosts, ok := instancesResult["hosts"].([]interface{})
			if ok {
				totalCount = len(hosts)
				for _, host := range hosts {
					hostMap, ok := host.(map[string]interface{})
					if ok {
						healthy, _ := hostMap["healthy"].(bool)
						if healthy {
							healthCount++
						}
					}
				}
			}
		}

		status := "down"
		if healthCount > 0 {
			status = "up"
		}

		services = append(services, NacosServiceItem{
			ServiceName: serviceName,
			HealthCount: healthCount,
			TotalCount:  totalCount,
			Status:      status,
			Clusters:    clusters,
		})
	}

	return services
}

// handleNacosUpdate 处理 Nacos WebSocket 更新
func handleNacosUpdate(msg WSMessage) {
	switch msg.Action {
	case "create":
		var req NacosConfigReq
		if data, ok := msg.Data.(map[string]interface{}); ok {
			req.DataID = getString(data, "data_id")
			req.Group = getString(data, "group")
			req.Content = getString(data, "content")
			req.Format = getString(data, "format")
			createNacosConfig(req)
		}
	case "update":
		var req NacosConfigReq
		if data, ok := msg.Data.(map[string]interface{}); ok {
			req.DataID = getString(data, "data_id")
			req.Group = getString(data, "group")
			req.Content = getString(data, "content")
			req.Format = getString(data, "format")
			updateNacosConfig(req)
		}
	case "delete":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			dataID := getString(data, "data_id")
			group := getString(data, "group")
			deleteNacosConfig(dataID, group)
		}
	}
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// 全局 Nacos 客户端引用
var nacosClient *nacos.Client

// SetNacosClient 设置 Nacos 客户端
func SetNacosClient(client *nacos.Client) {
	nacosClient = client
	log.Printf("[Nacos] 客户端已设置")
}
