// Package limiter 固定窗口限流器实现
package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// FixedWindowLimiter 固定窗口限流器
type FixedWindowLimiter struct {
	client    redis.Cmdable
	config    *Config
	keyPrefix string
}

// NewFixedWindowLimiter 创建固定窗口限流器
func NewFixedWindowLimiter(redisClient interface{}, config *Config) (*FixedWindowLimiter, error) {
	client, ok := redisClient.(redis.Cmdable)
	if !ok {
		return nil, fmt.Errorf("invalid redis client type")
	}

	if config.KeyPrefix == "" {
		config.KeyPrefix = "limiter:fw"
	}

	return &FixedWindowLimiter{
		client:    client,
		config:    config,
		keyPrefix: config.KeyPrefix,
	}, nil
}

// Redis Lua脚本：固定窗口算法
const fixedWindowScript = `
-- KEYS[1]: 计数器key
-- ARGV[1]: 限制数量(rate)
-- ARGV[2]: 时间窗口(window秒)
-- ARGV[3]: 请求数量
-- ARGV[4]: 当前时间戳

local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local requests = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- 计算当前窗口的开始时间
local window_start = math.floor(now / window) * window
local window_key = key .. ":" .. window_start

-- 获取当前窗口的请求计数
local current_requests = tonumber(redis.call('GET', window_key) or 0)

-- 检查是否超过限制
if current_requests + requests > limit then
    -- 计算重试时间（到下一个窗口的时间）
    local next_window = window_start + window
    local retry_after = next_window - now
    
    return {0, limit - current_requests, retry_after, current_requests}
else
    -- 允许请求，增加计数
    local new_count = redis.call('INCRBY', window_key, requests)
    redis.call('EXPIRE', window_key, window)
    
    return {1, limit - new_count, 0, new_count}
end
`

// getKey 生成Redis key
func (fw *FixedWindowLimiter) getKey(key string) string {
	return fmt.Sprintf("%s:%s", fw.keyPrefix, key)
}

// Allow 检查是否允许请求通过
func (fw *FixedWindowLimiter) Allow(ctx context.Context, key string) (*LimitResult, error) {
	return fw.AllowN(ctx, key, 1)
}

// AllowN 检查是否允许N个请求通过
func (fw *FixedWindowLimiter) AllowN(ctx context.Context, key string, n int64) (*LimitResult, error) {
	redisKey := fw.getKey(key)
	now := time.Now().Unix()

	result := fw.client.Eval(ctx, fixedWindowScript,
		[]string{redisKey},
		fw.config.Rate,                    // 限制数量
		int64(fw.config.Window.Seconds()), // 时间窗口
		n,                                 // 请求数量
		now,                               // 当前时间
	)

	if result.Err() != nil {
		return nil, fmt.Errorf("failed to execute fixed window script: %w", result.Err())
	}

	values, ok := result.Val().([]interface{})
	if !ok || len(values) != 4 {
		return nil, fmt.Errorf("unexpected script result format")
	}

	allowed := values[0].(int64) == 1
	remaining := values[1].(int64)
	retryAfter := time.Duration(values[2].(int64)) * time.Second
	totalRequests := values[3].(int64)

	return &LimitResult{
		Allowed:       allowed,
		Remaining:     remaining,
		RetryAfter:    retryAfter,
		TotalRequests: totalRequests,
	}, nil
}

// Reset 重置固定窗口
func (fw *FixedWindowLimiter) Reset(ctx context.Context, key string) error {
	redisKey := fw.getKey(key)

	// 删除所有相关的窗口key
	pattern := redisKey + ":*"
	iter := fw.client.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}

	if len(keys) > 0 {
		err := fw.client.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}

// GetInfo 获取固定窗口信息
func (fw *FixedWindowLimiter) GetInfo(ctx context.Context, key string) (*LimitInfo, error) {
	redisKey := fw.getKey(key)
	now := time.Now()
	nowUnix := now.Unix()

	// 计算当前窗口
	windowSeconds := int64(fw.config.Window.Seconds())
	windowStart := (nowUnix / windowSeconds) * windowSeconds
	windowKey := fmt.Sprintf("%s:%d", redisKey, windowStart)

	// 获取当前窗口的请求计数
	result := fw.client.Get(ctx, windowKey)
	currentRequests := int64(0)
	if result.Err() == nil {
		if count, err := result.Int64(); err == nil {
			currentRequests = count
		}
	}

	remaining := fw.config.Rate - currentRequests
	if remaining < 0 {
		remaining = 0
	}

	// 计算重置时间（下一个窗口的开始时间）
	resetTime := time.Unix(windowStart+windowSeconds, 0)

	return &LimitInfo{
		Limit:     fw.config.Rate,
		Remaining: remaining,
		Window:    fw.config.Window,
		ResetTime: resetTime,
	}, nil
}

// NewSlidingLogLimiter 滑动日志限流器（暂时使用固定窗口实现）
func NewSlidingLogLimiter(redisClient interface{}, config *Config) (*FixedWindowLimiter, error) {
	// 简化实现，暂时使用固定窗口
	return NewFixedWindowLimiter(redisClient, config)
}
