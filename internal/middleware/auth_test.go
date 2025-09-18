package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// MockJWTService 是用于测试的JWT服务模拟实现
type MockJWTService struct {
	validTokens   map[string]*service.Claims
	expiredTokens map[string]bool
}

func NewMockJWTService() *MockJWTService {
	return &MockJWTService{
		validTokens:   make(map[string]*service.Claims),
		expiredTokens: make(map[string]bool),
	}
}

func (m *MockJWTService) GenerateTokenPair(user *domain.User) (*service.TokenPair, error) {
	// 生成模拟令牌
	accessToken := "mock_access_token_" + user.Username
	refreshToken := "mock_refresh_token_" + user.Username

	// 存储claims用于验证
	claims := &service.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "access",
	}
	m.validTokens[accessToken] = claims

	refreshClaims := &service.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "refresh",
	}
	m.validTokens[refreshToken] = refreshClaims

	return &service.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (m *MockJWTService) ValidateAccessToken(tokenString string) (*service.Claims, error) {
	if m.expiredTokens[tokenString] {
		return nil, service.ErrTokenExpired
	}

	claims, exists := m.validTokens[tokenString]
	if !exists {
		return nil, service.ErrInvalidToken
	}

	if claims.Type != "access" {
		return nil, service.ErrInvalidToken
	}

	return claims, nil
}

func (m *MockJWTService) ValidateRefreshToken(tokenString string) (*service.Claims, error) {
	if m.expiredTokens[tokenString] {
		return nil, service.ErrTokenExpired
	}

	claims, exists := m.validTokens[tokenString]
	if !exists {
		return nil, service.ErrInvalidToken
	}

	if claims.Type != "refresh" {
		return nil, service.ErrInvalidToken
	}

	return claims, nil
}

func (m *MockJWTService) RefreshTokenPair(refreshToken string) (*service.TokenPair, error) {
	claims, err := m.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}

	return m.GenerateTokenPair(user)
}

func (m *MockJWTService) AddExpiredToken(token string) {
	m.expiredTokens[token] = true
}

func createTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authenticated"))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("not authenticated"))
		}
	}
}

func TestAuthMiddleware_Success(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	// 创建测试用户和令牌
	user := &domain.User{
		ID:       1,
		Username: "testuser",
		Role:     domain.UserRoleUser,
	}

	tokenPair, err := mockJWT.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// 创建带认证中间件的处理器
	middleware := AuthMiddleware(mockJWT, logger)
	handler := middleware(createTestHandler())

	// 创建带有Authorization头的请求
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "authenticated" {
		t.Errorf("Expected 'authenticated', got %s", rr.Body.String())
	}
}

func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	middleware := AuthMiddleware(mockJWT, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidAuthHeader(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	testCases := []struct {
		name   string
		header string
	}{
		{"missing Bearer prefix", "invalid_token"},
		{"empty token", "Bearer "},
		{"only Bearer", "Bearer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			middleware := AuthMiddleware(mockJWT, logger)
			handler := middleware(createTestHandler())

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tc.header)
			req = req.WithContext(withRequestID(req.Context(), "test-id"))

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", rr.Code)
			}
		})
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	middleware := AuthMiddleware(mockJWT, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	// 创建用户和令牌
	user := &domain.User{
		ID:       1,
		Username: "testuser",
		Role:     domain.UserRoleUser,
	}

	tokenPair, err := mockJWT.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// 标记令牌为过期
	mockJWT.AddExpiredToken(tokenPair.AccessToken)

	middleware := AuthMiddleware(mockJWT, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestRequireRole_Success(t *testing.T) {
	logger := zap.NewNop()

	// 创建管理员用户
	adminUser := &domain.User{
		ID:       1,
		Username: "admin",
		Role:     domain.UserRoleAdmin,
	}

	middleware := RequireRole(domain.UserRoleAdmin, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), contextKeyUser, adminUser)
	ctx = withRequestID(ctx, "test-id")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestRequireRole_InsufficientPermissions(t *testing.T) {
	logger := zap.NewNop()

	// 创建普通用户
	user := &domain.User{
		ID:       1,
		Username: "user",
		Role:     domain.UserRoleUser,
	}

	middleware := RequireRole(domain.UserRoleAdmin, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), contextKeyUser, user)
	ctx = withRequestID(ctx, "test-id")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}
}

func TestRequireRole_NoUserInContext(t *testing.T) {
	logger := zap.NewNop()

	middleware := RequireRole(domain.UserRoleAdmin, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestRequireAdmin(t *testing.T) {
	logger := zap.NewNop()

	// 测试管理员用户
	adminUser := &domain.User{
		ID:       1,
		Username: "admin",
		Role:     domain.UserRoleAdmin,
	}

	middleware := RequireAdmin(logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), contextKeyUser, adminUser)
	ctx = withRequestID(ctx, "test-id")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for admin user, got %d", rr.Code)
	}

	// 测试普通用户
	normalUser := &domain.User{
		ID:       2,
		Username: "user",
		Role:     domain.UserRoleUser,
	}

	req = httptest.NewRequest("GET", "/test", nil)
	ctx = context.WithValue(req.Context(), contextKeyUser, normalUser)
	ctx = withRequestID(ctx, "test-id")
	req = req.WithContext(ctx)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for normal user, got %d", rr.Code)
	}
}

func TestOptionalAuth_WithValidToken(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	// 创建测试用户和令牌
	user := &domain.User{
		ID:       1,
		Username: "testuser",
		Role:     domain.UserRoleUser,
	}

	tokenPair, err := mockJWT.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	middleware := OptionalAuth(mockJWT, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "authenticated" {
		t.Errorf("Expected 'authenticated', got %s", rr.Body.String())
	}
}

func TestOptionalAuth_WithoutToken(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	middleware := OptionalAuth(mockJWT, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}

	if rr.Body.String() != "not authenticated" {
		t.Errorf("Expected 'not authenticated', got %s", rr.Body.String())
	}
}

func TestOptionalAuth_WithInvalidToken(t *testing.T) {
	mockJWT := NewMockJWTService()
	logger := zap.NewNop()

	middleware := OptionalAuth(mockJWT, logger)
	handler := middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")
	req = req.WithContext(withRequestID(req.Context(), "test-id"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// 可选认证在令牌无效时不应阻止请求
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}

	if rr.Body.String() != "not authenticated" {
		t.Errorf("Expected 'not authenticated', got %s", rr.Body.String())
	}
}

func TestUserFromContext(t *testing.T) {
	user := &domain.User{
		ID:       1,
		Username: "testuser",
		Role:     domain.UserRoleUser,
	}

	// 测试从上下文中获取用户
	ctx := context.WithValue(context.Background(), contextKeyUser, user)
	retrievedUser := UserFromContext(ctx)

	if retrievedUser == nil {
		t.Fatal("Expected user from context, got nil")
	}

	if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %d, got %d", user.ID, retrievedUser.ID)
	}

	// 测试空上下文
	emptyCtx := context.Background()
	retrievedUser = UserFromContext(emptyCtx)

	if retrievedUser != nil {
		t.Error("Expected nil from empty context, got user")
	}
}
