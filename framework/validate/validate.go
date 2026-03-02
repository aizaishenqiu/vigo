package validate

import (
	"fmt"
	"regexp"
)

// Validator 简单验证器
type Validator struct {
	Errors map[string]string
}

func New() *Validator {
	return &Validator{
		Errors: make(map[string]string),
	}
}

// Required 验证必填
func (v *Validator) Required(field string, value string) *Validator {
	if value == "" {
		v.Errors[field] = fmt.Sprintf("%s is required", field)
	}
	return v
}

// Email 验证邮箱
func (v *Validator) Email(field string, value string) *Validator {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, value)
	if !matched {
		v.Errors[field] = fmt.Sprintf("%s must be a valid email", field)
	}
	return v
}

// Mobile 验证手机号
func (v *Validator) Mobile(field string, value string) *Validator {
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, value)
	if !matched {
		v.Errors[field] = fmt.Sprintf("%s must be a valid mobile number", field)
	}
	return v
}

// Min 验证最小长度
func (v *Validator) Min(field string, value string, min int) *Validator {
	if len(value) < min {
		v.Errors[field] = fmt.Sprintf("%s must be at least %d characters", field, min)
	}
	return v
}

// Max 验证最大长度
func (v *Validator) Max(field string, value string, max int) *Validator {
	if len(value) > max {
		v.Errors[field] = fmt.Sprintf("%s must be at most %d characters", field, max)
	}
	return v
}

// In 验证是否在范围内
func (v *Validator) In(field string, value interface{}, list []interface{}) *Validator {
	found := false
	for _, item := range list {
		if item == value {
			found = true
			break
		}
	}
	if !found {
		v.Errors[field] = fmt.Sprintf("%s is not in the allowed list", field)
	}
	return v
}

// IsValid 是否验证通过
func (v *Validator) IsValid() bool {
	return len(v.Errors) == 0
}
