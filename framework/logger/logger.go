// package logger 已统一到 framework/log 包
// 此文件保留向后兼容，使用 framework/log 的实现
package logger

import (
	flog "vigo/framework/log"
)

// SimpleLogger 向后兼容的简单日志（已迁移到 framework/log.FileLogger）
type SimpleLogger = flog.FileLogger

// New 创建日志实例（向后兼容）
func New() *SimpleLogger {
	return flog.NewFileLogger("runtime/log")
}
