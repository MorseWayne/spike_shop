// Package middleware 提供JWT认证和授权中间件。
package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/resp"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// 上下文键定义
const (
	contextKeyUser contextKey = "user"
)

// AuthMiddleware JWT认证中间件
// 验证请求头中的JWT令牌，并将用户信息注入到请求上下文中
func AuthMiddleware(jwtService service.JWTService, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := RequestIDFromContext(r.Context())

			// 从Authorization头中提取令牌
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn("missing authorization header", zap.String("request_id", reqID))
				resp.Error(w, http.StatusUnauthorized, resp.CodeInvalidParam, "authorization header required", reqID, "")
				return
			}

			// 检查Bearer前缀
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				logger.Warn("invalid authorization header format", zap.String("request_id", reqID))
				resp.Error(w, http.StatusUnauthorized, resp.CodeInvalidParam, "invalid authorization header format", reqID, "")
				return
			}

			// 提取令牌
			tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
			if tokenString == "" {
				logger.Warn("empty token", zap.String("request_id", reqID))
				resp.Error(w, http.StatusUnauthorized, resp.CodeInvalidParam, "token required", reqID, "")
				return
			}

			// 验证访问令牌
			claims, err := jwtService.ValidateAccessToken(tokenString)
			if err != nil {
				logger.Warn("token validation failed",
					zap.String("request_id", reqID),
					zap.Error(err),
				)

				// 根据错误类型返回不同的响应
				switch err {
				case service.ErrTokenExpired:
					resp.Error(w, http.StatusUnauthorized, resp.CodeInvalidParam, "token expired", reqID, "")
				case service.ErrTokenNotReady:
					resp.Error(w, http.StatusUnauthorized, resp.CodeInvalidParam, "token not ready", reqID, "")
				default:
					resp.Error(w, http.StatusUnauthorized, resp.CodeInvalidParam, "invalid token", reqID, "")
				}
				return
			}

			// 构建用户对象并注入到上下文
			user := &domain.User{
				ID:       claims.UserID,
				Username: claims.Username,
				Role:     claims.Role,
				IsActive: true, // 从有效令牌假设用户是活跃的
			}

			ctx := context.WithValue(r.Context(), contextKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole 角色授权中间件
// 要求用户具有指定角色才能访问受保护的资源
func RequireRole(requiredRole domain.UserRole, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := RequestIDFromContext(r.Context())
			user := UserFromContext(r.Context())

			// 检查用户是否存在（应该由AuthMiddleware确保）
			if user == nil {
				logger.Error("user not found in context", zap.String("request_id", reqID))
				resp.Error(w, http.StatusUnauthorized, resp.CodeInternalError, "authentication required", reqID, "")
				return
			}

			// 检查用户角色
			if user.Role != requiredRole {
				logger.Warn("insufficient permissions",
					zap.String("request_id", reqID),
					zap.Int64("user_id", user.ID),
					zap.String("user_role", string(user.Role)),
					zap.String("required_role", string(requiredRole)),
				)
				resp.Error(w, http.StatusForbidden, resp.CodeInvalidParam, "insufficient permissions", reqID, "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin 管理员权限中间件
// 这是RequireRole的便捷包装，要求用户具有管理员角色
func RequireAdmin(logger *zap.Logger) func(http.Handler) http.Handler {
	return RequireRole(domain.UserRoleAdmin, logger)
}

// UserFromContext 从请求上下文中获取当前用户信息
func UserFromContext(ctx context.Context) *domain.User {
	if user, ok := ctx.Value(contextKeyUser).(*domain.User); ok {
		return user
	}
	return nil
}

// OptionalAuth 可选认证中间件
// 如果存在有效的Authorization头则验证并注入用户信息，否则继续处理请求
// 适用于某些端点既支持匿名访问又支持认证访问的场景
func OptionalAuth(jwtService service.JWTService, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := RequestIDFromContext(r.Context())

			// 检查是否有Authorization头
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 没有认证头，继续处理请求
				next.ServeHTTP(w, r)
				return
			}

			// 检查Bearer前缀
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				// 格式不正确，继续处理请求（不抛出错误）
				next.ServeHTTP(w, r)
				return
			}

			// 提取并验证令牌
			tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
			if tokenString == "" {
				next.ServeHTTP(w, r)
				return
			}

			// 验证访问令牌
			claims, err := jwtService.ValidateAccessToken(tokenString)
			if err != nil {
				logger.Debug("optional auth token validation failed",
					zap.String("request_id", reqID),
					zap.Error(err),
				)
				// 令牌无效，但不阻止请求继续
				next.ServeHTTP(w, r)
				return
			}

			// 注入用户信息到上下文
			user := &domain.User{
				ID:       claims.UserID,
				Username: claims.Username,
				Role:     claims.Role,
				IsActive: true,
			}

			ctx := context.WithValue(r.Context(), contextKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
