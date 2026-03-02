// Package common 用户自定义公共方法示例
// 
// 本文件演示如何在 app/common 中添加自定义公共方法
// 这些方法可以在全局任意位置调用
//
// 使用方法:
//   import "vigo/app/common"
//   
//   // 调用自定义方法
//   result := common.FormatUsername("admin")
//   price := common.FormatPrice(99.99)

package common

import (
	"fmt"
	"vigo/framework/helper"
)

// FormatUsername 格式化用户名（示例）
// 将用户名转换为首字母大写的格式
func FormatUsername(username string) string {
	return helper.Ucfirst(helper.Trim(username))
}

// FormatPrice 格式化价格（示例）
// 将价格格式化为保留两位小数的字符串
func FormatPrice(price float64) string {
	return fmt.Sprintf("¥%.2f", price)
}

// GenerateOrderNo 生成订单号（示例）
// 格式：年月日时分秒 + 6 位随机数
func GenerateOrderNo() string {
	return helper.Datetime() + helper.RandomString(6)
}

// MaskPhone 手机号脱敏（示例）
// 将手机号中间 4 位替换为 *
func MaskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}

// IsAdult 判断是否成年（示例）
func IsAdult(age int) bool {
	return age >= 18
}

// ConcatStrings 连接多个字符串（示例）
func ConcatStrings(strs ...string) string {
	result := ""
	for _, str := range strs {
		result += str
	}
	return result
}

// 你可以在这里添加更多自定义的公共方法...
// 建议按功能分类到不同的文件中，例如：
// - string.go: 字符串处理函数
// - number.go: 数字处理函数
// - file.go: 文件处理函数
// - validate.go: 验证函数
// - format.go: 格式化函数
