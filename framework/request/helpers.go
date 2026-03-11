package request

import (
	"vigo/framework/mvc"
)

// R 获取 Request 处理器（简化版）
// 用法：req := R(c)
//
//	name := req.Get("name", "")
func R(c *mvc.Context) *Request {
	return New(c)
}

// Input 获取输入参数（已净化）
// 用法：name := Input(c, "name", "默认值")
func Input(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.Get(key, defaultValue)
}

// InputInt 获取整数输入参数
// 用法：age := InputInt(c, "age", 0)
func InputInt(c *mvc.Context, key string, defaultValue int) int {
	req := R(c)
	return req.GetInt(key, defaultValue)
}

// InputInt64 获取 int64 输入参数
// 用法：id := InputInt64(c, "id", 0)
func InputInt64(c *mvc.Context, key string, defaultValue int64) int64 {
	req := R(c)
	return req.GetInt64(key, defaultValue)
}

// InputFloat 获取浮点数输入参数
// 用法：price := InputFloat(c, "price", 0.0)
func InputFloat(c *mvc.Context, key string, defaultValue float64) float64 {
	req := R(c)
	return req.GetFloat(key, defaultValue)
}

// InputBool 获取布尔输入参数
// 用法：enabled := InputBool(c, "enabled", false)
func InputBool(c *mvc.Context, key string, defaultValue bool) bool {
	req := R(c)
	return req.GetBool(key, defaultValue)
}

// InputSlice 获取字符串切片输入参数
// 用法：tags := InputSlice(c, "tags", []string{})
func InputSlice(c *mvc.Context, key string, defaultValue []string) []string {
	req := R(c)
	return req.GetStringSlice(key, defaultValue)
}

// Query 获取 URL 查询参数（已净化）
// 用法：keyword := Query(c, "keyword", "")
func Query(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.Query(key, defaultValue)
}

// QueryInt 获取 URL 查询参数（整数）
// 用法：page := QueryInt(c, "page", 1)
func QueryInt(c *mvc.Context, key string, defaultValue int) int {
	req := R(c)
	return req.QueryInt(key, defaultValue)
}

// Post 获取 POST 参数（已净化）
// 用法：content := Post(c, "content", "")
func Post(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.Post(key, defaultValue)
}

// PostInt 获取 POST 参数（整数）
// 用法：count := PostInt(c, "count", 0)
func PostInt(c *mvc.Context, key string, defaultValue int) int {
	req := R(c)
	return req.PostInt(key, defaultValue)
}

// Put 获取 PUT 参数（已净化）
// 用法：name := Put(c, "name", "")
func Put(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.Put(key, defaultValue)
}

// PutInt 获取 PUT 参数（整数）
// 用法：id := PutInt(c, "id", 0)
func PutInt(c *mvc.Context, key string, defaultValue int) int {
	req := R(c)
	return req.PutInt(key, defaultValue)
}

// Delete 获取 DELETE 参数（已净化）
// 用法：id := Delete(c, "id", "")
func Delete(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.Delete(key, defaultValue)
}

// DeleteInt 获取 DELETE 参数（整数）
// 用法：id := DeleteInt(c, "id", 0)
func DeleteInt(c *mvc.Context, key string, defaultValue int) int {
	req := R(c)
	return req.DeleteInt(key, defaultValue)
}

// Patch 获取 PATCH 参数（已净化）
// 用法：name := Patch(c, "name", "")
func Patch(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.Patch(key, defaultValue)
}

// PatchInt 获取 PATCH 参数（整数）
// 用法：id := PatchInt(c, "id", 0)
func PatchInt(c *mvc.Context, key string, defaultValue int) int {
	req := R(c)
	return req.PatchInt(key, defaultValue)
}

// JSON 获取 JSON 请求体参数
// 用法：name := JSON(c, "name", "")
func JSON(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.JSON(key, defaultValue)
}

// JSONMap 获取 JSON 请求体（Map 形式）
// 用法：data := JSONMap(c)
func JSONMap(c *mvc.Context) map[string]interface{} {
	req := R(c)
	return req.JSONMap()
}

// Method 获取请求方法
// 用法：method := Method(c)
func Method(c *mvc.Context) string {
	req := R(c)
	return req.Method()
}

// IsGet 判断是否为 GET 请求
// 用法：if IsGet(c) { ... }
func IsGet(c *mvc.Context) bool {
	req := R(c)
	return req.IsGet()
}

// IsPost 判断是否为 POST 请求
// 用法：if IsPost(c) { ... }
func IsPost(c *mvc.Context) bool {
	req := R(c)
	return req.IsPost()
}

// IsPut 判断是否为 PUT 请求
// 用法：if IsPut(c) { ... }
func IsPut(c *mvc.Context) bool {
	req := R(c)
	return req.IsPut()
}

// IsDelete 判断是否为 DELETE 请求
// 用法：if IsDelete(c) { ... }
func IsDelete(c *mvc.Context) bool {
	req := R(c)
	return req.IsDelete()
}

// IsPatch 判断是否为 PATCH 请求
// 用法：if IsPatch(c) { ... }
func IsPatch(c *mvc.Context) bool {
	req := R(c)
	return req.IsPatch()
}

// IsAjax 判断是否为 Ajax 请求
// 用法：if IsAjax(c) { ... }
func IsAjax(c *mvc.Context) bool {
	req := R(c)
	return req.IsAjax()
}

// IP 获取客户端 IP
// 用法：ip := IP(c)
func IP(c *mvc.Context) string {
	req := R(c)
	return req.IP()
}

// UserAgent 获取 User-Agent
// 用法：ua := UserAgent(c)
func UserAgent(c *mvc.Context) string {
	req := R(c)
	return req.UserAgent()
}

// Referer 获取 Referer
// 用法：ref := Referer(c)
func Referer(c *mvc.Context) string {
	req := R(c)
	return req.Referer()
}

// Path 获取请求路径
// 用法：path := Path(c)
func Path(c *mvc.Context) string {
	req := R(c)
	return req.Path()
}

// FullPath 获取完整请求路径
// 用法：fullPath := FullPath(c)
func FullPath(c *mvc.Context) string {
	req := R(c)
	return req.FullPath()
}

// Header 获取 Header 参数
// 用法：token := Header(c, "Authorization", "")
func Header(c *mvc.Context, key string, defaultValue string) string {
	req := R(c)
	return req.Header(key, defaultValue)
}

// Cookie 获取 Cookie 参数
// 用法：sessionID := Cookie(c, "session_id", "")
func Cookie(c *mvc.Context, name string, defaultValue string) string {
	req := R(c)
	return req.Cookie(name, defaultValue)
}

// All 获取所有请求参数
// 用法：data := All(c)
func All(c *mvc.Context) map[string]interface{} {
	req := R(c)
	return req.All()
}

// Only 获取指定的请求参数
// 用法：data := Only(c, "name", "email", "age")
func Only(c *mvc.Context, keys ...string) map[string]interface{} {
	req := R(c)
	return req.Only(keys...)
}

// Except 获取除了指定参数外的所有参数
// 用法：data := Except(c, "password", "token")
func Except(c *mvc.Context, keys ...string) map[string]interface{} {
	req := R(c)
	return req.Except(keys...)
}

// Has 检查请求参数是否存在
// 用法：if Has(c, "username") { ... }
func Has(c *mvc.Context, key string) bool {
	req := R(c)
	return req.Has(key)
}

// GetMap 获取所有参数（Map 形式）
// 用法：data := GetMap(c)
func GetMap(c *mvc.Context) map[string]interface{} {
	req := R(c)
	return req.GetMap()
}
