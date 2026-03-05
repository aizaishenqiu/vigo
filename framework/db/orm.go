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
}

// NewORM 创建 ORM 实例
func NewORM(db *sql.DB) *ORM {
	return &ORM{
		db:    db,
		query: NewQuery(db),
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

// Where 添加条件
func (o *ORM) Where(field string, value interface{}) *ORM {
	o.query.Where(field, value)
	return o
}

// WhereIn 添加 IN 条件
func (o *ORM) WhereIn(field string, values interface{}) *ORM {
	o.query.WhereIn(field, values)
	return o
}

// Order 添加排序
func (o *ORM) Order(order string) *ORM {
	o.query.Order(order)
	return o
}

// Limit 设置限制
func (o *ORM) Limit(limit int) *ORM {
	o.query.Limit(limit)
	return o
}

// Offset 设置偏移
func (o *ORM) Offset(offset int) *ORM {
	o.query.offset = offset
	return o
}

// Page 分页
func (o *ORM) Page(page, pageSize int) *ORM {
	o.query.Page(page, pageSize)
	return o
}

// WhereOp 添加带操作符的条件（自定义实现）
func (o *ORM) WhereOp(field, operator string, value interface{}) *ORM {
	// 调用底层的 addWhere 方法
	switch operator {
	case "=", "!=", "<>", ">", "<", ">=", "<=":
		o.query.Where(field, operator, value)
	default:
		o.query.Where(field, value)
	}
	return o
}

// First 查询第一条记录
func (o *ORM) First(dest interface{}) error {
	o.query.Limit(1)
	result, err := o.query.Select()
	if err != nil {
		return err
	}
	if len(result) == 0 {
		return sql.ErrNoRows
	}
	return mapToStruct(result[0], dest)
}

// Find 查询所有记录
func (o *ORM) Find(dest interface{}) error {
	results, err := o.query.Select()
	if err != nil {
		return err
	}
	return mapsToStructs(results, dest)
}

// Count 统计数量
func (o *ORM) Count() (int64, error) {
	return o.query.Count()
}

// Create 创建记录
func (o *ORM) Create(model interface{}) error {
	data := structToMap(model)
	return o.Insert(data)
}

// Insert 插入数据
func (o *ORM) Insert(data map[string]interface{}) error {
	if o.tableName == "" {
		return fmt.Errorf("table name not set")
	}

	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	i := 1
	for col, val := range data {
		columns = append(columns, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		o.tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := o.db.ExecContext(ctx, sql, values...)
	return err
}

// Update 更新记录
func (o *ORM) Update(data map[string]interface{}) error {
	if o.tableName == "" {
		return fmt.Errorf("table name not set")
	}

	setParts := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	i := 1
	for col, val := range data {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", col, i))
		values = append(values, val)
		i++
	}

	sql := fmt.Sprintf("UPDATE %s SET %s", o.tableName, strings.Join(setParts, ", "))

	// 添加 WHERE 条件
	whereSQL, whereArgs := o.query.buildWhere()
	if whereSQL != "" {
		sql += " WHERE " + whereSQL
		values = append(values, whereArgs...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := o.db.ExecContext(ctx, sql, values...)
	return err
}

// Delete 删除记录
func (o *ORM) Delete() error {
	if o.tableName == "" {
		return fmt.Errorf("table name not set")
	}

	sql := fmt.Sprintf("DELETE FROM %s", o.tableName)

	whereSQL, whereArgs := o.query.buildWhere()
	if whereSQL != "" {
		sql += " WHERE " + whereSQL
	} else {
		return fmt.Errorf("delete without where clause is not allowed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := o.db.ExecContext(ctx, sql, whereArgs...)
	return err
}

// Transaction 事务处理
func (o *ORM) Transaction(fn func(tx *ORM) error) error {
	tx, err := o.db.Begin()
	if err != nil {
		return err
	}

	txORM := &ORM{
		db:        o.db,
		tx:        tx,
		tableName: o.tableName,
		query:     o.query,
	}

	if err := fn(txORM); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
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
			case []uint8:
				if t, err := time.Parse("2006-01-02 15:04:05", string(v)); err == nil {
					field.Set(reflect.ValueOf(t))
				}
			}
		}
	}
}

// parseInt64 解析 int64
func parseInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}

// parseFloat64 解析 float64
func parseFloat64(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
