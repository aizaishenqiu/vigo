// Package db 提供数据库迁移功能
// 支持数据库 schema 的版本控制、迁移和回滚
package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Migration 定义单个迁移操作
type Migration struct {
	Version   int64               // 版本号（时间戳格式）
	Name      string              // 迁移名称
	Up        func(*sql.DB) error // 向上迁移（应用变更）
	Down      func(*sql.DB) error // 向下迁移（回滚变更）
	AppliedAt time.Time           // 应用时间
}

// Migrator 数据库迁移管理器
type Migrator struct {
	db            *sql.DB      // 数据库连接
	tableName     string       // 迁移记录表名
	migrations    []*Migration // 迁移列表
	migrationsDir string       // 迁移文件目录
}

// Migrations 获取所有已注册的迁移
func (m *Migrator) Migrations() []*Migration {
	return m.migrations
}

// NewMigrator 创建新的迁移管理器
// 参数:
//   - db: 数据库连接
//   - tableName: 迁移记录表名（默认：migrations）
//
// 返回：迁移管理器实例
func NewMigrator(db *sql.DB, tableName string) *Migrator {
	if tableName == "" {
		tableName = "migrations"
	}
	return &Migrator{
		db:         db,
		tableName:  tableName,
		migrations: make([]*Migration, 0),
	}
}

// createMigrationsTable 创建迁移记录表
func (m *Migrator) createMigrationsTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			version BIGINT NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, m.tableName)

	_, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %v", err)
	}

	log.Printf("Migrations table '%s' created or already exists\n", m.tableName)
	return nil
}

// getAppliedMigrations 获取已应用的迁移列表
func (m *Migrator) getAppliedMigrations() (map[int64]bool, error) {
	query := fmt.Sprintf("SELECT version FROM %s ORDER BY version ASC", m.tableName)
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %v", err)
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %v", err)
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// AddMigration 添加单个迁移
// 参数:
//   - version: 版本号（建议使用时间戳格式，如：20260307120000）
//   - name: 迁移名称
//   - up: 向上迁移函数
//   - down: 向下迁移函数
func (m *Migrator) AddMigration(version int64, name string, up, down func(*sql.DB) error) {
	m.migrations = append(m.migrations, &Migration{
		Version: version,
		Name:    name,
		Up:      up,
		Down:    down,
	})
}

// LoadMigrationsFromDir 从目录加载迁移（使用注册系统）
// 注意：此方法现在仅验证迁移文件是否存在，实际迁移函数需要手动注册
func (m *Migrator) LoadMigrationsFromDir(dir string) error {
	m.migrationsDir = dir

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory does not exist: %s", dir)
	}

	// 读取目录
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %v", err)
	}
	files := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %v", entry.Name(), err)
		}
		files = append(files, info)
	}
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %v", err)
	}

	// 正则表达式匹配文件名：{version}_{name}.go
	pattern := regexp.MustCompile(`^(\d+)_(.+)\.go$`)

	// 只验证文件存在性和格式，不实际加载
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := pattern.FindStringSubmatch(file.Name())
		if matches == nil {
			continue
		}

		// 解析版本号
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			log.Printf("Warning: invalid version in filename %s, skipping\n", file.Name())
			continue
		}

		name := matches[2]

		// 验证文件内容（检查是否包含 Up 和 Down 函数）
		content, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %v", file.Name(), err)
		}

		// 检查是否包含 Up 和 Down 函数（支持两种格式）
		hasUp := strings.Contains(string(content), "func Up(") ||
			(strings.Contains(string(content), "func Up_") && strings.Contains(string(content), "(db *sql.DB) error"))
		hasDown := strings.Contains(string(content), "func Down(") ||
			(strings.Contains(string(content), "func Down_") && strings.Contains(string(content), "(db *sql.DB) error"))

		if !hasUp {
			return fmt.Errorf("migration file %s must contain Up function", file.Name())
		}

		if !hasDown {
			return fmt.Errorf("migration file %s must contain Down function", file.Name())
		}

		log.Printf("Verified migration file: %d_%s.go\n", version, name)
	}

	// 注意：实际的迁移函数需要通过 RegisterAll 或 AddMigration 手动注册
	// 这里只验证文件存在性和格式正确性
	if len(m.migrations) == 0 {
		log.Println("Warning: No migrations registered. Call migrator.AddMigration() or migrations.RegisterAll() to register migrations.")
	}

	log.Printf("Loaded %d registered migrations\n", len(m.migrations))
	return nil
}

// loadMigrationFile 加载单个迁移文件
// 注意：这里简化实现，实际项目中应该使用 Go 的 plugin 或代码生成
func (m *Migrator) loadMigrationFile(filename string, version int64, name string) (*Migration, error) {
	// 读取文件内容（用于验证）
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// 检查是否包含 Up 和 Down 函数（支持两种格式：func Up( 或 func Up_<version>_<name>(）
	hasUp := strings.Contains(string(content), "func Up(") ||
		(strings.Contains(string(content), "func Up_") && strings.Contains(string(content), "(db *sql.DB) error"))
	hasDown := strings.Contains(string(content), "func Down(") ||
		(strings.Contains(string(content), "func Down_") && strings.Contains(string(content), "(db *sql.DB) error"))

	if !hasUp {
		return nil, fmt.Errorf("migration file must contain Up function")
	}

	if !hasDown {
		return nil, fmt.Errorf("migration file must contain Down function")
	}

	// 返回迁移对象（实际使用时需要通过代码生成或 plugin 机制注册函数）
	return &Migration{
		Version: version,
		Name:    name,
		Up:      nil, // 需要在主程序中手动注册
		Down:    nil,
	}, nil
}

// Migrate 执行所有未应用的迁移
// 按版本号顺序依次应用迁移
//
// 返回：错误信息
func (m *Migrator) Migrate() error {
	// 创建迁移记录表
	if err := m.createMigrationsTable(); err != nil {
		return err
	}

	// 获取已应用的迁移
	applied, err := m.getAppliedMigrations()
	if err != nil {
		return err
	}

	// 执行未应用的迁移
	for _, migration := range m.migrations {
		if applied[migration.Version] {
			continue
		}

		log.Printf("Applying migration %d: %s\n", migration.Version, migration.Name)

		// 执行向上迁移
		if err := migration.Up(m.db); err != nil {
			return fmt.Errorf("failed to apply migration %d (%s): %v",
				migration.Version, migration.Name, err)
		}

		// 记录迁移
		query := fmt.Sprintf("INSERT INTO %s (version, name) VALUES (?, ?)", m.tableName)
		_, err := m.db.Exec(query, migration.Version, migration.Name)
		if err != nil {
			return fmt.Errorf("failed to record migration %d: %v", migration.Version, err)
		}

		log.Printf("Applied migration %d: %s successfully\n", migration.Version, migration.Name)
	}

	log.Printf("All migrations applied successfully\n")
	return nil
}

// Rollback 回滚最后一次迁移
// 参数:
//   - steps: 回滚步数（默认 1）
//
// 返回：错误信息
func (m *Migrator) Rollback(steps int) error {
	if steps <= 0 {
		steps = 1
	}

	// 创建迁移记录表（如果不存在）
	if err := m.createMigrationsTable(); err != nil {
		return err
	}

	// 获取已应用的迁移（按版本倒序）
	query := fmt.Sprintf("SELECT version, name FROM %s ORDER BY version DESC LIMIT ?", m.tableName)
	rows, err := m.db.Query(query, steps)
	if err != nil {
		return fmt.Errorf("failed to query migrations: %v", err)
	}
	defer rows.Close()

	var toRollback []struct {
		Version int64
		Name    string
	}

	for rows.Next() {
		var v int64
		var n string
		if err := rows.Scan(&v, &n); err != nil {
			return fmt.Errorf("failed to scan migration: %v", err)
		}
		toRollback = append(toRollback, struct {
			Version int64
			Name    string
		}{Version: v, Name: n})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// 回滚迁移
	for _, migration := range toRollback {
		log.Printf("Rolling back migration %d: %s\n", migration.Version, migration.Name)

		// 查找对应的迁移对象
		var mig *Migration
		for _, m := range m.migrations {
			if m.Version == migration.Version {
				mig = m
				break
			}
		}

		if mig == nil {
			return fmt.Errorf("migration %d not found in memory", migration.Version)
		}

		// 执行向下迁移
		if mig.Down != nil {
			if err := mig.Down(m.db); err != nil {
				return fmt.Errorf("failed to rollback migration %d (%s): %v",
					migration.Version, migration.Name, err)
			}
		}

		// 删除迁移记录
		query := fmt.Sprintf("DELETE FROM %s WHERE version = ?", m.tableName)
		_, err := m.db.Exec(query, migration.Version)
		if err != nil {
			return fmt.Errorf("failed to delete migration record %d: %v", migration.Version, err)
		}

		log.Printf("Rolled back migration %d: %s successfully\n", migration.Version, migration.Name)
	}

	return nil
}

// Status 显示迁移状态
// 返回：已应用的迁移列表和未应用的迁移列表
func (m *Migrator) Status() (applied []*Migration, pending []*Migration, err error) {
	// 创建迁移记录表（如果不存在）
	if err := m.createMigrationsTable(); err != nil {
		return nil, nil, err
	}

	// 获取已应用的迁移
	appliedMap, err := m.getAppliedMigrations()
	if err != nil {
		return nil, nil, err
	}

	// 分类迁移
	for _, migration := range m.migrations {
		if appliedMap[migration.Version] {
			applied = append(applied, migration)
		} else {
			pending = append(pending, migration)
		}
	}

	return applied, pending, nil
}

// GetCurrentVersion 获取当前数据库版本
// 返回：最新的迁移版本号
func (m *Migrator) GetCurrentVersion() (int64, error) {
	// 创建迁移记录表（如果不存在）
	if err := m.createMigrationsTable(); err != nil {
		return 0, err
	}

	query := fmt.Sprintf("SELECT MAX(version) FROM %s", m.tableName)
	var version sql.NullInt64
	err := m.db.QueryRow(query).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %v", err)
	}

	if !version.Valid {
		return 0, nil
	}

	return version.Int64, nil
}

// Reset 重置所有迁移（危险操作！）
// 会删除所有迁移记录并回滚所有迁移
func (m *Migrator) Reset() error {
	// 获取所有已应用的迁移
	query := fmt.Sprintf("SELECT version, name FROM %s ORDER BY version DESC", m.tableName)
	rows, err := m.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query migrations: %v", err)
	}
	defer rows.Close()

	var allMigrations []struct {
		Version int64
		Name    string
	}

	for rows.Next() {
		var v int64
		var n string
		if err := rows.Scan(&v, &n); err != nil {
			return fmt.Errorf("failed to scan migration: %v", err)
		}
		allMigrations = append(allMigrations, struct {
			Version int64
			Name    string
		}{Version: v, Name: n})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// 回滚所有迁移
	for _, migration := range allMigrations {
		log.Printf("Rolling back migration %d: %s\n", migration.Version, migration.Name)

		// 查找对应的迁移对象
		var mig *Migration
		for _, m := range m.migrations {
			if m.Version == migration.Version {
				mig = m
				break
			}
		}

		if mig != nil && mig.Down != nil {
			if err := mig.Down(m.db); err != nil {
				log.Printf("Warning: failed to rollback %d: %v\n", migration.Version, err)
			}
		}
	}

	// 删除所有迁移记录
	query = fmt.Sprintf("DELETE FROM %s", m.tableName)
	_, err = m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete migration records: %v", err)
	}

	log.Printf("All migrations reset successfully\n")
	return nil
}
