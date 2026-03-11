package validate

import (
	"fmt"
)

// ValidateRuleSet 验证规则集（类似 TP 8.1.2 的 ValidateRuleSet）
type ValidateRuleSet struct {
	rules      map[string][]RuleFunc // 字段规则
	messages   map[string]string     // 错误消息
	scenarios  map[string][]string   // 验证场景
	onlyFields map[string]bool       // 仅验证指定字段
	mustFields map[string]bool       // 必须验证的字段（类似 TP 8.1.0 的 must 属性）
}

// RuleFunc 验证规则函数
type RuleFunc func(field string, value interface{}) error

// NewRuleSet 创建规则集
func NewRuleSet() *ValidateRuleSet {
	return &ValidateRuleSet{
		rules:      make(map[string][]RuleFunc),
		messages:   make(map[string]string),
		scenarios:  make(map[string][]string),
		onlyFields: make(map[string]bool),
		mustFields: make(map[string]bool),
	}
}

// AddRule 添加规则到规则集（类似 TP 8.1.2）
func (vs *ValidateRuleSet) AddRule(field string, rules ...RuleFunc) *ValidateRuleSet {
	vs.rules[field] = rules
	return vs
}

// Scenario 设置验证场景（类似 TP 8.1.0）
func (vs *ValidateRuleSet) Scenario(name string, fields ...string) *ValidateRuleSet {
	vs.scenarios[name] = fields
	return vs
}

// Only 仅验证指定字段（类似 TP 8.1.0）
func (vs *ValidateRuleSet) Only(fields ...string) *ValidateRuleSet {
	for _, field := range fields {
		vs.onlyFields[field] = true
	}
	return vs
}

// Must 设置必须验证的字段（类似 TP 8.1.0 的 must 属性）
func (vs *ValidateRuleSet) Must(fields ...string) *ValidateRuleSet {
	for _, field := range fields {
		vs.mustFields[field] = true
	}
	return vs
}

// Validate 验证数据
func (vs *ValidateRuleSet) Validate(data map[string]interface{}, scenario string) map[string]string {
	errors := make(map[string]string)

	// 确定要验证的字段
	fieldsToValidate := make(map[string]bool)
	if len(vs.onlyFields) > 0 {
		// 仅验证指定字段
		fieldsToValidate = vs.onlyFields
	} else if len(scenario) > 0 && len(vs.scenarios[scenario]) > 0 {
		// 使用场景验证
		for _, field := range vs.scenarios[scenario] {
			fieldsToValidate[field] = true
		}
	} else {
		// 验证所有字段
		for field := range vs.rules {
			fieldsToValidate[field] = true
		}
	}

	// 必须验证的字段（类似 TP 8.1.0）
	for field := range vs.mustFields {
		fieldsToValidate[field] = true
	}

	// 执行验证
	for field, rules := range vs.rules {
		if !fieldsToValidate[field] {
			continue
		}

		value := data[field]
		for _, rule := range rules {
			if err := rule(field, value); err != nil {
				errors[field] = err.Error()
				break // 一个字段只返回第一个错误
			}
		}
	}

	return errors
}

// HasError 是否有错误
func (vs *ValidateRuleSet) HasError() bool {
	return len(vs.messages) > 0
}

// GetError 获取错误消息
func (vs *ValidateRuleSet) GetError() map[string]string {
	return vs.messages
}

// GetKey 获取错误字段名（类似 TP 8.1.0 的 getKey 方法）
func (vs *ValidateRuleSet) GetKey(field string) string {
	if _, ok := vs.messages[field]; ok {
		return field
	}
	return ""
}

// ValidateBatch 批量验证（类似 TP 8.1.2）
func (vs *ValidateRuleSet) ValidateBatch(dataList []map[string]interface{}) []map[string]string {
	allErrors := make([]map[string]string, 0)

	for i, data := range dataList {
		errors := vs.Validate(data, "")
		if len(errors) > 0 {
			// 添加索引信息
			indexedErrors := make(map[string]string)
			for field, msg := range errors {
				indexedErrors[fmt.Sprintf("[%d]%s", i, field)] = msg
			}
			allErrors = append(allErrors, indexedErrors)
		}
	}

	return allErrors
}

// Rules 通过 rules 方法定义验证规则（类似 TP 8.1.2）
func (vs *ValidateRuleSet) Rules() map[string][]RuleFunc {
	return vs.rules
}

// SetRules 设置验证规则（返回数组或验证对象，类似 TP 8.1.2）
func (vs *ValidateRuleSet) SetRules(rules map[string][]RuleFunc) *ValidateRuleSet {
	vs.rules = rules
	return vs
}

// GetRule 获取字段规则（类似 TP 8.1.2）
func (vs *ValidateRuleSet) GetRule(field string) []RuleFunc {
	return vs.rules[field]
}

// RemoveRule 移除字段规则
func (vs *ValidateRuleSet) RemoveRule(fields ...string) *ValidateRuleSet {
	for _, field := range fields {
		delete(vs.rules, field)
	}
	return vs
}

// ClearRules 清空所有规则
func (vs *ValidateRuleSet) ClearRules() *ValidateRuleSet {
	vs.rules = make(map[string][]RuleFunc)
	return vs
}

// MergeRules 合并规则
func (vs *ValidateRuleSet) MergeRules(other *ValidateRuleSet) *ValidateRuleSet {
	for field, rules := range other.rules {
		vs.rules[field] = append(vs.rules[field], rules...)
	}
	return vs
}

// RuleAliasManager 规则别名管理器（类似 TP 8.1.2）
type RuleAliasManager struct {
	aliases map[string][]RuleFunc
}

// NewRuleAliasManager 创建规则别名管理器
func NewRuleAliasManager() *RuleAliasManager {
	return &RuleAliasManager{
		aliases: make(map[string][]RuleFunc),
	}
}

// Define 定义规则别名（类似 TP 8.1.2）
func (ram *RuleAliasManager) Define(name string, rules ...RuleFunc) *RuleAliasManager {
	ram.aliases[name] = rules
	return ram
}

// Get 获取规则别名
func (ram *RuleAliasManager) Get(name string) []RuleFunc {
	return ram.aliases[name]
}

// Use 使用规则别名
func (ram *RuleAliasManager) Use(vs *ValidateRuleSet, name string, fields ...string) *ValidateRuleSet {
	rules := ram.aliases[name]
	if rules == nil {
		return vs
	}

	for _, field := range fields {
		vs.rules[field] = append(vs.rules[field], rules...)
	}

	return vs
}
