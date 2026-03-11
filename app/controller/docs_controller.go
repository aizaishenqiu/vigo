package controller

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"vigo/framework/mvc"
)

// DocsController 文档控制器
type DocsController struct {
	BaseController
}

// Show 文档首页（Vue 应用）
func (c *DocsController) Show(ctx *mvc.Context) {
	c.Init(ctx)
	c.View("docs/view.html", map[string]interface{}{
		"Title": "Vigo Framework - 文档",
	})
}

// APIGetContent 获取文档内容（API 接口）
func (c *DocsController) APIGetContent(ctx *mvc.Context) {
	file := ctx.Input("file")
	if file == "" {
		ctx.Json(400, map[string]interface{}{
			"code":    400,
			"message": "缺少文件路径参数",
		})
		return
	}

	// 安全校验：防止目录遍历攻击
	if filepath.IsAbs(file) || filepath.Clean(file) != file {
		ctx.Json(400, map[string]interface{}{
			"code":    400,
			"message": "无效的文件路径",
		})
		return
	}

	// 构建完整路径
	fullPath := filepath.Join(".", "使用文档", file)

	// 检查文件是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			ctx.Json(404, map[string]interface{}{
				"code":    404,
				"message": "文档不存在",
			})
			return
		}
		ctx.Json(500, map[string]interface{}{
			"code":    500,
			"message": "读取文件失败",
		})
		return
	}

	// 检查是否是文件
	if info.IsDir() {
		ctx.Json(400, map[string]interface{}{
			"code":    400,
			"message": "路径必须是文件",
		})
		return
	}

	// 读取文件内容
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		ctx.Json(500, map[string]interface{}{
			"code":    500,
			"message": "读取文件内容失败",
		})
		return
	}

	// 返回文档内容
	ctx.Json(200, map[string]interface{}{
		"code":    200,
		"message": "success",
		"data": map[string]string{
			"content": string(content),
			"path":    file,
		},
	})
}

// APIList 获取文档目录结构
func (c *DocsController) APIList(ctx *mvc.Context) {
	// 这里可以返回文档目录结构
	// 暂时返回静态数据
	ctx.Json(200, map[string]interface{}{
		"code":    200,
		"message": "success",
		"data": []map[string]interface{}{
			{
				"name":  "入门指南",
				"path":  "getting-started",
				"icon":  "🚀",
				"items": []string{"框架简介", "快速开始", "配置文件"},
			},
			{
				"name":  "核心功能",
				"path":  "core-features",
				"icon":  "⚡",
				"items": []string{"路由与控制器", "配置管理", "视图与模板"},
			},
		},
	})
}
