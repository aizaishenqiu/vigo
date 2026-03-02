package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
	"vigo/framework/db"
)

// Model 模型基类 (ThinkPHP/Laravel 风格 Active Record)
type Model struct {
	table      string
	primaryKey string
	data       map[string]interface{}
	origin     map[string]interface{}
	query      *db.Query

	// ThinkPHP/Laravel 风格配置
	autoWriteTimestamp bool   // 自动写入时间戳
	createTime         string // 创建时间字段名
	updateTime         string // 更新时间字段名
	deleteTime         string // 软删除字段名
	softDelete         bool   // 是否启用软删除

	// 输出控制
	hidden  []string // 隐藏字段
	visible []string // 可见字段
	append  []string // 追加字段

	// 只读字段
	readonly []string

	// 事件回调
	beforeInsert func(m *Model) error
	afterInsert  func(m *Model)
	beforeUpdate func(m *Model) error
	afterUpdate  func(m *Model)
	beforeDelete func(m *Model) error
	afterDelete  func(m *Model)

	// 软删除状态
	withTrashed bool
	onlyTrashed bool

	// 全局查询范围
	globalScopes map[string]func(q *db.Query) *db.Query

	// 是否为新记录
	isNew bool
}

// New 创建模型实例
func New(table string) *Model {
	return &Model{
		table:              table,
		primaryKey:         "id",
		data:               make(map[string]interface{}),
		origin:             make(map[string]interface{}),
		autoWriteTimestamp: true,
		createTime:         "create_time",
		updateTime:         "update_time",
		deleteTime:         "delete_time",
		softDelete:         true,
		globalScopes:       make(map[string]func(q *db.Query) *db.Query),
		isNew:              true,
	}
}

// ========== 配置方法 ==========

// SetPk 设置主键字段名
func (m *Model) SetPk(pk string) *Model {
	m.primaryKey = pk
	return m
}

// SetAutoTimestamp 设置是否自动写入时间戳
func (m *Model) SetAutoTimestamp(auto bool) *Model {
	m.autoWriteTimestamp = auto
	return m
}

// SetSoftDelete 设置软删除
func (m *Model) SetSoftDelete(enable bool, field ...string) *Model {
	m.softDelete = enable
	if len(field) > 0 {
		m.deleteTime = field[0]
	}
	return m
}

// Hidden 设置隐藏字段 (输出时排除)
func (m *Model) Hidden(fields ...string) *Model {
	m.hidden = fields
	return m
}

// Visible 设置可见字段 (只输出这些)
func (m *Model) Visible(fields ...string) *Model {
	m.visible = fields
	return m
}

// Append 设置追加字段 (获取器虚拟字段)
func (m *Model) Append(fields ...string) *Model {
	m.append = fields
	return m
}

// Readonly 设置只读字段 (更新时忽略)
func (m *Model) Readonly(fields ...string) *Model {
	m.readonly = fields
	return m
}

// ========== 事件方法 ==========

// BeforeInsert 注册插入前事件
func (m *Model) BeforeInsert(fn func(m *Model) error) *Model {
	m.beforeInsert = fn
	return m
}

// AfterInsert 注册插入后事件
func (m *Model) AfterInsert(fn func(m *Model)) *Model {
	m.afterInsert = fn
	return m
}

// BeforeUpdate 注册更新前事件
func (m *Model) BeforeUpdate(fn func(m *Model) error) *Model {
	m.beforeUpdate = fn
	return m
}

// AfterUpdate 注册更新后事件
func (m *Model) AfterUpdate(fn func(m *Model)) *Model {
	m.afterUpdate = fn
	return m
}

// BeforeDelete 注册删除前事件
func (m *Model) BeforeDelete(fn func(m *Model) error) *Model {
	m.beforeDelete = fn
	return m
}

// AfterDelete 注册删除后事件
func (m *Model) AfterDelete(fn func(m *Model)) *Model {
	m.afterDelete = fn
	return m
}

// ========== 查询构造器代理 ==========

// getQuery 获取内部查询构造器实例（懒加载）
func (m *Model) getQuery() *db.Query {
	if m.query == nil {
		m.query = db.NewQuery(db.GlobalDB).Table(m.table)
	}
	return m.query
}

// Db 获取独立的查询构造器
func (m *Model) Db() *db.Query {
	return db.NewQuery(db.GlobalDB).Table(m.table)
}

// Field 指定字段
func (m *Model) Field(fields interface{}) *Model {
	m.getQuery().Field(fields)
	return m
}

// Where 条件
func (m *Model) Where(field string, args ...interface{}) *Model {
	m.getQuery().Where(field, args...)
	return m
}

// WhereNot NOT 条件
func (m *Model) WhereNot(field string, args ...interface{}) *Model {
	m.getQuery().WhereNot(field, args...)
	return m
}

// WhereIn IN 条件
func (m *Model) WhereIn(field string, val interface{}) *Model {
	m.getQuery().WhereIn(field, val)
	return m
}

// WhereNotIn NOT IN 条件
func (m *Model) WhereNotIn(field string, val interface{}) *Model {
	m.getQuery().WhereNotIn(field, val)
	return m
}

// WhereOr OR 条件
func (m *Model) WhereOr(field string, args ...interface{}) *Model {
	m.getQuery().WhereOr(field, args...)
	return m
}

// WhereBetween BETWEEN 条件
func (m *Model) WhereBetween(field string, min, max interface{}) *Model {
	m.getQuery().WhereBetween(field, min, max)
	return m
}

// WhereNotBetween NOT BETWEEN 条件
func (m *Model) WhereNotBetween(field string, min, max interface{}) *Model {
	m.getQuery().WhereNotBetween(field, min, max)
	return m
}

// WhereNull IS NULL 条件
func (m *Model) WhereNull(field string) *Model {
	m.getQuery().WhereNull(field)
	return m
}

// WhereNotNull IS NOT NULL 条件
func (m *Model) WhereNotNull(field string) *Model {
	m.getQuery().WhereNotNull(field)
	return m
}

// WhereLike LIKE 条件
func (m *Model) WhereLike(field string, val string) *Model {
	m.getQuery().WhereLike(field, val)
	return m
}

// WhereRaw 原生条件
func (m *Model) WhereRaw(sql string, args ...interface{}) *Model {
	m.getQuery().WhereRaw(sql, args...)
	return m
}

// WhereMap 批量条件 (ThinkPHP/Laravel 风格)
func (m *Model) WhereMap(conditions map[string]interface{}) *Model {
	m.getQuery().WhereMap(conditions)
	return m
}

// Order 排序
func (m *Model) Order(order string) *Model {
	m.getQuery().Order(order)
	return m
}

// Limit 限制
func (m *Model) Limit(limit int) *Model {
	m.getQuery().Limit(limit)
	return m
}

// Page 分页
func (m *Model) Page(page, pageSize int) *Model {
	m.getQuery().Page(page, pageSize)
	return m
}

// Group 分组
func (m *Model) Group(group string) *Model {
	m.getQuery().Group(group)
	return m
}

// Having 分组过滤
func (m *Model) Having(condition string, args ...interface{}) *Model {
	m.getQuery().Having(condition, args...)
	return m
}

// Join 连表
func (m *Model) Join(table string, condition string, joinType string) *Model {
	m.getQuery().Join(table, condition, joinType)
	return m
}

// Alias 别名
func (m *Model) Alias(alias string) *Model {
	m.getQuery().Alias(alias)
	return m
}

// Distinct 去重
func (m *Model) Distinct() *Model {
	m.getQuery().Distinct()
	return m
}

// Lock 锁
func (m *Model) Lock(mode ...string) *Model {
	m.getQuery().Lock(mode...)
	return m
}

// ========== 软删除查询修饰 ==========

// WithTrashed 包含软删除记录
func (m *Model) WithTrashed() *Model {
	m.withTrashed = true
	return m
}

// OnlyTrashed 只查询软删除记录
func (m *Model) OnlyTrashed() *Model {
	m.onlyTrashed = true
	return m
}

// applySoftDelete 应用软删除条件到查询
func (m *Model) applySoftDelete() {
	if !m.softDelete || m.withTrashed {
		return
	}
	q := m.getQuery()
	if m.onlyTrashed {
		q.WhereNotNull(m.deleteTime)
	} else {
		q.WhereNull(m.deleteTime)
	}
}

// ========== 执行方法 ==========

// Find 查询单条
func (m *Model) Find(id ...interface{}) *Model {
	var res map[string]interface{}
	var err error

	if len(id) > 0 {
		q := db.NewQuery(db.GlobalDB).Table(m.table).Where(m.primaryKey, id[0])
		if m.softDelete && !m.withTrashed {
			if m.onlyTrashed {
				q.WhereNotNull(m.deleteTime)
			} else {
				q.WhereNull(m.deleteTime)
			}
		}
		res, err = q.Find()
	} else {
		m.applySoftDelete()
		res, err = m.getQuery().Find()
	}

	if err == nil && res != nil {
		m.data = res
		m.origin = make(map[string]interface{})
		for k, v := range res {
			m.origin[k] = v
		}
		m.isNew = false
	}
	m.query = nil
	return m
}

// Select 查询多条
func (m *Model) Select() ([]*Model, error) {
	m.applySoftDelete()
	rows, err := m.getQuery().Select()
	if err != nil {
		return nil, err
	}

	var results []*Model
	for _, row := range rows {
		nm := New(m.table)
		nm.primaryKey = m.primaryKey
		nm.softDelete = m.softDelete
		nm.deleteTime = m.deleteTime
		nm.autoWriteTimestamp = m.autoWriteTimestamp
		nm.createTime = m.createTime
		nm.updateTime = m.updateTime
		nm.hidden = m.hidden
		nm.visible = m.visible
		nm.append = m.append
		nm.data = row
		nm.isNew = false
		for k, v := range row {
			nm.origin[k] = v
		}
		results = append(results, nm)
	}

	m.query = nil
	return results, nil
}

// Paginate 分页查询 (ThinkPHP/Laravel paginate)
func (m *Model) Paginate(page, pageSize int) (*db.PaginateResult, error) {
	m.applySoftDelete()
	result, err := m.getQuery().Paginate(page, pageSize)
	m.query = nil
	return result, err
}

// Count 统计
func (m *Model) Count() (int64, error) {
	m.applySoftDelete()
	count, err := m.getQuery().Count()
	m.query = nil
	return count, err
}

// Sum 求和
func (m *Model) Sum(field string) (float64, error) {
	m.applySoftDelete()
	sum, err := m.getQuery().Sum(field)
	m.query = nil
	return sum, err
}

// Avg 平均值
func (m *Model) Avg(field string) (float64, error) {
	m.applySoftDelete()
	avg, err := m.getQuery().Avg(field)
	m.query = nil
	return avg, err
}

// Max 最大值
func (m *Model) Max(field string) (interface{}, error) {
	m.applySoftDelete()
	max, err := m.getQuery().Max(field)
	m.query = nil
	return max, err
}

// Min 最小值
func (m *Model) Min(field string) (interface{}, error) {
	m.applySoftDelete()
	min, err := m.getQuery().Min(field)
	m.query = nil
	return min, err
}

// Value 获取单个字段值
func (m *Model) Value(field string) (interface{}, error) {
	m.applySoftDelete()
	val, err := m.getQuery().Value(field)
	m.query = nil
	return val, err
}

// Column 获取某列所有值
func (m *Model) Column(field string) ([]interface{}, error) {
	m.applySoftDelete()
	col, err := m.getQuery().Column(field)
	m.query = nil
	return col, err
}

// Chunk 分块处理
func (m *Model) Chunk(size int, callback func(rows []map[string]interface{}) error) error {
	m.applySoftDelete()
	err := m.getQuery().Chunk(size, callback)
	m.query = nil
	return err
}

// Exists 判断记录是否存在
func (m *Model) Exists() (bool, error) {
	m.applySoftDelete()
	exists, err := m.getQuery().Exists()
	m.query = nil
	return exists, err
}

// ========== 写入方法 ==========

// Data 批量设置数据 (ThinkPHP/Laravel data() 风格)
func (m *Model) Data(data map[string]interface{}) *Model {
	for k, v := range data {
		m.data[k] = v
	}
	return m
}

// SetAttr 设置属性 (支持修改器)
func (m *Model) SetAttr(key string, value interface{}) *Model {
	methodName := "Set" + camelCase(key) + "Attr"
	v := reflect.ValueOf(m)
	method := v.MethodByName(methodName)
	if method.IsValid() {
		res := method.Call([]reflect.Value{reflect.ValueOf(value)})
		if len(res) > 0 {
			m.data[key] = res[0].Interface()
			return m
		}
	}
	m.data[key] = value
	return m
}

// GetAttr 获取属性 (支持获取器)
func (m *Model) GetAttr(key string) interface{} {
	methodName := "Get" + camelCase(key) + "Attr"
	v := reflect.ValueOf(m)
	method := v.MethodByName(methodName)
	if method.IsValid() {
		res := method.Call(nil)
		if len(res) > 0 {
			return res[0].Interface()
		}
	}
	return m.data[key]
}

// Save 保存 (ThinkPHP/Laravel 风格: 有主键更新，无主键新增)
func (m *Model) Save() (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")

	id, ok := m.data[m.primaryKey]
	if ok && id != nil && id != 0 && id != "" && !m.isNew {
		// 更新操作
		if m.beforeUpdate != nil {
			if err := m.beforeUpdate(m); err != nil {
				return 0, err
			}
		}

		updateData := make(map[string]interface{})
		for k, v := range m.data {
			if k == m.primaryKey {
				continue
			}
			if m.isReadonly(k) {
				continue
			}
			// 只更新变化的字段 (脏数据检测)
			if originVal, exists := m.origin[k]; exists {
				if originVal == v {
					continue
				}
			}
			updateData[k] = v
		}

		if len(updateData) == 0 {
			return 0, nil
		}

		if m.autoWriteTimestamp {
			updateData[m.updateTime] = now
		}

		affected, err := db.NewQuery(db.GlobalDB).Table(m.table).
			Where(m.primaryKey, id).Update(updateData)

		if err == nil && m.afterUpdate != nil {
			m.afterUpdate(m)
		}
		return affected, err
	}

	// 新增操作
	if m.beforeInsert != nil {
		if err := m.beforeInsert(m); err != nil {
			return 0, err
		}
	}

	if m.autoWriteTimestamp {
		m.data[m.createTime] = now
		m.data[m.updateTime] = now
	}

	newId, err := db.NewQuery(db.GlobalDB).Table(m.table).InsertGetId(m.data)

	if err == nil {
		m.data[m.primaryKey] = newId
		m.isNew = false
		if m.afterInsert != nil {
			m.afterInsert(m)
		}
	}
	return newId, err
}

// Create 创建记录 (ThinkPHP/Laravel 静态风格)
func Create(table string, data map[string]interface{}) (*Model, error) {
	m := New(table)
	m.Data(data)
	_, err := m.Save()
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Insert 直接插入 (不经过模型事件和时间戳)
func (m *Model) Insert(data map[string]interface{}) (int64, error) {
	return db.NewQuery(db.GlobalDB).Table(m.table).InsertGetId(data)
}

// InsertAll 批量插入
func (m *Model) InsertAll(dataList []map[string]interface{}) (int64, error) {
	if m.autoWriteTimestamp {
		now := time.Now().Format("2006-01-02 15:04:05")
		for _, data := range dataList {
			data[m.createTime] = now
			data[m.updateTime] = now
		}
	}
	return db.NewQuery(db.GlobalDB).Table(m.table).InsertAll(dataList)
}

// Update 直接更新 (条件更新，不加载数据)
func (m *Model) Update(data map[string]interface{}) (int64, error) {
	if m.autoWriteTimestamp {
		data[m.updateTime] = time.Now().Format("2006-01-02 15:04:05")
	}
	affected, err := m.getQuery().Update(data)
	m.query = nil
	return affected, err
}

// Delete 删除 (支持软删除)
func (m *Model) Delete() (int64, error) {
	if m.beforeDelete != nil {
		if err := m.beforeDelete(m); err != nil {
			return 0, err
		}
	}

	id, hasId := m.data[m.primaryKey]

	var affected int64
	var err error

	if m.softDelete {
		// 软删除
		deleteData := map[string]interface{}{
			m.deleteTime: time.Now().Format("2006-01-02 15:04:05"),
		}
		if hasId && id != nil {
			affected, err = db.NewQuery(db.GlobalDB).Table(m.table).
				Where(m.primaryKey, id).Update(deleteData)
		} else {
			affected, err = m.getQuery().Update(deleteData)
			m.query = nil
		}
	} else {
		// 物理删除
		if hasId && id != nil {
			affected, err = db.NewQuery(db.GlobalDB).Table(m.table).
				Where(m.primaryKey, id).Delete()
		} else {
			affected, err = m.getQuery().Delete()
			m.query = nil
		}
	}

	if err == nil && m.afterDelete != nil {
		m.afterDelete(m)
	}
	return affected, err
}

// ForceDelete 强制物理删除 (忽略软删除)
func (m *Model) ForceDelete() (int64, error) {
	id, hasId := m.data[m.primaryKey]
	if hasId && id != nil {
		return db.NewQuery(db.GlobalDB).Table(m.table).
			Where(m.primaryKey, id).Delete()
	}
	affected, err := m.getQuery().Delete()
	m.query = nil
	return affected, err
}

// Restore 恢复软删除记录
func (m *Model) Restore() (int64, error) {
	id, hasId := m.data[m.primaryKey]
	if !hasId || id == nil {
		return 0, nil
	}
	return db.NewQuery(db.GlobalDB).Table(m.table).
		Where(m.primaryKey, id).
		Update(map[string]interface{}{m.deleteTime: nil})
}

// Destroy 按主键批量删除 (ThinkPHP/Laravel destroy)
func Destroy(table string, ids ...interface{}) (int64, error) {
	m := New(table)
	if m.softDelete {
		return db.NewQuery(db.GlobalDB).Table(table).
			WhereIn(m.primaryKey, ids).
			Update(map[string]interface{}{
				m.deleteTime: time.Now().Format("2006-01-02 15:04:05"),
			})
	}
	return db.NewQuery(db.GlobalDB).Table(table).
		WhereIn(m.primaryKey, ids).Delete()
}

// Inc 字段自增 (ThinkPHP/Laravel Inc)
func (m *Model) Inc(field string, step ...int) (int64, error) {
	affected, err := m.getQuery().Inc(field, step...)
	m.query = nil
	return affected, err
}

// Dec 字段自减 (ThinkPHP/Laravel Dec)
func (m *Model) Dec(field string, step ...int) (int64, error) {
	affected, err := m.getQuery().Dec(field, step...)
	m.query = nil
	return affected, err
}

// ========== 数据输出 ==========

// GetData 获取所有原始数据
func (m *Model) GetData() map[string]interface{} {
	return m.data
}

// Data (别名，兼容旧代码)
// Deprecated: 使用 GetData() 或 ToArray()
func (m *Model) GetRawData() map[string]interface{} {
	return m.data
}

// ToArray 转为 Map (应用 hidden/visible/append)
func (m *Model) ToArray() map[string]interface{} {
	result := make(map[string]interface{})

	if len(m.visible) > 0 {
		for _, f := range m.visible {
			if val, ok := m.data[f]; ok {
				result[f] = val
			}
		}
	} else {
		for k, v := range m.data {
			result[k] = v
		}
		for _, f := range m.hidden {
			delete(result, f)
		}
	}

	// 追加虚拟字段 (通过获取器)
	for _, f := range m.append {
		result[f] = m.GetAttr(f)
	}

	return result
}

// ToJson 转为 JSON 字符串
func (m *Model) ToJson() string {
	b, _ := json.Marshal(m.ToArray())
	return string(b)
}

// IsEmpty 判断模型是否为空
func (m *Model) IsEmpty() bool {
	return len(m.data) == 0
}

// GetOrigin 获取原始数据
func (m *Model) GetOrigin(key ...string) interface{} {
	if len(key) > 0 {
		return m.origin[key[0]]
	}
	return m.origin
}

// IsDirty 判断字段是否被修改
func (m *Model) IsDirty(field string) bool {
	newVal, hasNew := m.data[field]
	oldVal, hasOld := m.origin[field]
	if hasNew != hasOld {
		return true
	}
	return newVal != oldVal
}

// GetChangedData 获取变化的数据
func (m *Model) GetChangedData() map[string]interface{} {
	changed := make(map[string]interface{})
	for k, v := range m.data {
		if originVal, exists := m.origin[k]; !exists || originVal != v {
			changed[k] = v
		}
	}
	return changed
}

// ========== 辅助方法 ==========

func (m *Model) isReadonly(field string) bool {
	for _, f := range m.readonly {
		if f == field {
			return true
		}
	}
	return false
}

func camelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// ========== ORM 增强功能（类似 TP 8.1.4） ==========

// WhereJSON JSON 字段查询（类似 TP 8.1.4）
func (m *Model) WhereJSON(field string, path string, value interface{}) *Model {
	query := field + "->>'$." + path + "' = ?"
	m.getQuery().WhereRaw(query, value)
	return m
}

// WhereJSONIn JSON 字段 IN 查询（类似 TP 8.1.4）
func (m *Model) WhereJSONIn(field string, path string, values []interface{}) *Model {
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	query := field + "->>'$." + path + "' IN (" + strings.Join(placeholders, ",") + ")"
	m.getQuery().WhereRaw(query, values...)
	return m
}

// WhereJSONLike JSON 字段 LIKE 查询（类似 TP 8.1.4）
func (m *Model) WhereJSONLike(field string, path string, pattern string) *Model {
	query := field + "->>'$." + path + "' LIKE ?"
	m.getQuery().WhereRaw(query, pattern)
	return m
}

// WhereJSONContains JSON 字段包含查询（类似 TP 8.1.4，MySQL JSON_CONTAINS）
func (m *Model) WhereJSONContains(field string, path string, value string) *Model {
	query := `JSON_CONTAINS(` + field + `, ?, '$.` + path + `')`
	m.getQuery().WhereRaw(query, value)
	return m
}

// ForceDeleteByID 根据 ID 强制删除
func (m *Model) ForceDeleteByID(id interface{}) (int64, error) {
	affected, err := db.NewQuery(db.GlobalDB).
		Table(m.table).
		Where(m.primaryKey, id).
		Delete()

	m.query = nil
	return affected, err
}

// RestoreByID 根据 ID 恢复软删除数据
func (m *Model) RestoreByID(id interface{}) (int64, error) {
	affected, err := db.NewQuery(db.GlobalDB).
		Table(m.table).
		Where(m.primaryKey, id).
		Update(map[string]interface{}{m.deleteTime: nil})

	m.query = nil
	return affected, err
}

// Trashed 检查当前模型是否已被软删除
func (m *Model) Trashed() bool {
	if val, ok := m.data[m.deleteTime]; ok && val != nil {
		return true
	}
	return false
}

// JSONField JSON 字段处理（类似 TP 8.1.4）
type JSONField map[string]interface{}

// MarshalJSON 序列化 JSON 字段
func (jf JSONField) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}(jf))
}

// UnmarshalJSON 反序列化 JSON 字段
func (jf *JSONField) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*jf = m
	return nil
}

// GetJSON 获取 JSON 字段值
func (m *Model) GetJSON(field string) (JSONField, error) {
	if val, ok := m.data[field]; ok {
		switch v := val.(type) {
		case []byte:
			var jf JSONField
			err := json.Unmarshal(v, &jf)
			return jf, err
		case string:
			var jf JSONField
			err := json.Unmarshal([]byte(v), &jf)
			return jf, err
		case map[string]interface{}:
			return JSONField(v), nil
		default:
			return nil, fmt.Errorf("field %s is not a JSON field", field)
		}
	}
	return nil, nil
}

// SetJSON 设置 JSON 字段值
func (m *Model) SetJSON(field string, value JSONField) {
	m.data[field] = value
}
