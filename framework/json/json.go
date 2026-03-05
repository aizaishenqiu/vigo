package json

import (
	jsoniter "github.com/json-iterator/go"
)

var (
	// ConfigDefault 默认配置（兼容标准库）
	ConfigDefault = jsoniter.ConfigCompatibleWithStandardLibrary
	
	// ConfigFast 快速配置（不转义 HTML，性能更好）
	ConfigFast = jsoniter.Config{
		EscapeHTML:              false,
		SortMapKeys:             false,
		ValidateJsonRawMessage:  false,
		UseNumber:               false,
		DisallowUnknownFields:   false,
	}.Froze()
)

// Marshal 序列化 JSON（默认配置）
func Marshal(v interface{}) ([]byte, error) {
	return ConfigDefault.Marshal(v)
}

// Unmarshal 反序列化 JSON（默认配置）
func Unmarshal(data []byte, v interface{}) error {
	return ConfigDefault.Unmarshal(data, v)
}

// MarshalToString 序列化为字符串
func MarshalToString(v interface{}) (string, error) {
	return ConfigDefault.MarshalToString(v)
}

// MarshalIndent 格式化序列化
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return ConfigDefault.MarshalIndent(v, prefix, indent)
}

// Get 快速获取 JSON 字段（无需定义结构体）
func Get(data []byte, path ...interface{}) jsoniter.Any {
	return ConfigDefault.Get(data, path...)
}

// NewDecoder 创建解码器
func NewDecoder(iter *jsoniter.Iterator) *jsoniter.Iterator {
	return iter
}

// NewEncoder 创建编码器
func NewEncoder(stream *jsoniter.Stream) *jsoniter.Stream {
	return stream
}

// RegisterExtension 注册自定义扩展
func RegisterExtension(extension jsoniter.Extension) {
	ConfigDefault.RegisterExtension(extension)
}
