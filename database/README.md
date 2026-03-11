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

## 使用方法

### 1. 创建迁移文件

在 `migrations/` 目录创建迁移文件，命名格式：`{timestamp}_{description}.go`

### 2. 编写迁移逻辑

编辑迁移文件，添加 `Up` 和 `Down` 函数：

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

### 3. 注册迁移

在 `migrations_registry.go` 中注册新的迁移：

```go
migrator.AddMigration(20260307120000, "create_users_table",
    Up_20260307120000_create_users_table,
    Down_20260307120000_create_users_table,
)
```

### 4. 执行迁移

迁移会在应用启动时自动执行。

## 最佳实践

1. **版本号**：使用时间戳格式确保唯一性
2. **幂等性**：确保 Up 和 Down 操作可以重复执行
3. **回滚测试**：在部署前测试回滚逻辑
4. **备份**：在生产环境执行迁移前备份数据
