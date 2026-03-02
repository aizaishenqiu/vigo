package validate

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// EnhancedValidator 增强验证器
type EnhancedValidator struct {
	Rules  map[string][]Rule
	Data   map[string]interface{}
	Errors map[string][]string
}

// Rule 验证规则接口
type Rule interface {
	Validate(value interface{}) bool
	Message() string
}

// ValidationFunc 验证函数类型
type ValidationFunc func(value interface{}) bool

// FuncRule 函数规则实现
type FuncRule struct {
	fn      ValidationFunc
	message string
}

func (fr *FuncRule) Validate(value interface{}) bool {
	return fr.fn(value)
}

func (fr *FuncRule) Message() string {
	return fr.message
}

// NewEnhanced 创建增强验证器
func NewEnhanced() *EnhancedValidator {
	return &EnhancedValidator{
		Rules:  make(map[string][]Rule),
		Data:   make(map[string]interface{}),
		Errors: make(map[string][]string),
	}
}

// AddRule 添加验证规则
func (ev *EnhancedValidator) AddRule(field string, rule Rule) *EnhancedValidator {
	ev.Rules[field] = append(ev.Rules[field], rule)
	return ev
}

// SetData 设置验证数据
func (ev *EnhancedValidator) SetData(data map[string]interface{}) *EnhancedValidator {
	ev.Data = data
	return ev
}

// Validate 执行验证
func (ev *EnhancedValidator) Validate() bool {
	for field, rules := range ev.Rules {
		value, exists := ev.Data[field]
		for _, rule := range rules {
			if !rule.Validate(value) {
				ev.Errors[field] = append(ev.Errors[field], rule.Message())
			}
		}
		if !exists && len(ev.Rules[field]) > 0 {
			ev.Errors[field] = append(ev.Errors[field], fmt.Sprintf("%s is required", field))
		}
	}
	return len(ev.Errors) == 0
}

// Required 验证必填
func Required() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if value == nil {
				return false
			}
			if str, ok := value.(string); ok {
				return strings.TrimSpace(str) != ""
			}
			return true
		},
		message: "This field is required",
	}
}

// Email 验证邮箱
func Email() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
				matched, _ := regexp.MatchString(pattern, str)
				return matched
			}
			return false
		},
		message: "Must be a valid email",
	}
}

// Mobile 验证手机号
func Mobile() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				pattern := `^1[3-9]\d{9}$`
				matched, _ := regexp.MatchString(pattern, str)
				return matched
			}
			return false
		},
		message: "Must be a valid mobile number",
	}
}

// Min 验证最小长度
func Min(min int) Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				return len(str) >= min
			}
			return false
		},
		message: fmt.Sprintf("Must be at least %d characters", min),
	}
}

// Max 验证最大长度
func Max(max int) Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				return len(str) <= max
			}
			return false
		},
		message: fmt.Sprintf("Must be at most %d characters", max),
	}
}

// Between 验证长度范围
func Between(min, max int) Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				return len(str) >= min && len(str) <= max
			}
			return false
		},
		message: fmt.Sprintf("Must be between %d and %d characters", min, max),
	}
}

// Numeric 验证数字
func Numeric() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				_, err := regexp.MatchString(`^-?\d+(\.\d+)?$`, str)
				return err == nil
			}
			return false
		},
		message: "Must be a numeric value",
	}
}

// Integer 验证整数
func Integer() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				_, err := regexp.MatchString(`^-?\d+$`, str)
				return err == nil
			}
			return false
		},
		message: "Must be an integer",
	}
}

// URL 验证URL
func URL() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				_, err := url.ParseRequestURI(str)
				return err == nil
			}
			return false
		},
		message: "Must be a valid URL",
	}
}

// Alpha 验证字母
func Alpha() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				_, err := regexp.MatchString(`^[a-zA-Z]+$`, str)
				return err == nil
			}
			return false
		},
		message: "Must contain only letters",
	}
}

// AlphaNum 验证字母数字
func AlphaNum() Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				_, err := regexp.MatchString(`^[a-zA-Z0-9]+$`, str)
				return err == nil
			}
			return false
		},
		message: "Must contain only letters and numbers",
	}
}

// Regex 验证正则表达式
func Regex(pattern string) Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			if str, ok := value.(string); ok {
				matched, _ := regexp.MatchString(pattern, str)
				return matched
			}
			return false
		},
		message: "Does not match the required pattern",
	}
}

// In 验证是否在范围内
func In(list []interface{}) Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			for _, item := range list {
				if item == value {
					return true
				}
			}
			return false
		},
		message: "Value is not in the allowed list",
	}
}

// NotIn 验证是否不在范围内
func NotIn(list []interface{}) Rule {
	return &FuncRule{
		fn: func(value interface{}) bool {
			for _, item := range list {
				if item == value {
					return false
				}
			}
			return true
		},
		message: "Value is not allowed",
	}
}

// SanitizeInput 输入净化
func SanitizeInput(input string) string {
	// 移除危险的HTML标签
	re := regexp.MustCompile(`(?i)<(script|iframe|object|embed|form)[^>]*>.*?</\1>`)
	input = re.ReplaceAllString(input, "")

	// 移除危险的属性
	re = regexp.MustCompile(`(?i)(on\w+)=["']?([^"'>]*)["']?`)
	input = re.ReplaceAllString(input, "")

	// 转义HTML特殊字符
	input = html.EscapeString(input)

	// 移除可能的JavaScript伪协议
	input = strings.Replace(input, "javascript:", "", -1)
	input = strings.Replace(input, "vbscript:", "", -1)
	input = strings.Replace(input, "data:", "", -1)

	return input
}

// IsValidEmail 验证邮箱的便捷函数
func IsValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}

// IsValidMobile 验证手机号的便捷函数
func IsValidMobile(mobile string) bool {
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, mobile)
	return matched
}

// IsValidURL 验证URL的便捷函数
func IsValidURL(urlStr string) bool {
	_, err := url.ParseRequestURI(urlStr)
	return err == nil
}