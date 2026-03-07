# 数据库迁移

本目录包含数据库迁移相关文件。

## 目录结构

```
database/
├── migrations/          # 迁移文件目录
│   ├── 20260307120000_create_users_table.go
│   ├── 20260307120100_create_articles_table.go
│   └── ...
└── README.md           # 本文件
```

## 数据迁移功能

Vigo 框架提供完整的数据库迁移功能，用于管理数据库 schema 的版本控制。

### 主要特性

- ✅ 版本控制：每个迁移都有唯一的版本号
- ✅ Up/Down 迁移：支持应用变更和回滚
- ✅ 自动记录：自动记录已应用的迁移
- ✅ CLI 工具：命令行操作
- ✅ Web 界面：可视化管理界面

## 使用方法

### 1. 创建迁移文件

```bash
go run cmd/migrate/main.go create create_users_table
```

这将在 `migrations/` 目录下创建一个新的迁移文件。

### 2. 编辑迁移文件

编辑生成的迁移文件，添加 `Up` 和 `Down` 函数：

```go
package migrations

import (
    "database/sql"
    "log"
)

func Up_20260307120000_create_users_table(db *sql.DB) error {
    log.Println("Creating users table...")
    
    query := `
        CREATE TABLE IF NOT EXISTS users (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            username VARCHAR(50) NOT NULL UNIQUE,
            email VARCHAR(100) NOT NULL UNIQUE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
    `
    
    _, err := db.Exec(query)
    return err
}

func Down_20260307120000_create_users_table(db *sql.DB) error {
    log.Println("Dropping users table...")
    _, err := db.Exec("DROP TABLE IF EXISTS users")
    return err
}
```

### 3. 执行迁移

```bash
# 执行所有未应用的迁移
go run cmd/migrate/main.go migrate

# 查看迁移状态
go run cmd/migrate/main.go status

# 回滚最后一个迁移
go run cmd/migrate/main.go rollback

# 回滚多个迁移
go run cmd/migrate/main.go rollback 3

# 重置所有迁移
go run cmd/migrate/main.go reset
```

### 4. Web 界面管理

访问迁移管理界面：

```
http://localhost:8080/migration
```

在 Web 界面中，你可以：
- 查看迁移状态
- 执行迁移
- 回滚迁移
- 重置所有迁移

## 迁移记录表

框架会自动创建 `migrations` 表来记录已应用的迁移：

```sql
CREATE TABLE migrations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    version BIGINT NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 注意事项

1. **版本号唯一**：每个迁移文件的版本号必须唯一
2. **顺序执行**：迁移按版本号顺序执行
3. **事务安全**：每个迁移在事务中执行，失败自动回滚
4. **幂等性**：Up 和 Down 操作应该是幂等的
5. **测试迁移**：在生产环境执行前，先在测试环境验证

## 相关文档

- [数据迁移使用指南](../使用文档/03.数据库/13.数据迁移使用指南.md) - 完整使用文档

## 示例迁移文件

查看 `migrations/` 目录下的示例文件：
- `20260307120000_create_users_table.go` - 创建用户表示例
- `20260307120100_create_articles_table.go` - 创建文章表示例
