# BaseController 中使用中间件

**更新日期**: 2026-03-06

---

## 📖 概述

在 ThinkPHP 中，通常可以在 `BaseController` 中通过 `initialize()` 方法调用中间件逻辑。Vigo 框架提供了类似的机制，通过 `Before()` 和 `After()` 方法实现。

本文档详细介绍如何在 Vigo 框架的 `BaseController` 中使用中间件逻辑。

---

## 🎯 ThinkPHP vs Vigo 对比

### ThinkPHP 方式

```php
// ThinkPHP 8.x - BaseController.php
namespace app\controller;

use app\BaseController as Base;
use think\middleware\CheckRequest;

class BaseController extends Base
{
    /**
     * 无需登录的方法
     * @var array
     */
    protected array $noNeedLogin = [];

    /**
     * 无需权限的方法
     * @var array
     */
    protected array $noNeedAuth = [];

    protected function initialize(): void
    {
        parent::initialize();

        // 在 initialize 中执行中间件逻辑
        $this->checkLogin();
        $this->checkAuth();
    }

    protected function checkLogin(): void
    {
        // 检查登录逻辑
    }

    protected function checkAuth(): void
    {
        // 检查权限逻辑
    }
}
```

### Vigo 框架方式

Vigo 框架提供了三种方式实现类似功能：

---

## ✅ 方式一：在 Before() 方法中执行（推荐）

这是最接近 ThinkPHP 的方式，在控制器的 `Before()` 方法中执行中间件逻辑。

### 实现步骤

#### 步骤 1：扩展 BaseController

```go
package controller

import (
    "vigo/framework/mvc"
    "strings"
)

// BaseController 基础控制器
type BaseController struct {
    mvc.Controller
}

// Before 前置操作（在每个控制器方法执行前调用）
func (c *BaseController) Before() {
    // 获取当前方法名
    method := c.Ctx.Request.URL.Path

    // 检查是否需要跳过验证
    if c.skipAuth(method) {
        return
    }

    // 执行认证检查
    if err := c.checkAuth(); err != nil {
        c.Error(401, err.Error())
        c.Ctx.Abort()
        return
    }

    // 执行权限检查
    if err := c.checkPermission(); err != nil {
        c.Error(403, err.Error())
        c.Ctx.Abort()
        return
    }
}

// After 后置操作（在每个控制器方法执行后调用）
func (c *BaseController) After() {
    // 全局后置操作，如日志记录、性能统计等
}

// skipAuth 判断是否跳过认证
func (c *BaseController) skipAuth(method string) bool {
    // 定义无需认证的路径
    skipPaths := []string{
        "/api/login",
        "/api/register",
        "/health",
    }

    for _, path := range skipPaths {
        if strings.HasPrefix(method, path) {
            return true
        }
    }

    return false
}

// checkAuth 检查认证
func (c *BaseController) checkAuth() error {
    // 从 Header 获取 Token
    token := c.Ctx.Request.Header.Get("Authorization")
    if token == "" {
        return fmt.Errorf("缺少认证信息")
    }

    // 解析 Token
    parts := strings.SplitN(token, " ", 2)
    if len(parts) != 2 || parts[0] != "Bearer" {
        return fmt.Errorf("Token 格式错误")
    }

    // 验证 Token
    userID, err := c.validateToken(parts[1])
    if err != nil {
        return fmt.Errorf("Token 无效")
    }

    // 存储用户信息到 Context
    c.Ctx.Set("userID", userID)

    return nil
}

// checkPermission 检查权限
func (c *BaseController) checkPermission() error {
    // 获取用户 ID
    userID := c.Ctx.Get("userID")
    if userID == nil {
        return fmt.Errorf("用户未登录")
    }

    // 检查权限逻辑
    // ...

    return nil
}

// validateToken 验证 Token
func (c *BaseController) validateToken(token string) (int, error) {
    // 实现 Token 验证逻辑
    // 可以调用 JWT 解析、数据库查询等
    return 1, nil
}
```

#### 步骤 2：控制器继承 BaseController

```go
package controller

import "vigo/framework/mvc"

// UserController 用户控制器
type UserController struct {
    BaseController
}

// GetProfile 获取用户资料
func (c *UserController) GetProfile(ctx *mvc.Context) {
    // Before() 已经执行了认证和权限检查

    // 直接获取用户 ID（在 Before 中已存储）
    userID := ctx.Get("userID").(int)

    // 业务逻辑
    c.Success(map[string]interface{}{
        "userID": userID,
        "name":   "张三",
    })
}

// UpdateProfile 更新用户资料
func (c *UserController) UpdateProfile(ctx *mvc.Context) {
    // Before() 已经执行了认证和权限检查

    userID := ctx.Get("userID").(int)

    // 业务逻辑
    c.Success(map[string]interface{}{
        "message": "更新成功",
    })
}
```

#### 步骤 3：确保路由系统调用 Before/After

需要在 MVC 框架中确保 `Before()` 和 `After()` 方法被调用。

查看 `framework/mvc/context.go` 中的执行逻辑：

```go
// framework/mvc/context.go
package mvc

// Next 执行下一个处理器
func (c *Context) Next() {
    c.index++
    for c.index < len(c.handlers) {
        c.handlers[c.index](c)
        c.index++
    }
}

// 在路由处理器中调用控制器的 Before/After
func (c *Context) handleController(controller interface{}) {
    // 类型断言到 BaseController
    if bc, ok := controller.(interface{ Before(); After() }); ok {
        bc.Before()

        // 如果 Before 中调用了 Abort，则不执行控制器方法
        if !c.IsAborted() {
            // 执行控制器方法
        }

        bc.After()
    }
}
```

---

## ✅ 方式二：使用中间件组合

在路由注册时，将中间件逻辑应用到路由分组。

### 实现方式

```go
package route

import (
    "vigo/framework/mvc"
    "vigo/framework/middleware"
    "vigo/app/controller"
)

func Init(r *mvc.Router) {
    // 定义认证中间件
    authMiddleware := func() mvc.HandlerFunc {
        return func(c *mvc.Context) {
            token := c.Request.Header.Get("Authorization")
            if token == "" {
                c.Error(401, "缺少认证信息")
                c.Abort()
                return
            }

            // 验证 Token
            userID := validateToken(token)
            c.Set("userID", userID)

            c.Next()
        }
    }

    // 定义权限中间件
    permissionMiddleware := func(requiredPerm string) mvc.HandlerFunc {
        return func(c *mvc.Context) {
            userID := c.Get("userID")
            if userID == nil {
                c.Error(401, "用户未登录")
                c.Abort()
                return
            }

            // 检查权限
            if !hasPermission(userID.(int), requiredPerm) {
                c.Error(403, "权限不足")
                c.Abort()
                return
            }

            c.Next()
        }
    }

    // 公开接口（无需认证）
    r.POST("/api/login", controller.Auth.Login)
    r.POST("/api/register", controller.Auth.Register)

    // 需要认证的接口
    authGroup := r.Group("/api", authMiddleware())

    // 用户相关接口
    userGroup := authGroup.Group("/user")
    userGroup.GET("/profile", controller.User.GetProfile)
    userGroup.POST("/profile/update", controller.User.UpdateProfile)

    // 需要特定权限的接口
    adminGroup := authGroup.Group("/admin", permissionMiddleware("admin"))
    adminGroup.GET("/users", controller.Admin.ListUsers)
    adminGroup.DELETE("/user/:id", controller.Admin.DeleteUser)
}
```

---

## ✅ 方式三：混合方式（Before + 中间件）

结合前两种方式的优点，在 `BaseController` 中处理业务逻辑，在路由层处理通用逻辑。

### 实现方式

#### BaseController 中处理业务相关检查

```go
package controller

import (
    "vigo/framework/mvc"
)

// BaseController 基础控制器
type BaseController struct {
    mvc.Controller
}

// Before 前置操作
func (c *BaseController) Before() {
    // 只处理业务相关的检查
    // 如：数据验证、业务规则检查等

    // 示例：检查租户
    if err := c.checkTenant(); err != nil {
        c.Error(400, err.Error())
        c.Ctx.Abort()
        return
    }
}

// After 后置操作
func (c *BaseController) After() {
    // 业务相关的后置操作
    // 如：操作日志记录、数据清理等
}

// checkTenant 检查租户
func (c *BaseController) checkTenant() error {
    tenantID := c.Ctx.Get("tenantID")
    if tenantID == nil {
        return fmt.Errorf("租户信息缺失")
    }
    return nil
}
```

#### 路由层处理通用安全检查

```go
package route

import (
    "vigo/framework/mvc"
    "vigo/framework/middleware"
)

func Init(r *mvc.Router) {
    // 全局中间件：处理通用安全检查
    r.Use(
        middleware.Recovery(),              // 异常恢复
        middleware.SecurityMiddleware(),    // 安全验证（SQL 注入、XSS 等）
        middleware.Logger(),                // 日志记录
    )

    // 认证中间件
    r.Use(middleware.JWTAuth())

    // 其他路由...
}
```

---

## 📋 三种方式对比

| 方式                      | 优点                         | 缺点                           | 适用场景                         |
| ------------------------- | ---------------------------- | ------------------------------ | -------------------------------- |
| **方式一：Before() 方法** | 接近 ThinkPHP 习惯，代码集中 | 每个请求都会执行，可能影响性能 | 业务逻辑复杂，需要灵活控制的场景 |
| **方式二：中间件组合**    | 性能更好，逻辑清晰           | 需要为不同场景配置不同中间件   | 通用认证、权限检查等场景         |
| **方式三：混合方式**      | 职责分离，灵活高效           | 需要维护两处代码               | 推荐！大型项目首选               |

---

## 🎯 推荐实践

### 推荐方案：混合方式

```go
// ========== 1. 路由层：通用安全检查 ==========
func Init(r *mvc.Router) {
    // 全局中间件
    r.Use(
        middleware.Recovery(),              // 异常恢复
        middleware.SecurityMiddleware(),    // 安全验证
        middleware.Logger(),                // 日志记录
    )

    // 认证中间件（所有 API 都需要）
    r.Use(middleware.JWTAuth())

    // 公开接口
    r.POST("/api/login", auth.Login)

    // 需要认证的接口
    apiGroup := r.Group("/api")
    apiGroup.GET("/user/profile", user.GetProfile)

    // 需要特殊权限的接口
    adminGroup := r.Group("/admin", middleware.RoleCheck("admin"))
    adminGroup.DELETE("/user/:id", admin.DeleteUser)
}

// ========== 2. BaseController：业务检查 ==========
type BaseController struct {
    mvc.Controller
}

func (c *BaseController) Before() {
    // 业务相关的检查
    c.checkTenant()
    c.checkDataPermission()
}

func (c *BaseController) After() {
    // 业务相关的后置操作
    c.logOperation()
}
```

---

## 💡 ThinkPHP 开发者迁移指南

### ThinkPHP 习惯

```php
// 在 ThinkPHP 中
class BaseController extends Base
{
    protected $middleware = [
        'checkLogin' => [],
        'checkAuth'  => ['except' => ['login', 'register']],
    ];

    protected function initialize(): void
    {
        $this->checkLogin();
        $this->checkAuth();
    }
}
```

### Vigo 对应方式

```go
// 在 Vigo 中
type BaseController struct {
    mvc.Controller
}

// 定义无需验证的方法
var noNeedAuth = map[string]bool{
    "Login":    true,
    "Register": true,
}

func (c *BaseController) Before() {
    // 获取当前方法名
    // 检查是否在跳过列表中
    // 执行验证逻辑
}
```

---

## 📝 完整示例

### 完整的 BaseController 实现

```go
package controller

import (
    "fmt"
    "strings"
    "time"
    "vigo/framework/mvc"
)

// BaseController 基础控制器
type BaseController struct {
    mvc.Controller
}

// 无需认证的方法列表
var noNeedAuth = map[string]bool{
    "Login":    true,
    "Register": true,
    "Captcha":  true,
}

// Before 前置操作
func (c *BaseController) Before() {
    // 1. 获取当前方法名
    methodName := c.getControllerMethod()

    // 2. 检查是否需要跳过认证
    if noNeedAuth[methodName] {
        return
    }

    // 3. 执行认证检查
    if err := c.checkAuth(); err != nil {
        c.Error(401, err.Error())
        c.Ctx.Abort()
        return
    }

    // 4. 执行权限检查（可选）
    if err := c.checkPermission(); err != nil {
        c.Error(403, err.Error())
        c.Ctx.Abort()
        return
    }
}

// After 后置操作
func (c *BaseController) After() {
    // 记录操作日志
    c.logOperation()
}

// getControllerMethod 获取当前控制器方法名
func (c *BaseController) getControllerMethod() string {
    // 从 URL 路径推断方法名
    path := c.Ctx.Request.URL.Path
    parts := strings.Split(path, "/")
    if len(parts) > 0 {
        return parts[len(parts)-1]
    }
    return ""
}

// checkAuth 检查认证
func (c *BaseController) checkAuth() error {
    token := c.Ctx.Request.Header.Get("Authorization")
    if token == "" {
        return fmt.Errorf("缺少认证信息")
    }

    // 解析和验证 Token
    userID, err := c.validateToken(token)
    if err != nil {
        return fmt.Errorf("认证失败：%v", err)
    }

    // 存储用户信息
    c.Ctx.Set("userID", userID)
    c.Ctx.Set("loginTime", time.Now())

    return nil
}

// checkPermission 检查权限
func (c *BaseController) checkPermission() error {
    userID := c.Ctx.Get("userID")
    if userID == nil {
        return fmt.Errorf("用户未登录")
    }

    // 这里可以实现基于 RBAC 的权限检查
    // ...

    return nil
}

// validateToken 验证 Token
func (c *BaseController) validateToken(token string) (int, error) {
    // 实现 Token 验证逻辑
    return 1, nil
}

// logOperation 记录操作日志
func (c *BaseController) logOperation() {
    // 记录用户操作日志
    // 异步写入数据库或消息队列
}
```

---

## 🔧 注意事项

1. **Before/After 调用时机**：确保 MVC 框架在调用控制器方法前后正确调用 `Before()` 和 `After()`
2. **Abort 的使用**：在 `Before()` 中如果检查失败，记得调用 `c.Ctx.Abort()` 终止后续处理
3. **性能考虑**：避免在 `Before()` 中执行耗时操作
4. **方法名推断**：可以通过 URL 路径或反射获取当前方法名，用于判断是否跳过验证

---

## 📚 相关文档

- [自定义中间件开发指南](./自定义中间件开发指南.md)
- [中间件使用指南](./04.中间件.md)
- [路由与控制器](../02.核心功能/01.路由与控制器.md)

---

**总结**：Vigo 框架提供了灵活的方式在 `BaseController` 中实现中间件逻辑，ThinkPHP 开发者可以根据自己的习惯选择合适的方式。推荐使用**混合方式**，将通用安全检查放在路由层，业务检查放在 `BaseController` 中。
