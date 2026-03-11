# ORM 方法名称兼容性说明

## 兼容性保证

Vigo ORM **完全向后兼容**旧版本的方法名称，您可以继续使用熟悉的方法名！

## 方法名称对照表

| 旧名称 | 新名称 | 说明 | 兼容性 |
|--------|--------|------|--------|
| `Select()` | `Get()` | 获取所有记录 | ✅ 两者都可用 |
| `Column()` | `Pluck()` | 获取单列值 | ✅ 两者都可用 |
| `Order()` | `OrderBy()` | 排序 | ✅ 两者都可用 |
| `Inc()` | `Incr()` | 字段自增 | ✅ 两者都可用 |
| `Dec()` | `Decr()` | 字段自减 | ✅ 两者都可用 |
| `InsertAll()` | `InsertMulti()` | 批量插入 | ✅ 两者都可用 |

## 使用示例

### 旧名称（仍然可用）

```go
// 使用旧名称 - 完全兼容
users, _ := db.Table("users").
    Where("status", 1).
    Order("create_time DESC").
    Select()

email, _ := db.Table("users").
    Where("id", 1).
    Column("email")

db.Table("users").
    Where("id", 1).
    Inc("views")

db.Table("users").InsertAll(users)
```

### 新名称（推荐使用）

```go
// 使用新名称 - 更清晰
users, _ := db.Table("users").
    Where("status", 1).
    OrderByDesc("create_time").
    Get()

email, _ := db.Table("users").
    Where("id", 1).
    Pluck("email")

db.Table("users").
    Where("id", 1).
    Incr("views")

db.Table("users").InsertMulti(len(users), users)
```

## 建议

- ✅ **旧代码**：可以继续使用旧名称，无需修改
- ✅ **新代码**：建议使用新名称，更清晰易懂
- ✅ **混合使用**：新旧名称可以混合使用，不会冲突

## 实现方式

旧名称方法通过**别名**实现，内部调用新名称方法：

```go
// Column 是 Pluck 的别名
func (o *ORM) Column(column string) ([]interface{}, error) {
	return o.Pluck(column)
}

// Inc 是 Incr 的别名
func (o *ORM) Inc(field string, amount ...int) (int64, error) {
	return o.Incr(field, amount...)
}

// Dec 是 Decr 的别名
func (o *ORM) Dec(field string, amount ...int) (int64, error) {
	return o.Decr(field, amount...)
}

// InsertAll 是 InsertMulti 的别名
func (o *ORM) InsertAll(dataList []map[string]interface{}) (int64, error) {
	return o.InsertMulti(len(dataList), dataList)
}
```

## 总结

- ✅ **完全兼容**：旧代码无需修改，直接可用
- ✅ **平滑过渡**：可以逐步迁移到新名称
- ✅ **灵活选择**：根据个人习惯选择使用旧名称或新名称

---

**版本**: v2.0.12  
**更新**: 2026-03-10  
**作者**: Vigo Framework Team
