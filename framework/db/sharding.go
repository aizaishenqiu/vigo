package db

import (
	"database/sql"
	"fmt"
	"hash/crc32"
	"sync"
)

// ========== 分库分表配置 ==========

// ShardingConfig 分库分表配置
type ShardingConfig struct {
	Table       string                // 表名
	ShardKey    string                // 分片键（字段名）
	ShardType   ShardType             // 分片类型
	ShardCount  int                   // 分片数量
	DatabaseMap map[int]*sql.DB       // 数据库映射（分库用）
	TablePrefix string                // 表前缀
	ShardFunc   func(interface{}) int // 自定义分片函数（可选）
}

// ShardType 分片类型
type ShardType int

const (
	ShardTypeMod    ShardType = iota + 1 // 取模分片
	ShardTypeHash                        // 哈希分片
	ShardTypeRange                       // 范围分片
	ShardTypeCustom                      // 自定义分片
)

// ========== 分库分表路由器 ==========

// ShardingRouter 分库分表路由器
type ShardingRouter struct {
	config     *ShardingConfig
	tableCache sync.Map // map[int]string 缓存分片表名
}

// NewShardingRouter 创建分库分表路由器
// 参数:
//   - config: 分片配置
//
// 示例:
//
//	// 按用户 ID 取模分表（4 张表）
//	router := db.NewShardingRouter(&db.ShardingConfig{
//	    Table:      "orders",
//	    ShardKey:   "user_id",
//	    ShardType:  db.ShardTypeMod,
//	    ShardCount: 4,
//	})
//
//	// 按订单 ID 哈希分库（2 个库，每个库 4 张表）
//	router := db.NewShardingRouter(&db.ShardingConfig{
//	    Table:       "orders",
//	    ShardKey:    "order_id",
//	    ShardType:   db.ShardTypeHash,
//	    ShardCount:  8,
//	    DatabaseMap: map[int]*sql.DB{0: db1, 1: db2},
//	})
func NewShardingRouter(config *ShardingConfig) *ShardingRouter {
	if config.ShardCount <= 0 {
		config.ShardCount = 4 // 默认 4 个分片
	}

	return &ShardingRouter{
		config: config,
	}
}

// GetShard 获取分片信息
// 返回：数据库、表名、错误
func (r *ShardingRouter) GetShard(shardKeyValue interface{}) (*sql.DB, string, error) {
	// 计算分片索引
	shardIndex := r.calculateShardIndex(shardKeyValue)

	// 获取数据库
	var targetDB *sql.DB
	if len(r.config.DatabaseMap) > 0 {
		dbIndex := shardIndex % len(r.config.DatabaseMap)
		targetDB = r.config.DatabaseMap[dbIndex]
		if targetDB == nil {
			return nil, "", fmt.Errorf("database not found for index %d", dbIndex)
		}
	} else {
		targetDB = GlobalDB
	}

	// 生成表名
	tableName := r.generateTableName(shardIndex)

	return targetDB, tableName, nil
}

// GetQuery 获取分片 Query 对象
func (r *ShardingRouter) GetQuery(shardKeyValue interface{}) (*Query, error) {
	targetDB, tableName, err := r.GetShard(shardKeyValue)
	if err != nil {
		return nil, err
	}

	return NewQuery(targetDB).Table(tableName), nil
}

// GetModel 获取分片 Model 对象（返回 Query，由调用者转换为 Model）
func (r *ShardingRouter) GetModel(shardKeyValue interface{}) (*Query, error) {
	targetDB, tableName, err := r.GetShard(shardKeyValue)
	if err != nil {
		return nil, err
	}

	return NewQuery(targetDB).Table(tableName), nil
}

// GetAllShards 获取所有分片信息（用于批量查询）
func (r *ShardingRouter) GetAllShards() []ShardInfo {
	shards := make([]ShardInfo, 0, r.config.ShardCount)

	for i := 0; i < r.config.ShardCount; i++ {
		var targetDB *sql.DB
		if len(r.config.DatabaseMap) > 0 {
			dbIndex := i % len(r.config.DatabaseMap)
			targetDB = r.config.DatabaseMap[dbIndex]
		} else {
			targetDB = GlobalDB
		}

		tableName := r.generateTableName(i)
		shards = append(shards, ShardInfo{
			Index:     i,
			Database:  targetDB,
			TableName: tableName,
		})
	}

	return shards
}

// calculateShardIndex 计算分片索引
func (r *ShardingRouter) calculateShardIndex(keyValue interface{}) int {
	// 使用自定义分片函数
	if r.config.ShardType == ShardTypeCustom && r.config.ShardFunc != nil {
		return r.config.ShardFunc(keyValue) % r.config.ShardCount
	}

	// 获取哈希值
	hashValue := r.hashKey(keyValue)

	// 根据分片类型计算索引
	switch r.config.ShardType {
	case ShardTypeMod:
		// 取模分片（适合数值型）
		switch v := keyValue.(type) {
		case int:
			return v % r.config.ShardCount
		case int32:
			return int(v) % r.config.ShardCount
		case int64:
			return int(v) % r.config.ShardCount
		default:
			return int(hashValue) % r.config.ShardCount
		}

	case ShardTypeHash:
		// 哈希分片（适合字符串）
		return int(hashValue) % r.config.ShardCount

	case ShardTypeRange:
		// 范围分片（需要自定义范围映射）
		// 这里简化处理，按哈希值范围
		return int(hashValue) % r.config.ShardCount

	default:
		return int(hashValue) % r.config.ShardCount
	}
}

// hashKey 计算键的哈希值
func (r *ShardingRouter) hashKey(keyValue interface{}) uint32 {
	var keyStr string

	switch v := keyValue.(type) {
	case string:
		keyStr = v
	case []byte:
		keyStr = string(v)
	default:
		keyStr = fmt.Sprintf("%v", v)
	}

	return crc32.ChecksumIEEE([]byte(keyStr))
}

// generateTableName 生成表名
func (r *ShardingRouter) generateTableName(shardIndex int) string {
	// 从缓存获取
	if cached, ok := r.tableCache.Load(shardIndex); ok {
		return cached.(string)
	}

	// 生成表名
	var tableName string
	if r.config.TablePrefix != "" {
		tableName = fmt.Sprintf("%s_%s_%02d", r.config.TablePrefix, r.config.Table, shardIndex)
	} else {
		tableName = fmt.Sprintf("%s_%02d", r.config.Table, shardIndex)
	}

	// 缓存
	r.tableCache.Store(shardIndex, tableName)

	return tableName
}

// ShardInfo 分片信息
type ShardInfo struct {
	Index     int
	Database  *sql.DB
	TableName string
}

// ========== 分库分表管理器 ==========

// ShardingManager 分库分表管理器
type ShardingManager struct {
	routers map[string]*ShardingRouter // 表名 -> 路由器
	mu      sync.RWMutex
}

// GlobalShardingManager 全局分片管理器
var GlobalShardingManager = &ShardingManager{
	routers: make(map[string]*ShardingRouter),
}

// AddTable 添加分片表配置
func (m *ShardingManager) AddTable(config *ShardingConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.routers[config.Table]; exists {
		return fmt.Errorf("table %s already configured", config.Table)
	}

	m.routers[config.Table] = NewShardingRouter(config)
	return nil
}

// GetRouter 获取表的路由器
func (m *ShardingManager) GetRouter(table string) (*ShardingRouter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	router, exists := m.routers[table]
	if !exists {
		return nil, fmt.Errorf("table %s not configured for sharding", table)
	}

	return router, nil
}

// GetQuery 获取分片 Query（便捷函数）
func (m *ShardingManager) GetQuery(table string, shardKeyValue interface{}) (*Query, error) {
	router, err := m.GetRouter(table)
	if err != nil {
		return nil, err
	}

	return router.GetQuery(shardKeyValue)
}

// GetModel 获取分片 Model（便捷函数，返回 Query）
func (m *ShardingManager) GetModel(table string, shardKeyValue interface{}) (*Query, error) {
	router, err := m.GetRouter(table)
	if err != nil {
		return nil, err
	}

	return router.GetModel(shardKeyValue)
}

// ========== 批量操作 ==========

// BatchInsert 批量插入（跨分片）
func (r *ShardingRouter) BatchInsert(dataList []map[string]interface{}) error {
	// 按分片分组
	shardDataMap := make(map[int][]map[string]interface{})

	for _, data := range dataList {
		shardKeyValue := data[r.config.ShardKey]
		shardIndex := r.calculateShardIndex(shardKeyValue)
		shardDataMap[shardIndex] = append(shardDataMap[shardIndex], data)
	}

	// 并行插入
	var wg sync.WaitGroup
	errChan := make(chan error, len(shardDataMap))

	for shardIndex, shardData := range shardDataMap {
		wg.Add(1)
		go func(index int, data []map[string]interface{}) {
			defer wg.Done()

			targetDB, tableName, err := r.GetShard(index)
			if err != nil {
				errChan <- err
				return
			}

			query := NewQuery(targetDB).Table(tableName)
			for _, d := range data {
				_, err = query.Insert(d)
				if err != nil {
					errChan <- err
					return
				}
			}
		}(shardIndex, shardData)
	}

	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// BatchQuery 批量查询（跨所有分片）
func (r *ShardingRouter) BatchQuery(queryFunc func(q *Query) error) error {
	shards := r.GetAllShards()
	var wg sync.WaitGroup
	errChan := make(chan error, len(shards))

	for _, shard := range shards {
		wg.Add(1)
		go func(s ShardInfo) {
			defer wg.Done()

			query := NewQuery(s.Database).Table(s.TableName)
			if err := queryFunc(query); err != nil {
				errChan <- err
			}
		}(shard)
	}

	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// ========== 便捷函数 ==========

// AddShardingTable 添加分片表（全局）
func AddShardingTable(config *ShardingConfig) error {
	return GlobalShardingManager.AddTable(config)
}

// GetShardingQuery 获取分片 Query（全局）
func GetShardingQuery(table string, shardKeyValue interface{}) (*Query, error) {
	return GlobalShardingManager.GetQuery(table, shardKeyValue)
}

// GetModel 获取分片 Model（便捷函数，返回 Query）
func GetShardingModel(table string, shardKeyValue interface{}) (*Query, error) {
	return GlobalShardingManager.GetModel(table, shardKeyValue)
}
