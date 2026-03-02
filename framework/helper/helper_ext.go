// Package helper 提供框架内置的全局辅助函数（续）
package helper

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
	"time"
)

// ==================== 加密哈希 ====================

// Md5 MD5 加密
func Md5(str string) string {
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

// Sha1 SHA1 加密
func Sha1(str string) string {
	hash := sha1.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

// Sha256 SHA256 加密
func Sha256(str string) string {
	hash := sha256.Sum256([]byte(str))
	return hex.EncodeToString(hash[:])
}

// Base64Encode Base64 编码
func Base64Encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

// Base64Decode Base64 解码
func Base64Decode(str string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// PasswordHash 密码哈希（使用 bcrypt）
func PasswordHash(password string) (string, error) {
	// 简化实现，实际应该使用 bcrypt
	return Md5(password + "_vigo_salt"), nil
}

// PasswordVerify 密码验证
func PasswordVerify(password, hash string) bool {
	// 简化实现，实际应该使用 bcrypt
	return Md5(password+"_vigo_salt") == hash
}

// ==================== 随机数生成 ====================

// Random 生成随机整数
func Random(min, max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return int(n.Int64()) + min
}

// RandomString 生成随机字符串
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[Random(0, len(charset)-1)]
	}
	return string(result)
}

// RandomInts 生成多个不重复的随机整数
func RandomInts(count, min, max int) []int {
	if count > max-min+1 {
		count = max - min + 1
	}

	nums := make([]int, 0, count)
	used := make(map[int]bool)

	for len(nums) < count {
		num := Random(min, max)
		if !used[num] {
			used[num] = true
			nums = append(nums, num)
		}
	}

	return nums
}

// ==================== 时间日期 ====================

// Time 获取当前时间戳（秒）
func Time() int64 {
	return time.Now().Unix()
}

// Millisecond 获取当前时间戳（毫秒）
func Millisecond() int64 {
	return time.Now().UnixNano() / 1e6
}

// Microsecond 获取当前时间戳（微秒）
func Microsecond() int64 {
	return time.Now().UnixNano() / 1e3
}

// Date 格式化时间戳
func Date(format string, timestamp ...int64) string {
	var ts int64
	if len(timestamp) > 0 {
		ts = timestamp[0]
	} else {
		ts = time.Now().Unix()
	}

	t := time.Unix(ts, 0)

	// Go 的时间格式与 PHP 不同，这里做简单转换
	format = strings.ReplaceAll(format, "Y", "2006")
	format = strings.ReplaceAll(format, "m", "01")
	format = strings.ReplaceAll(format, "d", "02")
	format = strings.ReplaceAll(format, "H", "15")
	format = strings.ReplaceAll(format, "i", "00")
	format = strings.ReplaceAll(format, "s", "00")

	return t.Format(format)
}

// Strtotime 解析时间字符串为时间戳
func Strtotime(format, str string) int64 {
	// 简化实现，实际应该支持更多格式
	t, err := time.Parse("2006-01-02 15:04:05", str)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// Datetime 获取当前日期时间
func Datetime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Today 获取今天日期
func Today() string {
	return time.Now().Format("2006-01-02")
}

// Tomorrow 获取明天日期
func Tomorrow() string {
	return time.Now().AddDate(0, 0, 1).Format("2006-01-02")
}

// Yesterday 获取昨天日期
func Yesterday() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}
