package model

import "vigo/framework/db"

// ========== 关联查询 (ThinkPHP/Laravel 风格) ==========

// HasOne 一对一关联
func (m *Model) HasOne(table string, foreignKey string, localKey string) map[string]interface{} {
	localVal := m.data[localKey]
	if localVal == nil {
		return nil
	}
	res, _ := db.Table(table).Where(foreignKey, localVal).Find()
	return res
}

// HasMany 一对多关联
func (m *Model) HasMany(table string, foreignKey string, localKey string) []map[string]interface{} {
	localVal := m.data[localKey]
	if localVal == nil {
		return nil
	}
	res, _ := db.Table(table).Where(foreignKey, localVal).Select()
	return res
}

// BelongsTo 反向一对一关联
func (m *Model) BelongsTo(table string, foreignKey string, ownerKey string) map[string]interface{} {
	foreignVal := m.data[foreignKey]
	if foreignVal == nil {
		return nil
	}
	res, _ := db.Table(table).Where(ownerKey, foreignVal).Find()
	return res
}

// BelongsToMany 多对多关联 (通过中间表)
func (m *Model) BelongsToMany(table string, pivotTable string, foreignKey string, relatedKey string) []map[string]interface{} {
	localVal := m.data[m.primaryKey]
	if localVal == nil {
		return nil
	}

	// 从中间表获取关联 ID
	pivotRows, err := db.Table(pivotTable).Where(foreignKey, localVal).Select()
	if err != nil || len(pivotRows) == 0 {
		return nil
	}

	var relatedIds []interface{}
	for _, row := range pivotRows {
		if id, ok := row[relatedKey]; ok {
			relatedIds = append(relatedIds, id)
		}
	}

	if len(relatedIds) == 0 {
		return nil
	}

	results, _ := db.Table(table).WhereIn("id", relatedIds).Select()
	return results
}

// HasOneThrough 远程一对一 (ThinkPHP/Laravel hasOneThrough)
func (m *Model) HasOneThrough(table string, throughTable string, localKey, throughKey, foreignKey, throughPk string) map[string]interface{} {
	localVal := m.data[localKey]
	if localVal == nil {
		return nil
	}

	throughRow, _ := db.Table(throughTable).Where(throughKey, localVal).Find()
	if throughRow == nil {
		return nil
	}

	throughVal := throughRow[throughPk]
	res, _ := db.Table(table).Where(foreignKey, throughVal).Find()
	return res
}

// HasManyThrough 远程一对多 (ThinkPHP/Laravel hasManyThrough)
func (m *Model) HasManyThrough(table string, throughTable string, localKey, throughKey, foreignKey, throughPk string) []map[string]interface{} {
	localVal := m.data[localKey]
	if localVal == nil {
		return nil
	}

	throughRows, _ := db.Table(throughTable).Where(throughKey, localVal).Select()
	if len(throughRows) == 0 {
		return nil
	}

	var throughIds []interface{}
	for _, row := range throughRows {
		if id, ok := row[throughPk]; ok {
			throughIds = append(throughIds, id)
		}
	}

	if len(throughIds) == 0 {
		return nil
	}

	results, _ := db.Table(table).WhereIn(foreignKey, throughIds).Select()
	return results
}
