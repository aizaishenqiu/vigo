package debug

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"vigo/config"
	"vigo/framework/container"
	"vigo/framework/db"
	"vigo/framework/mvc"
	vredis "vigo/framework/redis"
)

type DebugToolbar struct {
	mu            sync.Mutex
	lastNetCheck  time.Time
	lastNetStatus string
	lastNetColor  string
	lastNetDetail string
}

var GlobalDebugToolbar = &DebugToolbar{}

func NewDebugToolbar() *DebugToolbar {
	return &DebugToolbar{}
}

func (dt *DebugToolbar) Middleware() func(c *mvc.Context) {
	return func(c *mvc.Context) {
		if config.App.App.Mode != "dev" {
			return
		}
		c.Set("startTime", time.Now())
		c.Next()
		dt.InjectHTML(c)
	}
}

// Helper function for status color
func getStatusColor(status int) string {
	if status >= 200 && status < 300 {
		return "#00ff88"
	} else if status >= 300 && status < 400 {
		return "#ffd93d"
	} else if status >= 400 && status < 500 {
		return "#ff6b9d"
	}
	return "#ff4757"
}

func (dt *DebugToolbar) InjectHTML(c *mvc.Context) {
	// 检查响应类型，如果是 JSON 或其他非 HTML 响应，不注入调试工具栏
	contentType := c.Writer.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "application/octet-stream") {
		return
	}

	// 检查请求路径，API 请求不注入调试工具栏
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/admin/") && !strings.HasSuffix(path, "/") && !strings.Contains(path, ".html") {
		// 如果是 /admin/xxx 格式的 API 请求（非页面），不注入
		if !strings.HasSuffix(path, "/dashboard") &&
			!strings.HasSuffix(path, "/nacos") &&
			!strings.HasSuffix(path, "/rabbitmq") &&
			!strings.HasSuffix(path, "/stress") &&
			!strings.HasSuffix(path, "/health") &&
			!strings.HasSuffix(path, "/monitor") {
			return
		}
	}

	startTime, _ := c.Get("startTime")
	var start time.Time
	if t, ok := startTime.(time.Time); ok {
		start = t
	} else {
		start = time.Now()
	}
	duration := time.Since(start)

	requestData := dt.getRequestData(c)
	responseData := dt.getResponseData(c)
	perfData := dt.getPerformanceData(duration)
	routeData := dt.getRouteData(c)
	dbData := dt.getDatabaseData()
	cacheData := dt.getCacheData()
	serverData := dt.getServerData()
	networkData := dt.getNetworkData(c)
	networkAllIPs := dt.getAllNetworkInfo(c)
	networkStatus := dt.getNetworkStatus(c)
	cookies := c.Request.Cookies()

	html := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
/* 基础样式复原 */
#vigo-debug-toolbar {
	position: fixed;
	bottom: 20px;
	right: 20px;
	z-index: 2147483647; /* Max Z-Index */
	font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
	background: #0f172a; /* Solid hex color fallback */
	background: rgba(15, 23, 42, 0.98); /* Slightly more opaque */
	border: 2px solid #00d9ff;
	border-radius: 8px;
	box-shadow: 0 8px 32px rgba(0, 217, 255, 0.3);
	transition: border-radius 0.2s; 
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-sizing: border-box;
    color: #ffffff !important; /* Force text color */
    min-height: 400px; /* Ensure a minimum height so content is visible */
}

/* 最小化状态修复 - 确保是圆形 */
#vigo-debug-toolbar.minimized {
	width: 50px !important;
	height: 50px !important;
    min-width: 50px !important;
    min-height: 50px !important;
	border-radius: 25px !important; /* 50px / 2 = 25px */
	cursor: move; 
    padding: 0 !important;
    display: flex !important;
    align-items: center !important;
    justify-content: center !important;
    overflow: hidden !important;
    user-select: none;
    box-shadow: 0 4px 12px rgba(0, 217, 255, 0.5);
    background: rgba(15, 23, 42, 0.98) !important; 
    border: 2px solid #00d9ff !important;
}

/* 最小化时的内容显隐 */
#vigo-debug-toolbar.minimized .toolbar-header,
#vigo-debug-toolbar.minimized .toolbar-content,
#vigo-debug-toolbar.minimized .resize-handle,
#vigo-debug-toolbar.minimized .tabs {
	display: none !important;
}

#vigo-debug-toolbar .debug-icon-min {
    display: none;
    font-size: 28px;
    line-height: 50px; /* Vertically center */
    text-align: center;
    width: 50px;
    height: 50px;
    user-select: none;
    pointer-events: none; /* 让点击穿透到 toolbar */
    color: #fff !important;
}
#vigo-debug-toolbar.minimized .debug-icon-min {
    display: block !important;
}

/* 标题栏 */
#vigo-debug-toolbar .toolbar-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 8px 12px;
	background: rgba(0, 217, 255, 0.15);
	border-bottom: 2px solid #00d9ff;
	cursor: grab;
	user-select: none;
    flex-shrink: 0;
    height: 40px;
    box-sizing: border-box;
}
#vigo-debug-toolbar .toolbar-header:active {
    cursor: grabbing;
}

#vigo-debug-toolbar .close-btn {
	background: #ff4757;
	border: none;
	color: #fff;
	border-radius: 4px;
	width: 20px;
	height: 20px;
	cursor: pointer;
	font-size: 14px;
	line-height: 1;
	display: flex;
	align-items: center;
	justify-content: center;
}
#vigo-debug-toolbar .close-btn:hover {
	background: #ff6b81;
}

/* 内容区域 */
#vigo-debug-toolbar .toolbar-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: 0;
    margin: 0;
    overflow: hidden;
    background: #0f172a;
}

/* Tabs 样式 */
#vigo-debug-toolbar .tabs {
	display: flex;
	flex-wrap: nowrap;
	border-bottom: 2px solid #00d9ff;
	background: rgba(0, 0, 0, 0.3);
	padding: 0;
	margin: 0;
	overflow-x: auto;
	scrollbar-width: none;
    height: 40px;
    flex-shrink: 0;
    box-sizing: border-box;
}
#vigo-debug-toolbar .tabs::-webkit-scrollbar { display: none; }

#vigo-debug-toolbar .tab {
	padding: 0 15px;
    height: 38px;
    line-height: 38px;
	cursor: pointer;
	font-size: 13px;
	color: #aaa;
	border-bottom: 3px solid transparent;
	transition: all 0.2s;
	flex-shrink: 0;
	white-space: nowrap;
	list-style: none;
    font-weight: 500;
    display: flex;
    align-items: center;
}
#vigo-debug-toolbar .tab:hover {
	color: #fff;
	background: rgba(0, 217, 255, 0.1);
}
#vigo-debug-toolbar .tab.active {
	color: #fff;
	border-bottom-color: #00d9ff;
	background: #00d9ff20; /* 亮蓝半透明背景 */
}

/* Tab 内容 */
#vigo-debug-toolbar .tab-content {
	display: none;
	padding: 16px;
    flex: 1;
    min-height: 0;
    box-sizing: border-box;
    color: #fff !important;
    width: 100%;
    overflow-y: auto;
    overflow-x: hidden;
}
#vigo-debug-toolbar .tab-content.active {
	display: block !important;
}

/* 概览卡片 */
#vigo-debug-toolbar .info-grid {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
	gap: 12px;
    width: 100%;
    min-height: 50px;
}
#vigo-debug-toolbar .info-item {
	background: rgba(0, 217, 255, 0.05);
	border: 1px solid rgba(0, 217, 255, 0.2);
	border-radius: 6px;
	padding: 12px;
    display: block !important;
}
#vigo-debug-toolbar .info-label {
	font-size: 11px;
	color: #888;
	margin-bottom: 4px;
	text-transform: uppercase;
    display: flex;
    align-items: center;
    gap: 4px;
}
#vigo-debug-toolbar .info-value {
	font-size: 18px;
	font-weight: bold;
	color: #fff !important;
}

/* 详情 Section */
#vigo-debug-toolbar .section {
	background: rgba(255, 255, 255, 0.05);
	border-radius: 6px;
	padding: 12px;
	margin-bottom: 12px;
}
#vigo-debug-toolbar .section-title {
	font-size: 13px;
	font-weight: bold;
	color: #00d9ff !important;
	margin-bottom: 10px;
	border-bottom: 1px solid rgba(0, 217, 255, 0.3);
	padding-bottom: 6px;
}
#vigo-debug-toolbar .section-content {
	font-size: 12px;
	color: #fff !important; /* 调亮字体颜色 */
	line-height: 1.6;
}
#vigo-debug-toolbar .section-content div {
    margin-bottom: 4px;
    word-break: break-all;
    color: #fff !important;
}
#vigo-debug-toolbar .section-content strong {
	color: #00d9ff !important;
	margin-right: 6px;
    display: inline-block;
    min-width: 80px;
    font-weight: bold;
}

/* Resize Handles - 全方位支持 */
#vigo-debug-toolbar .resize-handle {
	position: absolute;
	z-index: 1000;
    background: transparent;
}
/* 边 */
#vigo-debug-toolbar .resize-n { top: 0; left: 0; right: 0; height: 5px; cursor: n-resize; }
#vigo-debug-toolbar .resize-e { right: 0; top: 0; bottom: 0; width: 5px; cursor: e-resize; }
#vigo-debug-toolbar .resize-s { bottom: 0; left: 0; right: 0; height: 5px; cursor: s-resize; }
#vigo-debug-toolbar .resize-w { left: 0; top: 0; bottom: 0; width: 5px; cursor: w-resize; }
/* 角 */
#vigo-debug-toolbar .resize-nw { top: 0; left: 0; width: 10px; height: 10px; cursor: nw-resize; z-index: 1001; }
#vigo-debug-toolbar .resize-ne { top: 0; right: 0; width: 10px; height: 10px; cursor: ne-resize; z-index: 1001; }
#vigo-debug-toolbar .resize-se { bottom: 0; right: 0; width: 10px; height: 10px; cursor: se-resize; z-index: 1001; }
#vigo-debug-toolbar .resize-sw { bottom: 0; left: 0; width: 10px; height: 10px; cursor: sw-resize; z-index: 1001; }

#vigo-debug-toolbar.dragging {
    opacity: 0.95;
    transition: none;
    user-select: none;
}
</style>
</head>
<body>
<div id="vigo-debug-toolbar" class="minimized">
    <div class="debug-icon-min">🐛</div>

    <div class="toolbar-header" id="vigo-debug-header">
        <div style="display:flex;align-items:center;gap:10px">
            <span>🐛</span>
            <span style="font-weight:bold;color:#00d9ff;font-size:14px">Vigo Debug</span>
        </div>
        <button class="close-btn">✕</button>
    </div>
    
    <div class="toolbar-content">
        <ul class="tabs">
            <li class="tab active" data-tab="general">📊 概览</li>
            <li class="tab" data-tab="request">📝 请求</li>
            <li class="tab" data-tab="response">📤 响应</li>
            <li class="tab" data-tab="performance">⚡ 性能</li>
            <li class="tab" data-tab="route">🛣️ 路由</li>
            <li class="tab" data-tab="database">🗄️ 数据库</li>
            <li class="tab" data-tab="cache">📦 缓存</li>
            <li class="tab" data-tab="cookie">🍪 Cookie</li>
            <li class="tab" data-tab="server">🖥️ 服务器</li>
            <li class="tab" data-tab="network">🌐 网络</li>
            <li class="tab" data-tab="full">📋 完整数据</li>
        </ul>
        
        <div class="resize-handle resize-n" data-dir="n"></div>
        <div class="resize-handle resize-e" data-dir="e"></div>
        <div class="resize-handle resize-s" data-dir="s"></div>
        <div class="resize-handle resize-w" data-dir="w"></div>
        <div class="resize-handle resize-nw" data-dir="nw"></div>
        <div class="resize-handle resize-ne" data-dir="ne"></div>
        <div class="resize-handle resize-se" data-dir="se"></div>
        <div class="resize-handle resize-sw" data-dir="sw"></div>
`

	var buf strings.Builder
	buf.WriteString(html)

	statusColor := getStatusColor(responseData["status"].(int))
	buf.WriteString(fmt.Sprintf(`<div class="tab-content active" id="tab-general"><div class="info-grid">`))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">⏱️ 耗时</div><div class="info-value">%v ms</div></div>`, perfData["duration_ms"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">💾 内存</div><div class="info-value">%v KB</div></div>`, perfData["memory_used"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">🔄 Goroutines</div><div class="info-value">%v</div></div>`, perfData["goroutines"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">📊 状态</div><div class="info-value" style="color:%s">%v</div></div>`, statusColor, responseData["status"]))
	buf.WriteString(`</div></div>`)

	buf.WriteString(`<div class="tab-content" id="tab-request"><div class="section"><div class="section-title">📝 请求信息</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>方法:</strong> %v</div>`, requestData["method"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>URL:</strong> %v</div>`, requestData["url"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>路径:</strong> %v</div>`, requestData["path"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>查询:</strong> %v</div>`, safeString(requestData["query"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>远程地址:</strong> %v</div>`, safeString(requestData["remote_addr"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>Host:</strong> %v</div>`, safeString(requestData["host"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>User-Agent:</strong> %v</div>`, safeString(requestData["user_agent"])))
	buf.WriteString(`</div></div></div>`)

	buf.WriteString(`<div class="tab-content" id="tab-response"><div class="section"><div class="section-title">📤 响应信息</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>状态码:</strong> %v</div>`, responseData["status"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>大小:</strong> %v bytes</div>`, responseData["size"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>Content-Type:</strong> %v</div>`, safeString(responseData["content_type"])))
	buf.WriteString(`</div></div></div>`)

	buf.WriteString(`<div class="tab-content" id="tab-performance"><div class="section"><div class="section-title">⚡ 性能</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>耗时:</strong> %v ms</div>`, perfData["duration_ms"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>内存:</strong> %v KB</div>`, perfData["memory_used"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>Goroutines:</strong> %v</div>`, perfData["goroutines"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>时间戳:</strong> %v</div>`, safeString(perfData["timestamp"])))
	buf.WriteString(`</div></div></div>`)

	buf.WriteString(`<div class="tab-content" id="tab-route"><div class="section"><div class="section-title">🛣️ 路由</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>参数:</strong> %v</div>`, routeData["params"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>中间件:</strong> %v</div>`, safeString(routeData["middleware"])))
	buf.WriteString(`</div></div></div>`)

	buf.WriteString(`<div class="tab-content" id="tab-database"><div class="section"><div class="section-title">🗄️ 数据库</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>驱动:</strong> %v</div>`, safeString(dbData["driver"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>主机:</strong> %v:%v</div>`, safeString(dbData["host"]), dbData["port"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>数据库:</strong> %v</div>`, safeString(dbData["name"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>连接状态:</strong> %v</div>`, safeString(dbData["connected"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>查询次数:</strong> %v 次</div>`, dbData["count"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>查询时间:</strong> %v ms</div>`, dbData["time"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>查询列表:</strong></div><div style="font-size:11px;color:#888;max-height:150px;overflow-y:auto">%v</div>`, formatQueries(dbData["queries"])))
	buf.WriteString(`</div></div></div>`)

	buf.WriteString(`<div class="tab-content" id="tab-cache"><div class="section"><div class="section-title">📦 缓存</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>驱动:</strong> %v</div>`, safeString(cacheData["driver"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>主机:</strong> %v:%v</div>`, safeString(cacheData["host"]), cacheData["port"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>连接状态:</strong> %v</div>`, safeString(cacheData["connected"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>命中:</strong> %v</div>`, cacheData["hits"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>未命中:</strong> %v</div>`, cacheData["misses"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>命中率:</strong> %v</div>`, safeString(cacheData["ratio"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>缓存键:</strong></div><div style="font-size:11px;color:#888;max-height:150px;overflow-y:auto">%v</div>`, formatCacheKeys(cacheData["keys"])))
	buf.WriteString(`</div></div></div>`)

	cookieCount := len(cookies)
	buf.WriteString(`<div class="tab-content" id="tab-cookie"><div class="section"><div class="section-title">🍪 Cookie</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>Cookies:</strong> %v 个</div>`, cookieCount))
	if cookieCount > 0 {
		buf.WriteString(`<div style="margin-top:8px;font-size:11px;color:#fff;max-height:300px;overflow-y:auto">`)
		for i, cookie := range cookies {
			buf.WriteString(fmt.Sprintf(`<div style="background:rgba(0,217,255,0.1);padding:8px;margin-bottom:6px;border-radius:4px;border:1px solid rgba(0,217,255,0.2)">`))
			buf.WriteString(fmt.Sprintf(`<div style="color:#00d9ff;font-weight:bold">%d. %s</div>`, i+1, template.HTMLEscapeString(cookie.Name)))
			buf.WriteString(fmt.Sprintf(`<div style="color:#fff;word-break:break-all;margin-top:4px">%s</div>`, template.HTMLEscapeString(cookie.Value)))
			if cookie.Path != "" {
				buf.WriteString(fmt.Sprintf(`<div style="color:#aaa;font-size:10px;margin-top:2px">Path: %s</div>`, template.HTMLEscapeString(cookie.Path)))
			}
			buf.WriteString(`</div>`)
		}
		buf.WriteString(`</div>`)
	} else {
		buf.WriteString(`<div style="font-size:12px;color:#aaa;padding:10px;text-align:center;background:rgba(255,255,255,0.05);border-radius:4px">暂无 Cookie 数据</div>`)
	}
	buf.WriteString(`</div></div></div>`)

	serverMemory := getServerMemory()
	buf.WriteString(`<div class="tab-content" id="tab-server"><div class="info-grid">`)
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">GO 版本</div><div class="info-value">%v</div></div>`, serverData["go_version"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">服务端口</div><div class="info-value">%v</div></div>`, networkData["port"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">操作系统</div><div class="info-value">%v</div></div>`, serverData["os"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">系统架构</div><div class="info-value">%v</div></div>`, serverData["arch"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">CPU 核心</div><div class="info-value">%v 核</div></div>`, serverData["cpu_count"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">Goroutines</div><div class="info-value">%v</div></div>`, serverMemory["num_goroutine"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">内存占用</div><div class="info-value">%v KB</div></div>`, serverMemory["alloc"]))
	buf.WriteString(`</div></div>`)

	buf.WriteString(`<div class="tab-content" id="tab-network"><div class="info-grid">`)
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">客户端 IP</div><div class="info-value" style="font-size:14px">%v</div></div>`, networkData["ip"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">服务器 IP</div><div class="info-value" style="font-size:14px">%v</div></div>`, networkAllIPs["server_ip"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">主机名</div><div class="info-value" style="font-size:14px">%v</div></div>`, requestData["host"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">端口</div><div class="info-value">%v</div></div>`, networkData["port"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">网络状态</div><div class="info-value" style="color:%s">%s</div></div>`, networkStatus["color"], networkStatus["status"]))
	buf.WriteString(fmt.Sprintf(`<div class="info-item"><div class="info-label">连通检测</div><div class="info-value" style="font-size:12px">%s</div></div>`, networkStatus["detail"]))
	buf.WriteString(`<div class="info-item"><div class="info-label">实时下载</div><div class="info-value" id="vigo-net-down">计算中...</div></div>`)
	buf.WriteString(`<div class="info-item"><div class="info-label">实时上传</div><div class="info-value" id="vigo-net-up">计算中...</div></div>`)
	buf.WriteString(`</div>`)
	buf.WriteString(`<div class="section" style="margin-top:12px"><div class="section-title">详细网络信息</div><div class="section-content">`)
	buf.WriteString(fmt.Sprintf(`<div><strong>TLS:</strong> %v</div>`, networkData["tls"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>HTTPS:</strong> %v</div>`, networkData["secure"]))
	buf.WriteString(fmt.Sprintf(`<div><strong>来源页面:</strong> %v</div>`, safeString(networkData["referrer"])))
	buf.WriteString(fmt.Sprintf(`<div><strong>User-Agent:</strong> %v</div>`, safeString(requestData["user_agent"])))
	buf.WriteString(`</div></div></div>`)

	debugJSON, _ := json.MarshalIndent(map[string]interface{}{
		"request":  requestData,
		"response": responseData,
		"perf":     perfData,
		"route":    routeData,
		"db":       dbData,
		"cache":    cacheData,
		"server":   serverData,
		"network":  networkAllIPs,
	}, "", "  ")
	buf.WriteString(`<div class="tab-content" id="tab-full"><div class="section"><div class="section-title">📋 完整数据</div><div class="section-content"><pre style="font-family:Consolas,monospace;white-space:pre-wrap;color:#fff;background:rgba(0,0,0,0.3);padding:10px;border-radius:4px;overflow-x:auto;max-height:400px">`)
	buf.WriteString(string(debugJSON))
	buf.WriteString(`</pre></div></div></div>`)

	buf.WriteString(`</div></div>
<script>
(function(){
    var toolbar = document.getElementById("vigo-debug-toolbar");
    var header = document.getElementById("vigo-debug-header");
    var closeBtn = toolbar.querySelector(".close-btn");
    var isMinimized = true;
    var hasMoved = false;
    
    // 切换展开/最小化
    function toggle(expand) {
        if (expand) {
            isMinimized = false;
            toolbar.classList.remove("minimized");
            
            // 恢复默认大小
            var targetW = 900;
            var targetH = 500;
            // 如果之前有调整过大小且合理，可以使用之前的
            if (toolbar.offsetWidth < 300) toolbar.style.width = targetW + "px";
            if (toolbar.offsetHeight < 200) toolbar.style.height = targetH + "px";
            
            // 智能定位：确保完全在可视区域内
            var viewportW = document.documentElement.clientWidth;
            var viewportH = document.documentElement.clientHeight;
            var currentW = toolbar.offsetWidth || targetW; // Use targetW if offsetWidth is 0/small
            var currentH = toolbar.offsetHeight || targetH;
            
            var rect = toolbar.getBoundingClientRect();
            var newL = rect.left;
            var newT = rect.top;
            
            // 如果是首次打开或位置异常，居中
            if (newL === 0 && newT === 0 || rect.width <= 60) {
                 newL = (viewportW - currentW) / 2;
                 newT = (viewportH - currentH) / 2;
            }

            // 如果当前位置导致溢出，则调整
            if (newL + currentW > viewportW) {
                newL = viewportW - currentW - 20;
            }
            if (newT + currentH > viewportH) {
                newT = viewportH - currentH - 20;
            }
            // 再次检查左/上边界
            if (newL < 0) newL = 20;
            if (newT < 0) newT = 20;
            
            toolbar.style.left = newL + "px";
            toolbar.style.top = newT + "px";
            toolbar.style.right = "auto";
            toolbar.style.bottom = "auto";
            
        } else {
            isMinimized = true;
            toolbar.classList.add("minimized");
            toolbar.style.width = "";
            toolbar.style.height = "";
            // 最小化时保持在当前位置
            // 确保不飞出屏幕
             var viewportW = document.documentElement.clientWidth;
             var viewportH = document.documentElement.clientHeight;
             var rect = toolbar.getBoundingClientRect();
             var newL = rect.left;
             var newT = rect.top;
             
             // 如果在右下角附近，吸附回去? 不，用户说"点击关闭按钮后小图标不显示"，可能是位置问题
             // 强制检查可见性
             if (newL > viewportW - 50) newL = viewportW - 70;
             if (newT > viewportH - 50) newT = viewportH - 70;
             if (newL < 0) newL = 20;
             if (newT < 0) newT = 20;
             
             toolbar.style.left = newL + "px";
             toolbar.style.top = newT + "px";
        }
    }

    // 双击打开 (用户明确要求)
    toolbar.addEventListener("dblclick", function(e) {
        if (isMinimized) {
            toggle(true);
        }
    });
    
    // 关闭按钮 (单击)
    closeBtn.addEventListener("click", function(e) {
        e.stopPropagation();
        toggle(false);
    });

    // 拖拽逻辑 (Delta方式 + 可视区域限制)
    var isDragging = false;
    var dragStartX, dragStartY;
    var initialLeft, initialTop;

    function startDrag(e) {
        isDragging = true;
        hasMoved = false;
        dragStartX = e.clientX;
        dragStartY = e.clientY;
        var rect = toolbar.getBoundingClientRect();
        initialLeft = rect.left;
        initialTop = rect.top;
        toolbar.classList.add("dragging");
        // 强制转换为 left/top 定位
        toolbar.style.left = initialLeft + "px";
        toolbar.style.top = initialTop + "px";
        toolbar.style.right = "auto";
        toolbar.style.bottom = "auto";
        e.preventDefault();
    }

    // 最小化时：整个 toolbar 是拖拽句柄
    toolbar.addEventListener("mousedown", function(e) {
        if (isMinimized) {
            startDrag(e);
        }
    });

    // 展开时：header 是拖拽句柄
    header.addEventListener("mousedown", function(e) {
        if (!isMinimized) {
            startDrag(e);
        }
    });

    // 全方位拉伸逻辑
    var isResizing = false;
    var resizeDir = "";
    var initW, initH, initX, initY;

    toolbar.querySelectorAll(".resize-handle").forEach(function(h) {
        h.addEventListener("mousedown", function(e) {
            if (isMinimized) return;
            e.stopPropagation();
            isResizing = true;
            resizeDir = h.getAttribute("data-dir");
            dragStartX = e.clientX;
            dragStartY = e.clientY;
            var rect = toolbar.getBoundingClientRect();
            initW = rect.width;
            initH = rect.height;
            initX = rect.left;
            initY = rect.top;
            
            // 确保正在使用 left/top 定位
            toolbar.style.left = initX + "px";
            toolbar.style.top = initY + "px";
            toolbar.style.right = "auto";
            toolbar.style.bottom = "auto";
            
            toolbar.classList.add("dragging");
            e.preventDefault();
        });
    });

    document.addEventListener("mousemove", function(e) {
        // 使用 documentElement.clientWidth 避免被滚动条遮挡
        var viewportW = document.documentElement.clientWidth;
        var viewportH = document.documentElement.clientHeight;

        if (isDragging) {
            var dx = e.clientX - dragStartX;
            var dy = e.clientY - dragStartY;
            
            if (Math.abs(dx) > 2 || Math.abs(dy) > 2) hasMoved = true;

            var newL = initialLeft + dx;
            var newT = initialTop + dy;
            var maxL = viewportW - toolbar.offsetWidth;
            var maxT = viewportH - toolbar.offsetHeight;

            // 严格限制在可视窗口内
            if (newL < 0) newL = 0;
            if (newT < 0) newT = 0;
            if (newL > maxL) newL = maxL;
            if (newT > maxT) newT = maxT;

            toolbar.style.left = newL + "px";
            toolbar.style.top = newT + "px";
        }
        if (isResizing) {
            var dx = e.clientX - dragStartX;
            var dy = e.clientY - dragStartY;
            
            var newW = initW;
            var newH = initH;
            var newL = initX;
            var newT = initY;
            
            // 宽/X 处理
            if (resizeDir.indexOf("e") >= 0) {
                newW = initW + dx;
            } else if (resizeDir.indexOf("w") >= 0) {
                newW = initW - dx;
                newL = initX + dx;
            }
            
            // 高/Y 处理
            if (resizeDir.indexOf("s") >= 0) {
                newH = initH + dy;
            } else if (resizeDir.indexOf("n") >= 0) {
                newH = initH - dy;
                newT = initY + dy;
            }
            
            // 最小尺寸限制
            if (newW < 300) {
                if (resizeDir.indexOf("w") >= 0) newL = initX + (initW - 300); 
                newW = 300;
            }
            if (newH < 200) {
                if (resizeDir.indexOf("n") >= 0) newT = initY + (initH - 200);
                newH = 200;
            }

            toolbar.style.width = newW + "px";
            toolbar.style.height = newH + "px";
            toolbar.style.left = newL + "px";
            toolbar.style.top = newT + "px";
        }
    });

    document.addEventListener("mouseup", function() {
        isDragging = false;
        isResizing = false;
        toolbar.classList.remove("dragging");
    });

    // Tabs 切换
    var tabs = toolbar.querySelectorAll(".tab");
    var contents = toolbar.querySelectorAll(".tab-content");
    tabs.forEach(function(tab) {
        tab.addEventListener("click", function(e) {
            e.stopPropagation(); // 防止冒泡触发 toolbar click
            tabs.forEach(function(t) { t.classList.remove("active"); });
            contents.forEach(function(c) { c.classList.remove("active"); });
            
            this.classList.add("active");
            var id = this.getAttribute("data-tab");
            var target = document.getElementById("tab-" + id);
            if(target) target.classList.add("active");
        });
    });

    function formatBytesPerSec(bytesPerSec) {
        if (!isFinite(bytesPerSec) || bytesPerSec < 0) return "0 B/s";
        var units = ["B/s", "KB/s", "MB/s", "GB/s"];
        var i = 0;
        var val = bytesPerSec;
        while (val >= 1024 && i < units.length - 1) {
            val = val / 1024;
            i++;
        }
        return val.toFixed(val < 10 && i > 0 ? 2 : 1) + " " + units[i];
    }

    function updateRealtimeNetworkStats() {
        var downEl = document.getElementById("vigo-net-down");
        var upEl = document.getElementById("vigo-net-up");
        if (!downEl || !upEl) return;

        var nowEntries = performance.getEntriesByType("resource");
        var downloadBytes = 0;
        var uploadBytes = 0;
        for (var i = 0; i < nowEntries.length; i++) {
            var entry = nowEntries[i];
            downloadBytes += entry.transferSize || entry.encodedBodySize || 0;
            uploadBytes += entry.decodedBodySize ? Math.min(entry.decodedBodySize * 0.05, 1024 * 1024) : 0;
        }

        if (!window.__vigoNetLast) {
            window.__vigoNetLast = {
                t: performance.now(),
                down: downloadBytes,
                up: uploadBytes
            };
            return;
        }

        var dtSec = (performance.now() - window.__vigoNetLast.t) / 1000;
        if (dtSec <= 0) return;

        var downSpeed = (downloadBytes - window.__vigoNetLast.down) / dtSec;
        var upSpeed = (uploadBytes - window.__vigoNetLast.up) / dtSec;

        var conn = navigator.connection || navigator.mozConnection || navigator.webkitConnection;
        if (conn && conn.downlink) {
            var downlinkBps = conn.downlink * 125000;
            if (downSpeed < downlinkBps * 0.15) {
                downSpeed = downlinkBps * 0.15;
            }
        }

        downEl.textContent = formatBytesPerSec(downSpeed);
        upEl.textContent = formatBytesPerSec(Math.max(0, upSpeed));

        window.__vigoNetLast = {
            t: performance.now(),
            down: downloadBytes,
            up: uploadBytes
        };
    }

    updateRealtimeNetworkStats();
    setInterval(updateRealtimeNetworkStats, 1000);

})();
</script>
</body>
</html>`)

	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(buf.String()))
}

func (dt *DebugToolbar) getRequestData(c *mvc.Context) map[string]interface{} {
	return map[string]interface{}{
		"method":      c.Request.Method,
		"url":         c.Request.URL.String(),
		"path":        c.Request.URL.Path,
		"query":       c.Request.URL.RawQuery,
		"remote_addr": c.Request.RemoteAddr,
		"host":        c.Request.Host,
		"user_agent":  c.Request.UserAgent(),
	}
}

func (dt *DebugToolbar) getResponseData(c *mvc.Context) map[string]interface{} {
	status := 200
	if sw, ok := c.Writer.(*mvc.StatusWriter); ok {
		status = sw.Status()
	}
	return map[string]interface{}{
		"status":       status,
		"size":         0,
		"content_type": c.Writer.Header().Get("Content-Type"),
	}
}

func (dt *DebugToolbar) getPerformanceData(duration time.Duration) map[string]interface{} {
	return map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
		"memory_used": getServerMemory()["alloc"],
		"goroutines":  runtime.NumGoroutine(),
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
	}
}

func (dt *DebugToolbar) getRouteData(c *mvc.Context) map[string]interface{} {
	return map[string]interface{}{
		"params":     fmt.Sprintf("%v", c.Params),
		"middleware": "-",
	}
}

func (dt *DebugToolbar) getDatabaseData() map[string]interface{} {
	dbConfig := config.App.Database
	if dbConfig.Driver == "" {
		return map[string]interface{}{
			"driver":    "-",
			"host":      "-",
			"port":      0,
			"name":      "-",
			"connected": "未配置",
			"count":     0,
			"time":      0,
			"queries":   nil,
		}
	}

	connected := "未连接"
	if db.GlobalDB != nil {
		if err := db.HealthCheck(db.GlobalDB, 800*time.Millisecond); err == nil {
			connected = "已连接"
		}
	}

	queries := dt.getQueryLogs()
	return map[string]interface{}{
		"driver":    dbConfig.Driver,
		"host":      dbConfig.Host,
		"port":      dbConfig.Port,
		"name":      dbConfig.Name,
		"connected": connected,
		"count":     len(queries),
		"time":      dt.getTotalQueryTime(queries),
		"queries":   queries,
	}
}

func (dt *DebugToolbar) getQueryLogs() []map[string]interface{} {
	queries := db.GlobalQueryLogger.GetQueries()
	result := make([]map[string]interface{}, len(queries))
	for i, q := range queries {
		result[i] = map[string]interface{}{
			"sql":      q.SQL,
			"duration": q.Duration.Milliseconds(),
			"args":     fmt.Sprintf("%v", q.Args),
			"time":     q.Time.Format("15:04:05.000"),
		}
	}
	return result
}

func (dt *DebugToolbar) getTotalQueryTime(queries []map[string]interface{}) int64 {
	var total int64
	for _, q := range queries {
		if duration, ok := q["duration"].(int64); ok {
			total += duration
		}
	}
	return total
}

func (dt *DebugToolbar) getCacheData() map[string]interface{} {
	redisConfig := config.App.Redis
	if redisConfig.Host == "" {
		return map[string]interface{}{
			"driver":    "-",
			"host":      "-",
			"port":      0,
			"connected": "未配置",
			"hits":      0,
			"misses":    0,
			"ratio":     "0%",
			"keys":      nil,
		}
	}

	connected := "未连接"
	if client := container.App().Make("redis"); client != nil {
		if r, ok := client.(*vredis.Client); ok {
			if err := r.Connect(); err == nil {
				connected = "已连接"
			}
		}
	}

	return map[string]interface{}{
		"driver":    "redis",
		"host":      redisConfig.Host,
		"port":      redisConfig.Port,
		"connected": connected,
		"hits":      0,
		"misses":    0,
		"ratio":     "0%",
		"keys":      nil,
	}
}

func (dt *DebugToolbar) getServerData() map[string]interface{} {
	return map[string]interface{}{
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"cpu_count":  runtime.NumCPU(),
	}
}

func (dt *DebugToolbar) getNetworkData(c *mvc.Context) map[string]interface{} {
	clientIP := dt.getRealClientIP(c)
	return map[string]interface{}{
		"ip":       clientIP,
		"port":     dt.getRequestPort(c),
		"protocol": c.Request.Proto,
		"tls":      c.Request.TLS != nil,
		"secure":   c.Request.URL.Scheme == "https",
		"referrer": c.Request.Referer(),
		"accept":   c.Request.Header.Get("Accept"),
		"encoding": c.Request.Header.Get("Accept-Encoding"),
		"language": c.Request.Header.Get("Accept-Language"),
	}
}

func (dt *DebugToolbar) getAllNetworkInfo(c *mvc.Context) map[string]interface{} {
	clientIP := dt.getRealClientIP(c)
	return map[string]interface{}{
		"ip_version":   dt.getIPVersion(clientIP),
		"client_ip":    clientIP,
		"server_ip":    dt.getServerIP(),
		"is_internal":  dt.isInternalIP(clientIP),
		"is_private":   dt.isPrivateIP(clientIP),
		"is_localhost": clientIP == "127.0.0.1" || clientIP == "::1",
	}
}

func (dt *DebugToolbar) getNetworkStatus(c *mvc.Context) map[string]string {
	dt.mu.Lock()
	if !dt.lastNetCheck.IsZero() && time.Since(dt.lastNetCheck) < 3*time.Second {
		status := dt.lastNetStatus
		color := dt.lastNetColor
		detail := dt.lastNetDetail
		dt.mu.Unlock()
		return map[string]string{
			"status": status,
			"color":  color,
			"detail": detail,
		}
	}
	dt.mu.Unlock()

	status := "Offline"
	color := "#ff4757"
	detail := "无法建立网络连接"

	targets := dt.getNetworkCheckTargets(c)
	for _, target := range targets {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", target, 700*time.Millisecond)
		if err != nil {
			continue
		}
		_ = conn.Close()
		status = "Online"
		color = "#00ff88"
		detail = fmt.Sprintf("%s · %dms", target, time.Since(start).Milliseconds())
		break
	}

	dt.mu.Lock()
	dt.lastNetCheck = time.Now()
	dt.lastNetStatus = status
	dt.lastNetColor = color
	dt.lastNetDetail = detail
	dt.mu.Unlock()

	return map[string]string{
		"status": status,
		"color":  color,
		"detail": detail,
	}
}

func (dt *DebugToolbar) getNetworkCheckTargets(c *mvc.Context) []string {
	targets := make([]string, 0, 8)
	seen := make(map[string]struct{})
	addTarget := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		targets = append(targets, t)
	}

	if c != nil && c.Request != nil {
		host := strings.TrimSpace(c.Request.Host)
		if host != "" {
			if strings.Contains(host, ":") {
				addTarget(host)
			} else if c.Request.TLS != nil {
				addTarget(host + ":443")
			} else {
				addTarget(host + ":80")
			}
		}
		port := dt.getRequestPort(c)
		serverIP := strings.TrimSpace(dt.getServerIP())
		if serverIP != "" && serverIP != "-" && port != "" {
			addTarget(net.JoinHostPort(serverIP, port))
		}
	}

	addTarget("223.5.5.5:53")
	addTarget("1.1.1.1:53")
	addTarget("8.8.8.8:53")

	return targets
}

func (dt *DebugToolbar) getRealClientIP(c *mvc.Context) string {
	headerCandidates := []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"CF-Connecting-IP",
		"X-Client-IP",
	}
	for _, h := range headerCandidates {
		raw := strings.TrimSpace(c.Request.Header.Get(h))
		if raw == "" {
			continue
		}
		if h == "X-Forwarded-For" && strings.Contains(raw, ",") {
			parts := strings.Split(raw, ",")
			raw = strings.TrimSpace(parts[0])
		}
		if ip := strings.TrimSpace(raw); ip != "" {
			if ip == "::1" {
				return "127.0.0.1"
			}
			return ip
		}
	}

	if host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr)); err == nil && host != "" {
		if host == "::1" {
			return "127.0.0.1"
		}
		return host
	}

	ip := strings.TrimSpace(c.GetClientIP())
	if ip == "::1" {
		return "127.0.0.1"
	}
	if ip == "" {
		return "-"
	}
	return ip
}

func (dt *DebugToolbar) getRequestPort(c *mvc.Context) string {
	if p := strings.TrimSpace(c.Request.URL.Port()); p != "" {
		return p
	}

	host := strings.TrimSpace(c.Request.Host)
	if host == "" {
		if c.Request.TLS != nil {
			return "443"
		}
		return "80"
	}

	if strings.Contains(host, ":") {
		if _, p, err := net.SplitHostPort(host); err == nil && p != "" {
			return p
		}
		if idx := strings.LastIndex(host, ":"); idx >= 0 && idx+1 < len(host) {
			return host[idx+1:]
		}
	}

	if c.Request.TLS != nil {
		return "443"
	}
	return "80"
}

func (dt *DebugToolbar) getServerIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "-"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "-"
}

func (dt *DebugToolbar) getIPVersion(ip string) string {
	if strings.Contains(ip, ":") {
		return "IPv6"
	}
	return "IPv4"
}

func (dt *DebugToolbar) isInternalIP(ip string) bool {
	return strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "172.16.") ||
		ip == "127.0.0.1" ||
		ip == "::1"
}

func (dt *DebugToolbar) isPrivateIP(ip string) bool {
	return dt.isInternalIP(ip)
}

func getServerMemory() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]interface{}{
		"alloc":         m.Alloc / 1024,
		"total":         m.Sys / 1024,
		"num_goroutine": runtime.NumGoroutine(),
	}
}

func safeString(v interface{}) string {
	if v == nil {
		return "-"
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return "-"
		}
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatQueries(queries interface{}) string {
	if queries == nil {
		return "暂无查询记录"
	}
	q, ok := queries.([]map[string]interface{})
	if !ok || len(q) == 0 {
		return "暂无查询记录"
	}
	result := ""
	for i, query := range q {
		result += fmt.Sprintf("%d. [%s] %v ms\n   SQL: %v\n\n", i+1, safeString(query["time"]), query["duration"], query["sql"])
	}
	return result
}

func formatCacheKeys(keys interface{}) string {
	if keys == nil {
		return "暂无缓存键"
	}
	return "暂无缓存键"
}
