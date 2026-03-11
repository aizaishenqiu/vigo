package contract

import "time"

// Logger 日志接口
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
}

// Cache 缓存接口（与 framework/cache 对齐）
type Cache interface {
	Get(key string) interface{}
	Set(key string, val interface{}, ttl time.Duration) error
	Delete(key string) error
	Has(key string) bool
}

// Queue 队列接口
type Queue interface {
	Push(topic string, message []byte) error
	Subscribe(topic string, handler func([]byte) error)
}

// Repository 数据仓储接口（便于测试替换）
type Repository interface {
	Find(id interface{}) (map[string]interface{}, error)
	FindAll(conditions map[string]interface{}) ([]map[string]interface{}, error)
	Create(data map[string]interface{}) (int64, error)
	Update(id interface{}, data map[string]interface{}) (int64, error)
	Delete(id interface{}) (int64, error)
}
