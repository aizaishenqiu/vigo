package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Job 任务接口
type Job interface {
	Handle() error  // 处理任务
	Retry() int     // 重试次数
	Delay() time.Duration // 延迟时间
}

// JobWrapper 任务包装器
type JobWrapper struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Data        interface{}   `json:"data"`
	RetryCount  int           `json:"retry_count"`
	MaxRetry    int           `json:"max_retry"`
	Delay       time.Duration `json:"delay"`
	CreatedAt   time.Time     `json:"created_at"`
	AvailableAt time.Time     `json:"available_at"` // 可执行时间（延迟队列）
	Priority    int           `json:"priority"`     // 优先级
}

// Driver 队列驱动接口
type Driver interface {
	Push(job *JobWrapper) error
	Pop(queue string) (*JobWrapper, error)
	Delete(job *JobWrapper) error
	Release(job *JobWrapper, delay time.Duration) error
	Peek(queue string) (*JobWrapper, error)
	Size(queue string) (int, error)
	Clear(queue string) error
}

// Queue 队列管理器
type Queue struct {
	driver      Driver
	handlers    map[string]func(*JobWrapper) error
	mu          sync.RWMutex
	running     bool
	wg          sync.WaitGroup
	stopChan    chan struct{}
}

// 全局队列实例
var defaultQueue *Queue

// DriverType 驱动类型
type DriverType string

const (
	DriverRedis    DriverType = "redis"
	DriverDatabase DriverType = "database"
	DriverRabbitMQ DriverType = "rabbitmq"
	DriverSync     DriverType = "sync"
)

// Config 队列配置
type Config struct {
	Driver      DriverType `yaml:"driver"`
	Queue       string     `yaml:"queue"`
	Connection  string     `yaml:"connection"`
	Sleep       int        `yaml:"sleep"`       // 无任务时休眠时间（毫秒）
	MaxTries    int        `yaml:"max_tries"`   // 最大重试次数
	Backoff     int        `yaml:"backoff"`     // 重试间隔（秒）
	Timeout     int        `yaml:"timeout"`     // 任务超时时间（秒）
	WorkerCount int        `yaml:"worker_count"` // 工作进程数量
}

// NewQueue 创建队列管理器
func NewQueue(driver Driver) *Queue {
	return &Queue{
		driver:   driver,
		handlers: make(map[string]func(*JobWrapper) error),
		stopChan: make(chan struct{}),
	}
}

// Default 获取默认队列实例
func Default() *Queue {
	if defaultQueue == nil {
		defaultQueue = NewQueue(NewSyncDriver())
	}
	return defaultQueue
}

// SetDefault 设置默认队列实例
func SetDefault(queue *Queue) {
	defaultQueue = queue
}

// Driver 获取驱动
func (q *Queue) Driver() Driver {
	return q.driver
}

// Push 推入任务
func (q *Queue) Push(job Job) error {
	wrapper := &JobWrapper{
		ID:          generateJobID(),
		Name:        getJobName(job),
		Data:        job,
		MaxRetry:    job.Retry(),
		Delay:       job.Delay(),
		CreatedAt:   time.Now(),
		AvailableAt: time.Now().Add(job.Delay()),
		Priority:    0,
	}

	return q.driver.Push(wrapper)
}

// PushWithDelay 推入延迟任务
func (q *Queue) PushWithDelay(job Job, delay time.Duration) error {
	wrapper := &JobWrapper{
		ID:          generateJobID(),
		Name:        getJobName(job),
		Data:        job,
		MaxRetry:    job.Retry(),
		Delay:       delay,
		CreatedAt:   time.Now(),
		AvailableAt: time.Now().Add(delay),
		Priority:    0,
	}

	return q.driver.Push(wrapper)
}

// PushWithPriority 推入优先级任务
func (q *Queue) PushWithPriority(job Job, priority int) error {
	wrapper := &JobWrapper{
		ID:          generateJobID(),
		Name:        getJobName(job),
		Data:        job,
		MaxRetry:    job.Retry(),
		Delay:       job.Delay(),
		CreatedAt:   time.Now(),
		AvailableAt: time.Now().Add(job.Delay()),
		Priority:    priority,
	}

	return q.driver.Push(wrapper)
}

// Pop 弹出任务
func (q *Queue) Pop(queue string) (*JobWrapper, error) {
	return q.driver.Pop(queue)
}

// Delete 删除任务
func (q *Queue) Delete(job *JobWrapper) error {
	return q.driver.Delete(job)
}

// Release 释放任务（重新推入队列）
func (q *Queue) Release(job *JobWrapper, delay time.Duration) error {
	job.RetryCount++
	job.AvailableAt = time.Now().Add(delay)
	return q.driver.Release(job, delay)
}

// Listen 监听队列（启动工作进程）
func (q *Queue) Listen(queue string, workerCount int) error {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return errors.New("queue is already running")
	}
	q.running = true
	q.mu.Unlock()

	// 启动多个工作进程
	for i := 0; i < workerCount; i++ {
		q.wg.Add(1)
		go q.worker(queue)
	}

	<-q.stopChan
	return nil
}

// worker 工作进程
func (q *Queue) worker(queue string) {
	defer q.wg.Done()

	for {
		select {
		case <-q.stopChan:
			return
		default:
			job, err := q.driver.Pop(queue)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if job == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// 检查任务是否可执行（延迟队列）
			if time.Now().Before(job.AvailableAt) {
				// 还未到执行时间，释放回队列
				q.Release(job, job.AvailableAt.Sub(time.Now()))
				continue
			}

			// 处理任务
			q.processJob(job)
		}
	}
}

// processJob 处理任务
func (q *Queue) processJob(job *JobWrapper) {
	// 查找处理器
	handler, ok := q.handlers[job.Name]
	if !ok {
		// 没有处理器，尝试直接调用 Job 的 Handle 方法
		if j, ok := job.Data.(Job); ok {
			q.executeJob(job, j)
		}
		return
	}

	// 使用处理器执行
	if err := handler(job); err != nil {
		// 执行失败，重试
		q.handleFailure(job, err)
	} else {
		// 执行成功，删除任务
		q.Delete(job)
	}
}

// executeJob 执行任务
func (q *Queue) executeJob(job *JobWrapper, j Job) {
	if err := j.Handle(); err != nil {
		q.handleFailure(job, err)
	} else {
		q.Delete(job)
	}
}

// handleFailure 处理失败
func (q *Queue) handleFailure(job *JobWrapper, err error) {
	if job.RetryCount < job.MaxRetry {
		// 重试
		backoff := time.Duration(job.RetryCount+1) * 10 * time.Second
		q.Release(job, backoff)
		fmt.Printf("Job %s failed, retrying... (%d/%d) Error: %v\n", job.ID, job.RetryCount+1, job.MaxRetry, err)
	} else {
		// 达到最大重试次数，删除任务
		q.Delete(job)
		fmt.Printf("Job %s failed after %d retries, deleting. Error: %v\n", job.ID, job.MaxRetry, err)
	}
}

// Register 注册任务处理器
func (q *Queue) Register(name string, handler func(*JobWrapper) error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[name] = handler
}

// Stop 停止队列
func (q *Queue) Stop() {
	close(q.stopChan)
	q.wg.Wait()
	q.running = false
}

// Size 获取队列大小
func (q *Queue) Size(queue string) (int, error) {
	return q.driver.Size(queue)
}

// Clear 清空队列
func (q *Queue) Clear(queue string) error {
	return q.driver.Clear(queue)
}

// Peek 查看队列头部任务
func (q *Queue) Peek(queue string) (*JobWrapper, error) {
	return q.driver.Peek(queue)
}

// 辅助函数
func generateJobID() string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), randomString(8))
}

func getJobName(job Job) string {
	// 使用类型名称作为任务名称
	return fmt.Sprintf("%T", job)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// MarshalJob 序列化任务
func MarshalJob(job *JobWrapper) ([]byte, error) {
	return json.Marshal(job)
}

// UnmarshalJob 反序列化任务
func UnmarshalJob(data []byte) (*JobWrapper, error) {
	var job JobWrapper
	err := json.Unmarshal(data, &job)
	return &job, err
}
