// Package config 提供安全配置管理
package config

// SecurityConfig 安全配置
type SecurityConfig struct {
	JWT                      JWTConfig      `yaml:"jwt"`                        // JWT 认证配置
	Session                  SessionConfig  `yaml:"session"`                    // Session 会话配置
	Password                 PasswordConfig `yaml:"password"`                   // 密码策略配置
	CORS                     CORSConfig     `yaml:"cors"`                       // CORS 跨域配置
	DoS                      DoSConfig      `yaml:"dos"`                        // DoS 防护配置
	EnableSecurityMiddleware bool           `yaml:"enable_security_middleware"` // 是否启用全局安全中间件
	EnableCSRFProtection     bool           `yaml:"enable_csrf_protection"`     // 是否启用 CSRF 保护
	EnableRateLimit          bool           `yaml:"enable_rate_limit"`          // 是否启用速率限制
	RateLimit                int            `yaml:"rate_limit"`                 // 速率限制：每秒请求数
	EnableCorsDomainCheck    bool           `yaml:"enable_cors_domain_check"`   // 是否启用 CORS 域名验证
	AllowedDomains           []string       `yaml:"allowed_domains"`            // 允许的域名列表
	IPWhitelist              []string       `yaml:"ip_whitelist"`               // IP 白名单
	IPBlacklist              []string       `yaml:"ip_blacklist"`               // IP 黑名单
}

// JWTConfig JWT 认证配置
type JWTConfig struct {
	Secret string `yaml:"secret"` // JWT 密钥
	Expire int    `yaml:"expire"` // Token 过期时间（秒）
	Issuer string `yaml:"issuer"` // Token 签发者
}

// SessionConfig Session 会话配置
type SessionConfig struct {
	Lifetime int    `yaml:"lifetime"` // 会话生命周期（秒）
	Secure   bool   `yaml:"secure"`   // 是否仅通过 HTTPS 传输
	HttpOnly bool   `yaml:"httponly"` // 是否禁止 JavaScript 访问 Cookie
	SameSite string `yaml:"samesite"` // SameSite 策略：strict | lax | none
}

// PasswordConfig 密码策略配置
type PasswordConfig struct {
	MinLength        int  `yaml:"min_length"`        // 最小密码长度
	RequireUppercase bool `yaml:"require_uppercase"` // 是否必须包含大写字母
	RequireLowercase bool `yaml:"require_lowercase"` // 是否必须包含小写字母
	RequireNumber    bool `yaml:"require_number"`    // 是否必须包含数字
	RequireSpecial   bool `yaml:"require_special"`   // 是否必须包含特殊字符
}

// CORSConfig CORS 跨域配置
type CORSConfig struct {
	AllowOrigins []string `yaml:"allow_origins"` // 允许的来源列表
	AllowMethods []string `yaml:"allow_methods"` // 允许的 HTTP 方法
	AllowHeaders []string `yaml:"allow_headers"` // 允许的请求头
	MaxAge       int      `yaml:"max_age"`       // 预检请求缓存时间（秒）
}

// DoSConfig DoS 防护 / 限流配置
type DoSConfig struct {
	Enable  bool     `yaml:"enable"`   // 是否启用限流
	Limit   int      `yaml:"limit"`    // 每个 IP 的请求限制
	Window  int      `yaml:"window"`   // 限流时间窗口（秒）
	BlackIP []string `yaml:"black_ip"` // IP 黑名单列表
}
