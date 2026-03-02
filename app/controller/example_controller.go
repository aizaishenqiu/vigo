package controller

import (
	"net/http"
	"strings"
	"vigo/framework/mvc"
	"vigo/framework/request"
)

// ExampleController 展示 Request 安全模块的使用示例
type ExampleController struct {
	BaseController
}

// Index 首页
func (e *ExampleController) Index(c *mvc.Context) {
	e.Init(c)

	data := map[string]interface{}{
		"Title": "Request 安全模块示例",
	}
	c.HTML(http.StatusOK, "example/index.html", data)
}

// CreateUser 创建用户（展示基本用法）
func (e *ExampleController) CreateUser(c *mvc.Context) {

	// 使用 request 模块获取数据（自动安全验证）
	username := request.Input(c, "username", "")
	email := request.Input(c, "email", "")
	password := request.Input(c, "password", "")
	age := request.InputInt(c, "age", 0)
	phone := request.Input(c, "phone", "")

	// 验证必填字段
	if username == "" {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "用户名不能为空",
			"success": false,
		})
		return
	}

	// 验证邮箱格式
	if !validateEmail(email) {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "邮箱格式不正确",
			"success": false,
		})
		return
	}

	// 验证手机号格式
	if phone != "" && !validatePhone(phone) {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "手机号格式不正确",
			"success": false,
		})
		return
	}

	// 验证年龄范围
	if age < 18 || age > 150 {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "年龄必须在 18-150 之间",
			"success": false,
		})
		return
	}

	// 所有数据都是安全的，可以直接使用
	// 注意：request 模块已经自动过滤了 SQL 注入和 XSS 攻击
	user := map[string]interface{}{
		"username": username,
		"email":    email,
		"password": password, // 实际应用中需要加密
		"age":      age,
		"phone":    phone,
	}

	// TODO: 保存到数据库

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "用户创建成功",
		"success": true,
		"data":    user,
	})
}

// Search 搜索功能（展示如何处理搜索）
func (e *ExampleController) Search(c *mvc.Context) {

	// 获取搜索参数（已自动过滤 XSS 和 SQL 注入）
	keyword := request.Input(c, "keyword", "")
	category := request.Input(c, "category", "all")
	page := request.InputInt(c, "page", 1)
	pageSize := request.InputInt(c, "page_size", 10)
	sortBy := request.Input(c, "sort_by", "created_at")
	order := request.Input(c, "order", "DESC")

	// 验证分类（白名单）
	allowedCategories := []string{"all", "tech", "life", "work"}
	if !containsString(allowedCategories, category) {
		category = "all"
	}

	// 验证排序字段（白名单）
	allowedSortFields := []string{"created_at", "updated_at", "views", "likes"}
	if !containsString(allowedSortFields, sortBy) {
		sortBy = "created_at"
	}

	// 验证排序方向
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	// 验证分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 构建查询（数据已净化，安全）
	// 注意：这里可以直接使用 keyword，因为它已经被净化过了
	query := map[string]interface{}{
		"keyword":  keyword,
		"category": category,
		"page":     page,
		"pageSize": pageSize,
		"sortBy":   sortBy,
		"order":    order,
	}

	// TODO: 执行数据库查询

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "搜索成功",
		"success": true,
		"data":    query,
		"total":   0, // TODO: 实际总数
	})
}

// UpdateProfile 更新资料（展示 PUT 请求处理）
func (e *ExampleController) UpdateProfile(c *mvc.Context) {

	// 获取更新数据
	nickname := request.Input(c, "nickname", "")
	bio := request.Input(c, "bio", "")
	avatar := request.Input(c, "avatar", "")
	gender := request.InputInt(c, "gender", 0)

	// 验证昵称长度
	if nickname != "" && (len(nickname) < 2 || len(nickname) > 20) {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "昵称长度必须在 2-20 个字符之间",
			"success": false,
		})
		return
	}

	// 验证简介长度
	if bio != "" && len(bio) > 500 {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "简介不能超过 500 个字符",
			"success": false,
		})
		return
	}

	// 验证性别（0:未知，1:男，2:女）
	if gender != 0 && gender != 1 && gender != 2 {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "性别参数不正确",
			"success": false,
		})
		return
	}

	// 更新数据
	profile := map[string]interface{}{
		"nickname": nickname,
		"bio":      bio,
		"avatar":   avatar,
		"gender":   gender,
	}

	// TODO: 保存到数据库

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "资料更新成功",
		"success": true,
		"data":    profile,
	})
}

// BatchDelete 批量删除（展示数组参数处理）
func (e *ExampleController) BatchDelete(c *mvc.Context) {

	// 获取 ID 列表（逗号分隔）
	ids := request.InputSlice(c, "ids", []string{})

	if len(ids) == 0 {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "请选择要删除的项目",
			"success": false,
		})
		return
	}

	// 验证 ID 格式（应该是数字）
	for _, id := range ids {
		if !isNumeric(id) {
			c.Json(http.StatusBadRequest, map[string]interface{}{
				"code":    400,
				"message": "ID 格式不正确",
				"success": false,
			})
			return
		}
	}

	// TODO: 执行批量删除

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "批量删除成功",
		"success": true,
		"deleted": len(ids),
	})
}

// SubmitComment 提交评论（展示内容安全处理）
func (e *ExampleController) SubmitComment(c *mvc.Context) {

	// 获取评论内容
	content := request.Input(c, "content", "")
	articleID := request.InputInt(c, "article_id", 0)
	parentID := request.InputInt(c, "parent_id", 0)

	// 验证必填字段
	if content == "" {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "评论内容不能为空",
			"success": false,
		})
		return
	}

	// 验证内容长度
	if len(content) > 1000 {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "评论不能超过 1000 个字符",
			"success": false,
		})
		return
	}

	// 验证文章 ID
	if articleID <= 0 {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "文章 ID 不正确",
			"success": false,
		})
		return
	}

	// 注意：content 已经通过 request 模块自动净化
	// XSS 攻击代码会被过滤，如 <script>alert('xss')</script>
	// SQL 注入会被检测和阻止

	comment := map[string]interface{}{
		"content":   content,
		"articleID": articleID,
		"parentID":  parentID,
	}

	// TODO: 保存到数据库

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "评论提交成功",
		"success": true,
		"data":    comment,
	})
}

// UploadFile 文件上传（展示文件处理）
func (e *ExampleController) UploadFile(c *mvc.Context) {

	// 获取文件描述（已自动净化）
	title := request.Input(c, "title", "")
	description := request.Input(c, "description", "")
	category := request.Input(c, "category", "default")

	// 验证标题
	if title == "" {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "请填写文件标题",
			"success": false,
		})
		return
	}

	// 验证分类（白名单）
	allowedCategories := []string{"default", "image", "document", "video"}
	if !containsString(allowedCategories, category) {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "不支持的分类",
			"success": false,
		})
		return
	}

	// 获取上传文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "文件上传失败",
			"success": false,
		})
		return
	}
	defer file.Close()

	// TODO: 保存文件

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "文件上传成功",
		"success": true,
		"data": map[string]interface{}{
			"filename":    header.Filename,
			"title":       title,
			"description": description,
			"category":    category,
		},
	})
}

// GetSettings 获取设置（展示布尔值处理）
func (e *ExampleController) GetSettings(c *mvc.Context) {

	// 获取布尔值参数
	showDeleted := request.InputBool(c, "show_deleted", false)
	enableCache := request.InputBool(c, "enable_cache", true)
	debug := request.InputBool(c, "debug", false)

	settings := map[string]interface{}{
		"showDeleted": showDeleted,
		"enableCache": enableCache,
		"debug":       debug,
	}

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "获取成功",
		"success": true,
		"data":    settings,
	})
}

// AdvancedSearch 高级搜索（展示复杂参数处理）
func (e *ExampleController) AdvancedSearch(c *mvc.Context) {

	// 使用 Request 对象方式获取数据
	req := request.R(c)

	// 获取各种类型的数据
	keyword := req.Get("keyword", "")
	minPrice := req.GetFloat("min_price", 0)
	maxPrice := req.GetFloat("max_price", 999999)
	inStock := req.GetBool("in_stock", false)
	tags := req.GetStringSlice("tags", []string{})

	// 验证价格范围
	if minPrice < 0 {
		minPrice = 0
	}
	if maxPrice < minPrice {
		maxPrice = minPrice
	}

	// 验证标签数量
	if len(tags) > 10 {
		c.Json(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "最多选择 10 个标签",
			"success": false,
		})
		return
	}

	// 构建复杂查询条件
	query := map[string]interface{}{
		"keyword":  keyword,
		"minPrice": minPrice,
		"maxPrice": maxPrice,
		"inStock":  inStock,
		"tags":     tags,
	}

	c.Json(http.StatusOK, map[string]interface{}{
		"code":    200,
		"message": "搜索成功",
		"success": true,
		"data":    query,
	})
}

// ==================== 辅助函数 ====================

// validateEmail 验证邮箱格式
func validateEmail(email string) bool {
	if email == "" {
		return false
	}
	// 简单验证
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// validatePhone 验证手机号格式
func validatePhone(phone string) bool {
	if phone == "" {
		return false
	}
	// 简单验证：11 位数字，以 1 开头
	if len(phone) != 11 {
		return false
	}
	if phone[0] != '1' {
		return false
	}
	for _, c := range phone {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// containsString 检查字符串是否在切片中
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isNumeric 检查字符串是否为数字
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
