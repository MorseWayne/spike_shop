package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/MorseWayne/spike_shop/internal/resp"
)

// Timeout 为整个请求设置超时时间（基于标准库 http.TimeoutHandler）。
// 注意：http.TimeoutHandler 到时会自动写入 503；如需统一超时响应，
// 可在业务处理末尾调用 HandleTimeout 检查上下文错误并写入统一响应。
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// http.TimeoutHandler writes 503 by default; we intercept context error on write
			next.ServeHTTP(w, r)
		}), d, "")
	}
}

// HandleTimeout 在请求已超时/取消时写入统一超时响应，返回 true 表示已处理。
func HandleTimeout(w http.ResponseWriter, r *http.Request) bool {
	if err := r.Context().Err(); err == context.DeadlineExceeded || err == context.Canceled {
		reqID := RequestIDFromContext(r.Context())
		resp.Error(w, resp.HTTPStatusFromCode(resp.CodeTimeout), resp.CodeTimeout, "request timeout", reqID, "")
		return true
	}
	return false
}
