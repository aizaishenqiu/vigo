// Package db 提供 ORM 支持
// 简化数据库操作，提供类似 GORM 的链式调用体验
package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Model 基础模型接口
type Model interface {
	TableName() string
	PrimaryKey() string
}

// ORM 对象关系映射器
type ORM struct {
	db        *sql.DB
	tx        *sql.Tx
	model     interface{}
	tableName string
	query     *Query
	ctx       context.Context
	debug     bool
}

// NewORM 创建 ORM 实例
func NewORM(db *sql.DB) *ORM {
	return &ORM{
		db:    db,
		query: NewQuery(db),
		ctx:   context.Background(),
	}
}

// Model 设置模型
func (o *ORM) Model(model interface{}) *ORM {
	o.model = model
	o.tableName = getTableName(model)
	o.query.Table(o.tableName)
	return o
}

// Table 设置表名
func (o *ORM) Table(name string) *ORM {
	o.tableName = name
	o.query.Table(name)
	return o
}

// Debug 开启调试模式
func (o *ORM) Debug() *ORM {
	o.debug = true
	return o
}

// Context 设置上下文
func (o *ORM) Context(ctx context.Context) *ORM {
	o.ctx = ctx
	return o
}

// PrimaryKey 获取主键字段名
func (o *ORM) PrimaryKey() string {
	if o.model == nil {
		return "id"
	}

	// 检查模型是否实现了 Model 接口
	if m, ok := o.model.(Model); ok {
		return m.PrimaryKey()
	}

	// 默认返回 id
	return "id"
}

// --- 查询条件 ---

// Where 添加条件
func (o *ORM) Where(field string, args ...interface{}) *ORM {
	o.query.Where(field, args...)
	return o
}

// WhereIn 添加 IN 条件
func (o *ORM) WhereIn(field string, values interface{}) *ORM {
	o.query.WhereIn(field, values)
	return o
}

// WhereNotIn 添加 NOT IN 条件
func (o *ORM) WhereNotIn(field string, values interface{}) *ORM {
	o.query.WhereNotIn(field, values)
	return o
}

// WhereBetween 添加 BETWEEN 条件
func (o *ORM) WhereBetween(field string, min, max interface{}) *ORM {
	o.query.WhereBetween(field, min, max)
	return o
}

// WhereNotBetween 添加 NOT BETWEEN 条件
func (o *ORM) WhereNotBetween(field string, min, max interface{}) *ORM {
	o.query.WhereNotBetween(field, min, max)
	return o
}

// WhereNull 添加 IS NULL 条件
func (o *ORM) WhereNull(field string) *ORM {
	o.query.WhereNull(field)
	return o
}

// WhereNotNull 添加 IS NOT NULL 条件
func (o *ORM) WhereNotNull(field string) *ORM {
	o.query.WhereNotNull(field)
	return o
}

// WhereLike 添加 LIKE 条件
func (o *ORM) WhereLike(field string, pattern string) *ORM {
	o.query.WhereLike(field, pattern)
	return o
}

// WhereOr 添加 OR 条件
func (o *ORM) WhereOr(field string, args ...interface{}) *ORM {
	o.query.WhereOr(field, args...)
	return o
}

// WhereRaw 添加原生条件
func (o *ORM) WhereRaw(sql string, args ...interface{}) *ORM {
	o.query.WhereRaw(sql, args...)
	return o
}

// WhereOrRaw 添加原生 OR 条件
func (o *ORM) WhereOrRaw(sql string, args ...interface{}) *ORM {
	o.query.WhereOrRaw(sql, args...)
	return o
}

// WhereMap 批量添加条件
func (o *ORM) WhereMap(conditions map[string]interface{}) *ORM {
	o.query.WhereMap(conditions)
	return o
}

// --- 字段和表 ---

// Select 指定字段
func (o *ORM) Select(fields ...string) *ORM {
	o.query.Field(fields)
	return o
}

// Alias 表别名
func (o *ORM) Alias(alias string) *ORM {
	o.query.Alias(alias)
	return o
}

// Distinct 去重
func (o *ORM) Distinct() *ORM {
	o.query.Distinct()
	return o
}

// --- 连表 ---

// Join 连表
func (o *ORM) Join(table string, condition string, joinType ...string) *ORM {
	jt := "INNER"
	if len(joinType) > 0 {
		jt = strings.ToUpper(joinType[0])
	}
	o.query.Join(table, condition, jt)
	return o
}

// LeftJoin 左连接
func (o *ORM) LeftJoin(table string, condition string) *ORM {
	return o.Join(table, condition, "LEFT")
}

// RightJoin 右连接
func (o *ORM) RightJoin(table string, condition string) *ORM {
	return o.Join(table, condition, "RIGHT")
}

// --- 分组和排序 ---

// Group 分组
func (o *ORM) Group(group string) *ORM {
	o.query.Group(group)
	return o
}

// Having 分组过滤
func (o *ORM) Having(condition string, args ...interface{}) *ORM {
	o.query.Having(condition, args...)
	return o
}

// Order 排序
func (o *ORM) Order(order string) *ORM {
	o.query.Order(order)
	return o
}

// OrderBy 排序
func (o *ORM) OrderBy(field string, direction ...string) *ORM {
	dir := "ASC"
	if len(direction) > 0 {
		dir = strings.ToUpper(direction[0])
	}
	o.query.Order(fmt.Sprintf("%s %s", field, dir))
	return o
}

// OrderByDesc 倒序
func (o *ORM) OrderByDesc(field string) *ORM {
	return o.OrderBy(field, "DESC")
}

// --- 分页 ---

// Limit 限制
func (o *ORM) Limit(limit int) *ORM {
	o.query.Limit(limit)
	return o
}

// Offset 偏移
func (o *ORM) Offset(offset int) *ORM {
	o.query.offset = offset
	return o
}

// Page 分页
func (o *ORM) Page(page, pageSize int) *ORM {
	o.query.Page(page, pageSize)
	return o
}

// --- 锁 ---

// Lock 加锁
func (o *ORM) Lock(mode ...string) *ORM {
	o.query.Lock(mode...)
	return o
}

// LockForUpdate 排他锁
func (o *ORM) LockForUpdate() *ORM {
	return o.Lock("FOR UPDATE")
}

// --- 执行方法 ---

// Get 获取所有记录
func (o *ORM) Get() ([]map[string]interface{}, error) {
	return o.query.Select()
}

// First 获取第一条记录
func (o *ORM) First() (map[string]interface{}, error) {
	return o.query.Find()
}

// Find 查询所有记录到结构体
func (o *ORM) Find(dest interface{}) error {
	results, err := o.query.Select()
	if err != nil {
		return err
	}
	return mapsToStructs(results, dest)
}

// Scan 查询单条到结构体
func (o *ORM) Scan(dest interface{}) error {
	o.query.Limit(1)
	rows, err := o.query.Select()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return sql.ErrNoRows
	}
	return mapToStruct(rows[0], dest)
}

// Value 获取单个值
func (o *ORM) Value(column string) (interface{}, error) {
	return o.query.Value(column)
}

// Pluck 获取单列
func (o *ORM) Pluck(column string) ([]interface{}, error) {
	return o.query.Column(column)
}

// Count 统计数量
func (o *ORM) Count() (int64, error) {
	return o.query.Count()
}

// Sum 求和
func (o *ORM) Sum(column string) (float64, error) {
	return o.query.Sum(column)
}

// Avg 平均值
func (o *ORM) Avg(column string) (float64, error) {
	return o.query.Avg(column)
}

// Max 最大值
func (o *ORM) Max(column string) (interface{}, error) {
	return o.query.Max(column)
}

// Min 最小值
func (o *ORM) Min(column string) (interface{}, error) {
	return o.query.Min(column)
}

// --- 数据操作 ---

// Create 创建记录
func (o *ORM) Create(model interface{}) error {
	data := structToMap(model)
	_, err := o.Insert(data)
	return err
}

// Insert 插入数据
func (o *ORM) Insert(data map[string]interface{}) (int64, error) {
	return o.query.Insert(data)
}

// InsertGetId 插入并返回 ID
func (o *ORM) InsertGetId(data map[string]interface{}) (int64, error) {
	return o.query.InsertGetId(data)
}

// InsertMulti 批量插入
func (o *ORM) InsertMulti(count int, data interface{}) (int64, error) {
	// 转换为 []map[string]interface{}
	var dataList []map[string]interface{}
	switch v := data.(type) {
	case []map[string]interface{}:
		dataList = v
	case []interface{}:
		dataList = make([]map[string]interface{}, len(v))
		for i, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				dataList[i] = m
			}
		}
	}
	return o.query.InsertAll(dataList)
}

// Update 更新记录
func (o *ORM) Update(data map[string]interface{}) (int64, error) {
	return o.query.Update(data)
}

// Incr 字段自增
func (o *ORM) Incr(field string, amount ...int) (int64, error) {
	return o.query.Inc(field, amount...)
}

// Decr 字段自减
func (o *ORM) Decr(field string, amount ...int) (int64, error) {
	return o.query.Dec(field, amount...)
}

// Delete 删除记录
func (o *ORM) Delete() (int64, error) {
	return o.query.Delete()
}

// --- 高级功能 ---

// Exists 检查是否存在
func (o *ORM) Exists() (bool, error) {
	return o.query.Exists()
}

// DoesntExist 检查是否不存在
func (o *ORM) DoesntExist() (bool, error) {
	exists, err := o.Exists()
	return !exists, err
}

// Paginate 分页查询
func (o *ORM) Paginate(page, pageSize int) (*PaginateResult, error) {
	return o.query.Paginate(page, pageSize)
}

// Chunk 分块处理
func (o *ORM) Chunk(limit int, callback func([]map[string]interface{}) error) error {
	return o.query.Chunk(limit, callback)
}

// ToSQL 获取 SQL（不执行）
func (o *ORM) ToSQL() string {
	return o.query.GetLastSql()
}

// Dump 打印 SQL 和参数
func (o *ORM) Dump() *ORM {
	fmt.Printf("SQL: %s\n", o.query.GetLastSql())
	fmt.Printf("Args: %v\n", o.query.args)
	return o
}

// Reset 重置查询条件
func (o *ORM) Reset() *ORM {
	o.query = NewQuery(o.db)
	if o.tableName != "" {
		o.query.Table(o.tableName)
	}
	return o
}

// --- 事务 ---

// Begin 开启事务
func (o *ORM) Begin() (*ORM, error) {
	tx, err := o.db.Begin()
	if err != nil {
		return nil, err
	}
	return &ORM{
		db:        o.db,
		tx:        tx,
		tableName: o.tableName,
		query:     NewQuery(o.db),
		ctx:       o.ctx,
		debug:     o.debug,
	}, nil
}

// Commit 提交事务
func (o *ORM) Commit() error {
	if o.tx == nil {
		return fmt.Errorf("not in transaction")
	}
	return o.tx.Commit()
}

// Rollback 回滚事务
func (o *ORM) Rollback() error {
	if o.tx == nil {
		return fmt.Errorf("not in transaction")
	}
	return o.tx.Rollback()
}

// Transaction 事务处理（闭包）
func (o *ORM) Transaction(fn func(tx *ORM) error) error {
	tx, err := o.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// --- 工具方法 ---

// WhereOp 添加带操作符的条件
func (o *ORM) WhereOp(field, operator string, value interface{}) *ORM {
	switch operator {
	case "=", "!=", "<>", ">", "<", ">=", "<=":
		o.query.Where(field, operator, value)
	default:
		o.query.Where(field, value)
	}
	return o
}

// getTableName 获取表名
func getTableName(model interface{}) string {
	if m, ok := model.(Model); ok {
		return m.TableName()
	}

	// 从结构体名推断表名
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	name := t.Name()
	// 复数化（简单规则）
	if !strings.HasSuffix(name, "s") {
		name += "s"
	}
	return strings.ToLower(name)
}

// parseTag 解析结构体标签
func parseTag(tag string) (name string, opts map[string]string) {
	opts = make(map[string]string)
	if tag == "" {
		return "", opts
	}

	parts := strings.Split(tag, ";")
	name = parts[0]

	for _, part := range parts[1:] {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			opts[kv[0]] = kv[1]
		} else {
			opts[part] = ""
		}
	}

	return name, opts
}

// structToMap 结构体转 Map
func structToMap(model interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	v := reflect.ValueOf(model)
	t := v.Type()

	if t.Kind() == reflect.Ptr {
		v = v.Elem()
		t = v.Type()
	}

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// 跳过未导出字段
		if !value.CanInterface() {
			continue
		}

		// 获取 db 标签
		tag := field.Tag.Get("db")
		colName, opts := parseTag(tag)

		// 跳过 - 标签
		if colName == "-" {
			continue
		}

		// 使用字段名的小写
		if colName == "" {
			colName = strings.ToLower(field.Name)
		}

		// 检查 omitempty 选项
		if _, hasOmit := opts["omitempty"]; hasOmit {
			if isZeroValue(value) {
				continue
			}
		}

		result[colName] = value.Interface()
	}

	return result
}

// isZeroValue 检查是否为零值
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan:
		return v.IsNil()
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return v.Interface().(time.Time).IsZero()
		}
		return false
	}
	return false
}

// mapToStruct Map 转结构体
func mapToStruct(data map[string]interface{}, dest interface{}) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a pointer to struct")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// 跳过未导出字段
		if !fieldValue.CanSet() {
			continue
		}

		// 获取字段名
		tag := field.Tag.Get("db")
		fieldName, _ := parseTag(tag)
		if fieldName == "-" {
			continue
		}
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		// 查找对应值
		if val, ok := data[fieldName]; ok && val != nil {
			setFieldValue(fieldValue, val)
		}
	}

	return nil
}

// mapsToStructs Map 切片转结构体切片
func mapsToStructs(data []map[string]interface{}, dest interface{}) error {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr || destValue.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("dest must be a pointer to slice")
	}

	sliceValue := destValue.Elem()
	elemType := sliceValue.Type().Elem()

	for _, item := range data {
		elem := reflect.New(elemType).Interface()
		if err := mapToStruct(item, elem); err != nil {
			return err
		}
		sliceValue = reflect.Append(sliceValue, reflect.ValueOf(elem).Elem())
	}

	destValue.Elem().Set(sliceValue)
	return nil
}

// setFieldValue 设置字段值
func setFieldValue(field reflect.Value, value interface{}) {
	if value == nil {
		return
	}

	val := reflect.ValueOf(value)

	// 类型转换
	if field.Type() == val.Type() {
		field.Set(val)
		return
	}

	// 基本类型转换
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprintf("%v", value))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := value.(type) {
		case int64:
			field.SetInt(v)
		case int:
			field.SetInt(int64(v))
		case float64:
			field.SetInt(int64(v))
		case []uint8:
			field.SetInt(parseInt64(string(v)))
		case string:
			field.SetInt(parseInt64(v))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch v := value.(type) {
		case uint64:
			field.SetUint(v)
		case uint:
			field.SetUint(uint64(v))
		case int64:
			field.SetUint(uint64(v))
		case float64:
			field.SetUint(uint64(v))
		}
	case reflect.Float32, reflect.Float64:
		switch v := value.(type) {
		case float64:
			field.SetFloat(v)
		case float32:
			field.SetFloat(float64(v))
		case int64:
			field.SetFloat(float64(v))
		case int:
			field.SetFloat(float64(v))
		case []uint8:
			field.SetFloat(parseFloat64(string(v)))
		case string:
			field.SetFloat(parseFloat64(v))
		}
	case reflect.Bool:
		switch v := value.(type) {
		case bool:
			field.SetBool(v)
		case int64:
			field.SetBool(v != 0)
		case []uint8:
			field.SetBool(string(v) == "1" || string(v) == "true")
		case string:
			field.SetBool(v == "1" || v == "true")
		}
	case reflect.Struct:
		// 处理 time.Time
		if field.Type() == reflect.TypeOf(time.Time{}) {
			switch v := value.(type) {
			case time.Time:
				field.Set(reflect.ValueOf(v))
			case string:
				if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
					field.Set(reflect.ValueOf(t))
				}
			}
		}
	}
}

func parseInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}

func parseFloat64(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// ==================== 旧名称别名方法（向后兼容）====================

// Column 别名方法（旧名称）-> Pluck
func (o *ORM) Column(column string) ([]interface{}, error) {
	return o.Pluck(column)
}

// Inc 别名方法（旧名称）-> Incr
func (o *ORM) Inc(field string, amount ...int) (int64, error) {
	return o.Incr(field, amount...)
}

// Dec 别名方法（旧名称）-> Decr
func (o *ORM) Dec(field string, amount ...int) (int64, error) {
	return o.Decr(field, amount...)
}

// InsertAll 别名方法（旧名称）-> InsertMulti
func (o *ORM) InsertAll(dataList []map[string]interface{}) (int64, error) {
	return o.InsertMulti(len(dataList), dataList)
}
