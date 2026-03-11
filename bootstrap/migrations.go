package bootstrap

import (
	"vigo/framework/db"
	"vigo/database/migrations"
)

// RegisterMigrations 注册所有数据库迁移
func RegisterMigrations(migrator *db.Migrator) {
	migrations.RegisterAll(migrator)
}
