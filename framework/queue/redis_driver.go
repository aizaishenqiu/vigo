package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

// RedisDriver Redis 队列驱动
type RedisDriver struct {
	client *redis.Client
	queue  string
	sleep  int // 毫秒
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Queue    string `yaml:"queue"`
	Sleep    int    `yaml:"sleep"` // 毫秒
}

// NewRedisDriver 创建 Redis 队列驱动
func NewRedisDriver(config RedisConfig) *RedisDriver {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		panic(fmt.Sprintf("Redis connection failed: %v", err))
	}

	return &RedisDriver{
		client: client,
		queue:  config.Queue,
		sleep:  config.Sleep,
	}
}

// Push 推入任务
func (d *RedisDriver) Push(job *JobWrapper) error {
	ctx := context.Background()

	// 序列化任务
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	// 推入队列
	return d.client.LPush(ctx, d.queue, data).Err()
}

// Pop 弹出任务
func (d *RedisDriver) Pop(queue string) (*JobWrapper, error) {
	ctx := context.Background()

	// 使用 BRPOPLPUSH 实现阻塞弹出
	result := d.client.BRPop(ctx, time.Duration(d.sleep)*time.Millisecond, queue)
	if err := result.Err(); err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	vals := result.Val()
	if len(vals) == 0 {
		return nil, nil
	}

	// 反序列化任务
	var job JobWrapper
	if err := json.Unmarshal([]byte(vals[1]), &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// Delete 删除任务
func (d *RedisDriver) Delete(job *JobWrapper) error {
	// Redis 队列不需要显式删除，Pop 已经移除
	return nil
}

// Release 释放任务（重新推入队列）
func (d *RedisDriver) Release(job *JobWrapper, delay time.Duration) error {
	ctx := context.Background()

	// 如果有延迟，使用 zset 实现延迟队列
	if delay > 0 {
		score := float64(time.Now().Add(delay).UnixNano())
		data, _ := json.Marshal(job)
		return d.client.ZAdd(ctx, d.queue+"_delayed", redis.Z{
			Score:  score,
			Member: data,
		}).Err()
	}

	// 否则直接推入队列
	return d.client.LPush(ctx, d.queue, job).Err()
}

// Peek 查看队列头部任务
func (d *RedisDriver) Peek(queue string) (*JobWrapper, error) {
	ctx := context.Background()

	result := d.client.LIndex(ctx, queue, -1)
	if err := result.Err(); err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	val := result.Val()
	if val == "" {
		return nil, nil
	}

	var job JobWrapper
	if err := json.Unmarshal([]byte(val), &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// Size 获取队列大小
func (d *RedisDriver) Size(queue string) (int, error) {
	ctx := context.Background()
	result, err := d.client.LLen(ctx, queue).Result()
	return int(result), err
}

// Clear 清空队列
func (d *RedisDriver) Clear(queue string) error {
	ctx := context.Background()
	return d.client.Del(ctx, queue).Err()
}

// StartDelayedQueueProcessor 启动延迟队列处理器
func (d *RedisDriver) StartDelayedQueueProcessor() {
	go func() {
		ctx := context.Background()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// 获取所有已到期的延迟任务
			now := float64(time.Now().UnixNano())
			results, err := d.client.ZRangeByScore(ctx, d.queue+"_delayed", &redis.ZRangeBy{
				Min: "0",
				Max: fmt.Sprintf("%f", now),
			}).Result()

			if err != nil {
				continue
			}

			// 将到期的任务推入主队列
			for _, data := range results {
				var job JobWrapper
				if err := json.Unmarshal([]byte(data), &job); err != nil {
					continue
				}

				// 从延迟队列移除
				d.client.ZRem(ctx, d.queue+"_delayed", data)

				// 推入主队列
				jobData, _ := json.Marshal(&job)
				d.client.LPush(ctx, d.queue, jobData)
			}
		}
	}()
}
