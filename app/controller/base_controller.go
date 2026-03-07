package controller

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"vigo/framework/mvc"
)

// BaseController 基础控制器
// 所有控制器的基类，提供通用的前置/后置处理逻辑
//
// 使用示例：
//
//	type UserController struct {
//	    BaseController
//	}
//
//	func (c *UserController) GetProfile(ctx *mvc.Context) {
//	    // Before() 已自动执行认证和权限检查
//	    userID := ctx.Get("userID").(int)
//	    c.Success(map[string]interface{}{"userID": userID})
//	}
type BaseController struct {
	mvc.Controller
}

// ==================== 配置区域 ====================

// noNeedAuth 无需认证的方法列表
// 添加不需要登录验证的方法名（方法名指的是 URL 最后一段路径）
// 例如：/api/login -> "login", /api/register -> "register"
var noNeedAuth = map[string]bool{
	"login":    true, // 登录接口
	"register": true, // 注册接口
	"captcha":  true, // 验证码接口
	"health":   true, // 健康检查接口
}

// noNeedPermission 无需权限检查的方法列表
// 添加不需要权限验证但需要登录的方法名
var noNeedPermission = map[string]bool{
	"profile": true, // 查看个人资料
	"update":  true, // 更新个人资料
}

// ==================== 前置处理 ====================

// Before 前置操作
// 在每个控制器方法执行前自动调用
// 用于执行通用的认证、权限检查等逻辑
//
// 执行流程：
// 1. 获取当前请求的方法名
// 2. 检查是否在跳过认证列表中
// 3. 执行认证检查（检查登录状态）
// 4. 执行权限检查（检查操作权限）
//
// 注意：
// - 如果验证失败，会调用 c.Ctx.Abort() 终止后续处理
// - 验证通过后会调用 c.Next() 继续执行控制器方法
func (c *BaseController) Before() {
	// 1. 获取当前请求的方法名（从 URL 路径推断）
	methodName := c.getControllerMethod()

	// 2. 检查是否需要跳过认证
	// 如果在 noNeedAuth 列表中，直接返回，不执行后续验证
	if noNeedAuth[methodName] {
		return
	}

	// 3. 执行认证检查（检查用户是否登录）
	// 如果认证失败，会返回 401 错误并终止处理
	if err := c.checkAuth(); err != nil {
		c.Error(401, err.Error())
		c.Ctx.Abort() // 终止后续处理，不执行控制器方法
		return
	}

	// 4. 执行权限检查（检查用户是否有操作权限）
	// 如果方法在 noNeedPermission 列表中，跳过权限检查
	if !noNeedPermission[methodName] {
		if err := c.checkPermission(); err != nil {
			c.Error(403, err.Error())
			c.Ctx.Abort() // 终止后续处理
			return
		}
	}

	// 5. 验证通过，继续执行控制器方法
	// 注意：这里不需要显式调用 c.Next()，框架会自动调用
}

// ==================== 后置处理 ====================

// After 后置操作
// 在每个控制器方法执行后自动调用
// 用于执行操作日志记录、性能统计等逻辑
//
// 执行时机：
// - 控制器方法执行完成后
// - 响应发送给客户端之前
//
// 注意：
// - 即使控制器方法抛出异常，After() 也会被调用（被 Recovery 中间件捕获）
// - 避免在 After() 中执行耗时操作，建议使用异步处理
func (c *BaseController) After() {
	// 1. 记录操作日志
	// 异步写入数据库或消息队列，避免阻塞请求
	go c.logOperation()

	// 2. 性能统计（可选）
	// 可以在这里记录请求耗时等信息
}

// ==================== 辅助方法 ====================

// getControllerMethod 获取当前控制器方法名
// 从 URL 路径中推断方法名
//
// 示例：
// /api/user/profile -> "profile"
// /api/login -> "login"
// /admin/user/delete/123 -> "delete"
//
// 返回：
// 方法名（小写），如果无法推断则返回空字符串
func (c *BaseController) getControllerMethod() string {
	// 获取请求路径
	path := c.Ctx.Request.URL.Path

	// 按 "/" 分割路径
	parts := strings.Split(path, "/")

	// 返回最后一段路径（转换为小写）
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return strings.ToLower(parts[len(parts)-1])
	}

	return ""
}

// checkAuth 检查认证（检查用户是否登录）
//
// 验证逻辑：
// 1. 从 Authorization Header 中获取 Token
// 2. 验证 Token 格式（Bearer Token）
// 3. 验证 Token 有效性（调用 validateToken）
// 4. 将用户信息存储到 Context 中供后续使用
//
// 返回：
// - nil: 认证通过
// - error: 认证失败，返回错误信息
func (c *BaseController) checkAuth() error {
	// 1. 从 Header 获取 Token
	// 格式：Authorization: Bearer <token>
	token := c.Ctx.Request.Header.Get("Authorization")
	if token == "" {
		return fmt.Errorf("缺少认证信息")
	}

	// 2. 解析 Token
	// 期望格式："Bearer <token_value>"
	parts := strings.SplitN(token, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return fmt.Errorf("Token 格式错误")
	}

	// 3. 验证 Token 有效性
	// 调用具体的验证逻辑（可以是 JWT 解析、数据库查询等）
	userID, err := c.validateToken(parts[1])
	if err != nil {
		return fmt.Errorf("认证失败：%v", err)
	}

	// 4. 存储用户信息到 Context
	// 这样在控制器方法中可以通过 ctx.Get("userID") 获取
	c.Ctx.Set("userID", userID)
	c.Ctx.Set("loginTime", time.Now())

	return nil
}

// checkPermission 检查权限（检查用户是否有操作权限）
//
// 验证逻辑：
// 1. 获取已认证的用户 ID
// 2. 检查用户是否有所需的权限
// 3. 返回检查结果
//
// 返回：
// - nil: 权限检查通过
// - error: 权限不足，返回错误信息
func (c *BaseController) checkPermission() error {
	// 1. 获取用户 ID（在 checkAuth 中已存储）
	userID, ok := c.Ctx.Get("userID")
	if !ok || userID == nil {
		return fmt.Errorf("用户未登录")
	}

	// 2. 获取当前请求路径，判断所需权限
	// 这里可以实现基于 RBAC 的权限检查
	// 示例：根据 URL 路径判断所需权限
	path := c.Ctx.Request.URL.Path

	// 3. 检查用户权限
	// TODO: 实现具体的权限检查逻辑
	// 可以查询数据库中的用户角色和权限
	hasPermission := c.hasUserPermission(userID.(int), path)
	if !hasPermission {
		return fmt.Errorf("权限不足")
	}

	return nil
}

// validateToken 验证 Token 有效性
//
// 参数：
// - token: 客户端传入的 Token 字符串
//
// 返回：
// - userID: 用户 ID
// - error: 验证错误信息
//
// TODO: 请根据实际项目需求实现 Token 验证逻辑
// 可以使用 JWT、Session 或其他方式
func (c *BaseController) validateToken(token string) (int, error) {
	// ========== 方案一：JWT Token 验证 ==========
	// 如果使用 JWT，可以在这里解析和验证 JWT Token
	// claims, err := jwt.Parse(token, keyFunc)
	// if err != nil {
	//     return 0, err
	// }
	// userID := claims.GetUserID()
	// return userID, nil

	// ========== 方案二：Session 验证 ==========
	// 如果使用 Session，可以在这里查询 Session 数据
	// session := getSession(token)
	// if session == nil {
	//     return 0, fmt.Errorf("Session 无效")
	// }
	// return session.UserID, nil

	// ========== 方案三：数据库验证 ==========
	// 直接查询数据库验证 Token
	// var user User
	// err := db.Where("token = ?", token).First(&user)
	// if err != nil {
	//     return 0, err
	// }
	// return user.ID, nil

	// ========== 示例代码（临时使用） ==========
	// 注意：这只是示例，实际项目中请替换为真实的验证逻辑
	if token == "invalid_token" {
		return 0, fmt.Errorf("Token 无效")
	}

	// 示例：返回固定用户 ID（仅用于测试）
	return 1, nil
}

// hasUserPermission 检查用户是否有指定权限
//
// 参数：
// - userID: 用户 ID
// - path: 请求路径（用于判断所需权限）
//
// 返回：
// - bool: 是否有权限
//
// TODO: 请根据实际项目需求实现权限检查逻辑
// 可以使用 RBAC、ACL 等权限模型
func (c *BaseController) hasUserPermission(userID int, path string) bool {
	// ========== 方案一：基于角色的权限检查（RBAC） ==========
	// 1. 查询用户角色
	// roles := getUserRoles(userID)
	//
	// 2. 查询角色权限
	// permissions := getRolePermissions(roles)
	//
	// 3. 检查是否包含所需权限
	// return permissions.Contains(path)

	// ========== 方案二：基于资源的权限检查 ==========
	// 检查用户是否有所请求资源的访问权限
	// return hasResourceAccess(userID, resourceID)

	// ========== 示例代码（临时使用） ==========
	// 注意：这只是示例，实际项目中请替换为真实的权限检查逻辑

	// 示例：管理员拥有所有权限
	if userID == 1 {
		return true
	}

	// 示例：普通用户只能访问特定路径
	if strings.HasPrefix(path, "/api/user") {
		return true
	}

	return false
}

// logOperation 记录操作日志
//
// 记录内容：
// - 用户 ID
// - 请求路径
// - 请求方法
// - 请求时间
// - IP 地址
//
// 注意：
// - 使用 goroutine 异步执行，避免阻塞请求
// - 可以写入数据库、文件或消息队列
func (c *BaseController) logOperation() {
	// 获取用户 ID
	userID, ok := c.Ctx.Get("userID")
	if !ok || userID == nil {
		return
	}

	// 构建日志信息
	_ = map[string]interface{}{
		"user_id":    userID,
		"path":       c.Ctx.Request.URL.Path,
		"method":     c.Ctx.Request.Method,
		"ip":         getClientIP(c.Ctx.Request), // 使用辅助函数获取客户端 IP
		"user_agent": c.Ctx.Request.UserAgent(),
		"time":       time.Now(),
	}

	// ========== 方案一：写入数据库 ==========
	// db.Table("operation_logs").Insert(logData)

	// ========== 方案二：写入文件 ==========
	// log.Printf("[Operation] %+v", logData)

	// ========== 方案三：发送到消息队列 ==========
	// mq.Publish("operation_logs", logData)

	// 示例：打印到日志
	// fmt.Printf("[Operation Log] User %d accessed %s %s\n",
	//     userID.(int),
	//     c.Ctx.Request.Method,
	//     c.Ctx.Request.URL.Path,
	// )
}

// getClientIP 获取客户端真实 IP 地址
// 处理代理服务器、负载均衡等情况
func getClientIP(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ip := strings.Split(xForwardedFor, ",")[0]
		return strings.TrimSpace(ip)
	}

	// 从 X-Real-IP 获取
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// 从 RemoteAddr 获取
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// ==================== 便捷方法 ====================

// GetUserID 获取当前登录用户 ID
// 在控制器方法中可以直接调用
//
// 使用示例：
// userID := c.GetUserID()
//
// 返回：
// - int: 用户 ID
// - bool: 是否获取成功
func (c *BaseController) GetUserID() (int, bool) {
	userID, ok := c.Ctx.Get("userID")
	if !ok || userID == nil {
		return 0, false
	}

	id, ok := userID.(int)
	return id, ok
}

// IsLoggedIn 检查用户是否已登录
// 在控制器方法中可以直接调用
//
// 使用示例：
//
//	if c.IsLoggedIn() {
//	    // 用户已登录
//	}
//
// 返回：
// - bool: 是否已登录
func (c *BaseController) IsLoggedIn() bool {
	_, ok := c.Ctx.Get("userID")
	return ok
}

// GetLoginTime 获取用户登录时间
// 在控制器方法中可以直接调用
//
// 使用示例：
// loginTime := c.GetLoginTime()
//
// 返回：
// - time.Time: 登录时间
// - bool: 是否获取成功
func (c *BaseController) GetLoginTime() (time.Time, bool) {
	loginTime, ok := c.Ctx.Get("loginTime")
	if !ok || loginTime == nil {
		return time.Time{}, false
	}

	t, ok := loginTime.(time.Time)
	return t, ok
}
