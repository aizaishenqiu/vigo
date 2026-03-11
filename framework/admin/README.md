# Vigo 框架管理中心使用文档

## 一、管理中心简介

Vigo 框架管理中心是一个集成在框架核心的管理面板，类似 Swagger 的自动注册机制。它提供了系统监控、配置管理、消息队列监控、压力测试、健康检查等功能。

## 二、启用管理中心

### 2.1 配置文件配置

在 `config.yaml` 中添加以下配置：

```yaml
# ==============================================
# 管理面板配置（类似 Swagger 的自动注册）
# ==============================================
admin:
  # 是否启用管理面板（生产环境建议关闭或限制 IP 访问）
  enabled: true

  # 管理面板基础路径（访问地址：http://localhost:8080/admin）
  base_path: /admin

  # 管理面板用户名（建议通过环境变量设置）
  username: admin

  # 管理面板密码（建议通过环境变量设置）
  password: admin123

  # 允许的 IP 列表（* 表示允许所有 IP，生产环境建议限制）
  allow_ips:
    - "*"

  # 是否自动注册路由（启用后会在 app.Run() 时自动注册管理路由）
  auto_register: true
```

### 2.2 环境变量配置（推荐生产环境使用）

```bash
# 通过环境变量设置管理员账号
export VIGO_ADMIN_USERNAME=admin
export VIGO_ADMIN_PASSWORD=your_secure_password
export VIGO_ADMIN_ENABLED=true
export VIGO_ADMIN_BASE_PATH=/admin
```

### 2.3 自动注册机制

管理中心采用类似 Swagger 的自动注册机制，无需手动注册路由。只需在配置文件中启用，框架会在 `app.Run()` 时自动注册所有管理路由。

```go
// framework/app/app.go 中自动调用
func (app *App) Run(addr string) error {
    // ... 其他代码 ...

    // 13. 自动注册管理面板路由（类似 Swagger）
    app.initAdminPanel(r)

    // ... 其他代码 ...
}
```

## 三、访问管理中心

### 3.1 访问地址

启动应用后，访问：`http://localhost:8080/admin`

### 3.2 功能菜单

管理中心包含以下功能模块：

1. **仪表盘** (`/admin/dashboard`)
   - 路由总数统计
   - 数据库连接状态
   - 内存使用情况
   - 系统运行时长（实时计算）
   - 24 小时请求趋势图（ECharts 图表）

2. **Nacos 配置** (`/admin/nacos`)
   - 配置列表管理（data_id、group、content、md5、更新时间）
   - 服务注册与发现（服务名称、健康实例数、状态）
   - 配置版本控制

3. **RabbitMQ 监控** (`/admin/rabbitmq`)
   - 队列监控（队列名称、消息数、消费者数、状态）
   - 交换机监控（交换机名称、类型、绑定数、状态）
   - 消息积压情况

4. **压力测试** (`/admin/stress`)
   - HTTP 接口压测
   - 并发请求测试
   - 性能指标分析（请求数、成功数、失败数、平均延迟、QPS）

5. **健康检查** (`/admin/health`)
   - 数据库健康状态（状态、连接延迟、连接池）
   - Redis 健康状态（状态、连接延迟、内存使用）
   - 缓存系统状态（状态、命中率、缓存条目）
   - 消息队列状态（状态、队列数、积压消息）

6. **系统监控** (`/admin/monitor`)
   - CPU 使用率
   - 内存使用率（实时从 runtime 获取）
   - 磁盘使用率
   - 网络带宽监控
   - Goroutine 数量

## 四、在配置中心使用管理中心

### 4.1 Nacos 配置中心集成

如果您使用 Nacos 作为配置中心，可以在 Nacos 中管理管理中心的配置：

**Nacos 配置示例 (YAML 格式)：**

```yaml
# 在 Nacos 配置文件中添加
vigo:
  admin:
    enabled: true
    base-path: /admin
    username: ${ADMIN_USERNAME:admin}
    password: ${ADMIN_PASSWORD:admin123}
    allow-ips:
      - "127.0.0.1"
      - "192.168.1.*"
    auto-register: true
```

### 4.2 配置说明

| 配置项          | 说明             | 默认值     | 生产环境建议            |
| --------------- | ---------------- | ---------- | ----------------------- |
| `enabled`       | 是否启用管理面板 | `false`    | `true` (配合 IP 限制)   |
| `base_path`     | 管理面板访问路径 | `/admin`   | 自定义路径 (增加安全性) |
| `username`      | 管理员用户名     | `admin`    | 使用环境变量            |
| `password`      | 管理员密码       | `admin123` | 使用强密码 + 环境变量   |
| `allow_ips`     | 允许的 IP 列表   | `["*"]`    | 限制为内网 IP           |
| `auto_register` | 自动注册路由     | `true`     | `true`                  |

### 4.3 安全建议

**生产环境必须配置：**

```yaml
admin:
  enabled: true
  base_path: /manage-{{随机字符串}} # 自定义路径，防止被扫描
  username: ${ADMIN_USERNAME} # 从环境变量读取
  password: ${ADMIN_PASSWORD} # 从环境变量读取
  allow_ips:
    - "10.0.0.*" # 只允许内网访问
    - "192.168.1.100" # 只允许特定 IP
  auto_register: true
```

## 五、静态资源说明

管理中心使用离线前端资源，无需联网即可使用：

```
framework/admin/static/
├── echarts/
│   └── echarts.min.js          # ECharts 图表库 (5.4.0)
└── layui/
    ├── layui.js                # Layui 核心 JS (2.13.4)
    ├── layui.css               # Layui 核心 CSS
    ├── css/
    │   └── layui.css
    └── font/
        ├── iconfont.eot
        ├── iconfont.svg
        ├── iconfont.ttf
        ├── iconfont.woff
        └── iconfont.woff2
```

## 六、用户界面功能

### 6.1 顶部导航栏

管理中心顶部导航栏提供以下功能：

- **返回首页**：快速返回仪表盘页面
- **修改密码**：修改管理员密码（需实现后端接口）
- **退出登录**：退出管理系统（需实现会话管理）

### 6.2 左侧菜单

左侧菜单采用 Layui 风格设计，支持：

- 图标显示（清晰的 Layui 图标）
- 悬停高亮效果
- 当前激活状态指示
- 点击切换页面

### 6.3 响应式布局

管理中心采用 Layui 的响应式布局：

- 自适应不同屏幕尺寸
- 卡片式数据展示
- 图表自适应窗口大小

## 七、API 接口

管理中心提供以下 API 接口：

### 7.1 系统统计

```
GET /admin/api/stats
```

响应示例：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total_requests": 10000,
    "success_requests": 9950,
    "failed_requests": 50,
    "avg_response": "50ms"
  }
}
```

### 7.2 路由列表

```
GET /admin/api/routes
```

### 7.3 系统指标

```
GET /admin/monitor/metrics
```

## 八、常见问题

### Q1: 如何禁用管理中心？

**A:** 在配置文件中设置 `admin.enabled: false`

### Q2: 如何修改访问路径？

**A:** 修改配置文件中的 `admin.base_path`，例如改为 `/my-admin`

### Q3: 生产环境如何保证安全？

**A:**

1. 设置强密码并通过环境变量管理
2. 限制访问 IP（只允许内网）
3. 使用自定义访问路径
4. 启用 HTTPS

### Q4: 管理中心会影响性能吗？

**A:**

- 管理中心仅在访问时消耗资源
- 静态资源使用本地缓存
- 生产环境可设置访问限制减少暴露

### Q5: 如何扩展自定义管理功能？

**A:**
在 `framework/admin/admin.go` 的 `registerRoutes()` 函数中添加自定义路由：

```go
func registerRoutes(r *mvc.Router) {
    // ... 现有代码 ...

    // 添加自定义管理功能
    r.GET(basePath+"/custom", customHandler)
}
```

## 九、技术栈

- **后端框架**: Go + Vigo Framework
- **前端框架**: Layui 2.13.4 (离线)
- **图表库**: ECharts 5.4.0 (离线)
- **自动注册**: 类似 Swagger 的机制

## 十、更新日志

### v2.0.12 (2026-03-09)

**新增功能：**

- ✅ 完善 Nacos 配置管理功能（配置列表、服务管理）
- ✅ 完善 RabbitMQ 监控功能（队列监控、交换机监控）
- ✅ 完善健康检查页面（HTML 页面展示，非 JSON）
- ✅ 添加顶部用户下拉菜单（返回首页、修改密码、退出登录）
- ✅ 优化菜单样式（图标更清晰，文字颜色优化）
- ✅ 实时计算系统运行时长
- ✅ 实时获取内存使用数据（runtime.MemStats）
- ✅ 添加 Goroutine 数量监控

**修复问题：**

- ✅ 修复健康检查页面显示原始 JSON 问题
- ✅ 修复菜单样式看不清楚问题
- ✅ 修复静态文件路径错误（相对路径改为绝对路径）
- ✅ 修复 MIME 类型错误（使用 Handle 注册静态文件）

**技术改进：**

- ✅ 使用 `r.Handle` 注册静态文件处理器，提高性能
- ✅ 使用 Go template 渲染动态数据
- ✅ 优化代码结构和可读性

## 九、更新日志

### v1.0.0 (2026-03-09)

- ✅ 初始版本发布
- ✅ 集成 Dashboard 监控面板
- ✅ 集成 Nacos 配置管理
- ✅ 集成 RabbitMQ 监控
- ✅ 集成压力测试功能
- ✅ 集成健康检查功能
- ✅ 集成系统监控功能
- ✅ 使用 Layui 离线资源
- ✅ 使用 ECharts 离线资源
- ✅ 类似 Swagger 的自动注册机制

## 十、联系支持

如有问题或建议，请联系 Vigo 框架支持团队。
