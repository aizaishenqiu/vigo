package config

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// ConfigValidator 配置验证器
type ConfigValidator struct {
	rules map[string]ValidationRule
}

// ValidationRule 验证规则
type ValidationRule struct {
	Field     string
	Required  bool
	Type      string
	Min       interface{}
	Max       interface{}
	Pattern   string
	Default   interface{}
	Validator func(interface{}) error
}

// NewConfigValidator 创建配置验证器
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		rules: make(map[string]ValidationRule),
	}
}

// AddRule 添加验证规则
func (v *ConfigValidator) AddRule(field string, rule ValidationRule) *ConfigValidator {
	v.rules[field] = rule
	return v
}

// Validate 验证配置
func (v *ConfigValidator) Validate(config *AppConfig) error {
	var errs []error

	// 验证数据库配置
	if err := v.validateDatabase(&config.Database); err != nil {
		errs = append(errs, fmt.Errorf("database: %w", err))
	}

	// 验证多数据库配置
	for name, db := range config.Databases {
		if err := v.validateDBConfig(name, &db); err != nil {
			errs = append(errs, fmt.Errorf("databases.%s: %w", name, err))
		}
	}

	// 验证 Redis 配置
	if err := v.validateRedis(&config.Redis); err != nil {
		errs = append(errs, fmt.Errorf("redis: %w", err))
	}

	// 验证 Nacos 配置
	if err := v.validateNacos(&config.Nacos); err != nil {
		errs = append(errs, fmt.Errorf("nacos: %w", err))
	}

	// 验证 RabbitMQ 配置
	if err := v.validateRabbitMQ(&config.RabbitMQ); err != nil {
		errs = append(errs, fmt.Errorf("rabbitmq: %w", err))
	}

	// 验证 gRPC 配置
	if err := v.validateGRPC(&config.GRPC); err != nil {
		errs = append(errs, fmt.Errorf("grpc: %w", err))
	}

	// 验证安全配置
	if err := v.validateSecurity(&config.Security); err != nil {
		errs = append(errs, fmt.Errorf("security: %w", err))
	}

	// 验证应用配置
	if err := v.validateBase(&config.App); err != nil {
		errs = append(errs, fmt.Errorf("app: %w", err))
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateDatabase 验证数据库配置
func (v *ConfigValidator) validateDatabase(db *DatabaseConfig) error {
	var errs []error

	if db.Driver == "" {
		errs = append(errs, errors.New("driver 不能为空"))
	}

	if db.Host == "" {
		errs = append(errs, errors.New("host 不能为空"))
	}

	if db.User == "" {
		errs = append(errs, errors.New("user 不能为空"))
	}

	if db.Port <= 0 {
		errs = append(errs, errors.New("port 必须大于 0"))
	}

	if db.Name == "" {
		errs = append(errs, errors.New("name 不能为空"))
	}

	if db.MaxOpenConns <= 0 {
		db.MaxOpenConns = 100
	}

	if db.MaxIdleConns <= 0 {
		db.MaxIdleConns = 10
	}

	if db.ConnMaxLifetime <= 0 {
		db.ConnMaxLifetime = 3600
	}

	if db.ConnMaxIdleTime <= 0 {
		db.ConnMaxIdleTime = 300
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateDBConfig 验证数据库配置
func (v *ConfigValidator) validateDBConfig(name string, db *DBConfig) error {
	var errs []error

	if db.Driver == "" {
		errs = append(errs, errors.New("driver 不能为空"))
	}

	if db.Host == "" {
		errs = append(errs, errors.New("host 不能为空"))
	}

	if db.User == "" {
		errs = append(errs, errors.New("user 不能为空"))
	}

	if db.Port <= 0 {
		errs = append(errs, errors.New("port 必须大于 0"))
	}

	if db.Name == "" {
		errs = append(errs, errors.New("name 不能为空"))
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateRedis 验证 Redis 配置
func (v *ConfigValidator) validateRedis(redis *RedisConfig) error {
	var errs []error

	if redis.Host == "" {
		errs = append(errs, errors.New("host 不能为空"))
	}

	if redis.Port <= 0 {
		errs = append(errs, errors.New("port 必须大于 0"))
	}

	if redis.PoolSize <= 0 {
		redis.PoolSize = 100
	}

	if redis.MinIdleConns <= 0 {
		redis.MinIdleConns = 10
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateNacos 验证 Nacos 配置
func (v *ConfigValidator) validateNacos(nacos *NacosConfig) error {
	var errs []error

	if nacos.IpAddr == "" {
		errs = append(errs, errors.New("host 不能为空"))
	}

	if nacos.Port <= 0 {
		errs = append(errs, errors.New("port 必须大于 0"))
	}

	if nacos.NamespaceId == "" {
		nacos.NamespaceId = "public"
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateRabbitMQ 验证 RabbitMQ 配置
func (v *ConfigValidator) validateRabbitMQ(rabbitmq *RabbitMQConfig) error {
	var errs []error

	if rabbitmq.Host == "" {
		errs = append(errs, errors.New("host 不能为空"))
	}

	if rabbitmq.Port <= 0 {
		errs = append(errs, errors.New("port 必须大于 0"))
	}

	if rabbitmq.Vhost == "" {
		rabbitmq.Vhost = "/"
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateGRPC 验证 gRPC 配置
func (v *ConfigValidator) validateGRPC(grpc *GRPCConfig) error {
	var errs []error

	if grpc.Enabled {
		if grpc.Port <= 0 {
			errs = append(errs, errors.New("port 必须大于 0"))
		}
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateSecurity 验证安全配置
func (v *ConfigValidator) validateSecurity(sec *SecurityConfig) error {
	var errs []error

	if len(sec.JWT.Secret) < 32 {
		errs = append(errs, errors.New("jwt.secret 长度必须 >= 32"))
	}

	if sec.Session.Lifetime <= 0 {
		sec.Session.Lifetime = 7200
	}

	if sec.Password.MinLength <= 0 {
		sec.Password.MinLength = 6
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// validateBase 验证应用基础配置
func (v *ConfigValidator) validateBase(base *BaseConfig) error {
	var errs []error

	if base.Name == "" {
		base.Name = "vigo"
	}

	if base.Mode == "" {
		base.Mode = "dev"
	}

	if base.Port <= 0 {
		base.Port = 8080
	}

	if len(errs) > 0 {
		return errors.New(v.formatErrors(errs))
	}

	return nil
}

// formatErrors 格式化错误信息
func (v *ConfigValidator) formatErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	var lines []string
	for _, err := range errs {
		lines = append(lines, "- "+err.Error())
	}

	return strings.Join(lines, "\n")
}

// ValidateConfig 验证配置（便捷函数）
func ValidateConfig(config *AppConfig) error {
	validator := NewConfigValidator()
	return validator.Validate(config)
}

// ValidateEmail 验证邮箱格式
func ValidateEmail(email string) bool {
	pattern := `^[\w-]+(\.[\w-]+)*@[\w-]+(\.[\w-]+)+$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}

// ValidatePhone 验证手机号格式（中国）
func ValidatePhone(phone string) bool {
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, phone)
	return matched
}

// ValidateURL 验证 URL 格式
func ValidateURL(urlStr string) bool {
	pattern := `^(https?|ftp)://[^\s/$.?#].[^\s]*$`
	matched, _ := regexp.MatchString(pattern, urlStr)
	return matched
}

// LogValidationErrors 记录验证错误日志
func LogValidationErrors(err error) {
	if err == nil {
		return
	}

	log.Printf("[Config] 配置验证失败：%v\n", err)
}
