# Vigo - 现代化企业级 SaaS 开发框架

**Vigo** 是一个基于 Go 语言构建的现代化企业级 SaaS 开发框架，提供完整的 MVC 架构、 ORM、微服务支持、实时通讯等能力。

> "大道至简，开发由我"

---

## ✨ 特性

- **风格 ORM**: 数据库操作
- **完整 MVC 架构**: 清晰的分层设计，便于维护和扩展
- **微服务支持**: 集成 Nacos 服务发现、RabbitMQ 消息队列
- **高性能**: 基于 Go 语言的并发优势
- **热重载开发**: 使用 Air 实现实时代码更新
- **安全防护**: 内置 XSS、SQL 注入、CSRF 等安全防护
- **多租户支持**: 内置多租户架构支持

## 📋 核心功能

### 数据库 ORM

框架提供完整 ORM 能力：

- **查询构造器**: Where / WhereIn / WhereBetween / WhereNull / WhereLike / WhereRaw / WhereMap
- **链式操作**: Field / Order / Limit / Page / Group / Having / Join / Distinct / Lock
- **聚合查询**: Count / Sum / Avg / Max / Min / Exists
- **高级功能**: Paginate(分页) / Chunk(分块) / Inc(自增) / Dec(自减) / InsertAll(批量) / ToJson
- **模型(Model)**: Save / Create / Delete / ForceDelete / Restore / Destroy
- **模型特性**: 自动时间戳 / 软删除 / 获取器修改器 / 脏数据检测 / 只读字段 / 模型事件
- **关联查询**: HasOne / HasMany / BelongsTo / BelongsToMany / HasOneThrough / HasManyThrough
- **输出控制**: Hidden / Visible / Append / ToArray / ToJson

### 服务组件

- **Nacos**: 配置中心与服务发现
- **RabbitMQ**: 消息队列管理
- **Redis**: 高性能缓存
- **gRPC**: 微服务通信
- **WebSocket**: 实时通讯

## 🚀 快速开始

### 环境要求

- Go 1.21+
- Docker (可选，用于部署)

### 本地开发

```bash
# 克隆项目
git clone <repository-url>
cd vigo

# 安装依赖
go mod tidy

# 创建本地配置
cp config.yaml config.local.yaml
# 编辑 config.local.yaml 配置数据库、Redis 等连接信息

# 启动开发服务器（带热重载）
air

# 或直接运行
go run main.go
```

### Docker 部署

Vigo 提供完整的 Docker 部署方案，支持一键部署所有服务。

#### 准备配置文件

在部署前，需要准备适用于Docker环境的配置文件 `config.docker.yaml`：

```bash
# 复制示例配置文件并根据您的环境进行调整
cp config.yaml config.docker.yaml
# 编辑 config.docker.yaml 配置服务连接信息
```

#### 使用 Docker Compose

```bash
# 构建并启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f app

# 访问应用
# 应用主页: http://localhost:8080
# Nacos 控制台: http://localhost:8848/nacos
# RabbitMQ 控制台: http://localhost:15672
```

Docker Compose 配置包含以下服务：

- **app**: Vigo 应用服务
- **mysql**: MySQL 数据库
- **redis**: Redis 缓存
- **rabbitmq**: RabbitMQ 消息队列
- **nacos**: Nacos 服务发现与配置中心

> 详细部署说明请参阅 [Docker部署指南](使用文档/Docker部署指南.md)

### Linux 服务器部署

除了 Docker 方式，您还可以将应用直接部署到 Linux 服务器：

- 通过 Go 交叉编译生成 Linux 二进制文件
- 配置系统服务 (systemd/supervisor)
- 设置环境变量和配置文件
- 配置安全和性能优化

> 详细部署说明请参阅 [Go交叉编译与Linux部署指南](使用文档/Go交叉编译与Linux部署指南.md)

## 📚 文档

完整文档请查看 `使用文档/` 目录：

- [目录](使用文档/目录.md) - 完整文档索引
- [0.极速上手指南](使用文档/0.极速上手指南.md) - 框架核心优势和架构
- [1.快速开始](使用文档/1.快速开始.md) - 环境要求和安装启动
- [16.Nacos服务管理](使用文档/16.Nacos服务管理.md) - 服务发现与配置管理
- [15.RabbitMQ消息队列](使用文档/15.RabbitMQ消息队列.md) - 消息队列管理
- [Docker部署指南](Docker部署指南.md) - 完整的 Docker 部署说明

## 🔧 主要功能模块

| 路径           | 功能                                                       |
| -------------- | ---------------------------------------------------------- |
| `/`            | **控制面板** - 所有模块入口、服务状态总览                  |
| `/benchmark`   | **压力测试** - QPS/Redis/MySQL/MQ 压测，WebSocket 实时监控 |
| `/performance` | **性能测试** - 数据库基准、SQL 效率诊断                    |
| `/monitor`     | **系统监控** - CPU/内存/磁盘/网络大屏                      |
| `/rabbitmq`    | **RabbitMQ 管理** - 队列/交换机/消息管理                   |
| `/nacos`       | **Nacos 管理** - 配置管理/服务注册发现                     |
| `/docs`        | **API 文档** - Swagger UI                                  |
| `/health`      | **健康检查** - 微服务探活接口                              |

## 🤝 贡献

欢迎提交 Issue 和 Pull Request 来帮助改进 Vigo 框架。

## 📄 许可证

MIT License

---

**Vigo** - 让 Go 语言开发更简单、更高效
