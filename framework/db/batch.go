package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"vigo/framework/cache"
)

// BatchQuery 批量查询优化器
type BatchQuery struct {
	db         *sql.DB
	batchSize  int
	batchDelay time.Duration
	queue      chan *queryRequest
	mu         sync.Mutex
}

type queryRequest struct {
	query  string
	args   []interface{}
	result chan batchResult
	ctx    context.Context
}

type batchResult struct {
	rows *sql.Rows
	err  error
}

// NewBatchQuery 创建批量查询优化器
func NewBatchQuery(db *sql.DB, batchSize int, batchDelay time.Duration) *BatchQuery {
	bq := &BatchQuery{
		db:         db,
		batchSize:  batchSize,
		batchDelay: batchDelay,
		queue:      make(chan *queryRequest, 1000),
	}

	// 启动批处理协程
	go bq.processBatch()

	return bq
}

// Query 执行批量查询
func (bq *BatchQuery) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	req := &queryRequest{
		query:  query,
		args:   args,
		result: make(chan batchResult, 1),
		ctx:    ctx,
	}

	bq.queue <- req

	// 等待结果
	select {
	case res := <-req.result:
		return res.rows, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// processBatch 批处理查询
func (bq *BatchQuery) processBatch() {
	ticker := time.NewTicker(bq.batchDelay)
	defer ticker.Stop()

	var requests []*queryRequest

	for {
		select {
		case req := <-bq.queue:
			requests = append(requests, req)

			// 达到批次大小时立即处理
			if len(requests) >= bq.batchSize {
				bq.executeBatch(requests)
				requests = requests[:0]
			}

		case <-ticker.C:
			// 定时处理
			if len(requests) > 0 {
				bq.executeBatch(requests)
				requests = requests[:0]
			}
		}
	}
}

// executeBatch 执行批次查询
func (bq *BatchQuery) executeBatch(requests []*queryRequest) {
	if len(requests) == 0 {
		return
	}

	// 合并相同查询
	queryMap := make(map[string][]*queryRequest)
	for _, req := range requests {
		key := req.query
		queryMap[key] = append(queryMap[key], req)
	}

	// 执行每个查询
	for query, reqs := range queryMap {
		// 合并参数
		allArgs := make([]interface{}, 0)
		for _, req := range reqs {
			allArgs = append(allArgs, req.args...)
		}

		// 执行查询
		rows, err := bq.db.Query(query, allArgs...)

		// 返回结果
		for _, req := range reqs {
			req.result <- batchResult{
				rows: rows,
				err:  err,
			}
		}
	}
}

// BatchInserter 批量插入器
type BatchInserter struct {
	db        *sql.DB
	table     string
	columns   []string
	batchSize int
	values    [][]interface{}
	mu        sync.Mutex
	inserted  int64
}

// NewBatchInserter 创建批量插入器
func NewBatchInserter(db *sql.DB, table string, columns []string, batchSize int) *BatchInserter {
	return &BatchInserter{
		db:        db,
		table:     table,
		columns:   columns,
		batchSize: batchSize,
		values:    make([][]interface{}, 0, batchSize),
	}
}

// Add 添加一行数据
func (bi *BatchInserter) Add(values ...interface{}) error {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	if len(values) != len(bi.columns) {
		return ErrInvalidColumnCount
	}

	bi.values = append(bi.values, values)

	// 达到批次大小时自动执行
	if len(bi.values) >= bi.batchSize {
		return bi.flush()
	}

	return nil
}

// Flush 强制刷新
func (bi *BatchInserter) Flush() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	return bi.flush()
}

// flush 执行批量插入
func (bi *BatchInserter) flush() error {
	if len(bi.values) == 0 {
		return nil
	}

	// 构建批量插入 SQL
	placeholder := "(" + placeHolders(len(bi.columns)) + ")"
	placeholders := make([]string, len(bi.values))
	for i := range placeholders {
		placeholders[i] = placeholder
	}

	query := "INSERT INTO " + bi.table + " (" + strings.Join(bi.columns, ", ") + ") VALUES " + strings.Join(placeholders, ", ")

	// 合并所有参数
	args := make([]interface{}, 0, len(bi.values)*len(bi.columns))
	for _, values := range bi.values {
		args = append(args, values...)
	}

	// 执行插入
	_, err := bi.db.Exec(query, args...)
	if err != nil {
		return err
	}

	bi.inserted += int64(len(bi.values))
	bi.values = bi.values[:0]

	return nil
}

// Inserted 返回已插入的记录数
func (bi *BatchInserter) Inserted() int64 {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	return bi.inserted
}

// placeHolders 生成占位符
func placeHolders(n int) string {
	return strings.Repeat("?,", n)[:n*2-1]
}

// ErrInvalidColumnCount 列数不匹配错误
var ErrInvalidColumnCount = &invalidColumnCountError{}

type invalidColumnCountError struct{}

func (e *invalidColumnCountError) Error() string {
	return "invalid column count"
}

// QueryCache 查询缓存（减少重复查询）
type QueryCache struct {
	cache *cache.MemoryCache
	db    *sql.DB
	ttl   time.Duration
	mu    sync.Mutex
}

// NewQueryCache 创建查询缓存
func NewQueryCache(db *sql.DB, ttl time.Duration) (*QueryCache, error) {
	return &QueryCache{
		cache: cache.NewMemoryCache(),
		db:    db,
		ttl:   ttl,
	}, nil
}

// Query 查询（带缓存）
func (qc *QueryCache) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// 生成缓存 key
	key := query + "_" + fmt.Sprintf("%v", args)

	// 尝试从缓存获取
	if cached := qc.cache.Get(key); cached != nil {
		// 反序列化缓存数据
		var cachedData CachedQueryResult
		if err := json.Unmarshal([]byte(cached.(string)), &cachedData); err == nil {
			// 创建 mock rows（简化版本，实际应该使用 sqlmock 或类似工具）
			// 这里返回错误，提示使用 QueryRows 方法
			return nil, fmt.Errorf("cached data found but requires special handling")
		}
	}

	// 执行查询
	rows, err := qc.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// 缓存结果（使用 QueryRows 方法）
	return rows, nil
}

// QueryRows 查询并返回结果（带缓存）
func (qc *QueryCache) QueryRows(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	// 生成缓存 key
	key := "rows:" + query + "_" + fmt.Sprintf("%v", args)

	// 尝试从缓存获取
	if cached := qc.cache.Get(key); cached != nil {
		var cachedData CachedQueryResult
		if err := json.Unmarshal([]byte(cached.(string)), &cachedData); err == nil {
			return cachedData.Rows, nil
		}
	}

	// 执行查询
	rows, err := qc.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// 读取所有行
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		// 创建扫描目标
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// 扫描行
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// 转换为 map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			var val interface{}
			if values[i] == nil {
				val = nil
			} else {
				val = values[i]
			}
			rowMap[col] = val
		}

		results = append(results, rowMap)
	}

	// 缓存结果
	if len(results) > 0 {
		cachedData := CachedQueryResult{
			Query: query,
			Args:  args,
			Rows:  results,
			Time:  time.Now(),
		}
		
		if data, err := json.Marshal(cachedData); err == nil {
			qc.cache.Set(key, string(data), qc.ttl)
		}
	}

	return results, nil
}

// Invalidate 使缓存失效
func (qc *QueryCache) Invalidate(pattern string) error {
	// 清空缓存
	qc.cache.Flush()
	return nil
}

// CachedQueryResult 缓存的查询结果
type CachedQueryResult struct {
	Query string                 `json:"query"`
	Args  []interface{}          `json:"args"`
	Rows  []map[string]interface{} `json:"rows"`
	Time  time.Time              `json:"time"`
}
