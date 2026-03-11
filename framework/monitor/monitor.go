package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Monitor 监控器
type Monitor struct {
	config      *MonitorConfig
	metrics     *Metrics
	alerts      []*AlertRule
	alertChan   chan *AlertEvent
	handlers    []AlertHandler
	mu          sync.RWMutex
	running     bool
	stopChan    chan struct{}
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled         bool          `yaml:"enabled"`
	CheckInterval   time.Duration `yaml:"check_interval"`
	AlertThreshold  int           `yaml:"alert_threshold"`
	EnableMetrics   bool          `yaml:"enable_metrics"`
	EnableAlerts    bool          `yaml:"enable_alerts"`
	EnableLogger    bool          `yaml:"enable_logger"`
	PushgatewayURL  string        `yaml:"pushgateway_url"`
}

// Metrics 指标数据
type Metrics struct {
	// 系统指标
	CPUUsage       float64   `json:"cpu_usage"`
	MemoryUsage    uint64    `json:"memory_usage"`
	MemoryTotal    uint64    `json:"memory_total"`
	Goroutines     int       `json:"goroutines"`
	GCAllocations  uint64    `json:"gc_allocations"`
	GCPauses       uint64    `json:"gc_pauses"`
	
	// 请求指标
	RequestTotal   int64     `json:"request_total"`
	RequestQPS     float64   `json:"request_qps"`
	RequestLatency float64   `json:"request_latency"`
	RequestErrors  int64     `json:"request_errors"`
	
	// 数据库指标
	DBConnections  int       `json:"db_connections"`
	DBActive       int       `json:"db_active"`
	DBIdle         int       `json:"db_idle"`
	DBQueries      int64     `json:"db_queries"`
	DBQueryTime    float64   `json:"db_query_time"`
	
	// 缓存指标
	CacheHits      int64     `json:"cache_hits"`
	CacheMisses    int64     `json:"cache_misses"`
	CacheHitRate   float64   `json:"cache_hit_rate"`
	
	// 队列指标
	QueueSize      int       `json:"queue_size"`
	QueueProcessed int64     `json:"queue_processed"`
	QueueFailed    int64     `json:"queue_failed"`
	
	Timestamp      time.Time `json:"timestamp"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name      string        `json:"name"`
	Metric    string        `json:"metric"`
	Operator  string        `json:"operator"` // >, <, >=, <=, ==, !=
	Threshold float64       `json:"threshold"`
	Duration  time.Duration `json:"duration"`
	Level     string        `json:"level"` // warning, critical
	Message   string        `json:"message"`
	Enabled   bool          `json:"enabled"`
}

// AlertEvent 告警事件
type AlertEvent struct {
	Rule      *AlertRule  `json:"rule"`
	Value     float64     `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
	Level     string      `json:"level"`
	Message   string      `json:"message"`
}

// AlertHandler 告警处理器
type AlertHandler interface {
	Handle(event *AlertEvent) error
}

// NewMonitor 创建监控器
func NewMonitor(config *MonitorConfig) *Monitor {
	return &Monitor{
		config:    config,
		metrics:   &Metrics{},
		alerts:    make([]*AlertRule, 0),
		alertChan: make(chan *AlertEvent, 100),
		handlers:  make([]AlertHandler, 0),
		stopChan:  make(chan struct{}),
	}
}

// Start 启动监控
func (m *Monitor) Start() {
	if !m.config.Enabled {
		return
	}

	m.running = true
	
	// 启动指标收集
	go m.collectMetrics()
	
	// 启动告警检测
	go m.checkAlerts()
	
	// 启动告警处理
	go m.processAlerts()
}

// Stop 停止监控
func (m *Monitor) Stop() {
	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)
}

// collectMetrics 收集指标
func (m *Monitor) collectMetrics() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.collectSystemMetrics()
		case <-m.stopChan:
			return
		}
	}
}

// collectSystemMetrics 收集系统指标
func (m *Monitor) collectSystemMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mu.Lock()
	defer m.mu.Unlock()

	// CPU 使用率（简化实现）
	m.metrics.CPUUsage = 0.0 // 需要实际实现

	// 内存使用
	m.metrics.MemoryUsage = memStats.Alloc
	m.metrics.MemoryTotal = memStats.Sys
	m.metrics.GCAllocations = memStats.Mallocs
	m.metrics.GCPauses = memStats.PauseTotalNs

	// Goroutine 数量
	m.metrics.Goroutines = runtime.NumGoroutine()

	// 时间戳
	m.metrics.Timestamp = time.Now()
}

// checkAlerts 检查告警
func (m *Monitor) checkAlerts() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAlertRules()
		case <-m.stopChan:
			return
		}
	}
}

// checkAlertRules 检查告警规则
func (m *Monitor) checkAlertRules() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.alerts {
		if !rule.Enabled {
			continue
		}

		value := m.getMetricValue(rule.Metric)
		if m.evaluateRule(rule, value) {
			event := &AlertEvent{
				Rule:      rule,
				Value:     value,
				Timestamp: time.Now(),
				Level:     rule.Level,
				Message:   rule.Message,
			}

			select {
			case m.alertChan <- event:
			default:
				// 通道已满，丢弃
			}
		}
	}
}

// evaluateRule 评估规则
func (m *Monitor) evaluateRule(rule *AlertRule, value float64) bool {
	switch rule.Operator {
	case ">":
		return value > rule.Threshold
	case "<":
		return value < rule.Threshold
	case ">=":
		return value >= rule.Threshold
	case "<=":
		return value <= rule.Threshold
	case "==":
		return value == rule.Threshold
	case "!=":
		return value != rule.Threshold
	default:
		return false
	}
}

// getMetricValue 获取指标值
func (m *Monitor) getMetricValue(metric string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch metric {
	case "cpu_usage":
		return m.metrics.CPUUsage
	case "memory_usage":
		return float64(m.metrics.MemoryUsage)
	case "goroutines":
		return float64(m.metrics.Goroutines)
	case "request_qps":
		return m.metrics.RequestQPS
	case "request_latency":
		return m.metrics.RequestLatency
	case "request_errors":
		return float64(m.metrics.RequestErrors)
	case "db_connections":
		return float64(m.metrics.DBConnections)
	case "cache_hit_rate":
		return m.metrics.CacheHitRate
	default:
		return 0
	}
}

// processAlerts 处理告警
func (m *Monitor) processAlerts() {
	for {
		select {
		case event := <-m.alertChan:
			m.handleAlert(event)
		case <-m.stopChan:
			return
		}
	}
}

// handleAlert 处理告警事件
func (m *Monitor) handleAlert(event *AlertEvent) {
	for _, handler := range m.handlers {
		if err := handler.Handle(event); err != nil {
			// 记录错误
			fmt.Printf("处理告警失败：%v\n", err)
		}
	}
}

// AddAlertRule 添加告警规则
func (m *Monitor) AddAlertRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, rule)
}

// RemoveAlertRule 移除告警规则
func (m *Monitor) RemoveAlertRule(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, rule := range m.alerts {
		if rule.Name == name {
			m.alerts = append(m.alerts[:i], m.alerts[i+1:]...)
			break
		}
	}
}

// AddHandler 添加告警处理器
func (m *Monitor) AddHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// GetMetrics 获取指标数据
func (m *Monitor) GetMetrics() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// RecordRequest 记录请求指标
func (m *Monitor) RecordRequest(latency time.Duration, isError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.RequestTotal++
	m.metrics.RequestLatency = float64(latency.Nanoseconds()) / 1e6 // ms

	if isError {
		m.metrics.RequestErrors++
	}

	// 计算 QPS（简化）
	m.metrics.RequestQPS = float64(m.metrics.RequestTotal) / time.Since(m.metrics.Timestamp).Seconds()
}

// RecordDBQuery 记录数据库查询
func (m *Monitor) RecordDBQuery(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.DBQueries++
	m.metrics.DBQueryTime += float64(duration.Nanoseconds()) / 1e6
}

// RecordCache 记录缓存命中
func (m *Monitor) RecordCache(hit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hit {
		m.metrics.CacheHits++
	} else {
		m.metrics.CacheMisses++
	}

	total := m.metrics.CacheHits + m.metrics.CacheMisses
	if total > 0 {
		m.metrics.CacheHitRate = float64(m.metrics.CacheHits) / float64(total)
	}
}

// EmailAlertHandler 邮件告警处理器
type EmailAlertHandler struct {
	SMTPServer string
	SMTPPort   int
	Username   string
	Password   string
	From       string
	To         []string
}

// Handle 处理告警
func (h *EmailAlertHandler) Handle(event *AlertEvent) error {
	// 实现邮件发送逻辑
	subject := fmt.Sprintf("[%s] 告警通知：%s", event.Level, event.Rule.Name)
	body := fmt.Sprintf("告警规则：%s\n当前值：%.2f\n阈值：%.2f\n时间：%s\n消息：%s",
		event.Rule.Name,
		event.Value,
		event.Rule.Threshold,
		event.Timestamp.Format("2006-01-02 15:04:05"),
		event.Message,
	)

	// 发送邮件（简化实现）
	fmt.Printf("发送邮件告警：%s\n%s\n", subject, body)
	return nil
}

// WebhookAlertHandler Webhook 告警处理器
type WebhookAlertHandler struct {
	URL    string
	Secret string
}

// Handle 处理告警
func (h *WebhookAlertHandler) Handle(event *AlertEvent) error {
	payload := map[string]interface{}{
		"rule":      event.Rule.Name,
		"value":     event.Value,
		"level":     event.Level,
		"message":   event.Message,
		"timestamp": event.Timestamp.Unix(),
	}

	data, _ := json.Marshal(payload)

	// 发送 webhook
	resp, err := http.Post(h.URL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// LogAlertHandler 日志告警处理器
type LogAlertHandler struct {
	FilePath string
}

// Handle 处理告警
func (h *LogAlertHandler) Handle(event *AlertEvent) error {
	logLine := fmt.Sprintf("[%s] %s - %s: %.2f (阈值：%.2f)\n",
		event.Timestamp.Format("2006-01-02 15:04:05"),
		event.Level,
		event.Rule.Name,
		event.Value,
		event.Rule.Threshold,
	)

	// 写入日志文件（简化实现）
	fmt.Print(logLine)
	return nil
}

// DefaultAlertRules 默认告警规则
func DefaultAlertRules() []*AlertRule {
	return []*AlertRule{
		{
			Name:      "CPU 使用率过高",
			Metric:    "cpu_usage",
			Operator:  ">",
			Threshold: 80.0,
			Duration:  5 * time.Minute,
			Level:     "warning",
			Message:   "CPU 使用率超过 80%",
			Enabled:   true,
		},
		{
			Name:      "内存使用过高",
			Metric:    "memory_usage",
			Operator:  ">",
			Threshold: 1024 * 1024 * 1024, // 1GB
			Duration:  5 * time.Minute,
			Level:     "warning",
			Message:   "内存使用超过 1GB",
			Enabled:   true,
		},
		{
			Name:      "Goroutine 数量过多",
			Metric:    "goroutines",
			Operator:  ">",
			Threshold: 1000,
			Duration:  5 * time.Minute,
			Level:     "warning",
			Message:   "Goroutine 数量超过 1000",
			Enabled:   true,
		},
		{
			Name:      "请求错误率过高",
			Metric:    "request_errors",
			Operator:  ">",
			Threshold: 100,
			Duration:  1 * time.Minute,
			Level:     "critical",
			Message:   "请求错误数超过 100",
			Enabled:   true,
		},
		{
			Name:      "数据库连接过多",
			Metric:    "db_connections",
			Operator:  ">",
			Threshold: 100,
			Duration:  5 * time.Minute,
			Level:     "warning",
			Message:   "数据库连接数超过 100",
			Enabled:   true,
		},
	}
}
