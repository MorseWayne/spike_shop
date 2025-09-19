// Package limiter 令牌桶限流器实现
package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	client    redis.Cmdable
	config    *Config
	keyPrefix string
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(redisClient interface{}, config *Config) (*TokenBucketLimiter, error) {
	client, ok := redisClient.(redis.Cmdable)
	if !ok {
		return nil, fmt.Errorf("invalid redis client type")
	}

	if config.KeyPrefix == "" {
		config.KeyPrefix = "limiter:tb"
	}

	return &TokenBucketLimiter{
		client:    client,
		config:    config,
		keyPrefix: config.KeyPrefix,
	}, nil
}

// Redis Lua脚本：令牌桶算法
const tokenBucketScript = `
-- KEYS[1]: 令牌桶key
-- ARGV[1]: 容量(burst)
-- ARGV[2]: 补充速率(rate)  
-- ARGV[3]: 时间窗口(window秒)
-- ARGV[4]: 请求令牌数
-- ARGV[5]: 当前时间戳

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local tokens_requested = tonumber(ARGV[4])
local now = tonumber(ARGV[5])

-- 获取当前桶状态
local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1]) or capacity
local last_refill = tonumber(bucket[2]) or now

-- 计算需要补充的令牌数
local time_passed = math.max(0, now - last_refill)
local tokens_to_add = math.floor(time_passed * rate / window)
tokens = math.min(capacity, tokens + tokens_to_add)

-- 更新最后补充时间
last_refill = now

-- 检查是否有足够的令牌
if tokens >= tokens_requested then
    -- 扣除令牌
    tokens = tokens - tokens_requested
    
    -- 更新桶状态
    redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
    redis.call('EXPIRE', key, window * 2) -- 设置过期时间为窗口的2倍
    
    return {1, tokens, 0} -- {允许, 剩余令牌, 重试时间}
else
    -- 令牌不足，计算重试时间
    local tokens_needed = tokens_requested - tokens
    local retry_after = math.ceil(tokens_needed * window / rate)
    
    -- 更新桶状态（不扣除令牌）
    redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
    redis.call('EXPIRE', key, window * 2)
    
    return {0, tokens, retry_after} -- {拒绝, 剩余令牌, 重试时间}
end
`

// getKey 生成Redis key
func (tb *TokenBucketLimiter) getKey(key string) string {
	return fmt.Sprintf("%s:%s", tb.keyPrefix, key)
}

// Allow 检查是否允许请求通过
func (tb *TokenBucketLimiter) Allow(ctx context.Context, key string) (*LimitResult, error) {
	return tb.AllowN(ctx, key, 1)
}

// AllowN 检查是否允许N个请求通过
func (tb *TokenBucketLimiter) AllowN(ctx context.Context, key string, n int64) (*LimitResult, error) {
	redisKey := tb.getKey(key)
	now := time.Now().Unix()

	result := tb.client.Eval(ctx, tokenBucketScript,
		[]string{redisKey},
		tb.config.Burst,                   // 容量
		tb.config.Rate,                    // 速率
		int64(tb.config.Window.Seconds()), // 时间窗口
		n,                                 // 请求令牌数
		now,                               // 当前时间
	)

	if result.Err() != nil {
		return nil, fmt.Errorf("failed to execute token bucket script: %w", result.Err())
	}

	values, ok := result.Val().([]interface{})
	if !ok || len(values) != 3 {
		return nil, fmt.Errorf("unexpected script result format")
	}

	allowed := values[0].(int64) == 1
	remaining := values[1].(int64)
	retryAfter := time.Duration(values[2].(int64)) * time.Second

	return &LimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}

// Reset 重置令牌桶
func (tb *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	redisKey := tb.getKey(key)

	err := tb.client.Del(ctx, redisKey).Err()
	if err != nil {
		return fmt.Errorf("failed to reset token bucket: %w", err)
	}

	return nil
}

// GetInfo 获取令牌桶信息
func (tb *TokenBucketLimiter) GetInfo(ctx context.Context, key string) (*LimitInfo, error) {
	redisKey := tb.getKey(key)

	result := tb.client.HMGet(ctx, redisKey, "tokens", "last_refill")
	if result.Err() != nil {
		return nil, fmt.Errorf("failed to get token bucket info: %w", result.Err())
	}

	values := result.Val()
	tokens := tb.config.Burst // 默认满桶
	lastRefill := time.Now().Unix()

	if values[0] != nil {
		if t, ok := values[0].(string); ok {
			if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
				tokens = int64(parsed.Unix()) // 简化处理
			}
		}
	}

	if values[1] != nil {
		if t, ok := values[1].(string); ok {
			if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
				lastRefill = parsed.Unix()
			}
		}
	}

	// 计算当前令牌数
	now := time.Now().Unix()
	timePassed := now - lastRefill
	tokensToAdd := timePassed * tb.config.Rate / int64(tb.config.Window.Seconds())
	currentTokens := tokens + tokensToAdd
	if currentTokens > tb.config.Burst {
		currentTokens = tb.config.Burst
	}

	// 计算重置时间
	resetTime := time.Unix(lastRefill, 0).Add(tb.config.Window)

	return &LimitInfo{
		Limit:     tb.config.Burst,
		Remaining: currentTokens,
		Window:    tb.config.Window,
		ResetTime: resetTime,
	}, nil
}
