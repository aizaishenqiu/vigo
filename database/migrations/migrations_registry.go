package migrations

import (
	"database/sql"
	"log"
	"vigo/framework/db"
)

var globalMigrator *db.Migrator

// init 在包导入时自动注册迁移
func init() {
	log.Println("[Migrations] 迁移包已加载，等待 migrator 初始化...")
}

// RegisterAll 注册所有迁移
// 在应用启动时调用此函数
func RegisterAll(migrator *db.Migrator) {
	log.Println("[Migrations] 开始注册迁移函数...")

	// 20260307120000_create_users_table.go
	migrator.AddMigration(20260307120000, "create_users_table",
		Up_20260307120000_create_users_table,
		Down_20260307120000_create_users_table,
	)

	// 20260307120100_create_articles_table.go
	migrator.AddMigration(20260307120100, "create_articles_table",
		Up_20260307120100_create_articles_table,
		Down_20260307120100_create_articles_table,
	)

	// 20260307143348_create_test_table.go
	migrator.AddMigration(20260307143348, "create_test_table",
		Up_20260307143348_create_test_table,
		Down_20260307143348_create_test_table,
	)

	log.Printf("[Migrations] 已注册 %d 个迁移\n", len(migrator.Migrations()))
}

// AutoRegister 自动注册迁移（如果 migrator 不为 nil）
func AutoRegister(migrator *db.Migrator) {
	if migrator != nil {
		globalMigrator = migrator
		RegisterAll(migrator)
	}
}

// EmptyDB 用于测试的空数据库连接
var EmptyDB *sql.DB

// GetGlobalMigrator 获取全局 migrator 实例
func GetGlobalMigrator() *db.Migrator {
	return globalMigrator
}
