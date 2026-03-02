package request

import (
	"html"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"vigo/framework/mvc"
)

// Request 请求数据处理器（简化版 - 只负责数据获取和净化）
type Request struct {
	ctx      *mvc.Context
	safeData map[string]interface{}
	mu       sync.RWMutex
}

// New 创建请求处理器
func New(ctx *mvc.Context) *Request {
	return &Request{
		ctx:      ctx,
		safeData: make(map[string]interface{}),
	}
}

// sanitize 净化字符串（移除 HTML 标签、转义特殊字符）
func (r *Request) sanitize(value string) string {
	// 移除 HTML 标签
	value = strings.TrimSpace(value)
	value = html.EscapeString(value)
	return value
}

// Get 获取字符串参数（已净化）
func (r *Request) Get(key string, defaultValue string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 优先从已解析的数据中获取
	if val, ok := r.safeData[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	// 从请求中获取
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	return r.sanitize(value)
}

// GetString 获取字符串（已净化）
func (r *Request) GetString(key string, defaultValue string) string {
	return r.Get(key, defaultValue)
}

// GetInt 获取整数参数
func (r *Request) GetInt(key string, defaultValue int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// GetInt64 获取 int64 参数
func (r *Request) GetInt64(key string, defaultValue int64) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// GetFloat 获取浮点数参数
func (r *Request) GetFloat(key string, defaultValue float64) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}

	return floatVal
}

// GetBool 获取布尔参数
func (r *Request) GetBool(key string, defaultValue bool) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	// 支持多种布尔表示
	value = strings.ToLower(value)
	return value == "1" || value == "true" || value == "on" || value == "yes"
}

// GetStringSlice 获取字符串切片参数
func (r *Request) GetStringSlice(key string, defaultValue []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	values := r.ctx.Request.Form[key]
	if len(values) == 0 {
		return defaultValue
	}

	result := make([]string, len(values))
	for i, v := range values {
		result[i] = r.sanitize(v)
	}

	return result
}

// GetMap 获取所有参数（已净化）
func (r *Request) GetMap() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 解析表单
	r.ctx.Request.ParseForm()

	result := make(map[string]interface{})
	for key, values := range r.ctx.Request.Form {
		if len(values) == 1 {
			result[key] = r.sanitize(values[0])
		} else {
			sanitized := make([]string, len(values))
			for i, v := range values {
				sanitized[i] = r.sanitize(v)
			}
			result[key] = sanitized
		}
	}

	return result
}

// Set 设置参数值（用于内部使用）
func (r *Request) Set(key string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.safeData[key] = value
}

// Context 获取上下文
func (r *Request) Context() *mvc.Context {
	return r.ctx
}

// Query 获取 URL 查询参数（已净化）
func (r *Request) Query(key string, defaultValue string) string {
	value := r.ctx.Request.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	return r.sanitize(value)
}

// QueryInt 获取查询参数（整数）
func (r *Request) QueryInt(key string, defaultValue int) int {
	value := r.ctx.Request.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// Post 获取 POST 参数（已净化）
func (r *Request) Post(key string, defaultValue string) string {
	value := r.ctx.Request.PostFormValue(key)
	if value == "" {
		return defaultValue
	}
	return r.sanitize(value)
}

// PostInt 获取 POST 参数（整数）
func (r *Request) PostInt(key string, defaultValue int) int {
	value := r.ctx.Request.PostFormValue(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// Put 获取 PUT 参数（已净化）
func (r *Request) Put(key string, defaultValue string) string {
	r.ctx.Request.ParseForm()
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}
	return r.sanitize(value)
}

// PutInt 获取 PUT 参数（整数）
func (r *Request) PutInt(key string, defaultValue int) int {
	r.ctx.Request.ParseForm()
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// Delete 获取 DELETE 参数（已净化）
func (r *Request) Delete(key string, defaultValue string) string {
	r.ctx.Request.ParseForm()
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}
	return r.sanitize(value)
}

// DeleteInt 获取 DELETE 参数（整数）
func (r *Request) DeleteInt(key string, defaultValue int) int {
	r.ctx.Request.ParseForm()
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// Patch 获取 PATCH 参数（已净化）
func (r *Request) Patch(key string, defaultValue string) string {
	r.ctx.Request.ParseForm()
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}
	return r.sanitize(value)
}

// PatchInt 获取 PATCH 参数（整数）
func (r *Request) PatchInt(key string, defaultValue int) int {
	r.ctx.Request.ParseForm()
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// JSON 获取 JSON 请求体参数
func (r *Request) JSON(key string, defaultValue string) string {
	r.ctx.Request.ParseForm()
	value := r.ctx.Request.FormValue(key)
	if value == "" {
		return defaultValue
	}
	return r.sanitize(value)
}

// JSONMap 获取 JSON 请求体（Map 形式）
func (r *Request) JSONMap() map[string]interface{} {
	return r.GetMap()
}

// Method 获取请求方法
func (r *Request) Method() string {
	return r.ctx.Request.Method
}

// IsGet 判断是否为 GET 请求
func (r *Request) IsGet() bool {
	return r.ctx.Request.Method == "GET"
}

// IsPost 判断是否为 POST 请求
func (r *Request) IsPost() bool {
	return r.ctx.Request.Method == "POST"
}

// IsPut 判断是否为 PUT 请求
func (r *Request) IsPut() bool {
	return r.ctx.Request.Method == "PUT"
}

// IsDelete 判断是否为 DELETE 请求
func (r *Request) IsDelete() bool {
	return r.ctx.Request.Method == "DELETE"
}

// IsPatch 判断是否为 PATCH 请求
func (r *Request) IsPatch() bool {
	return r.ctx.Request.Method == "PATCH"
}

// IsAjax 判断是否为 Ajax 请求
func (r *Request) IsAjax() bool {
	return r.ctx.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// IP 获取客户端 IP
func (r *Request) IP() string {
	ip := r.ctx.Request.Header.Get("X-Forwarded-For")
	if ip != "" {
		return ip
	}
	ip = r.ctx.Request.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}
	return strings.Split(r.ctx.Request.RemoteAddr, ":")[0]
}

// UserAgent 获取 User-Agent
func (r *Request) UserAgent() string {
	return r.ctx.Request.Header.Get("User-Agent")
}

// Referer 获取 Referer
func (r *Request) Referer() string {
	return r.ctx.Request.Referer()
}

// Path 获取请求路径
func (r *Request) Path() string {
	return r.ctx.Request.URL.Path
}

// FullPath 获取完整请求路径（包括查询参数）
func (r *Request) FullPath() string {
	return r.ctx.Request.URL.String()
}

// Header 获取 Header 参数
func (r *Request) Header(key string, defaultValue string) string {
	value := r.ctx.Request.Header.Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Cookie 获取 Cookie 参数
func (r *Request) Cookie(name string, defaultValue string) string {
	cookie, err := r.ctx.Request.Cookie(name)
	if err != nil {
		return defaultValue
	}
	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return defaultValue
	}
	return value
}

// All 获取所有请求参数（包括 query 和 post）
func (r *Request) All() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make(map[string]interface{})

	// 解析所有参数
	r.ctx.Request.ParseForm()

	for key, values := range r.ctx.Request.Form {
		if len(values) == 1 {
			result[key] = r.sanitize(values[0])
		} else {
			sanitized := make([]string, len(values))
			for i, v := range values {
				sanitized[i] = r.sanitize(v)
			}
			result[key] = sanitized
		}
	}

	return result
}

// Only 获取指定的参数
func (r *Request) Only(keys ...string) map[string]interface{} {
	all := r.All()
	result := make(map[string]interface{})

	for _, key := range keys {
		if val, ok := all[key]; ok {
			result[key] = val
		}
	}

	return result
}

// Except 获取除了指定参数外的所有参数
func (r *Request) Except(keys ...string) map[string]interface{} {
	all := r.All()
	exclude := make(map[string]bool)

	for _, key := range keys {
		exclude[key] = true
	}

	result := make(map[string]interface{})
	for key, value := range all {
		if !exclude[key] {
			result[key] = value
		}
	}

	return result
}

// Has 检查参数是否存在
func (r *Request) Has(key string) bool {
	r.ctx.Request.ParseForm()
	_, ok := r.ctx.Request.Form[key]
	return ok
}

// Empty 检查参数是否为空
func (r *Request) Empty(key string) bool {
	value := r.Get(key, "")
	return value == ""
}

// Fill 填充默认值（如果参数不存在）
func (r *Request) Fill(key string, defaultValue string) string {
	if r.Has(key) {
		return r.Get(key, "")
	}
	r.Set(key, defaultValue)
	return defaultValue
}
