package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String 日志级别字符串表示
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	default:
		return "unknown"
	}
}

// LogEntry 日志条目
type LogEntry struct {
	Level     string                 `json:"level"`
	Timestamp string                 `json:"time"`
	Message   string                 `json:"msg"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
}

// StructuredLogger 结构化日志记录器
type StructuredLogger struct {
	logger *log.Logger
	level  LogLevel
}

// NewStructuredLogger 创建结构化日志记录器
func NewStructuredLogger(level LogLevel) *StructuredLogger {
	return &StructuredLogger{
		logger: log.New(os.Stdout, "", 0),
		level:  level,
	}
}

// logWithFields 记录带字段的日志
func (sl *StructuredLogger) logWithFields(level LogLevel, msg string, fields map[string]interface{}) {
	if level < sl.level {
		return
	}

	entry := LogEntry{
		Level:     level.String(),
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   msg,
		Fields:    fields,
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		sl.logger.Printf("Failed to marshal log entry: %v", err)
		return
	}

	sl.logger.Println(string(jsonData))
}

// Debug 记录调试日志
func (sl *StructuredLogger) Debug(msg string, fields map[string]interface{}) {
	sl.logWithFields(DEBUG, msg, fields)
}

// Info 记录信息日志
func (sl *StructuredLogger) Info(msg string, fields map[string]interface{}) {
	sl.logWithFields(INFO, msg, fields)
}

// Warn 记录警告日志
func (sl *StructuredLogger) Warn(msg string, fields map[string]interface{}) {
	sl.logWithFields(WARN, msg, fields)
}

// Error 记录错误日志
func (sl *StructuredLogger) Error(msg string, fields map[string]interface{}) {
	sl.logWithFields(ERROR, msg, fields)
}

// WithFields 创建带字段的子记录器
func (sl *StructuredLogger) WithFields(fields map[string]interface{}) *StructuredLogger {
	return &StructuredLogger{
		logger: sl.logger,
		level:  sl.level,
	}
}

// TraceContext 分布式追踪上下文
type TraceContext struct {
	TraceID  string
	SpanID   string
	ParentID string
}

// GenerateID 生成唯一ID
func GenerateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// StartTrace 开始新的追踪
func StartTrace() *TraceContext {
	return &TraceContext{
		TraceID: GenerateID(),
		SpanID:  GenerateID(),
	}
}

// ChildSpan 创建子跨度
func (tc *TraceContext) ChildSpan() *TraceContext {
	return &TraceContext{
		TraceID:  tc.TraceID,
		SpanID:   GenerateID(),
		ParentID: tc.SpanID,
	}
}

// ToFields 将追踪上下文转换为日志字段
func (tc *TraceContext) ToFields() map[string]interface{} {
	fields := make(map[string]interface{})
	if tc.TraceID != "" {
		fields["trace_id"] = tc.TraceID
	}
	if tc.SpanID != "" {
		fields["span_id"] = tc.SpanID
	}
	if tc.ParentID != "" {
		fields["parent_id"] = tc.ParentID
	}
	return fields
}

// GlobalLogger 全局结构化日志记录器
var GlobalLogger *StructuredLogger

// InitGlobalLogger 初始化全局日志记录器
func InitGlobalLogger(level LogLevel) {
	GlobalLogger = NewStructuredLogger(level)
}

// GetGlobalLogger 获取全局日志记录器
func GetGlobalLogger() *StructuredLogger {
	if GlobalLogger == nil {
		InitGlobalLogger(INFO)
	}
	return GlobalLogger
}