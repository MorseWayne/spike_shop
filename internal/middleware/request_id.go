package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	HeaderRequestID = "X-Request-ID"
)

// RequestID 确保每个请求都有请求 ID：
// 1) 优先读取请求头 X-Request-ID；
// 2) 若为空则生成 UUID；
// 3) 将该 ID 写入响应头与请求上下文。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(HeaderRequestID)
		if strings.TrimSpace(rid) == "" {
			rid = uuid.New().String()
		}
		w.Header().Set(HeaderRequestID, rid)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), rid)))
	})
}
