package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// StressTestReq 压力测试请求
type StressTestReq struct {
	URL           string            `json:"url"`            // 测试 URL
	Method        string            `json:"method"`         // HTTP 方法
	Concurrency   int               `json:"concurrency"`    // 并发数
	TotalRequests int               `json:"total_requests"` // 总请求数
	Timeout       int               `json:"timeout"`        // 超时时间 (秒)
	Body          string            `json:"body"`           // 请求体
	ContentType   string            `json:"content_type"`   // Content-Type
	Headers       map[string]string `json:"headers"`        // 自定义请求头
}

// StressTestProgress 压力测试进度
type StressTestProgress struct {
	ID            string  `json:"id"`             // 测试 ID
	Status        string  `json:"status"`         // running, completed, failed
	TotalRequests int     `json:"total_requests"` // 总请求数
	Completed     int32   `json:"completed"`      // 已完成请求数
	Success       int32   `json:"success"`        // 成功请求数
	Failed        int32   `json:"failed"`         // 失败请求数
	TotalBytes    int64   `json:"total_bytes"`    // 总字节数
	Duration      string  `json:"duration"`       // 已运行时长
	QPS           float64 `json:"qps"`            // 每秒请求数
	AvgLatency    string  `json:"avg_latency"`    // 平均延迟
	MinLatency    string  `json:"min_latency"`    // 最小延迟
	MaxLatency    string  `json:"max_latency"`    // 最大延迟
	P99Latency    string  `json:"p99_latency"`    // P99 延迟
	ErrorRate     float64 `json:"error_rate"`     // 错误率
	StartTime     int64   `json:"start_time"`     // 开始时间
}

// StressTestManager 压力测试管理器
type StressTestManager struct {
	mu         sync.RWMutex
	running    map[string]*StressTestProgress
	results    map[string]*StressTestProgress
	dataDir    string
	maxResults int
}

// NewStressProgress 创建新的压力测试进度
func NewStressProgress(req StressTestReq) *StressTestProgress {
	return &StressTestProgress{
		Status:        "running",
		TotalRequests: req.TotalRequests,
		StartTime:     time.Now().UnixNano() / 1e6,
	}
}

// GlobalStressManager 全局压力测试管理器
var GlobalStressManager *StressTestManager

func init() {
	dataDir := filepath.Join("runtime", "stress")
	GlobalStressManager = &StressTestManager{
		running:    make(map[string]*StressTestProgress),
		results:    make(map[string]*StressTestProgress),
		dataDir:    dataDir,
		maxResults: 100,
	}
	GlobalStressManager.loadResults()
}

func (m *StressTestManager) loadResults() {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		log.Printf("[StressTest] 创建数据目录失败: %v", err)
		return
	}

	files, err := os.ReadDir(m.dataDir)
	if err != nil {
		log.Printf("[StressTest] 读取数据目录失败: %v", err)
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(m.dataDir, file.Name()))
		if err != nil {
			continue
		}

		var progress StressTestProgress
		if err := json.Unmarshal(data, &progress); err != nil {
			continue
		}

		m.results[progress.ID] = &progress
	}

	log.Printf("[StressTest] 已加载 %d 条历史测试结果", len(m.results))
}

func (m *StressTestManager) saveResult(progress *StressTestProgress) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(m.dataDir, progress.ID+".json")
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (m *StressTestManager) deleteResult(testID string) error {
	filename := filepath.Join(m.dataDir, testID+".json")
	return os.Remove(filename)
}

func (m *StressTestManager) cleanupOldResults() {
	if len(m.results) <= m.maxResults {
		return
	}

	var oldestID string
	var oldestTime int64 = -1

	for id, progress := range m.results {
		if oldestTime == -1 || progress.StartTime < oldestTime {
			oldestTime = progress.StartTime
			oldestID = id
		}
	}

	if oldestID != "" {
		m.deleteResult(oldestID)
		delete(m.results, oldestID)
		log.Printf("[StressTest] 已清理旧测试结果: %s", oldestID)
	}
}

// StartStressTest 启动压力测试
func (m *StressTestManager) StartStressTest(req StressTestReq) (string, error) {
	// 生成测试 ID
	testID := fmt.Sprintf("stress_%d", time.Now().UnixNano())

	// 创建测试进度
	progress := &StressTestProgress{
		ID:            testID,
		Status:        "running",
		TotalRequests: req.TotalRequests,
		StartTime:     time.Now().UnixNano() / 1e6,
	}

	// 保存测试
	m.mu.Lock()
	m.running[testID] = progress
	m.mu.Unlock()

	// 启动测试协程
	go m.runStressTest(testID, req, progress)

	// 通知 WebSocket 客户端
	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("stress", WSMessage{
			Type:    "stress_progress",
			Channel: "stress",
			Action:  "start",
			Data:    progress,
		})
	}

	return testID, nil
}

// runStressTest 执行压力测试
func (m *StressTestManager) runStressTest(testID string, req StressTestReq, progress *StressTestProgress) {
	startTime := time.Now()

	// 创建客户端
	client := &http.Client{
		Timeout: time.Duration(req.Timeout) * time.Second,
	}

	// 创建信号量控制并发
	sem := make(chan struct{}, req.Concurrency)

	// 延迟统计
	var latencies []int64
	var latencyMu sync.Mutex

	// WaitGroup 等待所有请求完成
	var wg sync.WaitGroup

	// 上下文用于取消
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动进度上报协程
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			// 计算实时统计
			completed := atomic.LoadInt32(&progress.Completed)
			success := atomic.LoadInt32(&progress.Success)
			failed := atomic.LoadInt32(&progress.Failed)

			// 计算实时 QPS
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				progress.QPS = float64(success) / elapsed
				progress.ErrorRate = float64(failed) / float64(completed) * 100
			}

			// 计算延迟统计
			latencyMu.Lock()
			progress.AvgLatency = formatDuration2(calculateAvgLatency(latencies))
			minLatency, maxLatency := calculateMinMaxLatency(latencies)
			progress.MinLatency = formatDuration2(minLatency)
			progress.MaxLatency = formatDuration2(maxLatency)
			progress.P99Latency = formatDuration2(calculateP99Latency(latencies))
			latencyMu.Unlock()

			// 推送进度
			if GlobalWSManager != nil {
				GlobalWSManager.BroadcastToChannel("stress", WSMessage{
					Type:    "stress_progress",
					Channel: "stress",
					Data:    progress,
				})
			}

			// 如果测试已完成，退出
			if completed >= int32(req.TotalRequests) {
				return
			}
		}
	}()

	// 发送请求
	for i := 0; i < req.TotalRequests; i++ {
		sem <- struct{}{}
		wg.Add(1)

		go func(idx int) {
			defer func() {
				<-sem
				wg.Done()
			}()

			select {
			case <-ctx.Done():
				return
			default:
			}

			startReq := time.Now()
			_, bytes := m.sendRequest(client, req)
			latency := time.Since(startReq).Nanoseconds()

			atomic.AddInt32(&progress.Completed, 1)
			atomic.AddInt32(&progress.Success, 1)
			atomic.AddInt64(&progress.TotalBytes, bytes)

			latencyMu.Lock()
			latencies = append(latencies, latency)
			latencyMu.Unlock()
		}(i)
	}

	// 等待所有请求完成
	wg.Wait()

	// 计算最终统计
	elapsed := time.Since(startTime)
	completed := atomic.LoadInt32(&progress.Completed)
	success := atomic.LoadInt32(&progress.Success)
	failed := atomic.LoadInt32(&progress.Failed)

	progress.Status = "completed"
	progress.Duration = formatDuration2(elapsed)
	progress.QPS = float64(completed) / elapsed.Seconds()

	latencyMu.Lock()
	progress.AvgLatency = formatDuration2(calculateAvgLatency(latencies))
	minLatency, maxLatency := calculateMinMaxLatency(latencies)
	progress.MinLatency = formatDuration2(minLatency)
	progress.MaxLatency = formatDuration2(maxLatency)
	progress.P99Latency = formatDuration2(calculateP99Latency(latencies))
	latencyMu.Unlock()

	if completed > 0 {
		progress.ErrorRate = float64(failed) / float64(completed) * 100
	}

	// 移动测试结果
	m.mu.Lock()
	delete(m.running, testID)
	m.results[testID] = progress
	m.mu.Unlock()

	// 持久化保存测试结果
	if err := m.saveResult(progress); err != nil {
		log.Printf("[StressTest] 保存测试结果失败: %v", err)
	}

	// 清理旧结果
	m.cleanupOldResults()

	// 发送最终结果
	if GlobalWSManager != nil {
		GlobalWSManager.BroadcastToChannel("stress", WSMessage{
			Type:    "stress_completed",
			Channel: "stress",
			Action:  "completed",
			Data:    progress,
		})
	}

	log.Printf("[StressTest] 测试完成：%s, 总请求：%d, 成功：%d, 失败：%d, QPS: %.2f",
		testID, completed, success, failed, progress.QPS)
}

// sendRequest 发送单个 HTTP 请求
func (m *StressTestManager) sendRequest(client *http.Client, req StressTestReq) (bool, int64) {
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = bytes.NewReader([]byte(req.Body))
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return false, 0
	}

	// 设置请求头
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// 发送请求
	resp, err := client.Do(httpReq)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, 0
	}

	// 判断是否成功
	success := resp.StatusCode >= 200 && resp.StatusCode < 400

	return success, int64(len(body))
}

// GetStressProgress 获取测试进度
func (m *StressTestManager) GetStressProgress(testID string) *StressTestProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if progress, ok := m.running[testID]; ok {
		return progress
	}
	if progress, ok := m.results[testID]; ok {
		return progress
	}

	return nil
}

// GetStressResults 获取所有测试结果
func (m *StressTestManager) GetStressResults() []*StressTestProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*StressTestProgress, 0, len(m.results))
	for _, progress := range m.results {
		results = append(results, progress)
	}

	return results
}

// DeleteStressResult 删除测试结果
func (m *StressTestManager) DeleteStressResult(testID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.results[testID]; !ok {
		return fmt.Errorf("测试结果不存在: %s", testID)
	}

	if err := m.deleteResult(testID); err != nil {
		return err
	}

	delete(m.results, testID)
	return nil
}

// ClearAllResults 清空所有测试结果
func (m *StressTestManager) ClearAllResults() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for testID := range m.results {
		m.deleteResult(testID)
	}

	m.results = make(map[string]*StressTestProgress)
	return nil
}

// handleStressUpdate 处理压力测试 WebSocket 更新
func handleStressUpdate(msg WSMessage) {
	if msg.Action == "start" {
		if data, ok := msg.Data.(map[string]interface{}); ok {
			req := StressTestReq{
				URL:           getString(data, "url"),
				Method:        getString(data, "method"),
				Concurrency:   getInt(data, "concurrency"),
				TotalRequests: getInt(data, "total_requests"),
				Timeout:       getInt(data, "timeout"),
				Body:          getString(data, "body"),
				ContentType:   getString(data, "content_type"),
			}

			if req.Concurrency <= 0 {
				req.Concurrency = 10
			}
			if req.TotalRequests <= 0 {
				req.TotalRequests = 100
			}
			if req.Timeout <= 0 {
				req.Timeout = 30
			}
			if req.Method == "" {
				req.Method = "GET"
			}

			GlobalStressManager.StartStressTest(req)
		}
	}
}

// 辅助函数
func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
}

func formatDuration2(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func calculateAvgLatency(latencies []int64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var sum int64
	for _, l := range latencies {
		sum += l
	}

	return time.Duration(sum / int64(len(latencies)))
}

func calculateMinMaxLatency(latencies []int64) (time.Duration, time.Duration) {
	if len(latencies) == 0 {
		return 0, 0
	}

	min := latencies[0]
	max := latencies[0]

	for _, l := range latencies[1:] {
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
	}

	return time.Duration(min), time.Duration(max)
}

func calculateP99Latency(latencies []int64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// 排序
	sorted := make([]int64, len(latencies))
	copy(sorted, latencies)

	// 简单排序（实际应该用更高效的算法）
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// P99
	index := int(float64(len(sorted)) * 0.99)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return time.Duration(sorted[index])
}
