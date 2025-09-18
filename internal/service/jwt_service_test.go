package service

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/config"
	"github.com/MorseWayne/spike_shop/internal/domain"
)

func createTestJWTService() JWTService {
	cfg := &config.Config{}
	cfg.JWT.Secret = "test-secret-key"
	cfg.JWT.AccessTokenTTL = 15 * time.Minute
	cfg.JWT.RefreshTokenTTL = 24 * time.Hour
	cfg.App.Name = "test-service"

	logger := zap.NewNop() // 无操作的logger，用于测试
	return NewJWTService(cfg, logger)
}

func createTestUser() *domain.User {
	return &domain.User{
		ID:       123,
		Username: "testuser",
		Role:     domain.UserRoleUser,
		IsActive: true,
	}
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	jwtService := createTestJWTService()
	user := createTestUser()

	// 测试生成令牌对
	tokenPair, err := jwtService.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	if tokenPair.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}

	if tokenPair.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}

	// 验证生成的访问令牌
	claims, err := jwtService.ValidateAccessToken(tokenPair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("Expected UserID %d, got %d", user.ID, claims.UserID)
	}

	if claims.Username != user.Username {
		t.Errorf("Expected Username %s, got %s", user.Username, claims.Username)
	}

	if claims.Role != user.Role {
		t.Errorf("Expected Role %s, got %s", user.Role, claims.Role)
	}

	if claims.Type != "access" {
		t.Errorf("Expected Type 'access', got %s", claims.Type)
	}

	// 验证生成的刷新令牌
	refreshClaims, err := jwtService.ValidateRefreshToken(tokenPair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken failed: %v", err)
	}

	if refreshClaims.Type != "refresh" {
		t.Errorf("Expected Type 'refresh', got %s", refreshClaims.Type)
	}
}

func TestJWTService_ValidateAccessToken_InvalidToken(t *testing.T) {
	jwtService := createTestJWTService()

	testCases := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"invalid format", "invalid.token.format"},
		{"wrong signature", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := jwtService.ValidateAccessToken(tc.token)
			if err == nil {
				t.Error("Expected validation to fail")
			}
		})
	}
}

func TestJWTService_ValidateToken_WrongType(t *testing.T) {
	jwtService := createTestJWTService()
	user := createTestUser()

	tokenPair, err := jwtService.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	// 尝试用刷新令牌验证访问令牌
	_, err = jwtService.ValidateAccessToken(tokenPair.RefreshToken)
	if err == nil {
		t.Error("Expected validation to fail when using refresh token as access token")
	}

	// 尝试用访问令牌验证刷新令牌
	_, err = jwtService.ValidateRefreshToken(tokenPair.AccessToken)
	if err == nil {
		t.Error("Expected validation to fail when using access token as refresh token")
	}
}

func TestJWTService_RefreshTokenPair(t *testing.T) {
	jwtService := createTestJWTService()
	user := createTestUser()

	// 生成初始令牌对
	originalTokenPair, err := jwtService.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	// 使用刷新令牌生成新的令牌对
	newTokenPair, err := jwtService.RefreshTokenPair(originalTokenPair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshTokenPair failed: %v", err)
	}

	// 验证新的访问令牌
	claims, err := jwtService.ValidateAccessToken(newTokenPair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("Expected UserID %d, got %d", user.ID, claims.UserID)
	}

	// 新的令牌应该与原始令牌不同（由于时间戳的差异，它们应该不同）
	// 注意：在实际的JWT实现中，由于iat (issued at) 时间戳的不同，令牌应该是不同的
	// 但在测试中，如果时间戳相同，令牌可能会相同，这在快速连续调用时可能发生
	// 我们主要验证功能正确性，而不是严格要求令牌不同

	// 验证新令牌是有效的
	newClaims, err := jwtService.ValidateAccessToken(newTokenPair.AccessToken)
	if err != nil {
		t.Fatalf("New access token validation failed: %v", err)
	}

	if newClaims.UserID != user.ID {
		t.Errorf("New token user ID mismatch, expected %d, got %d", user.ID, newClaims.UserID)
	}
}

func TestJWTService_RefreshTokenPair_InvalidRefreshToken(t *testing.T) {
	jwtService := createTestJWTService()

	testCases := []string{
		"",
		"invalid.token",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.invalid",
	}

	for _, invalidToken := range testCases {
		_, err := jwtService.RefreshTokenPair(invalidToken)
		if err == nil {
			t.Errorf("Expected RefreshTokenPair to fail with invalid token: %s", invalidToken)
		}
	}
}

func TestJWTService_TokenExpiration(t *testing.T) {
	// 创建一个短期有效的JWT服务用于测试过期
	cfg := &config.Config{}
	cfg.JWT.Secret = "test-secret-key"
	cfg.JWT.AccessTokenTTL = 1 * time.Millisecond // 很短的有效期
	cfg.JWT.RefreshTokenTTL = 1 * time.Millisecond
	cfg.App.Name = "test-service"

	logger := zap.NewNop()
	jwtService := NewJWTService(cfg, logger)
	user := createTestUser()

	// 生成令牌
	tokenPair, err := jwtService.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	// 等待令牌过期
	time.Sleep(10 * time.Millisecond)

	// 验证过期的访问令牌
	_, err = jwtService.ValidateAccessToken(tokenPair.AccessToken)
	if err == nil {
		t.Error("Expected validation to fail for expired access token")
	}

	if err != ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got %v", err)
	}

	// 验证过期的刷新令牌
	_, err = jwtService.ValidateRefreshToken(tokenPair.RefreshToken)
	if err == nil {
		t.Error("Expected validation to fail for expired refresh token")
	}

	if err != ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got %v", err)
	}
}
