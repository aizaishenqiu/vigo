// Package helper 提供框架内置的全局辅助函数
// 用户可以直接调用这些函数，无需导入复杂的依赖
// 类似 ThinkPHP 的 helper 函数和 Laravel 的全局辅助函数
//
// 使用示例:
//
//	import "vigo/framework/helper"
//
//	// 字符串处理
//	name := helper.Str("hello")
//	length := helper.Strlen("你好")
//
//	// 数组处理
//	exists := helper.InArray("value", array)
//
//	// 加密哈希
//	hash := helper.Md5("password")
//
//	// 时间日期
//	timestamp := helper.Time()
//	dateStr := helper.Date("Y-m-d H:i:s")
//
//	// 文件操作
//	exists := helper.FileExists("path/to/file")
//
//	// 调试
//	helper.Dump(variable)
package helper

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// ==================== 字符串处理 ====================

// Str 将任意类型转换为字符串
func Str(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Int 将任意类型转换为 int
func Int(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case string:
		i, _ := strconv.Atoi(val)
		return i
	case float64:
		return int(val)
	default:
		return 0
	}
}

// Int64 将任意类型转换为 int64
func Int64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// Float 将任意类型转换为 float64
func Float(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// Bool 将任意类型转换为 bool
func Bool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1" || val == "yes"
	case int:
		return val != 0
	case int64:
		return val != 0
	default:
		return false
	}
}

// Substr 截取字符串（支持中文）
func Substr(str string, start, length int) string {
	if start < 0 || start >= len(str) {
		return ""
	}

	end := start + length
	if end > len(str) {
		end = len(str)
	}

	return str[start:end]
}

// Strlen 获取字符串长度（支持中文）
func Strlen(str string) int {
	return len([]rune(str))
}

// Strpos 查找字符串首次出现的位置
func Strpos(haystack, needle string) int {
	return strings.Index(haystack, needle)
}

// StrReplace 替换字符串
func StrReplace(search, replace, subject string) string {
	return strings.ReplaceAll(subject, search, replace)
}

// Explode 字符串分割
func Explode(delimiter, str string) []string {
	return strings.Split(str, delimiter)
}

// Implode 数组转字符串
func Implode(glue string, pieces []string) string {
	return strings.Join(pieces, glue)
}

// Ucfirst 首字母大写
func Ucfirst(str string) string {
	if len(str) == 0 {
		return str
	}
	return strings.ToUpper(str[:1]) + str[1:]
}

// Lcfirst 首字母小写
func Lcfirst(str string) string {
	if len(str) == 0 {
		return str
	}
	return strings.ToLower(str[:1]) + str[1:]
}

// Ucwords 每个单词首字母大写
func Ucwords(str string) string {
	return strings.Title(str)
}

// Trim 去除字符串首尾空白字符
func Trim(str string) string {
	return strings.TrimSpace(str)
}

// Htmlentities HTML 转义
func Htmlentities(str string) string {
	return html.EscapeString(str)
}

// HtmlEntityDecode HTML 转义反转义
func HtmlEntityDecode(str string) string {
	return html.UnescapeString(str)
}

// Nl2br 换行符转<br>
func Nl2br(str string) string {
	return strings.ReplaceAll(str, "\n", "<br>")
}

// StripTags 去除 HTML 标签
func StripTags(str string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(str, "")
}
