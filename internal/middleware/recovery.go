package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/MorseWayne/spike_shop/internal/resp"
	"go.uber.org/zap"
)

// Recovery 捕获 handler 链中的 panic：
// - 记录 panic 值与堆栈；
// - 返回统一的 500 错误响应；
// - 透传请求 ID 便于排查。
func Recovery(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", zap.Any("panic", rec), zap.ByteString("stack", debug.Stack()))
					reqID := RequestIDFromContext(r.Context())
					resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "internal server error", reqID, "")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
