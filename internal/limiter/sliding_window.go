// Package limiter 滑动窗口限流器实现
package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	client    redis.Cmdable
	config    *Config
	keyPrefix string
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(redisClient interface{}, config *Config) (*SlidingWindowLimiter, error) {
	client, ok := redisClient.(redis.Cmdable)
	if !ok {
		return nil, fmt.Errorf("invalid redis client type")
	}

	if config.KeyPrefix == "" {
		config.KeyPrefix = "limiter:sw"
	}

	// 设置默认精度为窗口的1/10
	if config.Precision == 0 {
		config.Precision = config.Window / 10
	}

	return &SlidingWindowLimiter{
		client:    client,
		config:    config,
		keyPrefix: config.KeyPrefix,
	}, nil
}

// Redis Lua脚本：滑动窗口算法
const slidingWindowScript = `
-- KEYS[1]: 计数器key
-- ARGV[1]: 限制数量(rate)
-- ARGV[2]: 时间窗口(window秒)
-- ARGV[3]: 精度(precision秒) 
-- ARGV[4]: 请求数量
-- ARGV[5]: 当前时间戳

local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local precision = tonumber(ARGV[3])
local requests = tonumber(ARGV[4])
local now = tonumber(ARGV[5])

-- 计算窗口数量
local windows = math.ceil(window / precision)
local window_size = window / windows

-- 清理过期的时间窗口
local cutoff = now - window
for i = 0, windows - 1 do
    local window_start = now - (i * window_size)
    if window_start < cutoff then
        local window_key = key .. ":" .. math.floor(window_start / window_size)
        redis.call('DEL', window_key)
    end
end

-- 计算当前窗口内的总请求数
local total_requests = 0
local current_window_requests = 0

for i = 0, windows - 1 do
    local window_start = now - (i * window_size)
    if window_start >= cutoff then
        local window_key = key .. ":" .. math.floor(window_start / window_size)
        local count = redis.call('GET', window_key) or 0
        total_requests = total_requests + tonumber(count)
        
        -- 记录当前窗口的请求数
        if i == 0 then
            current_window_requests = tonumber(count)
        end
    end
end

-- 检查是否超过限制
if total_requests + requests > limit then
    -- 计算重试时间
    local oldest_window_start = now - (windows - 1) * window_size
    local retry_after = math.ceil(oldest_window_start + window - now)
    if retry_after < 0 then
        retry_after = 1
    end
    
    return {0, limit - total_requests, retry_after, total_requests}
else
    -- 允许请求，增加当前窗口计数
    local current_window_key = key .. ":" .. math.floor(now / window_size)
    redis.call('INCRBY', current_window_key, requests)
    redis.call('EXPIRE', current_window_key, window + precision)
    
    total_requests = total_requests + requests
    return {1, limit - total_requests, 0, total_requests}
end
`

// getKey 生成Redis key
func (sw *SlidingWindowLimiter) getKey(key string) string {
	return fmt.Sprintf("%s:%s", sw.keyPrefix, key)
}

// Allow 检查是否允许请求通过
func (sw *SlidingWindowLimiter) Allow(ctx context.Context, key string) (*LimitResult, error) {
	return sw.AllowN(ctx, key, 1)
}

// AllowN 检查是否允许N个请求通过
func (sw *SlidingWindowLimiter) AllowN(ctx context.Context, key string, n int64) (*LimitResult, error) {
	redisKey := sw.getKey(key)
	now := time.Now().Unix()

	result := sw.client.Eval(ctx, slidingWindowScript,
		[]string{redisKey},
		sw.config.Rate,                       // 限制数量
		int64(sw.config.Window.Seconds()),    // 时间窗口
		int64(sw.config.Precision.Seconds()), // 精度
		n,                                    // 请求数量
		now,                                  // 当前时间
	)

	if result.Err() != nil {
		return nil, fmt.Errorf("failed to execute sliding window script: %w", result.Err())
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

// Reset 重置滑动窗口
func (sw *SlidingWindowLimiter) Reset(ctx context.Context, key string) error {
	redisKey := sw.getKey(key)

	// 删除所有相关的窗口key
	pattern := redisKey + ":*"
	iter := sw.client.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}

	if len(keys) > 0 {
		err := sw.client.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}

// GetInfo 获取滑动窗口信息
func (sw *SlidingWindowLimiter) GetInfo(ctx context.Context, key string) (*LimitInfo, error) {
	redisKey := sw.getKey(key)
	now := time.Now()
	nowUnix := now.Unix()

	// 计算窗口数量
	windows := int64(sw.config.Window / sw.config.Precision)
	windowSize := sw.config.Window / time.Duration(windows)

	// 统计当前窗口内的请求总数
	var totalRequests int64
	cutoff := nowUnix - int64(sw.config.Window.Seconds())

	for i := int64(0); i < windows; i++ {
		windowStart := nowUnix - (i * int64(windowSize.Seconds()))
		if windowStart >= cutoff {
			windowKey := fmt.Sprintf("%s:%d", redisKey, windowStart/int64(windowSize.Seconds()))

			result := sw.client.Get(ctx, windowKey)
			if result.Err() == nil {
				if count, err := result.Int64(); err == nil {
					totalRequests += count
				}
			}
		}
	}

	remaining := sw.config.Rate - totalRequests
	if remaining < 0 {
		remaining = 0
	}

	// 计算重置时间（下一个窗口的开始时间）
	currentWindowStart := nowUnix / int64(windowSize.Seconds()) * int64(windowSize.Seconds())
	resetTime := time.Unix(currentWindowStart+int64(windowSize.Seconds()), 0)

	return &LimitInfo{
		Limit:     sw.config.Rate,
		Remaining: remaining,
		Window:    sw.config.Window,
		ResetTime: resetTime,
	}, nil
}
