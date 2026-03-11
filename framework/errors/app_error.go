package errors

import (
	"errors"
	"fmt"
	"runtime"
	"time"
)

// AppError 应用错误类型
type AppError struct {
	Code     int       `json:"code"`
	Message  string    `json:"message"`
	Details  string    `json:"details,omitempty"`
	Cause    error     `json:"cause,omitempty"`
	Time     time.Time `json:"time"`
	Stack    string    `json:"stack,omitempty"`
	Severity string    `json:"severity,omitempty"` // debug, info, warn, error, fatal
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 实现错误包装接口
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewError 创建新的应用错误
func NewError(code int, message string) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// NewErrorWithDetails 创建带有详细信息的应用错误
func NewErrorWithDetails(code int, message, details string) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		Details:  details,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// WrapError 包装现有错误
func WrapError(err error, message string) *AppError {
	if err == nil {
		return nil
	}

	if appErr, ok := err.(*AppError); ok {
		// 如果已经是AppError，扩展它
		newErr := &AppError{
			Code:     appErr.Code,
			Message:  message,
			Details:  appErr.Details,
			Cause:    err,
			Time:     time.Now(),
			Severity: appErr.Severity,
			Stack:    getStackTrace(2),
		}
		return newErr
	}

	return &AppError{
		Code:     500,
		Message:  message,
		Cause:    err,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// WrapErrorWithCode 包装错误并指定代码
func WrapErrorWithCode(err error, code int, message string) *AppError {
	if err == nil {
		return nil
	}

	return &AppError{
		Code:     code,
		Message:  message,
		Cause:    err,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// IsErrorType 检查错误类型
func IsErrorType(err error, code int) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// getStackTrace 获取堆栈跟踪
func getStackTrace(skip int) string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stack string
	for {
		frame, more := frames.Next()
		stack += fmt.Sprintf("\n\t%s:%d %s", frame.File, frame.Line, frame.Function)
		if !more {
			break
		}
	}
	return stack
}

// BusinessError 业务错误
func BusinessError(message string) *AppError {
	return &AppError{
		Code:     400,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// ValidationError 验证错误
func ValidationError(message string) *AppError {
	return &AppError{
		Code:     422,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// NotFoundError 资源未找到错误
func NotFoundError(message string) *AppError {
	return &AppError{
		Code:     404,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// UnauthorizedError 未授权错误
func UnauthorizedError(message string) *AppError {
	return &AppError{
		Code:     401,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// ForbiddenError 禁止访问错误
func ForbiddenError(message string) *AppError {
	return &AppError{
		Code:     403,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// InternalError 内部服务器错误
func InternalError(message string) *AppError {
	return &AppError{
		Code:     500,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// TimeoutError 超时错误
func TimeoutError(message string) *AppError {
	return &AppError{
		Code:     408,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}

// RateLimitError 速率限制错误
func RateLimitError(message string) *AppError {
	return &AppError{
		Code:     429,
		Message:  message,
		Time:     time.Now(),
		Severity: "error",
		Stack:    getStackTrace(2),
	}
}