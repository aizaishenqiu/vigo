package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron      *cron.Cron
	tasks     map[string]*Task
	mu        sync.RWMutex
	running   bool
	location  *time.Location
}

// Task 定时任务
type Task struct {
	Name        string                 `json:"name"`
	Spec        string                 `json:"spec"` // Cron 表达式
	Handler     func()                 `json:"-"`
	HandlerWithCtx func(context.Context) `json:"-"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Running     bool                   `json:"running"`
	LastRun     time.Time              `json:"last_run"`
	NextRun     time.Time              `json:"next_run"`
	TotalRuns   int                    `json:"total_runs"`
	Metadata    map[string]interface{} `json:"metadata"`
	cronEntryID cron.EntryID
}

// TaskStatus 任务状态
type TaskStatus struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Running     bool      `json:"running"`
	LastRun     time.Time `json:"last_run"`
	NextRun     time.Time `json:"next_run"`
	TotalRuns   int       `json:"total_runs"`
}

// SchedulerOptions 调度器选项
type SchedulerOptions struct {
	// 时区
	Location *time.Location `yaml:"location"`
	// 是否立即执行第一次
	RunImmediately bool `yaml:"run_immediately"`
}

// NewScheduler 创建定时任务调度器
func NewScheduler(opts *SchedulerOptions) *Scheduler {
	if opts == nil {
		opts = &SchedulerOptions{}
	}

	var loc *time.Location
	if opts.Location != nil {
		loc = opts.Location
	} else {
		loc = time.Local
	}

	// 创建 cron 调度器
	c := cron.New(
		cron.WithLocation(loc),
		cron.WithSeconds(), // 支持秒级 cron
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
		),
	)

	return &Scheduler{
		cron:     c,
		tasks:    make(map[string]*Task),
		location: loc,
	}
}

// AddTask 添加定时任务
func (s *Scheduler) AddTask(name, spec string, handler func()) error {
	return s.AddTaskWithDesc(name, spec, "", handler)
}

// AddTaskWithDesc 添加带描述的定时任务
func (s *Scheduler) AddTaskWithDesc(name, spec, description string, handler func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[name]; exists {
		return fmt.Errorf("task %s already exists", name)
	}

	task := &Task{
		Name:        name,
		Spec:        spec,
		Handler:     handler,
		Description: description,
		Enabled:     true,
		Metadata:    make(map[string]interface{}),
	}

	// 如果调度器正在运行，立即启动任务
	if s.running {
		entryID, err := s.cron.AddFunc(spec, s.createTaskRunner(task))
		if err != nil {
			return fmt.Errorf("failed to add task: %v", err)
		}
		task.cronEntryID = entryID
		task.NextRun = s.cron.Entry(entryID).Next
	}

	s.tasks[name] = task
	return nil
}

// AddTaskWithCtx 添加带 Context 的定时任务
func (s *Scheduler) AddTaskWithCtx(name, spec string, handler func(context.Context)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[name]; exists {
		return fmt.Errorf("task %s already exists", name)
	}

	task := &Task{
		Name:         name,
		Spec:         spec,
		HandlerWithCtx: handler,
		Enabled:      true,
		Metadata:     make(map[string]interface{}),
	}

	if s.running {
		entryID, err := s.cron.AddFunc(spec, s.createTaskRunner(task))
		if err != nil {
			return fmt.Errorf("failed to add task: %v", err)
		}
		task.cronEntryID = entryID
		task.NextRun = s.cron.Entry(entryID).Next
	}

	s.tasks[name] = task
	return nil
}

// RemoveTask 移除定时任务
func (s *Scheduler) RemoveTask(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[name]
	if !exists {
		return fmt.Errorf("task %s not found", name)
	}

	if s.running {
		s.cron.Remove(task.cronEntryID)
	}

	delete(s.tasks, name)
	return nil
}

// EnableTask 启用任务
func (s *Scheduler) EnableTask(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[name]
	if !exists {
		return fmt.Errorf("task %s not found", name)
	}

	if !task.Enabled {
		task.Enabled = true
		if s.running {
			entryID, err := s.cron.AddFunc(task.Spec, s.createTaskRunner(task))
			if err != nil {
				return err
			}
			task.cronEntryID = entryID
			task.NextRun = s.cron.Entry(entryID).Next
		}
	}

	return nil
}

// DisableTask 禁用任务
func (s *Scheduler) DisableTask(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[name]
	if !exists {
		return fmt.Errorf("task %s not found", name)
	}

	if task.Enabled {
		task.Enabled = false
		if s.running {
			s.cron.Remove(task.cronEntryID)
		}
	}

	return nil
}

// GetTask 获取任务
func (s *Scheduler) GetTask(name string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[name]
	if !exists {
		return nil, fmt.Errorf("task %s not found", name)
	}

	return task, nil
}

// ListTasks 获取所有任务
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	// 按名称排序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Name < tasks[j].Name
	})

	return tasks
}

// ListTaskStatuses 获取所有任务状态
func (s *Scheduler) ListTaskStatuses() []*TaskStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]*TaskStatus, 0, len(s.tasks))
	for _, task := range s.tasks {
		statuses = append(statuses, &TaskStatus{
			Name:        task.Name,
			Description: task.Description,
			Enabled:     task.Enabled,
			Running:     task.Running,
			LastRun:     task.LastRun,
			NextRun:     task.NextRun,
			TotalRuns:   task.TotalRuns,
		})
	}

	return statuses
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	// 添加所有启用的任务
	for _, task := range s.tasks {
		if task.Enabled {
			entryID, err := s.cron.AddFunc(task.Spec, s.createTaskRunner(task))
			if err != nil {
				continue
			}
			task.cronEntryID = entryID
			task.NextRun = s.cron.Entry(entryID).Next
		}
	}

	s.cron.Start()
	s.running = true
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.running = false

	// 清除所有 cron entry ID
	for _, task := range s.tasks {
		task.cronEntryID = 0
	}
}

// RunTaskNow 立即执行任务
func (s *Scheduler) RunTaskNow(name string) error {
	s.mu.RLock()
	task, exists := s.tasks[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found", name)
	}

	go s.createTaskRunner(task)()
	return nil
}

// SetMetadata 设置任务元数据
func (s *Scheduler) SetMetadata(name string, key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[name]
	if !exists {
		return fmt.Errorf("task %s not found", name)
	}

	task.Metadata[key] = value
	return nil
}

// GetMetadata 获取任务元数据
func (s *Scheduler) GetMetadata(name string, key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[name]
	if !exists {
		return nil, fmt.Errorf("task %s not found", name)
	}

	value, ok := task.Metadata[key]
	if !ok {
		return nil, fmt.Errorf("metadata key %s not found", key)
	}

	return value, nil
}

// createTaskRunner 创建任务执行器
func (s *Scheduler) createTaskRunner(task *Task) func() {
	return func() {
		task.Running = true
		task.LastRun = time.Now()
		task.TotalRuns++

		defer func() {
			task.Running = false
			if r := recover(); r != nil {
				// 记录 panic
			}
		}()

		if task.Handler != nil {
			task.Handler()
		} else if task.HandlerWithCtx != nil {
			task.HandlerWithCtx(context.Background())
		}
	}
}

// GetNextRun 获取下次执行时间
func (s *Scheduler) GetNextRun(name string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[name]
	if !exists {
		return time.Time{}, fmt.Errorf("task %s not found", name)
	}

	if s.running {
		return s.cron.Entry(task.cronEntryID).Next, nil
	}

	// 如果调度器未运行，计算下次执行时间
	schedule, err := cron.ParseStandard(task.Spec)
	if err != nil {
		return time.Time{}, err
	}

	return schedule.Next(time.Now()), nil
}

// GetPrevRun 获取上次执行时间
func (s *Scheduler) GetPrevRun(name string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[name]
	if !exists {
		return time.Time{}, fmt.Errorf("task %s not found", name)
	}

	return task.LastRun, nil
}

// GetStats 获取统计信息
func (s *Scheduler) GetStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.tasks)
	enabled := 0
	running := 0

	for _, task := range s.tasks {
		if task.Enabled {
			enabled++
		}
		if task.Running {
			running++
		}
	}

	return &SchedulerStats{
		TotalTasks:   total,
		EnabledTasks: enabled,
		RunningTasks: running,
		IsRunning:    s.running,
	}
}

// SchedulerStats 调度器统计信息
type SchedulerStats struct {
	TotalTasks   int  `json:"total_tasks"`
	EnabledTasks int  `json:"enabled_tasks"`
	RunningTasks int  `json:"running_tasks"`
	IsRunning    bool `json:"is_running"`
}
