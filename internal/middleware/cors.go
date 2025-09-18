package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig 表示允许的跨域配置，均为白名单列表（大小写不敏感由调用方保证）。
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

// CORS 根据配置设置 CORS 响应头，并处理预检（OPTIONS）请求。
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := strings.Join(cfg.AllowedOrigins, ", ")
	allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Vary", "Access-Control-Request-Method")
			w.Header().Set("Vary", "Access-Control-Request-Headers")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
