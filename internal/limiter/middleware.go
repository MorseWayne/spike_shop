// Package limiter 限流中间件实现
package limiter

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MorseWayne/spike_shop/internal/resp"
	"github.com/gin-gonic/gin"
)

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	// 限流器
	Limiter Limiter

	// Key生成函数
	KeyGenerator func(*gin.Context) string

	// 错误处理函数
	ErrorHandler func(*gin.Context, error)

	// 限流回调函数
	OnLimitReached func(*gin.Context, *LimitResult)

	// 是否跳过限流检查
	Skip func(*gin.Context) bool

	// 响应头配置
	Headers *HeaderConfig
}

// HeaderConfig 响应头配置
type HeaderConfig struct {
	// 是否添加限流头
	Enable bool

	// 限流相关头名称
	LimitHeader      string // X-RateLimit-Limit
	RemainingHeader  string // X-RateLimit-Remaining
	ResetHeader      string // X-RateLimit-Reset
	RetryAfterHeader string // Retry-After
}

// DefaultHeaderConfig 默认头配置
func DefaultHeaderConfig() *HeaderConfig {
	return &HeaderConfig{
		Enable:           true,
		LimitHeader:      "X-RateLimit-Limit",
		RemainingHeader:  "X-RateLimit-Remaining",
		ResetHeader:      "X-RateLimit-Reset",
		RetryAfterHeader: "Retry-After",
	}
}

// DefaultKeyGenerator 默认Key生成器（基于IP）
func DefaultKeyGenerator(c *gin.Context) string {
	return fmt.Sprintf("global:%s", c.ClientIP())
}

// UserKeyGenerator 用户Key生成器
func UserKeyGenerator(c *gin.Context) string {
	userID := c.GetInt64("user_id")
	if userID > 0 {
		return fmt.Sprintf("user:%d", userID)
	}
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

// PathKeyGenerator 路径Key生成器
func PathKeyGenerator(c *gin.Context) string {
	return fmt.Sprintf("path:%s:%s", c.Request.Method, c.FullPath())
}

// CombinedKeyGenerator 组合Key生成器
func CombinedKeyGenerator(generators ...func(*gin.Context) string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		var parts []string
		for _, gen := range generators {
			parts = append(parts, gen(c))
		}
		return strings.Join(parts, ":")
	}
}

// RateLimitMiddleware 创建限流中间件
func RateLimitMiddleware(config *MiddlewareConfig) gin.HandlerFunc {
	// 设置默认值
	if config.KeyGenerator == nil {
		config.KeyGenerator = DefaultKeyGenerator
	}

	if config.ErrorHandler == nil {
		config.ErrorHandler = defaultErrorHandler
	}

	if config.OnLimitReached == nil {
		config.OnLimitReached = defaultOnLimitReached
	}

	if config.Headers == nil {
		config.Headers = DefaultHeaderConfig()
	}

	return func(c *gin.Context) {
		// 检查是否跳过限流
		if config.Skip != nil && config.Skip(c) {
			c.Next()
			return
		}

		// 生成限流Key
		key := config.KeyGenerator(c)

		// 执行限流检查
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		result, err := config.Limiter.Allow(ctx, key)
		if err != nil {
			config.ErrorHandler(c, err)
			return
		}

		// 设置响应头
		if config.Headers.Enable {
			setRateLimitHeaders(c, result, config.Headers)
		}

		// 检查是否被限流
		if !result.Allowed {
			config.OnLimitReached(c, result)
			return
		}

		c.Next()
	}
}

// setRateLimitHeaders 设置限流相关的响应头
func setRateLimitHeaders(c *gin.Context, result *LimitResult, headers *HeaderConfig) {
	if headers.RemainingHeader != "" {
		c.Header(headers.RemainingHeader, strconv.FormatInt(result.Remaining, 10))
	}

	if headers.RetryAfterHeader != "" && result.RetryAfter > 0 {
		c.Header(headers.RetryAfterHeader, strconv.FormatInt(int64(result.RetryAfter.Seconds()), 10))
	}
}

// defaultErrorHandler 默认错误处理器
func defaultErrorHandler(c *gin.Context, err error) {
	requestID := c.GetString("request_id")
	traceID := c.GetString("trace_id")
	resp.Error(c.Writer, http.StatusInternalServerError, resp.CodeInternalError, "限流服务异常", requestID, traceID)
}

// defaultOnLimitReached 默认限流回调
func defaultOnLimitReached(c *gin.Context, result *LimitResult) {
	requestID := c.GetString("request_id")
	traceID := c.GetString("trace_id")

	resp.Error(c.Writer, http.StatusTooManyRequests, resp.CodeInvalidParam,
		"请求过于频繁，请稍后重试", requestID, traceID)
}

// SpikeRateLimitMiddleware 秒杀专用限流中间件
func SpikeRateLimitMiddleware(limiter Limiter) gin.HandlerFunc {
	config := &MiddlewareConfig{
		Limiter: limiter,
		KeyGenerator: func(c *gin.Context) string {
			// 优先使用用户ID，其次使用IP
			userID := c.GetInt64("user_id")
			if userID > 0 {
				return fmt.Sprintf("spike:user:%d", userID)
			}
			return fmt.Sprintf("spike:ip:%s", c.ClientIP())
		},
		OnLimitReached: func(c *gin.Context, result *LimitResult) {
			requestID := c.GetString("request_id")
			traceID := c.GetString("trace_id")
			resp.Error(c.Writer, http.StatusTooManyRequests, resp.CodeInvalidParam,
				"秒杀请求过于频繁", requestID, traceID)
		},
		Headers: DefaultHeaderConfig(),
	}

	return RateLimitMiddleware(config)
}

// GlobalRateLimitMiddleware 全局限流中间件
func GlobalRateLimitMiddleware(limiter Limiter) gin.HandlerFunc {
	config := &MiddlewareConfig{
		Limiter:      limiter,
		KeyGenerator: DefaultKeyGenerator,
		Headers:      DefaultHeaderConfig(),
	}

	return RateLimitMiddleware(config)
}

// APIRateLimitMiddleware API接口限流中间件
func APIRateLimitMiddleware(limiter Limiter) gin.HandlerFunc {
	config := &MiddlewareConfig{
		Limiter: limiter,
		KeyGenerator: func(c *gin.Context) string {
			// 基于用户ID + 接口路径
			userID := c.GetInt64("user_id")
			path := c.FullPath()
			if userID > 0 {
				return fmt.Sprintf("api:user:%d:path:%s", userID, path)
			}
			return fmt.Sprintf("api:ip:%s:path:%s", c.ClientIP(), path)
		},
		Headers: DefaultHeaderConfig(),
	}

	return RateLimitMiddleware(config)
}

// MultiLevelRateLimitMiddleware 多级限流中间件
func MultiLevelRateLimitMiddleware(globalLimiter, userLimiter Limiter) gin.HandlerFunc {
	// 创建多重限流器
	multiLimiter := NewMultiLimiter([]Limiter{globalLimiter, userLimiter}, AllPass)

	config := &MiddlewareConfig{
		Limiter: multiLimiter,
		KeyGenerator: func(c *gin.Context) string {
			userID := c.GetInt64("user_id")
			if userID > 0 {
				return fmt.Sprintf("multi:user:%d", userID)
			}
			return fmt.Sprintf("multi:ip:%s", c.ClientIP())
		},
		Headers: DefaultHeaderConfig(),
	}

	return RateLimitMiddleware(config)
}
