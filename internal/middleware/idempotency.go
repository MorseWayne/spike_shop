// Package middleware 提供幂等性中间件
package middleware

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MorseWayne/spike_shop/internal/resp"
)

// IdempotencyConfig 幂等性中间件配置
type IdempotencyConfig struct {
	// 幂等键头名称
	IdempotencyKeyHeader string

	// 是否跳过GET请求
	SkipMethods []string

	// 缓存TTL
	CacheTTL time.Duration

	// 错误处理函数
	ErrorHandler func(*gin.Context, error)
}

// DefaultIdempotencyConfig 默认幂等性配置
func DefaultIdempotencyConfig() *IdempotencyConfig {
	return &IdempotencyConfig{
		IdempotencyKeyHeader: "X-Idempotency-Key",
		SkipMethods:          []string{"GET", "HEAD", "OPTIONS"},
		CacheTTL:             24 * time.Hour,
		ErrorHandler:         defaultIdempotencyErrorHandler,
	}
}

// IdempotencyMiddleware 幂等性中间件
func IdempotencyMiddleware(config ...*IdempotencyConfig) gin.HandlerFunc {
	cfg := DefaultIdempotencyConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return func(c *gin.Context) {
		// 检查是否需要跳过
		method := c.Request.Method
		for _, skipMethod := range cfg.SkipMethods {
			if method == skipMethod {
				c.Next()
				return
			}
		}

		// 获取幂等键
		idempotencyKey := c.GetHeader(cfg.IdempotencyKeyHeader)
		if idempotencyKey == "" {
			// 自动生成幂等键（基于请求内容）
			idempotencyKey = generateIdempotencyKey(c)
		}

		// 设置幂等键到上下文
		c.Set("idempotency_key", idempotencyKey)

		// TODO: 这里可以集成Redis缓存来存储幂等键
		// 目前只是将幂等键设置到上下文中，实际的幂等性检查在业务层处理

		c.Next()
	}
}

// generateIdempotencyKey 生成幂等键
func generateIdempotencyKey(c *gin.Context) string {
	// 基于请求方法、路径、用户ID和时间戳生成
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		userID = fmt.Sprintf("%v", uid)
	}

	// 使用MD5哈希生成短的唯一键
	content := fmt.Sprintf("%s:%s:%s:%d",
		c.Request.Method,
		c.Request.URL.Path,
		userID,
		time.Now().Unix()/60) // 按分钟取整，同一分钟内的请求有相同的幂等键

	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("auto_%x", hash)[:16] // 取前16位
}

// defaultIdempotencyErrorHandler 默认幂等性错误处理器
func defaultIdempotencyErrorHandler(c *gin.Context, err error) {
	requestID := getRequestID(c)
	traceID := getTraceID(c)

	resp.Error(c.Writer, http.StatusTooManyRequests, resp.CodeInvalidParam,
		"重复请求", requestID, traceID)
}

// getRequestID 获取请求ID
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// getTraceID 获取追踪ID
func getTraceID(c *gin.Context) string {
	if traceID, exists := c.Get("trace_id"); exists {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}
