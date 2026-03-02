package mvc

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

// StatusWriter 带状态码的ResponseWriter
type StatusWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

// NewStatusWriter 创建带状态码的ResponseWriter
func NewStatusWriter(w http.ResponseWriter) *StatusWriter {
	return &StatusWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader 写入响应头
func (sw *StatusWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.statusCode = code
		sw.wroteHeader = true
		sw.ResponseWriter.WriteHeader(code)
	}
}

// Status 获取状态码
func (sw *StatusWriter) Status() int {
	return sw.statusCode
}

// Header 获取响应头
func (sw *StatusWriter) Header() http.Header {
	return sw.ResponseWriter.Header()
}

// Write 写入响应体
func (sw *StatusWriter) Write(data []byte) (int, error) {
	if !sw.wroteHeader {
		sw.WriteHeader(sw.statusCode)
	}
	return sw.ResponseWriter.Write(data)
}

// Flush 刷新响应
func (sw *StatusWriter) Flush() {
	if flusher, ok := sw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack 实现 Hijacker 接口
func (sw *StatusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := sw.ResponseWriter.(http.Hijacker); ok {
		conn, rw, err := hijacker.Hijack()
		return conn, rw, err
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not support Hijacker interface")
}
