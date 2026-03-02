package log

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AsyncFileLogger 异步文件日志记录器
// 使用 channel + goroutine 实现异步写入，大幅提升高并发性能
type AsyncFileLogger struct {
	logDir        string
	level         Level
	console       bool
	logChan       chan *LogEntry
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	files         map[string]*fileWriter
	maxBuffer     int           // 最大缓冲日志数
	flushInterval time.Duration // 刷新间隔
}

// LogEntry 日志条目
type LogEntry struct {
	Level    Level
	LevelStr string
	Message  string
	Time     time.Time
}

// NewAsyncFileLogger 创建异步文件日志记录器
// 参数:
//   - dir: 日志目录
//   - maxBuffer: 最大缓冲日志数（默认 10000）
//   - flushInterval: 刷新间隔（默认 1 秒）
func NewAsyncFileLogger(dir string, maxBuffer int, flushInterval time.Duration) *AsyncFileLogger {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	if maxBuffer <= 0 {
		maxBuffer = 10000
	}
	if flushInterval <= 0 {
		flushInterval = 1 * time.Second
	}

	l := &AsyncFileLogger{
		logDir:        dir,
		level:         LevelDebug,
		console:       true,
		logChan:       make(chan *LogEntry, maxBuffer),
		stopChan:      make(chan struct{}),
		files:         make(map[string]*fileWriter),
		maxBuffer:     maxBuffer,
		flushInterval: flushInterval,
	}

	// 启动异步写入 goroutine
	l.wg.Add(1)
	go l.writeLoop()

	// 启动定期刷新和文件轮转检查
	go l.flushLoop()

	return l
}

// writeLoop 异步写入循环
func (l *AsyncFileLogger) writeLoop() {
	defer l.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[AsyncLog] Panic recovered in writeLoop: %v", r)
			// 重启 writeLoop，确保日志继续写入
			go l.writeLoop()
		}
	}()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	var buffer []*LogEntry
	bufferSize := 100 // 批量写入大小

	for {
		select {
		case entry, ok := <-l.logChan:
			if !ok {
				// 通道关闭，写入剩余日志
				if len(buffer) > 0 {
					l.writeBatch(buffer)
				}
				return
			}

			buffer = append(buffer, entry)

			// 达到批量大小，立即写入
			if len(buffer) >= bufferSize {
				l.writeBatch(buffer)
				buffer = buffer[:0]
			}

		case <-ticker.C:
			// 定时写入
			if len(buffer) > 0 {
				l.writeBatch(buffer)
				buffer = buffer[:0]
			}

		case <-l.stopChan:
			// 收到停止信号，写入剩余日志
			if len(buffer) > 0 {
				l.writeBatch(buffer)
			}
			// 清空通道中的剩余日志
			for len(l.logChan) > 0 {
				entry := <-l.logChan
				buffer = append(buffer, entry)
				if len(buffer) >= bufferSize {
					l.writeBatch(buffer)
					buffer = buffer[:0]
				}
			}
			if len(buffer) > 0 {
				l.writeBatch(buffer)
			}
			return
		}
	}
}

// writeBatch 批量写入日志
func (l *AsyncFileLogger) writeBatch(entries []*LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range entries {
		l.writeEntry(entry)
	}
}

// writeEntry 写入单条日志
func (l *AsyncFileLogger) writeEntry(entry *LogEntry) {
	content := fmt.Sprintf("[%s] [%s] %s\n",
		entry.Time.Format("2006-01-02 15:04:05"),
		entry.LevelStr,
		entry.Message)

	if w := l.getWriter(entry.LevelStr); w != nil {
		w.WriteString(content)
	}

	if l.console {
		fmt.Print(content)
	}
}

// flushLoop 定期刷新缓冲区到磁盘 + 清理过期文件句柄
func (l *AsyncFileLogger) flushLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			today := time.Now().Format("2006-01-02")
			for key, fw := range l.files {
				if fw.date != today {
					// 日期变更，关闭旧文件
					fw.writer.Flush()
					fw.file.Close()
					delete(l.files, key)
				} else {
					fw.writer.Flush()
				}
			}
			l.mu.Unlock()

		case <-l.stopChan:
			return
		}
	}
}

// getWriter 获取（或创建）指定级别的缓冲写入器
func (l *AsyncFileLogger) getWriter(level string) *bufio.Writer {
	today := time.Now().Format("2006-01-02")
	key := today + "-" + level

	if fw, ok := l.files[key]; ok && fw.date == today {
		return fw.writer
	}

	// 关闭所有过期日期的文件句柄，避免泄漏
	var toDelete []string
	for k, fw := range l.files {
		if fw.date != today && k != key {
			fw.writer.Flush()
			fw.file.Close()
			toDelete = append(toDelete, k)
		}
	}
	for _, k := range toDelete {
		delete(l.files, k)
	}

	filename := fmt.Sprintf("%s-%s.log", today, level)
	path := filepath.Join(l.logDir, filename)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("[Log] 无法打开日志文件 %s: %v\n", path, err)
		return nil
	}

	fw := &fileWriter{
		file:   file,
		writer: bufio.NewWriterSize(file, 8192), // 8KB 缓冲区
		date:   today,
	}
	l.files[key] = fw
	return fw.writer
}

func (l *AsyncFileLogger) write(level Level, levelStr string, args ...interface{}) {
	if level < l.level {
		return
	}

	msg := fmt.Sprint(args...)

	// 创建日志条目
	entry := &LogEntry{
		Level:    level,
		LevelStr: levelStr,
		Message:  msg,
		Time:     time.Now(),
	}

	// 非阻塞发送到通道
	select {
	case l.logChan <- entry:
		// 发送成功
	default:
		// 通道已满，丢弃日志并告警
		fmt.Printf("[Log] 日志缓冲区已满，丢弃日志：%s\n", msg)
	}
}

func (l *AsyncFileLogger) Debug(args ...interface{}) { l.write(LevelDebug, "DEBUG", args...) }
func (l *AsyncFileLogger) Info(args ...interface{})  { l.write(LevelInfo, "INFO", args...) }
func (l *AsyncFileLogger) Warn(args ...interface{})  { l.write(LevelWarn, "WARN", args...) }
func (l *AsyncFileLogger) Error(args ...interface{}) { l.write(LevelError, "ERROR", args...) }
func (l *AsyncFileLogger) Fatal(args ...interface{}) {
	l.write(LevelFatal, "FATAL", args...)
	l.Close()
	os.Exit(1)
}

// Close 关闭日志记录器（优雅关闭）
func (l *AsyncFileLogger) Close() {
	close(l.stopChan)
	l.wg.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, fw := range l.files {
		fw.writer.Flush()
		fw.file.Close()
	}
	l.files = make(map[string]*fileWriter)
}

// SetLevel 设置最低日志级别
func (l *AsyncFileLogger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetConsole 设置是否输出到控制台
func (l *AsyncFileLogger) SetConsole(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.console = enabled
}

// GetQueueSize 获取当前队列大小
func (l *AsyncFileLogger) GetQueueSize() int {
	return len(l.logChan)
}

// GetStats 获取日志统计
func (l *AsyncFileLogger) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"queue_size":     len(l.logChan),
		"max_buffer":     l.maxBuffer,
		"flush_interval": l.flushInterval.String(),
		"file_count":     len(l.files),
	}
}

// ==================== 全局异步日志实例 ====================

var AsyncLog *AsyncFileLogger

// InitAsync 初始化全局异步日志
// 参数:
//   - logDir: 日志目录
//   - maxBuffer: 最大缓冲日志数（可选，默认 10000）
//   - flushInterval: 刷新间隔（可选，默认 1 秒）
func InitAsync(logDir string, maxBuffer int, flushInterval time.Duration) {
	AsyncLog = NewAsyncFileLogger(logDir, maxBuffer, flushInterval)
}

// CloseAsync 关闭全局异步日志
func CloseAsync() {
	if AsyncLog != nil {
		AsyncLog.Close()
	}
}

// GetAsyncLogStats 获取全局异步日志统计
func GetAsyncLogStats() map[string]interface{} {
	if AsyncLog == nil {
		return nil
	}
	return AsyncLog.GetStats()
}
