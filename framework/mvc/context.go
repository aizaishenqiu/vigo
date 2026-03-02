package mvc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"vigo/framework/view"
)

// bufferPool JSON 响应缓冲池，复用 bytes.Buffer 减少内存分配
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// HandlerFunc 处理函数类型
type HandlerFunc func(c *Context)

// Context 请求上下文
// 封装了 HTTP 请求和响应，提供便捷的数据获取和响应方法
// 支持中间件链式调用、参数绑定、视图渲染等功能
type Context struct {
	Request    *http.Request          // HTTP 请求对象
	Writer     http.ResponseWriter    // HTTP 响应写入器
	Params     map[string]string      // 路由参数（如 :id）
	handlers   []HandlerFunc          // 中间件链
	index      int                    // 当前执行的中间件索引
	ViewEngine view.Engine            // 视图引擎
	keys       map[string]interface{} // 请求级别的键值存储
}

// abortIndex 中止索引，用于停止中间件链执行
const abortIndex int = math.MaxInt16 / 2

// Reset 重置上下文，用于对象池复用
// 彻底释放引用，避免长期运行内存增长
// 参数：
//   - w: HTTP 响应写入器
//   - r: HTTP 请求对象
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	// 使用带状态的 ResponseWriter 来追踪响应状态
	c.Writer = NewStatusWriter(w)
	c.Request = r
	c.index = -1
	c.handlers = nil
	c.keys = nil
	c.Params = make(map[string]string) // 新建 map，旧 map 交由 GC 回收
	// ViewEngine 由 Router 注入全局共享实例
	if c.ViewEngine == nil {
		c.ViewEngine = view.NewTemplateEngine("app/view")
	}
}

// NewContext 创建上下文（保留兼容）
// 参数：
//   - w: HTTP 响应写入器
//   - r: HTTP 请求对象
//
// 返回：
//   - *Context: 新创建的上下文对象
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Request:    r,
		Writer:     NewStatusWriter(w),
		Params:     make(map[string]string),
		index:      -1,
		ViewEngine: view.NewTemplateEngine("app/view"),
	}
}

// ==================== 中间件控制 ====================

// Next 执行下一个中间件
// 在中间件中调用此方法将控制权传递给下一个处理器
func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort 停止执行后续中间件
// 调用后，后续的中间件将不会被执行
func (c *Context) Abort() {
	c.index = abortIndex
}

// IsAborted 检查是否已中止
// 返回：
//   - bool: true 表示已中止，false 表示未中止
func (c *Context) IsAborted() bool {
	return c.index >= abortIndex
}

// ==================== 请求数据获取 ====================

// Input 获取请求参数（查询参数或表单参数）
// 优先从 URL 查询参数获取，其次从表单数据获取
// 参数：
//   - key: 参数名
//
// 返回：
//   - string: 参数值，不存在则返回空字符串
func (c *Context) Input(key string) string {
	return c.Request.FormValue(key)
}

// InputDefault 获取参数，如果为空则返回默认值
// 参数：
//   - key: 参数名
//   - defaultVal: 默认值
//
// 返回：
//   - string: 参数值或默认值
func (c *Context) InputDefault(key, defaultVal string) string {
	v := c.Request.FormValue(key)
	if v == "" {
		return defaultVal
	}
	return v
}

// InputInt 获取整型参数
// 参数：
//   - key: 参数名
//   - defaultVal: 默认值
//
// 返回：
//   - int: 参数值，解析失败返回默认值
func (c *Context) InputInt(key string, defaultVal int) int {
	v := c.Request.FormValue(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}

// InputFloat 获取浮点型参数
// 参数：
//   - key: 参数名
//   - defaultVal: 默认值
//
// 返回：
//   - float64: 参数值，解析失败返回默认值
func (c *Context) InputFloat(key string, defaultVal float64) float64 {
	v := c.Request.FormValue(key)
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// InputBool 获取布尔型参数
// 参数：
//   - key: 参数名
//
// 返回：
//   - bool: 参数值，"1", "true", "on", "yes" 返回 true
func (c *Context) InputBool(key string) bool {
	v := strings.ToLower(c.Request.FormValue(key))
	return v == "1" || v == "true" || v == "on" || v == "yes"
}

// Query 获取查询参数
// 参数：
//   - key: 参数名
//
// 返回：
//   - string: 参数值
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// QueryDefault 获取查询参数，如果为空则返回默认值
// 参数：
//   - key: 参数名
//   - defaultVal: 默认值
//
// 返回：
//   - string: 参数值或默认值
func (c *Context) QueryDefault(key, defaultVal string) string {
	v := c.Request.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	return v
}

// FormFile 获取上传文件
// 参数：
//   - key: 文件字段名
//
// 返回：
//   - *multipart.FileHeader: 文件头信息
//   - error: 错误信息
func (c *Context) FormFile(key string) (*multipart.FileHeader, error) {
	_, fileHeader, err := c.Request.FormFile(key)
	return fileHeader, err
}

// Cookie 获取 Cookie 值
// 参数：
//   - key: Cookie 名
//
// 返回：
//   - string: Cookie 值
//   - error: 错误信息
func (c *Context) Cookie(key string) (string, error) {
	cookie, err := c.Request.Cookie(key)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// GetHeader 获取请求头
// 参数：
//   - key: 请求头名
//
// 返回：
//   - string: 请求头值
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// GetClientIP 获取客户端 IP
// 按优先级依次尝试：X-Forwarded-For > X-Real-IP > RemoteAddr
// 返回：
//   - string: 客户端 IP 地址
func (c *Context) GetClientIP() string {
	// 优先从 X-Forwarded-For 获取
	xForwardedFor := c.Request.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ip := strings.Split(xForwardedFor, ",")[0]
		return strings.TrimSpace(ip)
	}

	// 从 X-Real-IP 获取
	xRealIP := c.Request.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// 从 RemoteAddr 获取
	host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	return host
}

// ==================== Context 存储 ====================

// Set 在请求上下文中存储键值对
// 用于在中间件之间传递数据
// 参数：
//   - key: 键名
//   - value: 值
func (c *Context) Set(key string, value interface{}) {
	if c.keys == nil {
		c.keys = make(map[string]interface{})
	}
	c.keys[key] = value
}

// Get 从请求上下文获取值
// 参数：
//   - key: 键名
//
// 返回：
//   - interface{}: 值
//   - bool: 是否存在
func (c *Context) Get(key string) (interface{}, bool) {
	if c.keys == nil {
		return nil, false
	}
	val, ok := c.keys[key]
	return val, ok
}

// MustGet 从请求上下文获取值（不存在则 panic）
// 参数：
//   - key: 键名
//
// 返回：
//   - interface{}: 值
func (c *Context) MustGet(key string) interface{} {
	val, ok := c.Get(key)
	if !ok {
		panic(fmt.Sprintf("Key %q 不存在于 Context 中", key))
	}
	return val
}

// ==================== 请求超时控制 ====================

// WithTimeout 为当前请求设置超时
// 参数：
//   - timeout: 超时时间
//
// 返回：
//   - context.Context: 带超时的上下文
//   - context.CancelFunc: 取消函数
func (c *Context) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), timeout)
}

// Deadline 获取请求的 context
// 返回：
//   - context.Context: 请求上下文
func (c *Context) Ctx() context.Context {
	return c.Request.Context()
}

// ==================== 响应输出 ====================

// Json 返回 JSON 响应
// 使用 sync.Pool 复用缓冲区，减少内存分配
// 参数：
//   - code: HTTP 状态码
//   - data: 响应数据
func (c *Context) Json(code int, data interface{}) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(data); err != nil {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(http.StatusInternalServerError)
		c.Writer.Write([]byte(`{"code":500,"msg":"JSON encode error"}`))
		return
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	c.Writer.Write(buf.Bytes())
}

// HTML 返回 HTML 响应
// 参数：
//   - code: HTTP 状态码
//   - name: 模板文件名
//   - data: 模板数据
func (c *Context) HTML(code int, name string, data interface{}) {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(code)
	if err := c.ViewEngine.Render(c.Writer, name, data); err != nil {
		http.Error(c.Writer, fmt.Sprintf("Template execute error: %v", err), http.StatusInternalServerError)
	}
}

// String 返回纯文本响应
// 参数：
//   - code: HTTP 状态码
//   - format: 格式化字符串
//   - values: 格式化参数
func (c *Context) String(code int, format string, values ...interface{}) {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	fmt.Fprintf(c.Writer, format, values...)
}

// Redirect 重定向
// 参数：
//   - code: HTTP 状态码（如 301, 302）
//   - url: 目标 URL
func (c *Context) Redirect(code int, url string) {
	http.Redirect(c.Writer, c.Request, url, code)
}

// Success 成功响应
// 返回标准成功格式：{"code": 0, "msg": "success", "data": data}
// 参数：
//   - data: 响应数据
func (c *Context) Success(data interface{}) {
	c.Json(http.StatusOK, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// Error 失败响应
// 返回标准错误格式：{"code": code, "msg": msg, "data": null}
// 参数：
//   - code: HTTP 状态码
//   - msg: 错误信息
func (c *Context) Error(code int, msg string) {
	c.Json(code, map[string]interface{}{
		"code": code,
		"msg":  msg,
		"data": nil,
	})
}

// Fail 结构化失败响应（支持自定义业务码）
// 参数：
//   - code: HTTP 状态码
//   - businessCode: 业务错误码
//   - msg: 错误信息
//   - data: 附加数据
func (c *Context) Fail(code int, businessCode int, msg string, data interface{}) {
	c.Json(code, map[string]interface{}{
		"code": businessCode,
		"msg":  msg,
		"data": data,
	})
}

// Status 获取响应状态码
// 返回：
//   - int: HTTP 状态码
func (c *Context) Status() int {
	if sw, ok := c.Writer.(*StatusWriter); ok {
		return sw.Status()
	}
	// 如果不是 StatusWriter，返回默认状态码
	return http.StatusOK
}

// Path 获取请求路径
// 返回：
//   - string: 请求路径
func (c *Context) Path() string {
	return c.Request.URL.Path
}

// IsAjax 判断是否为 AJAX 请求
// 返回：
//   - bool: true 表示是 AJAX 请求
func (c *Context) IsAjax() bool {
	return c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// ContentType 获取请求的 Content-Type
// 返回：
//   - string: Content-Type（不含 charset 等参数）
func (c *Context) ContentType() string {
	ct := c.Request.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = ct[:idx]
	}
	return strings.TrimSpace(ct)
}

// SetHeader 设置响应头
// 参数：
//   - key: 响应头名
//   - value: 响应头值
func (c *Context) SetHeader(key, value string) {
	c.Writer.Header().Set(key, value)
}

// Param 获取路由参数
// 参数：
//   - key: 参数名
//
// 返回：
//   - string: 参数值
func (c *Context) Param(key string) string {
	return c.Params[key]
}

// BindJSON 绑定 JSON 请求体到结构体
// 参数：
//   - obj: 目标结构体指针
//
// 返回：
//   - error: 解析错误
func (c *Context) BindJSON(obj interface{}) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields() // 不允许未知字段
	return decoder.Decode(obj)
}
