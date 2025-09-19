// Package limiter 提供多种限流算法的实现
package limiter

import (
	"context"
	"time"
)

// LimitResult 限流结果
type LimitResult struct {
	Allowed       bool          `json:"allowed"`        // 是否允许通过
	Remaining     int64         `json:"remaining"`      // 剩余配额
	RetryAfter    time.Duration `json:"retry_after"`    // 建议重试时间
	TotalRequests int64         `json:"total_requests"` // 总请求数
}

// Limiter 限流器接口
type Limiter interface {
	// Allow 检查是否允许请求通过
	Allow(ctx context.Context, key string) (*LimitResult, error)

	// AllowN 检查是否允许N个请求通过
	AllowN(ctx context.Context, key string, n int64) (*LimitResult, error)

	// Reset 重置限流状态
	Reset(ctx context.Context, key string) error

	// GetInfo 获取限流信息
	GetInfo(ctx context.Context, key string) (*LimitInfo, error)
}

// LimitInfo 限流信息
type LimitInfo struct {
	Limit     int64         `json:"limit"`      // 限流阈值
	Remaining int64         `json:"remaining"`  // 剩余配额
	Window    time.Duration `json:"window"`     // 时间窗口
	ResetTime time.Time     `json:"reset_time"` // 重置时间
}

// Config 限流配置
type Config struct {
	// 基础配置
	Rate   int64         `json:"rate"`   // 速率（请求数/时间窗口）
	Window time.Duration `json:"window"` // 时间窗口
	Burst  int64         `json:"burst"`  // 突发容量（令牌桶）

	// Redis配置
	RedisAddr     string `json:"redis_addr"`
	RedisPassword string `json:"redis_password"`
	RedisDB       int    `json:"redis_db"`

	// 高级配置
	Precision time.Duration `json:"precision"`  // 精度（滑动窗口）
	KeyPrefix string        `json:"key_prefix"` // Key前缀
}

// LimiterType 限流器类型
type LimiterType string

const (
	TokenBucket   LimiterType = "token_bucket"   // 令牌桶
	SlidingWindow LimiterType = "sliding_window" // 滑动窗口
	FixedWindow   LimiterType = "fixed_window"   // 固定窗口
	SlidingLog    LimiterType = "sliding_log"    // 滑动日志
)

// Factory 限流器工厂
type Factory struct {
	redisClient interface{} // Redis客户端
}

// NewFactory 创建限流器工厂
func NewFactory(redisClient interface{}) *Factory {
	return &Factory{
		redisClient: redisClient,
	}
}

// Create 创建指定类型的限流器
func (f *Factory) Create(limiterType LimiterType, config *Config) (Limiter, error) {
	switch limiterType {
	case TokenBucket:
		return NewTokenBucketLimiter(f.redisClient, config)
	case SlidingWindow:
		return NewSlidingWindowLimiter(f.redisClient, config)
	case FixedWindow:
		return NewFixedWindowLimiter(f.redisClient, config)
	case SlidingLog:
		return NewSlidingLogLimiter(f.redisClient, config)
	default:
		return NewTokenBucketLimiter(f.redisClient, config) // 默认使用令牌桶
	}
}

// MultiLimiter 多重限流器（可组合多种策略）
type MultiLimiter struct {
	limiters []Limiter
	strategy CombineStrategy
}

// CombineStrategy 组合策略
type CombineStrategy string

const (
	AllPass CombineStrategy = "all_pass" // 所有限流器都通过才允许
	AnyPass CombineStrategy = "any_pass" // 任意限流器通过就允许
)

// NewMultiLimiter 创建多重限流器
func NewMultiLimiter(limiters []Limiter, strategy CombineStrategy) *MultiLimiter {
	return &MultiLimiter{
		limiters: limiters,
		strategy: strategy,
	}
}

// Allow 检查是否允许请求通过
func (m *MultiLimiter) Allow(ctx context.Context, key string) (*LimitResult, error) {
	return m.AllowN(ctx, key, 1)
}

// AllowN 检查是否允许N个请求通过
func (m *MultiLimiter) AllowN(ctx context.Context, key string, n int64) (*LimitResult, error) {
	var results []*LimitResult
	var minRemaining int64 = -1
	var maxRetryAfter time.Duration

	for _, limiter := range m.limiters {
		result, err := limiter.AllowN(ctx, key, n)
		if err != nil {
			return nil, err
		}

		results = append(results, result)

		// 更新统计信息
		if result.Remaining >= 0 && (minRemaining == -1 || result.Remaining < minRemaining) {
			minRemaining = result.Remaining
		}
		if result.RetryAfter > maxRetryAfter {
			maxRetryAfter = result.RetryAfter
		}
	}

	// 根据策略决定最终结果
	allowed := false
	switch m.strategy {
	case AllPass:
		allowed = true
		for _, result := range results {
			if !result.Allowed {
				allowed = false
				break
			}
		}
	case AnyPass:
		for _, result := range results {
			if result.Allowed {
				allowed = true
				break
			}
		}
	}

	return &LimitResult{
		Allowed:    allowed,
		Remaining:  minRemaining,
		RetryAfter: maxRetryAfter,
	}, nil
}

// Reset 重置限流状态
func (m *MultiLimiter) Reset(ctx context.Context, key string) error {
	for _, limiter := range m.limiters {
		if err := limiter.Reset(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// GetInfo 获取限流信息（返回第一个限流器的信息）
func (m *MultiLimiter) GetInfo(ctx context.Context, key string) (*LimitInfo, error) {
	if len(m.limiters) == 0 {
		return nil, nil
	}
	return m.limiters[0].GetInfo(ctx, key)
}
