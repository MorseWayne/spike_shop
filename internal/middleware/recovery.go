package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/MorseWayne/spike_shop/internal/resp"
	"go.uber.org/zap"
)

// Recovery captures panics and responds with a structured error.
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
