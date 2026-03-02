package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

// StatusResponseWriter 自定义ResponseWriter，支持获取状态码
// 注意：这个类型是为了兼容其他可能需要的场景，Context中已经使用了mvc.StatusWriter
type StatusResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

// NewStatusResponseWriter 创建新的状态响应写入器
func NewStatusResponseWriter(w http.ResponseWriter) *StatusResponseWriter {
	return &StatusResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // 默认状态码
	}
}

// WriteHeader 写入响应头
func (sw *StatusResponseWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.statusCode = code
		sw.wroteHeader = true
		sw.ResponseWriter.WriteHeader(code)
	}
}

// Status 获取状态码
func (sw *StatusResponseWriter) Status() int {
	return sw.statusCode
}

// Header 获取响应头
func (sw *StatusResponseWriter) Header() http.Header {
	return sw.ResponseWriter.Header()
}

// Write 写入响应体
func (sw *StatusResponseWriter) Write(data []byte) (int, error) {
	if !sw.wroteHeader {
		sw.WriteHeader(sw.statusCode)
	}
	return sw.ResponseWriter.Write(data)
}

// Flush 刷新响应
func (sw *StatusResponseWriter) Flush() {
	if flusher, ok := sw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack 实现 Hijacker 接口
func (sw *StatusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := sw.ResponseWriter.(http.Hijacker); ok {
		conn, rw, err := hijacker.Hijack()
		return conn, rw, err
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not support Hijacker interface")
}
