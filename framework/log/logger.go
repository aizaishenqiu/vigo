package log

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// Logger 日志接口（统一签名，兼容 contract.Logger）
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
}

// FileLogger 带缓冲的文件日志实现
// 解决原来每次写入都 open/close 文件导致高并发性能极差的问题
type FileLogger struct {
	logDir  string
	level   Level
	mu      sync.Mutex
	files   map[string]*fileWriter
	console bool // 是否同时输出到控制台
}

type fileWriter struct {
	file   *os.File
	writer *bufio.Writer
	date   string // 当前日期，用于按天切割
}

// NewFileLogger 创建文件日志记录器
func NewFileLogger(dir string) *FileLogger {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	l := &FileLogger{
		logDir:  dir,
		level:   LevelDebug,
		files:   make(map[string]*fileWriter),
		console: true,
	}
	// 启动定期刷新和文件轮转检查
	go l.flushLoop()
	return l
}

// SetLevel 设置最低日志级别
func (l *FileLogger) SetLevel(level Level) {
	l.level = level
}

// SetConsole 设置是否输出到控制台
func (l *FileLogger) SetConsole(enabled bool) {
	l.console = enabled
}

// flushLoop 定期刷新缓冲区到磁盘 + 清理过期文件句柄
func (l *FileLogger) flushLoop() {
	ticker := time.NewTicker(3 * time.Second)
	for range ticker.C {
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
	}
}

// getWriter 获取（或创建）指定级别的缓冲写入器
func (l *FileLogger) getWriter(level string) *bufio.Writer {
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

func (l *FileLogger) write(level Level, levelStr string, args ...interface{}) {
	if level < l.level {
		return
	}

	msg := fmt.Sprint(args...)
	content := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), levelStr, msg)

	l.mu.Lock()
	if w := l.getWriter(levelStr); w != nil {
		w.WriteString(content)
	}
	l.mu.Unlock()

	if l.console {
		fmt.Print(content)
	}
}

func (l *FileLogger) Debug(args ...interface{}) { l.write(LevelDebug, "DEBUG", args...) }
func (l *FileLogger) Info(args ...interface{})  { l.write(LevelInfo, "INFO", args...) }
func (l *FileLogger) Warn(args ...interface{})  { l.write(LevelWarn, "WARN", args...) }
func (l *FileLogger) Error(args ...interface{}) { l.write(LevelError, "ERROR", args...) }
func (l *FileLogger) Fatal(args ...interface{}) {
	l.write(LevelFatal, "FATAL", args...)
	os.Exit(1)
}

// Close 关闭所有文件句柄（优雅关闭时调用）
func (l *FileLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, fw := range l.files {
		fw.writer.Flush()
		fw.file.Close()
	}
	l.files = make(map[string]*fileWriter)
}

// ==================== 全局实例 ====================

var Log Logger

func Init(logDir string) {
	Log = NewFileLogger(logDir)
}
