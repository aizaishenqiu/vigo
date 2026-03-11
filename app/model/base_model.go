package model

import (
	"vigo/framework/model"
)

// BaseModel 基础模型，所有业务模型建议继承此类
type BaseModel struct {
	*model.Model
}

// NewBaseModel 实例化基础模型
func NewBaseModel(table string) *BaseModel {
	return &BaseModel{
		Model: model.New(table),
	}
}

// 这里可以定义所有模型通用的逻辑，例如公共查询范围等
