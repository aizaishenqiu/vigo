package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	counters   map[string]*int64
	gauges     map[string]*float64
	histograms map[string]*Histogram
	timers     map[string]*Timer
	mu         sync.RWMutex
}

// Histogram 直方图统计
type Histogram struct {
	name    string
	buckets []time.Duration
	counts  []int64
	total   int64
	sum     int64
	mu      sync.Mutex
}

// Timer 计时器
type Timer struct {
	name string
	hist *Histogram
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		counters:   make(map[string]*int64),
		gauges:     make(map[string]*float64),
		histograms: make(map[string]*Histogram),
		timers:     make(map[string]*Timer),
	}

	return mc
}

// IncCounter 增加计数器
func (mc *MetricsCollector) IncCounter(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.counters[name]; !exists {
		mc.counters[name] = new(int64)
	}
	atomic.AddInt64(mc.counters[name], 1)
}

// DecCounter 减少计数器
func (mc *MetricsCollector) DecCounter(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.counters[name]; !exists {
		mc.counters[name] = new(int64)
	}
	atomic.AddInt64(mc.counters[name], -1)
}

// SetGauge 设置仪表盘值
func (mc *MetricsCollector) SetGauge(name string, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.gauges[name]; !exists {
		mc.gauges[name] = new(float64)
	}
	*mc.gauges[name] = value
}

// IncGauge 增加仪表盘值
func (mc *MetricsCollector) IncGauge(name string, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.gauges[name]; !exists {
		mc.gauges[name] = new(float64)
	}
	*mc.gauges[name] += value
}

// DecGauge 减少仪表盘值
func (mc *MetricsCollector) DecGauge(name string, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.gauges[name]; !exists {
		mc.gauges[name] = new(float64)
	}
	*mc.gauges[name] -= value
}

// NewHistogram 创建直方图
func (mc *MetricsCollector) NewHistogram(name string, buckets []time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.histograms[name]; !exists {
		mc.histograms[name] = &Histogram{
			name:    name,
			buckets: buckets,
			counts:  make([]int64, len(buckets)+1), // 最后一个bucket是"无穷大"
		}
	}
}

// ObserveHistogram 记录直方图观察值
func (mc *MetricsCollector) ObserveHistogram(name string, value time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	hist, exists := mc.histograms[name]
	if !exists {
		// 如果直方图不存在，使用默认buckets创建
		defaultBuckets := []time.Duration{
			1 * time.Millisecond,
			5 * time.Millisecond,
			10 * time.Millisecond,
			25 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
			250 * time.Millisecond,
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
			5 * time.Second,
			10 * time.Second,
		}
		hist = &Histogram{
			name:    name,
			buckets: defaultBuckets,
			counts:  make([]int64, len(defaultBuckets)+1),
		}
		mc.histograms[name] = hist
	}

	hist.Observe(value)
}

// ObserveTimer 记录计时器观察值
func (mc *MetricsCollector) ObserveTimer(name string, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	timer, exists := mc.timers[name]
	if !exists {
		// 如果计时器不存在，创建新的
		defaultBuckets := []time.Duration{
			1 * time.Millisecond,
			5 * time.Millisecond,
			10 * time.Millisecond,
			25 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
			250 * time.Millisecond,
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
			5 * time.Second,
			10 * time.Second,
		}

		hist := &Histogram{
			name:    name + "_histogram",
			buckets: defaultBuckets,
			counts:  make([]int64, len(defaultBuckets)+1),
		}

		timer = &Timer{
			name: name,
			hist: hist,
		}
		mc.timers[name] = timer
	}

	timer.hist.Observe(duration)
}

// StartTimer 启动计时器
func (mc *MetricsCollector) StartTimer(name string) *TimerHandle {
	start := time.Now()
	return &TimerHandle{
		collector: mc,
		name:      name,
		start:     start,
	}
}

// GetCounter 获取计数器值
func (mc *MetricsCollector) GetCounter(name string) int64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if counter, exists := mc.counters[name]; exists {
		return atomic.LoadInt64(counter)
	}
	return 0
}

// GetGauge 获取仪表盘值
func (mc *MetricsCollector) GetGauge(name string) float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if gauge, exists := mc.gauges[name]; exists {
		return *gauge
	}
	return 0.0
}

// GetHistogram 获取直方图数据
func (mc *MetricsCollector) GetHistogram(name string) *HistogramData {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	hist, exists := mc.histograms[name]
	if !exists {
		return nil
	}

	hist.mu.Lock()
	defer hist.mu.Unlock()

	counts := make([]int64, len(hist.counts))
	copy(counts, hist.counts)

	return &HistogramData{
		Name:    hist.name,
		Buckets: hist.buckets,
		Counts:  counts,
		Total:   hist.total,
		Sum:     hist.sum,
	}
}

// TimerHandle 计时器句柄
type TimerHandle struct {
	collector *MetricsCollector
	name      string
	start     time.Time
}

// Stop 停止计时器并记录
func (th *TimerHandle) Stop() time.Duration {
	duration := time.Since(th.start)
	th.collector.ObserveTimer(th.name, duration)
	return duration
}

// Observe 直接记录持续时间
func (h *Histogram) Observe(value time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.total++
	h.sum += value.Nanoseconds()

	// 找到对应的bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			atomic.AddInt64(&h.counts[i], 1)
			return
		}
	}

	// 如果超过了所有bucket，放入最后一个"无穷大"bucket
	atomic.AddInt64(&h.counts[len(h.buckets)], 1)
}

// HistogramData 直方图数据
type HistogramData struct {
	Name    string
	Buckets []time.Duration
	Counts  []int64
	Total   int64
	Sum     int64
}

// GetPercentile 获取百分位数值
func (hd *HistogramData) GetPercentile(percentile float64) time.Duration {
	if hd.Total == 0 {
		return 0
	}

	target := int64(float64(hd.Total) * percentile / 100.0)

	var cumulative int64
	for i, count := range hd.Counts {
		cumulative += count
		if cumulative >= target {
			if i < len(hd.Buckets) {
				return hd.Buckets[i]
			}
			// 最后一个bucket，返回一个大的值
			return 24 * time.Hour
		}
	}

	return 0
}

// GlobalMetricsCollector 全局指标收集器
var GlobalMetricsCollector *MetricsCollector

// InitGlobalMetricsCollector 初始化全局指标收集器
func InitGlobalMetricsCollector() {
	GlobalMetricsCollector = NewMetricsCollector()
}

// GetGlobalMetricsCollector 获取全局指标收集器
func GetGlobalMetricsCollector() *MetricsCollector {
	if GlobalMetricsCollector == nil {
		InitGlobalMetricsCollector()
	}
	return GlobalMetricsCollector
}
