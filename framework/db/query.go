package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	// SlowQueryThreshold 慢查询阈值，默认500毫秒
	SlowQueryThreshold = 500 * time.Millisecond
)

// Query 查询构造器 (ThinkPHP/Laravel 风格链式操作)
type Query struct {
	db       *sql.DB
	tx       *sql.Tx
	table    string
	alias    string
	fields   []string
	wheres   []whereClause
	orders   []string
	groups   []string
	limit    int
	offset   int
	join     []string
	args     []interface{}
	lastSql  string
	havings  []havingClause
	distinct bool
	lockMode string // "FOR UPDATE" / "LOCK IN SHARE MODE"
	rawWhere []rawWhereClause
}

type whereClause struct {
	field string
	op    string
	val   interface{}
	logic string // AND / OR
}

type havingClause struct {
	condition string
	args      []interface{}
}

type rawWhereClause struct {
	sql   string
	args  []interface{}
	logic string
}

// Paginate 分页结果结构
type PaginateResult struct {
	Total       int64                    `json:"total"`
	PerPage     int                      `json:"per_page"`
	CurrentPage int                      `json:"current_page"`
	LastPage    int                      `json:"last_page"`
	Data        []map[string]interface{} `json:"data"`
}

// NewQuery 创建查询构造器
func NewQuery(db *sql.DB) *Query {
	return &Query{
		db:     db,
		fields: []string{"*"},
	}
}

// Table 指定表名
func (q *Query) Table(name string) *Query {
	q.table = name
	return q
}

// Alias 别名
func (q *Query) Alias(alias string) *Query {
	q.alias = alias
	return q
}

// Field 指定字段
func (q *Query) Field(fields interface{}) *Query {
	switch v := fields.(type) {
	case string:
		q.fields = strings.Split(v, ",")
	case []string:
		q.fields = v
	}
	return q
}

// Where 添加条件
func (q *Query) Where(field string, args ...interface{}) *Query {
	return q.addWhere(field, "AND", args...)
}

// WhereNot 添加 NOT 条件
func (q *Query) WhereNot(field string, args ...interface{}) *Query {
	op := "<>"
	var val interface{}
	if len(args) == 1 {
		val = args[0]
	} else if len(args) == 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}
	return q.addWhere(field, "AND", op, val)
}

// WhereIn 添加 IN 条件
func (q *Query) WhereIn(field string, val interface{}) *Query {
	return q.addWhere(field, "AND", "IN", val)
}

// WhereNotIn 添加 NOT IN 条件
func (q *Query) WhereNotIn(field string, val interface{}) *Query {
	return q.addWhere(field, "AND", "NOT IN", val)
}

// WhereOr 添加 OR 条件
func (q *Query) WhereOr(field string, args ...interface{}) *Query {
	return q.addWhere(field, "OR", args...)
}

// Group 分组
func (q *Query) Group(group string) *Query {
	q.groups = append(q.groups, group)
	return q
}

// Having 分组过滤
func (q *Query) Having(condition string, args ...interface{}) *Query {
	q.havings = append(q.havings, havingClause{
		condition: condition,
		args:      args,
	})
	return q
}

func (q *Query) addWhere(field string, logic string, args ...interface{}) *Query {
	op := "="
	var val interface{}

	if len(args) == 1 {
		val = args[0]
	} else if len(args) == 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}

	q.wheres = append(q.wheres, whereClause{
		field: field,
		op:    op,
		val:   val,
		logic: logic,
	})
	return q
}

// Order 排序
func (q *Query) Order(order string) *Query {
	q.orders = append(q.orders, order)
	return q
}

// Limit 限制条数
func (q *Query) Limit(limit int) *Query {
	q.limit = limit
	return q
}

// Page 分页
func (q *Query) Page(page, pageSize int) *Query {
	q.offset = (page - 1) * pageSize
	q.limit = pageSize
	return q
}

// Join 连表
func (q *Query) Join(table string, condition string, joinType string) *Query {
	join := fmt.Sprintf("%s JOIN %s ON %s", strings.ToUpper(joinType), table, condition)
	q.join = append(q.join, join)
	return q
}

// Distinct 去重
func (q *Query) Distinct() *Query {
	q.distinct = true
	return q
}

// Lock 加锁 (SELECT ... FOR UPDATE)
func (q *Query) Lock(mode ...string) *Query {
	if len(mode) > 0 {
		q.lockMode = mode[0]
	} else {
		q.lockMode = "FOR UPDATE"
	}
	return q
}

// WhereBetween BETWEEN 条件
func (q *Query) WhereBetween(field string, min, max interface{}) *Query {
	q.rawWhere = append(q.rawWhere, rawWhereClause{
		sql:   fmt.Sprintf("%s BETWEEN ? AND ?", field),
		args:  []interface{}{min, max},
		logic: "AND",
	})
	return q
}

// WhereNotBetween NOT BETWEEN 条件
func (q *Query) WhereNotBetween(field string, min, max interface{}) *Query {
	q.rawWhere = append(q.rawWhere, rawWhereClause{
		sql:   fmt.Sprintf("%s NOT BETWEEN ? AND ?", field),
		args:  []interface{}{min, max},
		logic: "AND",
	})
	return q
}

// WhereNull IS NULL 条件
func (q *Query) WhereNull(field string) *Query {
	q.rawWhere = append(q.rawWhere, rawWhereClause{
		sql:   fmt.Sprintf("%s IS NULL", field),
		logic: "AND",
	})
	return q
}

// WhereNotNull IS NOT NULL 条件
func (q *Query) WhereNotNull(field string) *Query {
	q.rawWhere = append(q.rawWhere, rawWhereClause{
		sql:   fmt.Sprintf("%s IS NOT NULL", field),
		logic: "AND",
	})
	return q
}

// WhereLike LIKE 条件 (自动加 %%)
func (q *Query) WhereLike(field string, val string) *Query {
	return q.addWhere(field, "AND", "LIKE", "%"+val+"%")
}

// WhereRaw 原生 WHERE 条件
func (q *Query) WhereRaw(sql string, args ...interface{}) *Query {
	q.rawWhere = append(q.rawWhere, rawWhereClause{
		sql:   sql,
		args:  args,
		logic: "AND",
	})
	return q
}

// WhereOrRaw 原生 OR 条件
func (q *Query) WhereOrRaw(sql string, args ...interface{}) *Query {
	q.rawWhere = append(q.rawWhere, rawWhereClause{
		sql:   sql,
		args:  args,
		logic: "OR",
	})
	return q
}

// WhereMap 通过 Map 批量设置条件 (ThinkPHP/Laravel 风格)
func (q *Query) WhereMap(conditions map[string]interface{}) *Query {
	for field, val := range conditions {
		q.Where(field, val)
	}
	return q
}

// GetLastSql 获取最后执行的 SQL
func (q *Query) GetLastSql() string {
	return q.lastSql
}

// --- 执行方法 ---

// Find 查询单条
func (q *Query) Find() (map[string]interface{}, error) {
	start := time.Now()
	q.Limit(1)
	rows, err := q.Select()
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start)
	if elapsed > SlowQueryThreshold {
		log.Printf("Slow query detected: %s took %v\n", q.GetLastSql(), elapsed)
	}
	if len(rows) > 0 {
		return rows[0], nil
	}
	return nil, nil
}

// Select 查询多条
func (q *Query) Select() ([]map[string]interface{}, error) {
	start := time.Now()

	sqlStr, args := q.buildSelectSql()
	q.lastSql = sqlStr

	// 读写分离逻辑：Select 走从库
	executor := q.db
	if q.tx == nil {
		executor = GetReadDB()
	}

	result, err := q.query(executor, sqlStr, args...)

	elapsed := time.Since(start)
	if elapsed > SlowQueryThreshold {
		log.Printf("[SLOW QUERY] %s took %v\n", q.lastSql, elapsed)
	}

	return result, err
}

// Value 获取单个值
func (q *Query) Value(field string) (interface{}, error) {
	start := time.Now()

	q.Field(field).Limit(1)
	row, err := q.Find()
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	elapsed := time.Since(start)
	if elapsed > SlowQueryThreshold {
		log.Printf("[SLOW QUERY] %s took %v\n", q.GetLastSql(), elapsed)
	}

	return row[field], nil
}

// Column 获取单列值切片
func (q *Query) Column(field string) ([]interface{}, error) {
	q.Field(field)
	rows, err := q.Select()
	if err != nil {
		return nil, err
	}
	var results []interface{}
	for _, row := range rows {
		results = append(results, row[field])
	}
	return results, nil
}

// Count 统计
func (q *Query) Count() (int64, error) {
	q.Field("COUNT(*) as count")
	row, err := q.Find()
	if err != nil || row == nil {
		return 0, err
	}
	val := row["count"]
	// 处理不同类型的返回值
	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		var c int64
		fmt.Sscanf(v, "%d", &c)
		return c, nil
	case []uint8: // MySQL count 返回可能是 []byte
		var c int64
		fmt.Sscanf(string(v), "%d", &c)
		return c, nil
	default:
		return 0, fmt.Errorf("unexpected type for count: %T", v)
	}
}

// Sum 求和
func (q *Query) Sum(field string) (float64, error) {
	alias := "sum_val"
	q.Field(fmt.Sprintf("SUM(%s) as %s", field, alias))
	val, err := q.Value(alias)
	if err != nil || val == nil {
		return 0, err
	}
	return toFloat64(val)
}

// Avg 平均值
func (q *Query) Avg(field string) (float64, error) {
	alias := "avg_val"
	q.Field(fmt.Sprintf("AVG(%s) as %s", field, alias))
	val, err := q.Value(alias)
	if err != nil || val == nil {
		return 0, err
	}
	return toFloat64(val)
}

// Max 最大值
func (q *Query) Max(field string) (interface{}, error) {
	alias := "max_val"
	q.Field(fmt.Sprintf("MAX(%s) as %s", field, alias))
	return q.Value(alias)
}

// Min 最小值
func (q *Query) Min(field string) (interface{}, error) {
	alias := "min_val"
	q.Field(fmt.Sprintf("MIN(%s) as %s", field, alias))
	return q.Value(alias)
}

func toFloat64(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case string:
		var f float64
		fmt.Sscanf(v, "%f", &f)
		return f, nil
	case []uint8:
		var f float64
		fmt.Sscanf(string(v), "%f", &f)
		return f, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// Insert 插入数据
func (q *Query) Insert(data map[string]interface{}) (int64, error) {
	sqlStr, args := q.buildInsertSql(data)
	q.lastSql = sqlStr
	return q.Execute(sqlStr, args...)
}

// InsertGetId 插入并返回 ID
func (q *Query) InsertGetId(data map[string]interface{}) (int64, error) {
	sqlStr, args := q.buildInsertSql(data)
	q.lastSql = sqlStr

	res, err := q.exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update 更新数据
func (q *Query) Update(data map[string]interface{}) (int64, error) {
	sqlStr, args := q.buildUpdateSql(data)
	q.lastSql = sqlStr
	return q.Execute(sqlStr, args...)
}

// Delete 删除数据
func (q *Query) Delete() (int64, error) {
	sqlStr, args := q.buildDeleteSql()
	q.lastSql = sqlStr
	return q.Execute(sqlStr, args...)
}

// InsertAll 批量插入 (ThinkPHP/Laravel saveAll 风格)
func (q *Query) InsertAll(dataList []map[string]interface{}) (int64, error) {
	if len(dataList) == 0 {
		return 0, nil
	}
	// 取第一条的 key 作为字段列表
	var fields []string
	for k := range dataList[0] {
		fields = append(fields, k)
	}

	var placeholderGroups []string
	var args []interface{}
	idx := 1
	for _, data := range dataList {
		var ph []string
		for _, f := range fields {
			if CurrentDriver == "postgres" {
				ph = append(ph, fmt.Sprintf("$%d", idx))
			} else {
				ph = append(ph, "?")
			}
			args = append(args, data[f])
			idx++
		}
		placeholderGroups = append(placeholderGroups, "("+strings.Join(ph, ",")+")")
	}

	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		q.table, strings.Join(fields, ","), strings.Join(placeholderGroups, ","))
	q.lastSql = sqlStr
	return q.Execute(sqlStr, args...)
}

// Inc 字段自增 (ThinkPHP/Laravel Inc)
func (q *Query) Inc(field string, step ...int) (int64, error) {
	s := 1
	if len(step) > 0 {
		s = step[0]
	}
	sqlStr := fmt.Sprintf("UPDATE %s SET %s = %s + ?", q.table, field, field)
	whereSql, whereArgs := q.buildWhere()
	rawSql, rawArgs := q.buildRawWhere(len(whereArgs))
	allArgs := []interface{}{s}
	if whereSql != "" {
		sqlStr += " WHERE " + whereSql
		allArgs = append(allArgs, whereArgs...)
	}
	if rawSql != "" {
		if whereSql == "" {
			sqlStr += " WHERE " + rawSql
		} else {
			sqlStr += " AND " + rawSql
		}
		allArgs = append(allArgs, rawArgs...)
	}
	q.lastSql = sqlStr
	return q.Execute(sqlStr, allArgs...)
}

// Dec 字段自减 (ThinkPHP/Laravel Dec)
func (q *Query) Dec(field string, step ...int) (int64, error) {
	s := 1
	if len(step) > 0 {
		s = step[0]
	}
	sqlStr := fmt.Sprintf("UPDATE %s SET %s = %s - ?", q.table, field, field)
	whereSql, whereArgs := q.buildWhere()
	rawSql, rawArgs := q.buildRawWhere(len(whereArgs))
	allArgs := []interface{}{s}
	if whereSql != "" {
		sqlStr += " WHERE " + whereSql
		allArgs = append(allArgs, whereArgs...)
	}
	if rawSql != "" {
		if whereSql == "" {
			sqlStr += " WHERE " + rawSql
		} else {
			sqlStr += " AND " + rawSql
		}
		allArgs = append(allArgs, rawArgs...)
	}
	q.lastSql = sqlStr
	return q.Execute(sqlStr, allArgs...)
}

// Paginate 分页查询 (ThinkPHP/Laravel 风格，返回结构化分页数据)
func (q *Query) Paginate(page, pageSize int) (*PaginateResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 15
	}

	// 先统计总数（clone 查询条件）
	countQ := q.clone()
	total, err := countQ.Count()
	if err != nil {
		return nil, err
	}

	// 计算总页数
	lastPage := int(total) / pageSize
	if int(total)%pageSize > 0 {
		lastPage++
	}

	// 查询当前页数据
	q.offset = (page - 1) * pageSize
	q.limit = pageSize
	data, err := q.Select()
	if err != nil {
		return nil, err
	}

	return &PaginateResult{
		Total:       total,
		PerPage:     pageSize,
		CurrentPage: page,
		LastPage:    lastPage,
		Data:        data,
	}, nil
}

// Chunk 分块处理大数据 (ThinkPHP/Laravel chunk)
func (q *Query) Chunk(size int, callback func(rows []map[string]interface{}) error) error {
	page := 1
	for {
		chunkQ := q.clone()
		chunkQ.offset = (page - 1) * size
		chunkQ.limit = size
		rows, err := chunkQ.Select()
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		if err := callback(rows); err != nil {
			return err
		}
		if len(rows) < size {
			break
		}
		page++
	}
	return nil
}

// Exists 判断记录是否存在
func (q *Query) Exists() (bool, error) {
	count, err := q.Count()
	return count > 0, err
}

// ToJson 将查询结果转为 JSON 字符串
func (q *Query) ToJson() (string, error) {
	rows, err := q.Select()
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(rows)
	return string(b), err
}

// clone 克隆查询构造器（用于分页等需要复用条件的场景）
func (q *Query) clone() *Query {
	newQ := &Query{
		db:       q.db,
		tx:       q.tx,
		table:    q.table,
		alias:    q.alias,
		fields:   []string{"*"},
		limit:    q.limit,
		offset:   q.offset,
		distinct: q.distinct,
		lockMode: q.lockMode,
	}
	newQ.wheres = append(newQ.wheres, q.wheres...)
	newQ.orders = append(newQ.orders, q.orders...)
	newQ.groups = append(newQ.groups, q.groups...)
	newQ.join = append(newQ.join, q.join...)
	newQ.havings = append(newQ.havings, q.havings...)
	newQ.rawWhere = append(newQ.rawWhere, q.rawWhere...)
	return newQ
}

// query 内部通用查询
func (q *Query) query(executor *sql.DB, sqlStr string, args ...interface{}) ([]map[string]interface{}, error) {
	stmt, err := executor.Prepare(sqlStr)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 结果集映射
	columns, _ := rows.Columns()
	count := len(columns)
	values := make([]interface{}, count)
	scanArgs := make([]interface{}, count)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	var results []map[string]interface{}

	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			return nil, err
		}

		entry := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// 处理 []byte 转 string (MySQL 默认行为)
			if b, ok := val.([]byte); ok {
				entry[col] = string(b)
			} else {
				entry[col] = val
			}
		}
		results = append(results, entry)
	}
	return results, nil
}

// Query 原生查询 (默认走从库)
func (q *Query) Query(sqlStr string, args ...interface{}) ([]map[string]interface{}, error) {
	executor := q.db
	if q.tx == nil {
		executor = GetReadDB()
	}
	return q.query(executor, sqlStr, args...)
}

// Execute 原生执行
func (q *Query) Execute(sqlStr string, args ...interface{}) (int64, error) {
	start := time.Now()

	res, err := q.exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}

	elapsed := time.Since(start)
	if elapsed > SlowQueryThreshold {
		log.Printf("[SLOW QUERY] %s took %v\n", sqlStr, elapsed)
	}

	return res.RowsAffected()
}

func (q *Query) exec(sqlStr string, args ...interface{}) (sql.Result, error) {
	if q.tx != nil {
		stmt, err := q.tx.Prepare(sqlStr)
		if err != nil {
			return nil, err
		}
		defer stmt.Close()
		return stmt.Exec(args...)
	}

	stmt, err := q.db.Prepare(sqlStr)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	return stmt.Exec(args...)
}

// buildRawWhere 构建原生 WHERE 子句
func (q *Query) buildRawWhere(argOffset int) (string, []interface{}) {
	if len(q.rawWhere) == 0 {
		return "", nil
	}
	var sqlStr strings.Builder
	var args []interface{}
	for i, rw := range q.rawWhere {
		if i > 0 {
			sqlStr.WriteString(" " + rw.logic + " ")
		}
		sqlStr.WriteString(rw.sql)
		args = append(args, rw.args...)
	}
	return sqlStr.String(), args
}

// buildSelectSql 构建 Select SQL
func (q *Query) buildSelectSql() (string, []interface{}) {
	var sqlStr strings.Builder
	if q.distinct {
		sqlStr.WriteString("SELECT DISTINCT ")
	} else {
		sqlStr.WriteString("SELECT ")
	}
	sqlStr.WriteString(strings.Join(q.fields, ","))
	sqlStr.WriteString(" FROM ")
	sqlStr.WriteString(q.table)

	if q.alias != "" {
		sqlStr.WriteString(" AS ")
		sqlStr.WriteString(q.alias)
	}

	for _, j := range q.join {
		sqlStr.WriteString(" ")
		sqlStr.WriteString(j)
	}

	whereSql, whereArgs := q.buildWhere()
	rawSql, rawArgs := q.buildRawWhere(len(whereArgs))
	if whereSql != "" || rawSql != "" {
		sqlStr.WriteString(" WHERE ")
		parts := []string{}
		if whereSql != "" {
			parts = append(parts, whereSql)
			q.args = append(q.args, whereArgs...)
		}
		if rawSql != "" {
			parts = append(parts, rawSql)
			q.args = append(q.args, rawArgs...)
		}
		sqlStr.WriteString(strings.Join(parts, " AND "))
	}

	if len(q.groups) > 0 {
		sqlStr.WriteString(" GROUP BY ")
		sqlStr.WriteString(strings.Join(q.groups, ","))
	}

	if len(q.havings) > 0 {
		sqlStr.WriteString(" HAVING ")
		for i, h := range q.havings {
			if i > 0 {
				sqlStr.WriteString(" AND ")
			}
			sqlStr.WriteString(h.condition)
			q.args = append(q.args, h.args...)
		}
	}

	if len(q.orders) > 0 {
		sqlStr.WriteString(" ORDER BY ")
		sqlStr.WriteString(strings.Join(q.orders, ","))
	}

	// 驱动特定的分页处理
	if q.limit > 0 {
		switch CurrentDriver {
		case "sqlserver":
			// SQL Server 分页通常需要 OFFSET FETCH 或 TOP
			if q.offset > 0 {
				sqlStr.WriteString(fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", q.offset, q.limit))
			} else {
				// 简单的 TOP 逻辑
				oldSql := sqlStr.String()
				sqlStr.Reset()
				sqlStr.WriteString(strings.Replace(oldSql, "SELECT", fmt.Sprintf("SELECT TOP %d", q.limit), 1))
			}
		default:
			// MySQL, PostgreSQL, SQLite 均支持 LIMIT OFFSET
			sqlStr.WriteString(fmt.Sprintf(" LIMIT %d", q.limit))
			if q.offset > 0 {
				sqlStr.WriteString(fmt.Sprintf(" OFFSET %d", q.offset))
			}
		}
	}

	// Lock 子句
	if q.lockMode != "" {
		sqlStr.WriteString(" " + q.lockMode)
	}

	return sqlStr.String(), q.args
}

func (q *Query) buildWhere() (string, []interface{}) {
	if len(q.wheres) == 0 {
		return "", nil
	}
	var sqlStr strings.Builder
	var args []interface{}
	for i, w := range q.wheres {
		if i > 0 {
			sqlStr.WriteString(" " + w.logic + " ")
		}
		// 驱动特定的占位符处理
		placeholder := "?"
		switch CurrentDriver {
		case "postgres":
			placeholder = fmt.Sprintf("$%d", i+1)
		}

		if w.op == "IN" || w.op == "NOT IN" {
			// 处理 IN 条件
			vList, ok := w.val.([]interface{})
			if !ok {
				// 尝试处理切片
				// ... 简化逻辑
				sqlStr.WriteString(fmt.Sprintf("%s %s (%s)", w.field, w.op, placeholder))
				args = append(args, w.val)
			} else {
				placeholders := make([]string, len(vList))
				for j := range vList {
					if CurrentDriver == "postgres" {
						placeholders[j] = fmt.Sprintf("$%d", len(args)+j+1)
					} else {
						placeholders[j] = "?"
					}
				}
				sqlStr.WriteString(fmt.Sprintf("%s %s (%s)", w.field, w.op, strings.Join(placeholders, ",")))
				args = append(args, vList...)
			}
		} else {
			sqlStr.WriteString(fmt.Sprintf("%s %s %s", w.field, w.op, placeholder))
			args = append(args, w.val)
		}
	}
	return sqlStr.String(), args
}

func (q *Query) buildInsertSql(data map[string]interface{}) (string, []interface{}) {
	var fields []string
	var placeholders []string
	var args []interface{}
	i := 1
	for k, v := range data {
		fields = append(fields, k)
		if CurrentDriver == "postgres" {
			placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		} else {
			placeholders = append(placeholders, "?")
		}
		args = append(args, v)
		i++
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.table, strings.Join(fields, ","), strings.Join(placeholders, ","))
	return sqlStr, args
}

func (q *Query) buildUpdateSql(data map[string]interface{}) (string, []interface{}) {
	var sets []string
	var args []interface{}
	i := 1
	for k, v := range data {
		if CurrentDriver == "postgres" {
			sets = append(sets, fmt.Sprintf("%s=$%d", k, i))
		} else {
			sets = append(sets, fmt.Sprintf("%s=?", k))
		}
		args = append(args, v)
		i++
	}
	whereSql, whereArgs := q.buildWhere()
	rawSql, rawArgs := q.buildRawWhere(len(whereArgs))
	// PostgreSQL 占位符序号接续
	if CurrentDriver == "postgres" && whereSql != "" {
		for j := range whereArgs {
			old := fmt.Sprintf("$%d", j+1)
			newPh := fmt.Sprintf("$%d", i+j)
			whereSql = strings.Replace(whereSql, old, newPh, 1)
		}
	}
	args = append(args, whereArgs...)
	where := whereSql
	if rawSql != "" {
		if where != "" {
			where += " AND " + rawSql
		} else {
			where = rawSql
		}
		args = append(args, rawArgs...)
	}
	sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s", q.table, strings.Join(sets, ","), where)
	return sqlStr, args
}

func (q *Query) buildDeleteSql() (string, []interface{}) {
	whereSql, whereArgs := q.buildWhere()
	rawSql, rawArgs := q.buildRawWhere(len(whereArgs))
	where := whereSql
	allArgs := whereArgs
	if rawSql != "" {
		if where != "" {
			where += " AND " + rawSql
		} else {
			where = rawSql
		}
		allArgs = append(allArgs, rawArgs...)
	}
	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE %s", q.table, where)
	return sqlStr, allArgs
}

// ========== 原子操作 ==========

// Increment 原子递增指定字段的值
// 参数:
//   - column: 字段名
//   - amount: 递增量
//
// 返回: 影响的行数
func (q *Query) Increment(column string, amount int64) (int64, error) {
	if q.table == "" {
		return 0, fmt.Errorf("table is not specified")
	}

	sql := fmt.Sprintf("UPDATE %s SET %s = %s + ? WHERE ", q.table, column, column)
	whereSql, whereArgs := q.buildWhere()
	sql += whereSql

	args := append([]interface{}{amount}, whereArgs...)
	result, err := q.db.Exec(sql, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Decrement 原子递减指定字段的值
// 参数:
//   - column: 字段名
//   - amount: 递减量
//
// 返回: 影响的行数
func (q *Query) Decrement(column string, amount int64) (int64, error) {
	return q.Increment(column, -amount)
}

// IncrementBy 原子递增（支持小数）
func (q *Query) IncrementBy(column string, amount float64) (int64, error) {
	if q.table == "" {
		return 0, fmt.Errorf("table is not specified")
	}

	sql := fmt.Sprintf("UPDATE %s SET %s = %s + ? WHERE ", q.table, column, column)
	whereSql, whereArgs := q.buildWhere()
	sql += whereSql

	args := append([]interface{}{amount}, whereArgs...)
	result, err := q.db.Exec(sql, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// DecrementBy 原子递减（支持小数）
func (q *Query) DecrementBy(column string, amount float64) (int64, error) {
	return q.IncrementBy(column, -amount)
}
