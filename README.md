# Vigo - 现代化企业级 SaaS 开发框架 v2.0.12

<div align="center">

![Vigo Logo](https://img.shields.io/badge/Vigo-v2.0.12-blue)
![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-MIT-green)
![Build Status](https://img.shields.io/badge/build-passing-brightgreen)

**大道至简，开发由我**

[🌐 官网](public/index.html) • [📖 文档](使用文档/00.目录/目录.md) • [🚀 快速开始](使用文档/01.入门指南/1.快速开始.md) • [📝 示例](#-快速示例) • [💬 社区](#-社区)

[🇺🇸 English Version](README_EN.md)

</div>

---

## 📖 简介

**Vigo** 是一个基于 Go 1.21+ 构建的现代化企业级 SaaS 开发框架，集高性能、完整功能、易用性于一体。框架提供 MVC 架构、ORM、微服务、实时通讯等全套解决方案，助力开发者快速构建高并发、高可用的企业级应用。

### 🎯 核心优势

| 优势         | 说明                                   | 收益                    |
| ------------ | -------------------------------------- | ----------------------- |
| **极致性能** | Go 编译型 + 协程并发 + 连接池优化      | QPS 提升 **18-30 倍**   |
| **功能完整** | ORM/队列/缓存/监控/支付/GraphQL 全覆盖 | 开发效率提升 **60%+**   |
| **易于上手** | 清晰架构 + 代码生成 + 完善文档         | 学习成本降低 **50%**    |
| **生产就绪** | 监控告警 + Docker 部署 + 安全防护      | 系统稳定性提升 **90%+** |

---

## ✨ 核心特性

### 🔥 性能领先

- **高并发**: 单节点支持 **10 万+** 并发连接
- **低延迟**: 平均响应时间 **< 1ms**
- **低资源**: 内存占用仅 **15-25MB**
- **快启动**: 冷启动时间 **< 10ms**
- **自研路由**: 基于 Radix Tree 的高性能路由系统

### 🛠️ 功能完善

#### 数据库 ORM

- ✅ 完整查询构造器（Where/Join/Group/Order）
- ✅ 链式操作 + 聚合查询
- ✅ 模型关联（HasOne/HasMany/BelongsToMany）
- ✅ JSON 查询 + 软删除 + 自动时间戳
- ✅ 乐观锁 + 分布式锁 + 原子操作
- ✅ 分库分表 + 读写分离
- ✅ 多数据库支持（MySQL/PostgreSQL/SQLite/SQL Server）

#### 开发效率工具

- ✅ CLI 代码生成器（Controller/Model/Service/Middleware）
- ✅ 一键 CRUD 生成（Scaffold）
- ✅ 调试工具栏 + 性能分析器
- ✅ 查询日志 + 内存分析

#### 自研 MVC 框架

- ✅ 基于 Radix Tree 的高性能路由
- ✅ 支持动态参数和通配符路由
- ✅ 路由分组和中间件
- ✅ 视图引擎和模板渲染
- ✅ 请求上下文管理
- ✅ 对象池优化性能

#### 第三方服务集成

- ✅ **支付**: 支付宝/微信支付完整支持
- ✅ **OSS**: 阿里云/七牛云/腾讯云
- ✅ **短信**: 阿里云/腾讯云

#### 监控运维

- ✅ 系统指标监控（CPU/内存/Goroutine）
- ✅ 请求指标监控（QPS/延迟/错误率）
- ✅ 多渠道告警（邮件/Webhook/日志）
- ✅ 数据库/缓存连接池监控

#### 高级特性

- ✅ **GraphQL**: 完整查询/变更支持
- ✅ **队列系统**: Redis/Database/RabbitMQ
- ✅ **缓存系统**: 多级缓存 + 标签管理
- ✅ **微服务**: gRPC + Nacos + RabbitMQ
- ✅ **WebSocket**: 实时通讯支持

### 🆕 v2.0 新增功能

#### Nacos 配置中心

- ✅ 动态配置加载和实时更新
- ✅ 支持 JSON、YAML、Properties 格式
- ✅ 配置变更监听和自动刷新
- ✅ 配置缓存和多环境支持
- ✅ 配置加密和安全存储

#### Nacos 服务发现

- ✅ 服务注册与自动注销
- ✅ 服务发现和健康检查
- ✅ 负载均衡（随机/轮询/最少连接）
- ✅ 服务元数据管理
- ✅ 服务变更监听

#### Redis 缓存适配器

- ✅ 完整的 Redis 缓存支持
- ✅ 支持字符串/对象/批量操作
- ✅ 标签缓存管理
- ✅ 连接池优化
- ✅ 自动序列化和 TTL 管理

#### 分布式限流器

- ✅ 基于 Redis 的分布式限流
- ✅ 令牌桶算法
- ✅ 滑动窗口算法
- ✅ Lua 脚本保证原子性
- ✅ 支持突发流量控制

#### 定时任务调度器

- ✅ 基于 Cron 的定时任务
- ✅ 支持秒级任务调度
- ✅ 任务启用/禁用管理
- ✅ 任务状态监控
- ✅ 并发控制和超时处理

#### 负载均衡器

- ✅ 多种负载均衡算法
- ✅ 随机/轮询/最少连接/加权
- ✅ 服务实例管理
- ✅ 健康检查集成

#### gRPC 连接池

- ✅ gRPC 客户端连接池
- ✅ 连接复用和自动扩缩容
- ✅ 健康检查
- ✅ 性能优化（5x 提升）

#### HTTP/2 支持

- ✅ HTTP/2 协议支持
- ✅ h2c（明文 HTTP/2）
- ✅ h2（TLS HTTP/2）
- ✅ 多路复用
- ✅ 性能优化（2x 提升）

#### Prometheus 指标监控

- ✅ Prometheus 指标收集
- ✅ 自定义指标
- ✅ 指标暴露
- ✅ Grafana 集成

### 🔒 安全可靠

- **输入验证**: 自动 XSS/SQL 注入防护
- **CSRF 防护**: 跨站请求伪造防御
- **JWT 认证**: 完整的身份验证机制
- **限流熔断**: Gateway 层流量控制
- **安全配置**: 敏感信息环境变量管理

---

## 📊 性能对比

### 基准测试（QPS）

| 场景                | Vigo     |
| ------------------- | -------- |
| **Hello World**     | 150,000+ |
| **Database Query**  | 50,000+  |
| **JSON API**        | 100,000+ |
| **Cache Operation** | 200,000+ |

### 资源占用对比

| 指标                       | Vigo    |
| -------------------------- | ------- |
| **Memory Usage**           | 15-25MB |
| **Concurrent Connections** | 10 万+  |
| **Startup Time**           | <10ms   |

> 💡 **测试环境**: Intel i9-13900K / 32GB DDR5 / MySQL 8.0 / 1000 并发连接

---

## 🚀 快速开始

### 环境要求

- ✅ Go 1.21+
- ✅ MySQL 8.0+ / PostgreSQL 14+
- ✅ Redis 6.0+
- ✅ Nacos 2.0+（可选，微服务需要）
- ✅ Docker & Docker Compose（可选）

### 安装步骤

#### 1. 克隆项目

**方式一：从 Gitee 克隆（推荐国内用户）**

```bash
git clone https://gitee.com/yjk100_admin/vigo.git
cd vigo
```

**方式二：从 GitHub 克隆**

```bash
git clone https://github.com/aizaishenqiu/vigo.git
cd vigo
```

#### 2. 安装依赖

```bash
go mod tidy
```

#### 3. 配置环境

```bash
# 复制配置文件
cp config.yaml config.local.yaml

# 编辑配置（数据库、Redis、Nacos 等）
vim config.local.yaml
```

#### 4. 启动服务

```bash
# 方式一：直接运行
go run main.go

# 方式二：热重载开发（推荐）
air

# 方式三：编译后运行
go build -o vigo main.go
./vigo
```

#### 5. 访问应用

- 🌐 **应用主页**: <http://localhost:8080>
- 📊 **系统监控**: <http://localhost:8080/monitor>
- 🧪 **压力测试**: <http://localhost:8080/benchmark>
- 📖 **API 文档**: <http://localhost:8080/docs>

---

## 💻 快速示例

### 代码生成（推荐）

```bash
# 一键生成完整 CRUD
vigo make:crud User

# 生成单个组件
vigo make:controller Product
vigo make:model Order
vigo make:service Payment
vigo make:middleware Auth
```

### 数据库操作

```go
package main

import (
 "vigo/framework/model"
)

type User struct {
 *model.Model
 ID   int64  `json:"id"`
 Name string `json:"name"`
 Age  int    `json:"age"`
}

// 创建用户
user := &User{Name: "John", Age: 25}
user.Create()

// 查询用户
u, _ := User.Where("age", ">", 18).First()

// 更新用户
u.Update(map[string]interface{}{"age": 26})

// 删除用户
u.Delete()
```

### Nacos 配置中心

```go
import "vigo/framework/config"

// 创建 Nacos 配置
nacosCfg, _ := config.NewNacosConfig(&config.NacosConfigOptions{
 ServerAddr:  "127.0.0.1:8848",
 DataId:      "app.yaml",
 Group:       "DEFAULT_GROUP",
 Format:      "yaml",
})

// 加载配置
nacosCfg.Load(ctx)

// 监听配置变化
go nacosCfg.Watch(ctx)

// 获取配置
dbDSN, _ := nacosCfg.Get("database.dsn")
```

### Redis 缓存

```go
import "vigo/framework/cache"

// 创建缓存适配器
redisCache := cache.NewRedisCacheAdapter(client, &cache.RedisCacheOptions{
 Prefix: "app:",
})

// 设置缓存
redisCache.Set("user:1", user, time.Hour)

// 获取缓存
value, _ := redisCache.Get("user:1")

// 删除缓存
redisCache.Delete("user:1")
```

### 分布式限流

```go
import "vigo/framework/limiter"

// 创建限流器
limiter := limiter.NewDistributedLimiter(&limiter.DistributedLimiterOptions{
 Client:   redisClient,
 Key:      "api:limit:user:1",
 Rate:     100,
 Burst:    200,
 Interval: time.Second,
})

// 使用限流器
if limiter.Allow() {
 // 处理请求
} else {
 // 拒绝请求
}
```

### 定时任务

```go
import "vigo/framework/scheduler"

// 创建调度器
sched := scheduler.NewScheduler(&scheduler.SchedulerOptions{
 Location: time.Local,
})

// 添加定时任务（每天凌晨 2 点）
sched.AddTask("daily-report", "0 0 2 * * *", func() {
 generateReport()
})

// 启动调度器
sched.Start()
```

---

## 📚 文档导航

### 入门指南

- [快速开始](使用文档/01.入门指南/1.快速开始.md)
- [环境配置](使用文档/01.入门指南/0.极速上手指南.md)
- [目录结构](使用文档/01.入门指南/1.快速开始.md)

### 核心功能

- [路由与控制器](使用文档/02.核心功能/01.路由与控制器.md)
- [数据库 ORM](使用文档/03.数据库/01.连接数据库.md)
- [缓存系统](使用文档/04.安全防护/02.缓存管理.md)
- [中间件](使用文档/04.安全防护/04.中间件.md)

### v2.0 新增功能

- [新增功能总览](使用文档/11.框架优化/00.新增功能文档索引.md)
- [Nacos 配置中心](使用文档/11.框架优化/02.Nacos 配置中心使用指南.md)
- [Nacos 服务发现](使用文档/11.框架优化/03.Nacos 服务发现使用指南.md)
- [Redis 缓存适配器](使用文档/11.框架优化/04.Redis 缓存适配器使用指南.md)
- [分布式限流器](使用文档/11.框架优化/05.分布式限流器使用指南.md)
- [定时任务调度器](使用文档/11.框架优化/06.定时任务调度器使用指南.md)

### 性能优化

- [性能优化报告](使用文档/11.框架优化/07.框架 v2.0 优化实施报告.md)
- [优化总结](使用文档/11.框架优化/08.框架 v2.0 优化总结.md)
- [快速参考](使用文档/11.框架优化/09.框架快速参考.md)

---

## 🏗️ 架构视图

### 单体架构

```
┌─────────────────────────────────────────────────────────┐
│                      Vigo 应用                           │
├─────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │Controller│  │ Service  │  │  Model   │              │
│  └──────────┘  └──────────┘  └──────────┘              │
│         │             │             │                   │
│  ┌──────┴─────────────┴─────────────┴──────┐           │
│  │  Nacos  │  Redis  │  MySQL  │  RabbitMQ │          │
│  └─────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────┘
```

### 微服务架构

```
┌─────────────────────────────────────────────────────────┐
│                     API Gateway                          │
└─────────────────────────────────────────────────────────┘
        │                │                │
        ▼                ▼                ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│  用户服务     │ │  订单服务     │ │  商品服务     │
│  (Vigo)      │ │  (Vigo)      │ │  (Vigo)      │
└──────────────┘ └──────────────┘ └──────────────┘
        │                │                │
        ▼                ▼                ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│  用户数据库   │ │  订单数据库   │ │  商品数据库   │
└──────────────┘ └──────────────┘ └──────────────┘

              ┌────────────────────────┐
              │   Nacos 注册发现中心    │
              └────────────────────────┘
```

---

## 🎯 适用场景

- ✅ **企业级 SaaS 应用**: 多租户、高并发、可扩展
- ✅ **微服务架构**: 服务拆分、独立部署、弹性伸缩
- ✅ **API 网关**: 统一入口、限流鉴权、协议转换
- ✅ **实时通讯**: WebSocket、消息推送、在线客服
- ✅ **数据处理**: 批量处理、定时任务、异步队列
- ✅ **监控系统**: 指标收集、告警通知、可视化

---

## 🔧 开发工具

### vigoctl

```bash
# 安装 vigoctl
go install github.com/vigo/vigoctl@latest

# 生成 CRUD 代码
vigo make:crud User

# 生成 API 代码
vigo api generate --api=user.api

# 生成模型代码
vigo model generate --table=users
```

### Air（热重载）

```bash
# 安装
go install github.com/cosmtrek/air@latest

# 运行
air
```

---

## 📦 部署

### Docker 部署

```bash
# 构建镜像
docker build -t vigo-app .

# 运行容器
docker run -d -p 8080:8080 vigo-app
```

### Docker Compose

```yaml
version: "3"
services:
  app:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - mysql
      - redis
      - nacos

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root

  redis:
    image: redis:7-alpine

  nacos:
    image: nacos/nacos-server:2.3.0
    environment:
      MODE: standalone
```

---

## 🤝 社区

- 💬 QQ 群：1085098216
- 📖 **官方文档**: [使用文档/00.目录/目录.md](使用文档/00.目录/目录.md)
- 🌐 **官网文档**: [官网文档](https://doc.foucui.cn)
- 📧 **联系我们**: <yjk150@qq.com>
- 🐙 **代码仓库**: [Gitee](https://gitee.com/yjk100_admin/vigo) | [GitHub](https://github.com/aizaishenqiu/vigo)

---

## 📄 开源协议

Vigo 框架采用 [MIT](LICENSE) 开源协议。

---

## 🙏 致谢

感谢以下开源项目：

- [Go](https://golang.org/) - 强大的编程语言
- [Gin](https://github.com/gin-gonic/gin) - 高性能 Web 框架
- [Nacos](https://nacos.io/) - 配置中心和服务发现
- [Redis](https://redis.io/) - 高性能缓存
- [RabbitMQ](https://www.rabbitmq.com/) - 消息队列
- [Prometheus](https://prometheus.io/) - 指标监控

---

<div align="center">

**Made with ❤️ by Vigo Team**

![Star](https://img.shields.io/github/stars/aizaishenqiu/vigo?style=social)
![Fork](https://img.shields.io/github/forks/aizaishenqiu/vigo?style=social)
![Watch](https://img.shields.io/github/watchers/aizaishenqiu/vigo?style=social)

[🌟 Star on GitHub](https://github.com/aizaishenqiu/vigo) | [🌟 Star on Gitee](https://gitee.com/yjk100_admin/vigo)

</div>
