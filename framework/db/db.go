// Package db 提供数据库连接管理和查询功能
// 支持 MySQL、PostgreSQL、SQLite、SQL Server 等多种数据库
// 支持读写分离、多数据库连接和多主库负载均衡
package db

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/denisenkom/go-mssqldb" // SQL Server 驱动
	mysqlDriver "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"           // PostgreSQL 驱动
	_ "github.com/mattn/go-sqlite3" // SQLite 驱动
)

// QueryLog SQL查询日志记录
type QueryLog struct {
	SQL      string        // SQL语句
	Duration time.Duration // 执行时间
	Args     []interface{} // 参数
	Time     time.Time     // 执行时间点
}

// QueryLogger 查询日志记录器（带大小限制的环形缓冲区）
type QueryLogger struct {
	mu        sync.RWMutex
	queries   []QueryLog
	maxSize   int           // 最大记录数（防止内存泄漏）
	enabled   bool
	evictIdx  int           // 淘汰索引（用于环形缓冲区）
}

// GlobalQueryLogger 全局查询日志记录器
// 默认最多保留 1000 条记录，超过后自动淘汰最旧的记录
var GlobalQueryLogger = &QueryLogger{
	queries: make([]QueryLog, 0, 1000),
	maxSize: 1000,
	enabled: true,
}

// AddQuery 添加查询记录（环形缓冲区实现，自动淘汰最旧记录）
func (ql *QueryLogger) AddQuery(sql string, duration time.Duration, args ...interface{}) {
	if !ql.enabled {
		return
	}
	ql.mu.Lock()
	defer ql.mu.Unlock()

	query := QueryLog{
		SQL:      sql,
		Duration: duration,
		Args:     args,
		Time:     time.Now(),
	}

	// 如果达到最大容量，使用环形缓冲区策略淘汰最旧的记录
	if ql.maxSize > 0 && len(ql.queries) >= ql.maxSize {
		// 环形缓冲区：覆盖最旧的记录
		ql.queries[ql.evictIdx] = query
		ql.evictIdx = (ql.evictIdx + 1) % ql.maxSize
	} else {
		ql.queries = append(ql.queries, query)
	}
}

// SetMaxSize 设置最大记录数（0 表示不限制）
func (ql *QueryLogger) SetMaxSize(size int) {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	ql.maxSize = size
	if size > 0 && len(ql.queries) > size {
		// 如果新限制小于当前记录数，截断数组
		ql.queries = ql.queries[:size]
	}
}

// GetQueries 获取查询记录
func (ql *QueryLogger) GetQueries() []QueryLog {
	ql.mu.RLock()
	defer ql.mu.RUnlock()
	result := make([]QueryLog, len(ql.queries))
	copy(result, ql.queries)
	return result
}

// Clear 清空查询记录
func (ql *QueryLogger) Clear() {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	ql.queries = make([]QueryLog, 0)
}

// Enable 启用查询记录
func (ql *QueryLogger) Enable() {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	ql.enabled = true
}

// Disable 禁用查询记录
func (ql *QueryLogger) Disable() {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	ql.enabled = false
}

// GlobalDB 全局主数据库连接（用于写操作）
var GlobalDB *sql.DB

// CurrentDriver 当前使用的数据库驱动类型
var CurrentDriver string = "mysql"

// ReadDBs 只读数据库连接池（用于读操作，读写分离）
var ReadDBs []*sql.DB

// WriteDBs 写数据库连接池（用于多主库写操作）
var WriteDBs []*sql.DB

// readCounter 只读数据库轮询计数器（原子操作）
var readCounter uint64

// writeCounter 写数据库轮询计数器（原子操作）
var writeCounter uint64

// DBManager 多数据库管理器
type DBManager struct {
	mu       sync.RWMutex
	connections map[string]*DBConnection
}

// DBConnection 单个数据库连接组（包含主库和从库）
type DBConnection struct {
	Name     string    // 连接名称
	Driver   string    // 驱动类型
	Write    *sql.DB   // 写库连接
	Reads    []*sql.DB // 读库连接列表
	readIdx  uint64    // 读库轮询索引
}

// globalDBManager 全局数据库管理器实例
var globalDBManager = &DBManager{
	connections: make(map[string]*DBConnection),
}

// silentMySQLLogger 静默 MySQL 日志记录器
// 用于抑制 MySQL 驱动内部的连接错误日志，避免高并发时控制台刷屏
type silentMySQLLogger struct{}

func (s silentMySQLLogger) Print(_ ...interface{}) {}

// Init 初始化主数据库连接
// 参数:
//   - driver: 数据库驱动类型 (mysql, postgres, sqlite3, mssql)
//   - dsn: 数据源连接字符串
//   - maxOpen: 最大打开连接数
//   - maxIdle: 最大空闲连接数
//   - maxLifeTime: 连接最大生命周期（秒）
//   - maxIdleTime: 空闲连接回收时间（秒）
//
// 返回: 错误信息
func Init(driver, dsn string, maxOpen, maxIdle, maxLifeTime, maxIdleTime int) error {
	CurrentDriver = driver

	// 抑制 MySQL 驱动内部日志（高并发压测时避免控制台刷屏）
	_ = mysqlDriver.SetLogger(silentMySQLLogger{})

	var err error
	GlobalDB, err = sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("failed to open database (%s): %v", driver, err)
	}

	// 配置数据库连接池
	configureDB(GlobalDB, maxOpen, maxIdle, maxLifeTime, maxIdleTime)

	// 测试数据库连接
	if err = GlobalDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database (%s): %v", driver, err)
	}

	log.Printf("Main Database (%s) connected successfully\n", driver)
	return nil
}

// InitReadDBs 初始化只读数据库连接池
// 用于实现读写分离，主库负责写，从库负责读
// 参数:
//   - driver: 数据库驱动类型
//   - dsns: 只读数据库 DSN 列表
//   - maxOpen: 最大打开连接数
//   - maxIdle: 最大空闲连接数
//   - maxLifeTime: 连接最大生命周期（秒）
//   - maxIdleTime: 空闲连接回收时间（秒）
func InitReadDBs(driver string, dsns []string, maxOpen, maxIdle, maxLifeTime, maxIdleTime int) error {
	for _, dsn := range dsns {
		db, err := sql.Open(driver, dsn)
		if err != nil {
			log.Printf("Warning: failed to open read database (%s): %v", dsn, err)
			continue
		}
		configureDB(db, maxOpen, maxIdle, maxLifeTime, maxIdleTime)
		if err = db.Ping(); err != nil {
			log.Printf("Warning: failed to ping read database (%s): %v", dsn, err)
			continue
		}
		ReadDBs = append(ReadDBs, db)
	}
	if len(ReadDBs) > 0 {
		log.Printf("%d Read Databases (%s) connected successfully", len(ReadDBs), driver)
	}
	return nil
}

// InitWriteDBs 初始化多写数据库连接池
// 用于多主库写入负载均衡
// 参数:
//   - driver: 数据库驱动类型
//   - dsns: 写数据库 DSN 列表
//   - maxOpen: 最大打开连接数
//   - maxIdle: 最大空闲连接数
//   - maxLifeTime: 连接最大生命周期（秒）
//   - maxIdleTime: 空闲连接回收时间（秒）
func InitWriteDBs(driver string, dsns []string, maxOpen, maxIdle, maxLifeTime, maxIdleTime int) error {
	for _, dsn := range dsns {
		db, err := sql.Open(driver, dsn)
		if err != nil {
			log.Printf("Warning: failed to open write database (%s): %v", dsn, err)
			continue
		}
		configureDB(db, maxOpen, maxIdle, maxLifeTime, maxIdleTime)
		if err = db.Ping(); err != nil {
			log.Printf("Warning: failed to ping write database (%s): %v", dsn, err)
			continue
		}
		WriteDBs = append(WriteDBs, db)
	}
	if len(WriteDBs) > 0 {
		log.Printf("%d Write Databases (%s) connected successfully", len(WriteDBs), driver)
	}
	return nil
}

// RegisterConnection 注册命名数据库连接
// 用于多数据库场景，每个业务库独立管理
func RegisterConnection(name, driver string, writeDSN string, readDSNs []string, maxOpen, maxIdle, maxLifeTime, maxIdleTime int) error {
	conn := &DBConnection{
		Name:   name,
		Driver: driver,
		Reads:  make([]*sql.DB, 0),
	}

	// 初始化写库
	writeDB, err := sql.Open(driver, writeDSN)
	if err != nil {
		return fmt.Errorf("failed to open write database '%s': %v", name, err)
	}
	configureDB(writeDB, maxOpen, maxIdle, maxLifeTime, maxIdleTime)
	if err = writeDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping write database '%s': %v", name, err)
	}
	conn.Write = writeDB

	// 初始化读库
	for _, dsn := range readDSNs {
		readDB, err := sql.Open(driver, dsn)
		if err != nil {
			log.Printf("Warning: failed to open read database '%s': %v", name, err)
			continue
		}
		configureDB(readDB, maxOpen, maxIdle, maxLifeTime, maxIdleTime)
		if err = readDB.Ping(); err != nil {
			log.Printf("Warning: failed to ping read database '%s': %v", name, err)
			continue
		}
		conn.Reads = append(conn.Reads, readDB)
	}

	// 注册到管理器
	globalDBManager.mu.Lock()
	globalDBManager.connections[name] = conn
	globalDBManager.mu.Unlock()

	log.Printf("Database '%s' registered (writes: 1, reads: %d)", name, len(conn.Reads))
	return nil
}

// GetConnection 获取命名数据库连接
func GetConnection(name string) (*DBConnection, error) {
	globalDBManager.mu.RLock()
	defer globalDBManager.mu.RUnlock()

	conn, ok := globalDBManager.connections[name]
	if !ok {
		return nil, fmt.Errorf("database connection '%s' not found", name)
	}
	return conn, nil
}

// GetWriteDB 获取写数据库连接（命名连接）
func (c *DBConnection) GetWriteDB() *sql.DB {
	return c.Write
}

// GetReadDB 获取读数据库连接（命名连接，轮询负载均衡）
func (c *DBConnection) GetReadDB() *sql.DB {
	n := len(c.Reads)
	if n == 0 {
		return c.Write
	}
	idx := atomic.AddUint64(&c.readIdx, 1) % uint64(n)
	return c.Reads[idx]
}

// configureDB 配置数据库连接池参数
// 参数:
//   - db: 数据库连接对象
//   - maxOpen: 最大打开连接数
//   - maxIdle: 最大空闲连接数
//   - maxLifeTime: 连接最大生命周期（秒）
//   - maxIdleTime: 空闲连接回收时间（秒）
func configureDB(db *sql.DB, maxOpen, maxIdle, maxLifeTime, maxIdleTime int) {
	if maxOpen <= 0 {
		maxOpen = 100
	}
	if maxIdle <= 0 {
		maxIdle = 10
	}
	if maxLifeTime <= 0 {
		maxLifeTime = 3600
	}
	if maxIdleTime <= 0 {
		maxIdleTime = 300
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(maxLifeTime) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(maxIdleTime) * time.Second)
}

// GetReadDB 获取只读数据库连接（原子轮询）
// 如果没有配置只读数据库，则返回主数据库连接
// 使用原子操作实现轮询负载均衡
func GetReadDB() *sql.DB {
	// 优先使用多写库中的读库
	n := len(ReadDBs)
	if n == 0 {
		// 如果配置了多写库，从写库中轮询读取
		if len(WriteDBs) > 0 {
			idx := atomic.AddUint64(&writeCounter, 1) % uint64(len(WriteDBs))
			return WriteDBs[idx]
		}
		return GlobalDB
	}
	idx := atomic.AddUint64(&readCounter, 1) % uint64(n)
	return ReadDBs[idx]
}

// GetWriteDB 获取写数据库连接（支持多主库负载均衡）
// 如果配置了多写库，则轮询选择；否则返回主库
func GetWriteDB() *sql.DB {
	n := len(WriteDBs)
	if n == 0 {
		return GlobalDB
	}
	idx := atomic.AddUint64(&writeCounter, 1) % uint64(n)
	return WriteDBs[idx]
}

// Table 创建查询构造器（主库）
// 参数:
//   - name: 表名
//
// 返回: 查询构造器对象
func Table(name string) *Query {
	return NewQuery(GlobalDB).Table(name)
}

// TableWithConn 使用指定连接创建查询构造器
func TableWithConn(connName, tableName string) (*Query, error) {
	conn, err := GetConnection(connName)
	if err != nil {
		return nil, err
	}
	return NewQuery(conn.Write).Table(tableName), nil
}

// Name Table 的别名方法
func Name(name string) *Query {
	return Table(name)
}

// QuerySQL 执行原生 SQL 查询（主库）
// 参数:
//   - sql: SQL 语句
//   - args: 查询参数
//
// 返回: 查询结果（map 数组）和错误信息
func QuerySQL(sql string, args ...interface{}) ([]map[string]interface{}, error) {
	return NewQuery(GlobalDB).Query(sql, args...)
}

// QuerySQLRead 执行原生 SQL 查询（从库读取）
func QuerySQLRead(sql string, args ...interface{}) ([]map[string]interface{}, error) {
	return NewQuery(GetReadDB()).Query(sql, args...)
}

// ExecuteSQL 执行原生 SQL 增删改操作（主库）
// 参数:
//   - sql: SQL 语句
//   - args: 查询参数
//
// 返回: 影响行数和错误信息
func ExecuteSQL(sql string, args ...interface{}) (int64, error) {
	return NewQuery(GetWriteDB()).Execute(sql, args...)
}

// Transaction 执行数据库事务（闭包方式）
// 参数:
//   - closure: 事务闭包函数，接收查询构造器
//
// 返回: 错误信息
func Transaction(closure func(q *Query) error) error {
	tx, err := GlobalDB.Begin()
	if err != nil {
		return err
	}

	q := NewQuery(GlobalDB)
	q.tx = tx

	if err := closure(q); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// TransactionWithConn 使用指定连接执行事务
func TransactionWithConn(connName string, closure func(q *Query) error) error {
	conn, err := GetConnection(connName)
	if err != nil {
		return err
	}

	tx, err := conn.Write.Begin()
	if err != nil {
		return err
	}

	q := NewQuery(conn.Write)
	q.tx = tx

	if err := closure(q); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// CloseAll 关闭所有数据库连接
func CloseAll() {
	// 关闭主库
	if GlobalDB != nil {
		GlobalDB.Close()
		GlobalDB = nil
	}

	// 关闭读库
	for _, db := range ReadDBs {
		if db != nil {
			db.Close()
		}
	}
	ReadDBs = nil

	// 关闭写库
	for _, db := range WriteDBs {
		if db != nil {
			db.Close()
		}
	}
	WriteDBs = nil

	// 关闭命名连接
	globalDBManager.mu.Lock()
	for name, conn := range globalDBManager.connections {
		if conn.Write != nil {
			conn.Write.Close()
		}
		for _, db := range conn.Reads {
			if db != nil {
				db.Close()
			}
		}
		delete(globalDBManager.connections, name)
	}
	globalDBManager.mu.Unlock()
}

// BuildDSN 根据数据库驱动构建数据源连接字符串
// 支持 MySQL、PostgreSQL、SQLite、SQL Server
func BuildDSN(driver, user, pass, host string, port int, name, charset string) string {
	switch driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			user, pass, host, port, name, charset)
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, pass, name)
	case "sqlite3":
		return name
	case "sqlserver":
		return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			user, pass, host, port, name)
	default:
		return ""
	}
}
