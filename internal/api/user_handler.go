// Package api 提供HTTP API处理器实现。
// API层负责处理HTTP请求/响应，进行数据验证和格式转换。
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/middleware"
	"github.com/MorseWayne/spike_shop/internal/resp"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// UserHandler 用户相关的HTTP处理器
type UserHandler struct {
	userService service.UserService
	logger      *zap.Logger
}

// NewUserHandler 创建用户处理器实例
func NewUserHandler(userService service.UserService, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// Register 处理用户注册请求
// POST /api/v1/auth/register
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体
	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateRegisterRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层进行注册
	user, err := h.userService.Register(&req)
	if err != nil {
		// 根据不同的错误类型返回不同的HTTP状态码
		if errors.Is(err, service.ErrUserExists) {
			resp.Error(w, http.StatusConflict, resp.CodeInvalidParam, "username or email already exists", reqID, "")
			return
		}

		h.logger.Error("register failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "register failed", reqID, "")
		return
	}

	// 返回成功响应（不包含敏感信息）
	userResp := map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"role":       user.Role,
		"is_active":  user.IsActive,
		"created_at": user.CreatedAt,
	}

	resp.OK(w, &userResp, reqID, "")
}

// Login 处理用户登录请求
// POST /api/v1/auth/login
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateLoginRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层进行登录
	user, err := h.userService.Login(&req)
	if err != nil {
		// 根据不同的错误类型返回不同的HTTP状态码
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrInvalidCredentials) {
			resp.Error(w, http.StatusUnauthorized, resp.CodeInvalidParam, "invalid username or password", reqID, "")
			return
		}
		if errors.Is(err, service.ErrUserInactive) {
			resp.Error(w, http.StatusForbidden, resp.CodeInvalidParam, "user is inactive", reqID, "")
			return
		}

		h.logger.Error("login failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "login failed", reqID, "")
		return
	}

	// TODO: 生成JWT令牌（下一步实现）
	// 现在先返回用户信息，等JWT实现后再添加令牌
	loginResp := map[string]interface{}{
		"user": map[string]interface{}{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"role":       user.Role,
			"is_active":  user.IsActive,
			"created_at": user.CreatedAt,
		},
		// "access_token": "待实现",
		// "refresh_token": "待实现",
	}

	resp.OK(w, &loginResp, reqID, "")
}

// GetProfile 获取当前用户信息
// GET /api/v1/users/profile
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// TODO: 从JWT中获取用户ID（下一步实现）
	// 现在先从查询参数获取用户ID作为临时方案
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "user_id is required", reqID, "")
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid user_id", reqID, "")
		return
	}

	// 获取用户信息
	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "user not found", reqID, "")
			return
		}

		h.logger.Error("get profile failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get profile failed", reqID, "")
		return
	}

	// 返回用户信息（不包含密码哈希）
	userResp := map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"role":       user.Role,
		"is_active":  user.IsActive,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}

	resp.OK(w, &userResp, reqID, "")
}

// validateRegisterRequest 验证注册请求
func (h *UserHandler) validateRegisterRequest(req *domain.RegisterRequest) error {
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return errors.New("username must be between 3 and 32 characters")
	}

	if len(req.Password) < 6 || len(req.Password) > 72 {
		return errors.New("password must be between 6 and 72 characters")
	}

	if req.Email == "" {
		return errors.New("email is required")
	}

	// 简单的邮箱格式验证
	if !isValidEmail(req.Email) {
		return errors.New("invalid email format")
	}

	return nil
}

// validateLoginRequest 验证登录请求
func (h *UserHandler) validateLoginRequest(req *domain.LoginRequest) error {
	if req.Username == "" {
		return errors.New("username is required")
	}

	if req.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

// isValidEmail 简单的邮箱格式验证
func isValidEmail(email string) bool {
	// 这是一个简化的邮箱验证，生产环境建议使用更严格的验证
	return len(email) > 0 &&
		len(email) <= 254 &&
		containsChar(email, '@') &&
		containsChar(email, '.')
}

// containsChar 检查字符串是否包含指定字符
func containsChar(s string, c rune) bool {
	for _, char := range s {
		if char == c {
			return true
		}
	}
	return false
}
