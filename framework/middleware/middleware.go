package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"vigo/framework/mvc"

	"golang.org/x/crypto/bcrypt"
)

// ==================== 内部路径优化 ====================

// isInternalPath 判断是否为内部高频路径（压测/监控等），跳过重型中间件
func isInternalPath(path string) bool {
	switch path {
	case "/health", "/health/full",
		"/benchmark/stats", "/benchmark/qps", "/benchmark/ws",
		"/benchmark/start", "/benchmark/start-http",
		"/benchmark/stop", "/benchmark/reset",
		"/performance/run", "/performance/clear",
		"/performance/results", "/performance/system",
		"/performance/database", "/monitor/data",
		"/rabbitmq/status", "/rabbitmq/queues", "/rabbitmq/exchanges",
		"/rabbitmq/queue/create", "/rabbitmq/queue/delete", "/rabbitmq/queue/purge",
		"/rabbitmq/exchange/create", "/rabbitmq/exchange/delete", "/rabbitmq/publish",
		"/nacos/status", "/nacos/config", "/nacos/config/publish", "/nacos/config/delete",
		"/nacos/services", "/nacos/instances", "/nacos/service/register":
		return true
	}
	return false
}

// ==================== SQL 注入 / 安全威胁检测 ====================
// 注意：此中间件作为兜底保护，即使开发者忘记使用 request 模块也能拦截攻击

// 安全检测正则：只匹配明显攻击模式，避免误拦截正常输入（如搜索词 "select"）
// 要求多词组合或危险模式：如 select.*from、union select、;--、路径穿越、危险函数等
var securityPattern = regexp.MustCompile(
	`(?i)` +
		// 完整 SQL 语句特征（多词）
		`(\bselect\s+.*\s+from\b` +
		`|\bunion\s+(all\s+)?select\b` +
		`|\binsert\s+into\s+` +
		`|\b(drop|truncate|alter|create|delete)\s+(table|database|index|view|from)\b` +
		`|\bexec\s*\(` +
		`|\b(load_file|into\s+outfile|into\s+dumpfile|benchmark|sleep)\s*\(` +
		// SQL 注释注入
		`|;\s*--\s*$` +
		`|/\*.*\*/` +
		// 命令/协议注入
		`|\b(xp_cmdshell|shell_exec|eval\s*\(|assert\s*\(|system\s*\()\b` +
		// 路径穿越
		`|\.\./|\.\.\\\\` +
		// 敏感路径
		`|etc/passwd|etc/shadow|windows\\\\system32|web\.config` +
		`|ldap://|xpath\s*\(|javascript\s*:|vbscript\s*:`,
)

// SecurityMiddleware 统一安全验证中间件（兜底保护）
// 注意：即使开发者忘记使用 request 模块，此中间件也能拦截大部分攻击
// 建议：在配置文件中启用，作为全局中间件
func SecurityMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 验证查询参数
		query := c.Request.URL.RawQuery
		if query != "" && securityPattern.MatchString(query) {
			log.Printf("[Security] 安全威胁检测 (query): IP=%s Path=%s Query=%s",
				GetClientIP(c.Request), c.Request.URL.Path, query)
			c.Error(http.StatusBadRequest, "安全验证失败：检测到恶意请求")
			c.Abort()
			return
		}

		// 验证 POST/PUT/PATCH 数据
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			_ = c.Request.ParseForm()
			for _, values := range c.Request.PostForm {
				for _, v := range values {
					if securityPattern.MatchString(v) {
						log.Printf("[Security] 安全威胁检测 (body): IP=%s Path=%s",
							GetClientIP(c.Request), c.Request.URL.Path)
						c.Error(http.StatusBadRequest, "安全验证失败：检测到恶意请求")
						c.Abort()
						return
					}
				}
			}
		}

		// 验证 Headers
		userAgent := c.Request.Header.Get("User-Agent")
		if userAgent != "" && securityPattern.MatchString(userAgent) {
			log.Printf("[Security] 安全威胁检测 (header): IP=%s Path=%s",
				GetClientIP(c.Request), c.Request.URL.Path)
			c.Error(http.StatusBadRequest, "安全验证失败：检测到恶意请求")
			c.Abort()
			return
		}

		c.Next()
	}
}

// SQLInjection SQL 注入及常见漏洞检测中间件（已废弃，请使用 SecurityMiddleware）
// Deprecated: 使用 SecurityMiddleware 代替
func SQLInjection() mvc.HandlerFunc {
	return SecurityMiddleware()
}

// ==================== 安全响应头 ====================

// SecurityHeaders 设置安全响应头中间件
func SecurityHeaders() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		h := c.Writer.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline';")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}

// ==================== CSRF 防护（增强版） ====================

// CSRF 防护中间件（双重提交 Cookie 模式 + SameSite + HttpOnly）
func CSRF() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		var csrfToken string
		cookie, err := c.Request.Cookie("csrf_token")
		secureCookie := false
		if os.Getenv("APP_ENV") == "prod" || os.Getenv("HTTPS") == "1" {
			secureCookie = true
		}
		if err != nil || cookie.Value == "" {
			b := make([]byte, 32)
			rand.Read(b)
			csrfToken = hex.EncodeToString(b)
			http.SetCookie(c.Writer, &http.Cookie{
				Name:     "csrf_token",
				Value:    csrfToken,
				Path:     "/",
				MaxAge:   3600,
				HttpOnly: false,
				Secure:   secureCookie,
				SameSite: http.SameSiteStrictMode,
			})
		} else {
			csrfToken = cookie.Value
		}

		// 安全方法不检查 Token
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" || c.Request.Method == "TRACE" {
			c.Next()
			return
		}

		// 从多个来源获取 Token
		reqToken := c.Request.Header.Get("X-CSRF-Token")
		if reqToken == "" {
			reqToken = c.Request.Header.Get("X-XSRF-Token")
		}
		if reqToken == "" {
			reqToken = c.Request.FormValue("csrf_token")
		}

		if reqToken == "" || reqToken != csrfToken {
			log.Printf("[Security] CSRF Token 不匹配：IP=%s", GetClientIP(c.Request))
			c.Error(http.StatusForbidden, "CSRF Token mismatch")
			c.Abort()
			return
		}
		c.Next()
	}
}

// ==================== 请求体大小限制 ====================

// RequestSizeLimit 请求体大小限制中间件
func RequestSizeLimit(maxSize int64) mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if c.Request.ContentLength > maxSize {
			c.Error(http.StatusRequestEntityTooLarge, "Request Entity Too Large")
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// ==================== 密码哈希（bcrypt 安全算法） ====================

// PasswordHash 使用 bcrypt 生成密码哈希（替代不安全的 SHA256）
func PasswordHash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// PasswordVerifyHash 使用 bcrypt 验证密码与哈希是否匹配
func PasswordVerifyHash(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// PasswordStrength 验证密码强度（至少 8 位，含大小写和数字）
func PasswordStrength(password string) bool {
	if len(password) < 8 {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

// ==================== XSS 过滤 ====================

// XSS 跨站脚本过滤中间件
func XSS() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			_ = c.Request.ParseForm()
			for k, v := range c.Request.PostForm {
				for i, s := range v {
					c.Request.PostForm[k][i] = html.EscapeString(s)
				}
			}
		}
		query := c.Request.URL.Query()
		for k, v := range query {
			for i, s := range v {
				query[k][i] = html.EscapeString(s)
			}
		}
		c.Request.URL.RawQuery = query.Encode()
		c.Next()
	}
}

// ==================== 速率限制（分片锁 + 自动清理） ====================

// rateLimitShard 分片结构（减少锁争用）
type rateLimitShard struct {
	mu       sync.Mutex
	ips      map[string][]time.Time
	blackIPs map[string]bool
}

const shardCount = 16

var (
	shards     [shardCount]*rateLimitShard
	shardsOnce sync.Once
)

func initShards() {
	shardsOnce.Do(func() {
		for i := 0; i < shardCount; i++ {
			shards[i] = &rateLimitShard{
				ips:      make(map[string][]time.Time),
				blackIPs: make(map[string]bool),
			}
		}
		// 启动后台清理协程
		go rateLimitCleanup()
	})
}

func getShard(ip string) *rateLimitShard {
	h := uint32(0)
	for _, c := range ip {
		h = h*31 + uint32(c)
	}
	return shards[h%shardCount]
}

// rateLimitCleanup 定期清理过期 IP 记录，防止内存泄漏
func rateLimitCleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		for _, shard := range shards {
			shard.mu.Lock()
			now := time.Now()
			for ip, history := range shard.ips {
				var fresh []time.Time
				for _, t := range history {
					if now.Sub(t) < 5*time.Minute {
						fresh = append(fresh, t)
					}
				}
				if len(fresh) == 0 {
					delete(shard.ips, ip)
				} else {
					shard.ips[ip] = fresh
				}
			}
			shard.mu.Unlock()
		}
	}
}

// SetBlackIPs 设置黑名单 IP
func SetBlackIPs(list []string) {
	initShards()
	for _, ip := range list {
		shard := getShard(ip)
		shard.mu.Lock()
		shard.blackIPs[ip] = true
		shard.mu.Unlock()
	}
}

// RateLimit 速率限制中间件（分片锁版本，高并发下性能更优）
func RateLimit(limit int, window time.Duration) mvc.HandlerFunc {
	initShards()
	return func(c *mvc.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		ip := GetClientIP(c.Request)
		shard := getShard(ip)

		shard.mu.Lock()
		if shard.blackIPs[ip] {
			shard.mu.Unlock()
			c.Error(403, "Access Denied - IP in Blacklist")
			c.Abort()
			return
		}

		now := time.Now()
		var fresh []time.Time
		if history, ok := shard.ips[ip]; ok {
			for _, t := range history {
				if now.Sub(t) < window {
					fresh = append(fresh, t)
				}
			}
		}

		if len(fresh) >= limit {
			if len(fresh) >= limit*2 {
				shard.blackIPs[ip] = true
				log.Printf("[Security] IP %s 因请求过多被自动拉黑\n", ip)
			}
			shard.mu.Unlock()
			c.Error(429, "Too Many Requests - DoS Protection Triggered")
			c.Abort()
			return
		}

		shard.ips[ip] = append(fresh, now)
		shard.mu.Unlock()
		c.Next()
	}
}

// ==================== CORS 跨域中间件 ====================

// CORSConfig CORS 配置
type CORSConfig struct {
	AllowOrigins     []string // 允许的源，["*"] 表示全部
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int // 预检请求缓存时间（秒）
}

// DefaultCORSConfig 默认 CORS 配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           86400,
	}
}

// CORS 跨域资源共享中间件
func CORS(cfgs ...CORSConfig) mvc.HandlerFunc {
	cfg := DefaultCORSConfig()
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := fmt.Sprintf("%d", cfg.MaxAge)

	return func(c *mvc.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// 检查是否允许该源
		allowed := false
		for _, o := range cfg.AllowOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}
		if !allowed {
			c.Next()
			return
		}

		h := c.Writer.Header()
		if cfg.AllowOrigins[0] == "*" && !cfg.AllowCredentials {
			h.Set("Access-Control-Allow-Origin", "*")
		} else {
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
		}
		h.Set("Access-Control-Allow-Methods", allowMethods)
		h.Set("Access-Control-Allow-Headers", allowHeaders)
		if exposeHeaders != "" {
			h.Set("Access-Control-Expose-Headers", exposeHeaders)
		}
		if cfg.AllowCredentials {
			h.Set("Access-Control-Allow-Credentials", "true")
		}
		h.Set("Access-Control-Max-Age", maxAge)

		// 预检请求直接返回
		if c.Request.Method == "OPTIONS" {
			c.Writer.WriteHeader(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

// ==================== API 签名验证中间件 ====================

// HMACSign HMAC 签名验证中间件
// 客户端需在 Header 中附加:
//
//	X-Api-Key: <api_key>
//	X-Api-Timestamp: <unix_timestamp>
//	X-Api-Signature: <hmac_sha256(method+path+timestamp+body, secret)>
func HMACSign(secrets map[string]string, toleranceSec int64) mvc.HandlerFunc {
	if toleranceSec <= 0 {
		toleranceSec = 300 // 默认 5 分钟时间窗口
	}
	return func(c *mvc.Context) {
		apiKey := c.Request.Header.Get("X-Api-Key")
		timestamp := c.Request.Header.Get("X-Api-Timestamp")
		signature := c.Request.Header.Get("X-Api-Signature")

		if apiKey == "" || timestamp == "" || signature == "" {
			c.Error(http.StatusUnauthorized, "缺少 API 签名参数")
			c.Abort()
			return
		}

		// 验证 API Key
		secret, ok := secrets[apiKey]
		if !ok {
			c.Error(http.StatusUnauthorized, "无效的 API Key")
			c.Abort()
			return
		}

		// 验证时间戳（防重放攻击）
		ts := parseInt64(timestamp)
		now := time.Now().Unix()
		if abs64(now-ts) > toleranceSec {
			c.Error(http.StatusUnauthorized, "请求已过期")
			c.Abort()
			return
		}

		// 计算签名: HMAC-SHA256(method + path + timestamp + sorted_query, secret)
		data := c.Request.Method + c.Request.URL.Path + timestamp
		// 对查询参数排序后拼接
		params := c.Request.URL.Query()
		if len(params) > 0 {
			keys := make([]string, 0, len(params))
			for k := range params {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				data += k + "=" + strings.Join(params[k], ",")
			}
		}

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(data))
		expected := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(expected), []byte(signature)) {
			c.Error(http.StatusUnauthorized, "签名验证失败")
			c.Abort()
			return
		}
		c.Next()
	}
}

// ==================== 日志中间件（敏感信息过滤） ====================

// 敏感参数名列表
var sensitiveParams = map[string]bool{
	"password": true, "pass": true, "passwd": true, "pwd": true,
	"token": true, "secret": true, "api_key": true, "apikey": true,
	"access_token": true, "refresh_token": true,
	"credit_card": true, "card_number": true, "cvv": true,
}

// filterSensitiveQuery 过滤 URL 中的敏感参数
func filterSensitiveQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && sensitiveParams[strings.ToLower(kv[0])] {
			parts[i] = kv[0] + "=***"
		}
	}
	return strings.Join(parts, "&")
}

// responseRecorder 包装 ResponseWriter 以捕获 HTTP 状态码
type responseRecorder struct {
	http.ResponseWriter
	status  int
	written int64
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = 200
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

// Logger 日志中间件（过滤敏感信息，记录真实 HTTP 状态码）
func Logger() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		path := c.Request.URL.Path

		if isInternalPath(path) {
			c.Next()
			return
		}

		start := time.Now()
		raw := c.Request.URL.RawQuery
		rec := &responseRecorder{ResponseWriter: c.Writer, status: 200}
		c.Writer = rec

		c.Next()

		latency := time.Since(start)
		status := rec.status
		if status == 0 {
			status = 200
		}
		filteredQuery := filterSensitiveQuery(raw)
		if filteredQuery != "" {
			path = path + "?" + filteredQuery
		}

		log.Printf("| %3d | %13v | %15s | %-7s %s\n",
			status,
			latency,
			GetClientIP(c.Request),
			c.Request.Method,
			path,
		)
	}
}

// ==================== 异常恢复 ====================

// Recovery 异常恢复中间件
func Recovery() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[Recovery] panic: %s\n", err)
				var trace [4096]byte
				n := runtime.Stack(trace[:], false)
				log.Printf("[Recovery] stack:\n%s\n", trace[:n])
				c.Error(500, fmt.Sprintf("Internal Server Error: %v", err))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// ==================== 辅助函数 ====================

func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		} else {
			return 0
		}
	}
	return n
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

type responseCacheItem struct {
	body       []byte
	statusCode int
	headers    http.Header
	expiry     time.Time
}

type ResponseCache struct {
	mu      sync.RWMutex
	entries map[string]*responseCacheItem
	ttl     time.Duration
}

var globalResponseCache = &ResponseCache{
	entries: make(map[string]*responseCacheItem),
	ttl:     5 * time.Second,
}

func (rc *ResponseCache) Get(key string) (*responseCacheItem, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	item, ok := rc.entries[key]
	if !ok || time.Now().After(item.expiry) {
		return nil, false
	}
	return item, true
}

func (rc *ResponseCache) Set(key string, item *responseCacheItem) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	item.expiry = time.Now().Add(rc.ttl)
	rc.entries[key] = item
}

type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (w *cacheResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *cacheResponseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.Write([]byte(s))
}

func ResponseCacheMiddleware(ttl time.Duration) mvc.HandlerFunc {
	globalResponseCache.ttl = ttl
	return func(c *mvc.Context) {
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		cacheKey := c.Request.URL.Path + "?" + c.Request.URL.RawQuery
		if item, ok := globalResponseCache.Get(cacheKey); ok {
			for k, v := range item.headers {
				c.Writer.Header()[k] = v
			}
			c.Writer.WriteHeader(item.statusCode)
			c.Writer.Write(item.body)
			c.Abort()
			return
		}

		cacheWriter := &cacheResponseWriter{
			ResponseWriter: c.Writer,
			statusCode:     200,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = cacheWriter

		c.Next()

		if cacheWriter.statusCode < 400 {
			globalResponseCache.Set(cacheKey, &responseCacheItem{
				body:       cacheWriter.body.Bytes(),
				statusCode: cacheWriter.statusCode,
				headers:    c.Writer.Header().Clone(),
			})
		}
	}
}
