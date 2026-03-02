package queue

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DatabaseDriver 数据库队列驱动
type DatabaseDriver struct {
	db    *sql.DB
	queue string
	mu    sync.Mutex
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver   string `yaml:"driver"` // mysql, postgres, sqlite
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Table    string `yaml:"table"` // 队列表名
	Sleep    int    `yaml:"sleep"` // 毫秒
}

// NewDatabaseDriver 创建数据库队列驱动
func NewDatabaseDriver(config DatabaseConfig) (*DatabaseDriver, error) {
	// 构建 DSN
	var dsn string
	switch config.Driver {
	case "mysql":
		dsn = formatMySQLDSN(config)
	case "postgres":
		dsn = formatPostgresDSN(config)
	case "sqlite":
		dsn = config.Database
	default:
		dsn = formatMySQLDSN(config)
	}

	// 连接数据库
	db, err := sql.Open(config.Driver, dsn)
	if err != nil {
		return nil, err
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// 创建队列表
	if err := createQueueTable(db, config.Table); err != nil {
		return nil, err
	}

	return &DatabaseDriver{
		db:    db,
		queue: config.Table,
	}, nil
}

// Push 推入任务
func (d *DatabaseDriver) Push(job *JobWrapper) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	query := `INSERT INTO ` + d.queue + ` (payload, available_at, created_at) VALUES (?, ?, ?)`
	_, err = d.db.Exec(query, data, job.AvailableAt, job.CreatedAt)
	return err
}

// Pop 弹出任务
func (d *DatabaseDriver) Pop(queue string) (*JobWrapper, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 查询可用任务
	query := `SELECT id, payload FROM ` + d.queue + ` WHERE available_at <= ? AND reserved_at IS NULL ORDER BY id ASC LIMIT 1`
	rows, err := d.db.Query(query, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var id int64
	var payload []byte
	if err := rows.Scan(&id, &payload); err != nil {
		return nil, err
	}

	// 标记为已保留
	_, err = d.db.Exec(`UPDATE `+d.queue+` SET reserved_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		return nil, err
	}

	// 反序列化
	var job JobWrapper
	if err := json.Unmarshal(payload, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// Delete 删除任务
func (d *DatabaseDriver) Delete(job *JobWrapper) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 从数据库删除（这里简化处理，实际需要 job ID）
	return nil
}

// Release 释放任务
func (d *DatabaseDriver) Release(job *JobWrapper, delay time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 更新可用时间
	availableAt := time.Now().Add(delay)
	_, err := d.db.Exec(`UPDATE `+d.queue+` SET reserved_at = NULL, available_at = ?, retry_count = retry_count + 1 WHERE id = ?`, availableAt, job.ID)
	return err
}

// Peek 查看队列头部任务
func (d *DatabaseDriver) Peek(queue string) (*JobWrapper, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `SELECT payload FROM ` + d.queue + ` WHERE reserved_at IS NULL ORDER BY id ASC LIMIT 1`
	row := d.db.QueryRow(query)

	var payload []byte
	if err := row.Scan(&payload); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var job JobWrapper
	if err := json.Unmarshal(payload, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// Size 获取队列大小
func (d *DatabaseDriver) Size(queue string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var count int
	query := `SELECT COUNT(*) FROM ` + d.queue + ` WHERE reserved_at IS NULL`
	err := d.db.QueryRow(query).Scan(&count)
	return count, err
}

// Clear 清空队列
func (d *DatabaseDriver) Clear(queue string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM ` + d.queue)
	return err
}

// 辅助函数

func formatMySQLDSN(config DatabaseConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.User, config.Password, config.Host, config.Port, config.Database)
}

func formatPostgresDSN(config DatabaseConfig) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.User, config.Password, config.Host, config.Port, config.Database)
}

func createQueueTable(db *sql.DB, table string) error {
	createSQL := `
	CREATE TABLE IF NOT EXISTS ` + table + ` (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		payload TEXT NOT NULL,
		queue VARCHAR(255) DEFAULT 'default',
		available_at DATETIME NOT NULL,
		reserved_at DATETIME NULL,
		retry_count INT DEFAULT 0,
		created_at DATETIME NOT NULL,
		INDEX available_at (available_at),
		INDEX reserved_at (reserved_at)
	)`

	_, err := db.Exec(createSQL)
	return err
}

// SyncDriver 同步队列驱动（用于测试）
type SyncDriver struct {
	jobs []*JobWrapper
	mu   sync.Mutex
}

// NewSyncDriver 创建同步队列驱动
func NewSyncDriver() *SyncDriver {
	return &SyncDriver{
		jobs: make([]*JobWrapper, 0),
	}
}

// Push 推入任务
func (d *SyncDriver) Push(job *JobWrapper) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.jobs = append(d.jobs, job)
	return nil
}

// Pop 弹出任务
func (d *SyncDriver) Pop(queue string) (*JobWrapper, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.jobs) == 0 {
		return nil, nil
	}

	job := d.jobs[0]
	d.jobs = d.jobs[1:]
	return job, nil
}

// Delete 删除任务
func (d *SyncDriver) Delete(job *JobWrapper) error {
	// 同步驱动不需要显式删除
	return nil
}

// Release 释放任务
func (d *SyncDriver) Release(job *JobWrapper, delay time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	job.AvailableAt = time.Now().Add(delay)
	d.jobs = append(d.jobs, job)
	return nil
}

// Peek 查看队列头部任务
func (d *SyncDriver) Peek(queue string) (*JobWrapper, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.jobs) == 0 {
		return nil, nil
	}

	return d.jobs[0], nil
}

// Size 获取队列大小
func (d *SyncDriver) Size(queue string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.jobs), nil
}

// Clear 清空队列
func (d *SyncDriver) Clear(queue string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.jobs = make([]*JobWrapper, 0)
	return nil
}
