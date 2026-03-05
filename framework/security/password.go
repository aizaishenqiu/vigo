package security

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// PasswordPolicy 密码策略配置
// 用于定义密码强度要求和验证规则
//
// 示例:
//
//	policy := &security.PasswordPolicy{
//	    MinLength:      8,
//	    MaxLength:      128,
//	    RequireUpper:   true,
//	    RequireLower:   true,
//	    RequireNumber:  true,
//	    RequireSpecial: false,
//	}
//
//	if err := policy.CheckPasswordStrength("MyPass123"); err != nil {
//	    // 密码强度不足
//	}
type PasswordPolicy struct {
	MinLength      int  // 最小长度
	MaxLength      int  // 最大长度（0 = 不限制）
	RequireUpper   bool // 要求大写字母
	RequireLower   bool // 要求小写字母
	RequireNumber  bool // 要求数字
	RequireSpecial bool // 要求特殊字符
}

// DefaultPasswordPolicy 默认密码策略
// - 最小长度：8 位
// - 最大长度：128 位
// - 要求大写字母
// - 要求小写字母
// - 要求数字
// - 不要求特殊字符
var DefaultPasswordPolicy = &PasswordPolicy{
	MinLength:      8,
	MaxLength:      128,
	RequireUpper:   true,
	RequireLower:   true,
	RequireNumber:  true,
	RequireSpecial: false,
}

// StrongPasswordPolicy 强密码策略
// - 最小长度：12 位
// - 最大长度：128 位
// - 要求大写字母
// - 要求小写字母
// - 要求数字
// - 要求特殊字符
var StrongPasswordPolicy = &PasswordPolicy{
	MinLength:      12,
	MaxLength:      128,
	RequireUpper:   true,
	RequireLower:   true,
	RequireNumber:  true,
	RequireSpecial: true,
}

// CheckPasswordStrength 检查密码强度
// 参数:
//   - password: 待检查的密码
//
// 返回:
//   - error: 密码强度不足的错误信息，通过则返回 nil
//
// 检查项目:
// 1. 最小长度
// 2. 最大长度（如果设置了）
// 3. 大写字母要求
// 4. 小写字母要求
// 5. 数字要求
// 6. 特殊字符要求
//
// 示例:
//
//	policy := security.DefaultPasswordPolicy
//	err := policy.CheckPasswordStrength("weak")
//	if err != nil {
//	    fmt.Println("密码强度不足:", err)
//	}
func (p *PasswordPolicy) CheckPasswordStrength(password string) error {
	if len(password) < p.MinLength {
		return fmt.Errorf("密码长度至少为 %d 位", p.MinLength)
	}

	if p.MaxLength > 0 && len(password) > p.MaxLength {
		return fmt.Errorf("密码长度不能超过 %d 位", p.MaxLength)
	}

	if p.RequireUpper && !hasUpper(password) {
		return errors.New("密码必须包含大写字母")
	}

	if p.RequireLower && !hasLower(password) {
		return errors.New("密码必须包含小写字母")
	}

	if p.RequireNumber && !hasNumber(password) {
		return errors.New("密码必须包含数字")
	}

	if p.RequireSpecial && !hasSpecial(password) {
		return errors.New("密码必须包含特殊字符 (!@#$%^&* 等)")
	}

	return nil
}

// 辅助函数：检查是否包含大写字母
func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// 辅助函数：检查是否包含小写字母
func hasLower(s string) bool {
	for _, r := range s {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

// 辅助函数：检查是否包含数字
func hasNumber(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// 辅助函数：检查是否包含特殊字符
func hasSpecial(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// HashPassword 密码加密（使用 bcrypt）
// 参数:
//   - password: 明文密码
//
// 返回:
//   - string: bcrypt 加密后的哈希值
//   - error: 错误信息
//
// 特性:
// - 自动加盐（每个密码独立盐）
// - 成本参数可配置（推荐 12）
// - 抗暴力破解（故意设计得慢）
// - 行业标准（OWASP 推荐）
//
// 示例:
//
//	hashed, err := security.HashPassword("myPassword123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("加密后的密码:", hashed)
func HashPassword(password string) (string, error) {
	// cost 参数：10-14，越大越安全但越慢
	// 推荐值：
	// - 10: 快速，适合开发环境
	// - 12: 平衡，适合生产环境（默认）
	// - 14: 非常安全，适合高安全场景
	cost := 12

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", errors.New("密码加密失败")
	}

	return string(hash), nil
}

// HashPasswordWithCost 使用指定成本参数加密密码
// 参数:
//   - password: 明文密码
//   - cost: 成本参数（10-14）
//
// 返回:
//   - string: bcrypt 加密后的哈希值
//   - error: 错误信息
//
// 成本参数说明:
// - 每增加 1，计算时间翻倍
// - 10: ~100ms
// - 12: ~400ms
// - 14: ~1600ms
//
// 示例:
//
//	hashed, err := security.HashPasswordWithCost("myPassword123", 14)
func HashPasswordWithCost(password string, cost int) (string, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", errors.New("密码加密失败")
	}

	return string(hash), nil
}

// VerifyPassword 密码验证
// 参数:
//   - password: 明文密码
//   - hash: bcrypt 加密后的哈希值
//
// 返回:
//   - bool: 验证是否通过
//
// 特性:
// - 自动提取盐值
// - 恒定时间比较（防止时序攻击）
// - 自动处理不同版本的 bcrypt 哈希
//
// 示例:
//
//	hashed, _ := security.HashPassword("myPassword123")
//	if security.VerifyPassword("myPassword123", hashed) {
//	    fmt.Println("密码正确")
//	} else {
//	    fmt.Println("密码错误")
//	}
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// NeedsRehash 检查是否需要重新加密
// 参数:
//   - hash: 现有的密码哈希
//   - targetCost: 目标成本参数
//
// 返回:
//   - bool: 是否需要重新加密
//
// 使用场景:
// - 升级成本参数时
// - 迁移旧系统密码时
// - 优化安全策略时
//
// 示例:
//
//	hashed := "$2a$10$..." // 旧的哈希（cost=10）
//	if security.NeedsRehash(hashed, 12) {
//	    // 需要重新加密（cost 从 10 升级到 12）
//	    newHashed, _ := security.HashPassword(password)
//	    // 保存新哈希
//	}
func NeedsRehash(hash string, targetCost int) bool {
	// bcrypt 哈希格式：$2a$cost$...
	// 解析当前 cost 参数
	if len(hash) < 7 {
		return true
	}

	// 简单检查：如果哈希不包含目标 cost，可能需要重新加密
	// 实际应该解析哈希字符串提取 cost
	expectedPrefix := fmt.Sprintf("$2a$%02d$", targetCost)
	return hash[:7] != expectedPrefix
}

// GenerateSecureToken 生成安全令牌
// 参数:
//   - length: 令牌长度（字节）
//
// 返回:
//   - string: Base64 编码的令牌
//   - error: 错误信息
//
// 使用场景:
// - API Token
// - 重置密码令牌
// - 会话令牌
// - 验证码
//
// 示例:
//
//	token, err := security.GenerateSecureToken(32)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("令牌:", token)
func GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		length = 32
	}

	token := make([]byte, length)
	if _, err := rand.Read(token); err != nil {
		return "", errors.New("无法生成安全令牌")
	}

	return base64Encode(token), nil
}

// ==================== 全局工具函数 ====================

// CheckPassword 使用默认策略检查密码强度
func CheckPassword(password string) error {
	return DefaultPasswordPolicy.CheckPasswordStrength(password)
}

// CheckStrongPassword 使用强密码策略检查密码强度
func CheckStrongPassword(password string) error {
	return StrongPasswordPolicy.CheckPasswordStrength(password)
}

// Hash 密码加密（全局函数）
func Hash(password string) (string, error) {
	return HashPassword(password)
}

// Verify 密码验证（全局函数）
func Verify(password, hash string) bool {
	return VerifyPassword(password, hash)
}

// base64Encode Base64 编码
func base64Encode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}
