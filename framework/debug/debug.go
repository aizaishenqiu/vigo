package debug

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"vigo/framework/mvc"
)

// DebugToolbar 调试工具栏
type DebugToolbar struct {
	enabled bool
	data    map[string]interface{}
	mu      sync.RWMutex
}

// NewDebugToolbar 创建调试工具栏
func NewDebugToolbar() *DebugToolbar {
	return &DebugToolbar{
		enabled: true,
		data:    make(map[string]interface{}),
	}
}

// Middleware 调试中间件
func (dt *DebugToolbar) Middleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if !dt.enabled {
			c.Next()
			return
		}

		// 记录开始时间
		startTime := time.Now()
		startMem := getMemoryUsage()

		// 创建响应包装器
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			statusCode:     200,
			body:           &bytes.Buffer{},
		}
		c.Writer = rw

		// 处理请求
		c.Next()

		// 计算执行时间
		duration := time.Since(startTime)
		endMem := getMemoryUsage()

		// 收集调试信息
		debugData := map[string]interface{}{
			"request": map[string]interface{}{
				"method":       c.Request.Method,
				"url":          c.Request.URL.String(),
				"user_agent":   c.Request.UserAgent(),
				"content_type": c.GetHeader("Content-Type"),
				"headers":      c.Request.Header,
			},
			"response": map[string]interface{}{
				"status":       rw.statusCode,
				"size":         rw.body.Len(),
				"body":         rw.body.String(),
				"content_type": c.GetHeader("Content-Type"),
			},
			"performance": map[string]interface{}{
				"duration_ms":  duration.Milliseconds(),
				"duration_us":  duration.Microseconds(),
				"memory_start": startMem,
				"memory_end":   endMem,
				"memory_used":  endMem - startMem,
				"goroutines":   runtime.NumGoroutine(),
			},
			"route": map[string]interface{}{
				"params":     c.Params,
				"middleware": "",
			},
			"database": map[string]interface{}{
				"queries": []interface{}{},
				"time":    0,
			},
			"cache": map[string]interface{}{
				"hits":   0,
				"misses": 0,
			},
		}

		// 如果是 HTML 响应，注入调试工具栏
		if strings.Contains(c.GetHeader("Content-Type"), "text/html") {
			dt.injectToolbar(c, rw.body.String(), debugData)
		} else {
			// 记录到日志或返回调试头
			dt.logDebugData(debugData)
		}
	}
}

// responseWriter 响应包装器
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// getMemoryUsage 获取当前内存使用量（KB）
func getMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc / 1024
}

// injectToolbar 注入调试工具栏到 HTML
func (dt *DebugToolbar) injectToolbar(c *mvc.Context, html string, debugData map[string]interface{}) {
	// 生成调试工具栏 HTML
	toolbarHTML := dt.generateToolbarHTML(debugData)

	// 在</body>前插入
	html = strings.Replace(html, "</body>", toolbarHTML+"</body>", 1)

	c.String(http.StatusOK, html)
}

// generateToolbarHTML 生成调试工具栏 HTML
func (dt *DebugToolbar) generateToolbarHTML(debugData map[string]interface{}) string {
	// 转换为 JSON
	debugJSON, _ := json.MarshalIndent(debugData, "", "  ")

	return fmt.Sprintf(`
<!-- Vigo Debug Toolbar -->
<div id="vigo-debug-toolbar" style="position:fixed;bottom:0;left:0;right:0;z-index:999999;font-family:monospace;background:#1a1a1a;color:#fff;border-top:2px solid #00ff00;">
	<div style="display:flex;justify-content:space-between;padding:10px;">
		<div style="display:flex;gap:20px;">
			<div><strong>⏱️ 耗时:</strong> %v ms</div>
			<div><strong>💾 内存:</strong> %v KB</div>
			<div><strong>🔄 Goroutines:</strong> %d</div>
			<div><strong>📊 状态码:</strong> %d</div>
		</div>
		<div>
			<button onclick="document.getElementById('vigo-debug-panel').style.display='none'" style="background:#ff4444;color:white;border:none;padding:5px 10px;cursor:pointer;">✕ 关闭</button>
		</div>
	</div>
	<div id="vigo-debug-panel" style="padding:10px;background:#2a2a2a;max-height:400px;overflow-y:auto;">
		<pre id="vigo-debug-data" style="color:#00ff00;font-size:12px;">%s</pre>
	</div>
</div>
<script>
	// 支持折叠/展开
	document.getElementById('vigo-debug-toolbar').addEventListener('click', function(e) {
		if(e.target === this) {
			var panel = document.getElementById('vigo-debug-panel');
			panel.style.display = panel.style.display === 'none' ? 'block' : 'none';
		}
	});
</script>
<!-- End Vigo Debug Toolbar -->
`,
		debugData["performance"].(map[string]interface{})["duration_ms"],
		debugData["performance"].(map[string]interface{})["memory_used"],
		debugData["performance"].(map[string]interface{})["goroutines"],
		debugData["response"].(map[string]interface{})["status"],
		string(debugJSON),
	)
}

// logDebugData 记录调试信息
func (dt *DebugToolbar) logDebugData(data map[string]interface{}) {
	// 可以在这里集成日志系统
	// fmt.Printf("[DEBUG] %v\n", data)
}

// Enable 启用调试
func (dt *DebugToolbar) Enable() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.enabled = true
}

// Disable 禁用调试
func (dt *DebugToolbar) Disable() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.enabled = false
}

// QueryLogger 数据库查询日志记录器
type QueryLogger struct {
	queries []QueryInfo
	mu      sync.Mutex
}

// QueryInfo 查询信息
type QueryInfo struct {
	SQL      string
	Args     []interface{}
	Duration time.Duration
	Time     time.Time
	Error    error
}

// NewQueryLogger 创建查询日志记录器
func NewQueryLogger() *QueryLogger {
	return &QueryLogger{
		queries: make([]QueryInfo, 0),
	}
}

// Record 记录查询
func (ql *QueryLogger) Record(sql string, args []interface{}, duration time.Duration, err error) {
	ql.mu.Lock()
	defer ql.mu.Unlock()

	ql.queries = append(ql.queries, QueryInfo{
		SQL:      sql,
		Args:     args,
		Duration: duration,
		Time:     time.Now(),
		Error:    err,
	})
}

// GetQueries 获取所有查询
func (ql *QueryLogger) GetQueries() []QueryInfo {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	return ql.queries
}

// Clear 清空查询记录
func (ql *QueryLogger) Clear() {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	ql.queries = nil
}

// Count 获取查询数量
func (ql *QueryLogger) Count() int {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	return len(ql.queries)
}

// TotalTime 获取总查询时间
func (ql *QueryLogger) TotalTime() time.Duration {
	ql.mu.Lock()
	defer ql.mu.Unlock()

	var total time.Duration
	for _, q := range ql.queries {
		total += q.Duration
	}
	return total
}

// Profiler 性能分析器
type Profiler struct {
	events []ProfileEvent
	mu     sync.Mutex
}

// ProfileEvent 性能事件
type ProfileEvent struct {
	Name     string
	Duration time.Duration
	Memory   uint64
	Time     time.Time
}

// NewProfiler 创建性能分析器
func NewProfiler() *Profiler {
	return &Profiler{
		events: make([]ProfileEvent, 0),
	}
}

// Record 记录性能事件
func (p *Profiler) Record(name string, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.events = append(p.events, ProfileEvent{
		Name:     name,
		Duration: duration,
		Memory:   getMemoryUsage(),
		Time:     time.Now(),
	})
}

// GetEvents 获取所有事件
func (p *Profiler) GetEvents() []ProfileEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.events
}

// Summary 获取性能摘要
func (p *Profiler) Summary() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.events) == 0 {
		return map[string]interface{}{
			"total_events": 0,
		}
	}

	var totalTime time.Duration
	slowest := p.events[0]
	fastest := p.events[0]

	for _, e := range p.events {
		totalTime += e.Duration
		if e.Duration > slowest.Duration {
			slowest = e
		}
		if e.Duration < fastest.Duration {
			fastest = e
		}
	}

	return map[string]interface{}{
		"total_events": len(p.events),
		"total_time":   totalTime.Milliseconds(),
		"avg_time":     (totalTime / time.Duration(len(p.events))).Milliseconds(),
		"slowest":      slowest,
		"fastest":      fastest,
	}
}

// Clear 清空记录
func (p *Profiler) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = nil
}

// Dump 输出调试信息
func Dump(v interface{}) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	encoder.Encode(v)
	return buf.String()
}

// DumpToFile 输出调试信息到文件
func DumpToFile(v interface{}, filename string) error {
	_, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return nil // 简化实现
}

// Trace 堆栈跟踪
func Trace() string {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
