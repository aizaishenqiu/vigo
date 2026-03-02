package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// AESGCM AES-256-GCM 加密器
// 提供安全的认证加密，支持前后端统一加密方案
type AESGCM struct {
	key []byte
}

// NewAESGCM 创建 AES-256-GCM 加密器
// 参数:
//   - key: 加密密钥（任意长度，会自动转换为 32 字节）
// 返回:
//   - *AESGCM: 加密器实例
//   - error: 错误信息
//
// 示例:
//
//	crypto, err := NewAESGCM("my-secret-key-0123456789abcdef")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	encrypted, err := crypto.Encrypt("敏感数据")
//	decrypted, err := crypto.Decrypt(encrypted)
func NewAESGCM(key string) (*AESGCM, error) {
	// 使用 SHA-256 将任意长度的密钥转换为 32 字节
	hash := sha256.Sum256([]byte(key))
	
	return &AESGCM{
		key: hash[:],
	}, nil
}

// Encrypt 加密数据
// 参数:
//   - plaintext: 明文数据
// 返回:
//   - string: Base64 编码的密文
//   - error: 错误信息
//
// 加密流程:
// 1. 生成随机 nonce (12 字节)
// 2. 使用 AES-256-GCM 模式加密
// 3. 组合 nonce + 密文
// 4. Base64 编码输出
//
// 输出格式: Base64(nonce[12] + ciphertext)
func (a *AESGCM) Encrypt(plaintext string) (string, error) {
	// 创建 AES cipher
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}

	// 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", errors.New("无法生成随机数")
	}

	// 加密数据（包含认证标签）
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	
	// Base64 编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
// 参数:
//   - ciphertext: Base64 编码的密文
// 返回:
//   - string: 明文数据
//   - error: 错误信息
//
// 解密流程:
// 1. Base64 解码
// 2. 分离 nonce 和密文
// 3. 使用 AES-256-GCM 模式解密
// 4. 验证认证标签（自动）
//
// 安全性:
// - GCM 模式提供完整性保护
// - 自动验证认证标签
// - 防止篡改和伪造
func (a *AESGCM) Decrypt(ciphertext string) (string, error) {
	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", errors.New("无效的 Base64 编码")
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}

	// 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 检查数据长度
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("密文太短")
	}

	// 分离 nonce 和密文
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// 解密并验证
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", errors.New("解密失败：密文可能被篡改")
	}

	return string(plaintext), nil
}

// EncryptBytes 加密字节数据
// 参数:
//   - plaintext: 明文字节
// 返回:
//   - []byte: 加密后的字节（包含 nonce）
//   - error: 错误信息
func (a *AESGCM) EncryptBytes(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New("无法生成随机数")
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptBytes 解密字节数据
// 参数:
//   - ciphertext: 加密后的字节（包含 nonce）
// 返回:
//   - []byte: 明文字节
//   - error: 错误信息
func (a *AESGCM) DecryptBytes(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("密文太短")
	}

	nonce, ciphertextBytes := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, errors.New("解密失败")
	}

	return plaintext, nil
}

// GenerateKey 生成随机密钥
// 参数:
//   - length: 密钥长度（字节），推荐 32
// 返回:
//   - string: Base64 编码的密钥
//   - error: 错误信息
//
// 示例:
//
//	key, err := GenerateKey(32)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("保存密钥:", key)
func GenerateKey(length int) (string, error) {
	if length <= 0 {
		length = 32 // 默认 256 位
	}
	
	key := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", errors.New("无法生成随机密钥")
	}
	
	return base64.StdEncoding.EncodeToString(key), nil
}

// ==================== 全局实例 ====================

// GlobalCrypto 全局加密器实例
var GlobalCrypto *AESGCM

// InitGlobalCrypto 初始化全局加密器
// 参数:
//   - key: 加密密钥
// 返回:
//   - error: 错误信息
//
// 示例:
//
//	err := InitGlobalCrypto("my-secret-key")
//	if err != nil {
//	    log.Fatal(err)
//	}
func InitGlobalCrypto(key string) error {
	crypto, err := NewAESGCM(key)
	if err != nil {
		return err
	}
	GlobalCrypto = crypto
	return nil
}

// EncryptGlobal 使用全局加密器加密
func EncryptGlobal(plaintext string) (string, error) {
	if GlobalCrypto == nil {
		return "", errors.New("全局加密器未初始化")
	}
	return GlobalCrypto.Encrypt(plaintext)
}

// DecryptGlobal 使用全局加密器解密
func DecryptGlobal(ciphertext string) (string, error) {
	if GlobalCrypto == nil {
		return "", errors.New("全局加密器未初始化")
	}
	return GlobalCrypto.Decrypt(ciphertext)
}
