package validate

import (
	"fmt"
	"strings"
)

// ArrayValidator 数组验证器（类似 TP 8.1.0 的多维数组验证）
type ArrayValidator struct {
	rules map[string][]RuleFunc
}

// NewArrayValidator 创建数组验证器
func NewArrayValidator() *ArrayValidator {
	return &ArrayValidator{
		rules: make(map[string][]RuleFunc),
	}
}

// AddRule 添加数组字段验证规则（支持指定键名，类似 TP 8.1.0）
func (av *ArrayValidator) AddRule(field string, rules ...RuleFunc) *ArrayValidator {
	av.rules[field] = rules
	return av
}

// Validate 验证数组数据（支持多维数组验证，类似 TP 8.1.0）
func (av *ArrayValidator) Validate(data map[string]interface{}) map[string]string {
	errors := make(map[string]string)

	for field, rules := range av.rules {
		value, exists := data[field]
		if !exists {
			// 检查是否是数组字段（items.*.name）
			if strings.Contains(field, ".*.") {
				// 多维数组验证
				av.validateMultiArray(field, rules, data, errors)
			} else if strings.HasSuffix(field, ".*") {
				// 一维数组验证
				baseField := strings.TrimSuffix(field, ".*")
				av.validateArray(baseField, rules, data, errors)
			}
			continue
		}

		// 单个字段验证
		for _, rule := range rules {
			if err := rule(field, value); err != nil {
				errors[field] = err.Error()
				break
			}
		}
	}

	return errors
}

// validateArray 验证一维数组
func (av *ArrayValidator) validateArray(baseField string, rules []RuleFunc, data map[string]interface{}, errors map[string]string) {
	value, exists := data[baseField]
	if !exists {
		return
	}

	// 断言为数组
	arr, ok := value.([]interface{})
	if !ok {
		errors[baseField] = fmt.Sprintf("%s 必须是数组", baseField)
		return
	}

	// 验证每个数组元素
	for i, item := range arr {
		field := fmt.Sprintf("%s[%d]", baseField, i)
		for _, rule := range rules {
			if err := rule(field, item); err != nil {
				errors[field] = err.Error()
				break
			}
		}
	}
}

// validateMultiArray 验证多维数组（类似 TP 8.1.0 的支持指定键名）
func (av *ArrayValidator) validateMultiArray(field string, rules []RuleFunc, data map[string]interface{}, errors map[string]string) {
	// 解析字段路径，例如：items.*.name
	parts := strings.Split(field, ".*.")
	if len(parts) != 2 {
		return
	}

	baseField := parts[0]
	subField := parts[1]

	// 获取基础数组
	baseValue, exists := data[baseField]
	if !exists {
		return
	}

	// 断言为数组
	arr, ok := baseValue.([]interface{})
	if !ok {
		errors[baseField] = fmt.Sprintf("%s 必须是数组", baseField)
		return
	}

	// 验证每个数组元素的子字段
	for i, item := range arr {
		// 尝试获取子字段
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		subValue, exists := itemMap[subField]
		if !exists {
			continue
		}

		// 验证子字段
		fieldName := fmt.Sprintf("%s[%d].%s", baseField, i, subField)
		for _, rule := range rules {
			if err := rule(fieldName, subValue); err != nil {
				errors[fieldName] = err.Error()
				break
			}
		}
	}
}
