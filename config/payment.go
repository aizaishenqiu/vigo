// Package config 提供支付和 OAuth 配置
package config

// PaymentConfig 支付配置
type PaymentConfig struct {
	Alipay    AlipayConfig    `yaml:"alipay"`    // 支付宝配置
	WechatPay WechatPayConfig `yaml:"wechat_pay"` // 微信支付配置
}

// AlipayConfig 支付宝配置
type AlipayConfig struct {
	Enabled         bool   `yaml:"enabled"`         // 是否启用
	AppID           string `yaml:"app_id"`          // 应用 ID
	AppPrivateKey   string `yaml:"app_private_key"` // 应用私钥
	AlipayPublicKey string `yaml:"alipay_public_key"` // 支付宝公钥
	NotifyURL       string `yaml:"notify_url"`      // 回调地址
	ReturnURL       string `yaml:"return_url"`      // 返回地址
	IsProvider      bool   `yaml:"is_provider"`     // 服务商模式
	ProviderPID     string `yaml:"provider_pid"`    // 服务商 PID
}

// WechatPayConfig 微信支付配置
type WechatPayConfig struct {
	Enabled         bool   `yaml:"enabled"`         // 是否启用
	MchID           string `yaml:"mch_id"`          // 商户号
	APIKey          string `yaml:"api_key"`         // API 密钥
	AppID           string `yaml:"app_id"`          // 应用 ID
	NotifyURL       string `yaml:"notify_url"`      // 回调地址
	IsProvider      bool   `yaml:"is_provider"`     // 服务商模式
	ProviderMchID   string `yaml:"provider_mch_id"` // 服务商商户号
}

// OAuthConfig 第三方登录配置
type OAuthConfig struct {
	QQ      QQOAuthConfig      `yaml:"qq"`      // QQ 登录
	Wechat  WechatOAuthConfig  `yaml:"wechat"`  // 微信登录
	Alipay  AlipayOAuthConfig  `yaml:"alipay"`  // 支付宝登录
}

// QQOAuthConfig QQ 登录配置
type QQOAuthConfig struct {
	Enabled     bool   `yaml:"enabled"`     // 是否启用
	AppID       string `yaml:"app_id"`      // App ID
	AppKey      string `yaml:"app_key"`     // App Key
	RedirectURI string `yaml:"redirect_uri"` // 回调地址
}

// WechatOAuthConfig 微信登录配置
type WechatOAuthConfig struct {
	Enabled     bool   `yaml:"enabled"`     // 是否启用
	AppID       string `yaml:"app_id"`      // App ID
	AppSecret   string `yaml:"app_secret"`  // App Secret
	RedirectURI string `yaml:"redirect_uri"` // 回调地址
}

// AlipayOAuthConfig 支付宝登录配置
type AlipayOAuthConfig struct {
	Enabled         bool   `yaml:"enabled"`         // 是否启用
	AppID           string `yaml:"app_id"`          // App ID
	AppPrivateKey   string `yaml:"app_private_key"` // 应用私钥
	AlipayPublicKey string `yaml:"alipay_public_key"` // 支付宝公钥
	RedirectURI     string `yaml:"redirect_uri"`    // 回调地址
}
