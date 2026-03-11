package debug

import (
	"testing"

	"vigo/config"
	"vigo/framework/container"
	"vigo/framework/db"
)

func TestGetDatabaseData_WhenDBNotReady_ShouldBeDisconnected(t *testing.T) {
	origin := config.App.Database
	defer func() { config.App.Database = origin }()

	config.App.Database.Driver = "mysql"
	config.App.Database.Host = "127.0.0.1"
	config.App.Database.Port = 3306
	config.App.Database.Name = "test"

	oldDB := db.GlobalDB
	db.GlobalDB = nil
	defer func() { db.GlobalDB = oldDB }()

	dt := &DebugToolbar{}
	data := dt.getDatabaseData()
	if data["connected"] != "未连接" {
		t.Fatalf("expected 未连接, got %v", data["connected"])
	}
}

func TestGetCacheData_WhenRedisNotReady_ShouldBeDisconnected(t *testing.T) {
	origin := config.App.Redis
	defer func() { config.App.Redis = origin }()

	config.App.Redis.Host = "127.0.0.1"
	config.App.Redis.Port = 6379

	container.App().Singleton("redis", nil)

	dt := &DebugToolbar{}
	data := dt.getCacheData()
	if data["connected"] != "未连接" {
		t.Fatalf("expected 未连接, got %v", data["connected"])
	}
}
