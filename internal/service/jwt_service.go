// Package service 提供JWT令牌的生成、验证和刷新功能。
package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/config"
	"github.com/MorseWayne/spike_shop/internal/domain"
)

// JWT相关错误定义
var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrTokenExpired    = errors.New("token expired")
	ErrTokenNotReady   = errors.New("token used before valid")
	ErrRefreshRequired = errors.New("refresh token required")
)

// Claims 定义JWT载荷结构
// 继承jwt.RegisteredClaims以获得标准声明字段
type Claims struct {
	UserID   int64           `json:"user_id"`
	Username string          `json:"username"`
	Role     domain.UserRole `json:"role"`
	Type     string          `json:"type"` // "access" 或 "refresh"
	jwt.RegisteredClaims
}

// TokenPair 表示访问令牌和刷新令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// JWTService 定义JWT服务接口
type JWTService interface {
	GenerateTokenPair(user *domain.User) (*TokenPair, error)
	ValidateAccessToken(tokenString string) (*Claims, error)
	ValidateRefreshToken(tokenString string) (*Claims, error)
	RefreshTokenPair(refreshToken string) (*TokenPair, error)
}

// jwtService 是JWTService接口的实现
type jwtService struct {
	config *config.Config
	logger *zap.Logger
}

// NewJWTService 创建JWT服务实例
func NewJWTService(cfg *config.Config, logger *zap.Logger) JWTService {
	return &jwtService{
		config: cfg,
		logger: logger,
	}
}

// GenerateTokenPair 为用户生成访问令牌和刷新令牌对
// 访问令牌：短期有效，用于API访问
// 刷新令牌：长期有效，用于刷新访问令牌
func (s *jwtService) GenerateTokenPair(user *domain.User) (*TokenPair, error) {
	now := time.Now()

	// 生成访问令牌
	accessClaims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.JWT.AccessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.App.Name,
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.config.JWT.Secret))
	if err != nil {
		s.logger.Error("failed to sign access token", zap.Error(err))
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	// 生成刷新令牌
	refreshClaims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.JWT.RefreshTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.App.Name,
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.config.JWT.Secret))
	if err != nil {
		s.logger.Error("failed to sign refresh token", zap.Error(err))
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	s.logger.Info("token pair generated",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.Duration("access_ttl", s.config.JWT.AccessTokenTTL),
		zap.Duration("refresh_ttl", s.config.JWT.RefreshTokenTTL),
	)

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
	}, nil
}

// ValidateAccessToken 验证访问令牌
func (s *jwtService) ValidateAccessToken(tokenString string) (*Claims, error) {
	return s.validateToken(tokenString, "access")
}

// ValidateRefreshToken 验证刷新令牌
func (s *jwtService) ValidateRefreshToken(tokenString string) (*Claims, error) {
	return s.validateToken(tokenString, "refresh")
}

// validateToken 验证令牌的通用方法
func (s *jwtService) validateToken(tokenString, expectedType string) (*Claims, error) {
	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil {
		// 根据错误类型返回特定错误
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrTokenNotReady
		}
		s.logger.Warn("token validation failed", zap.Error(err))
		return nil, ErrInvalidToken
	}

	// 提取声明
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// 验证令牌类型
	if claims.Type != expectedType {
		s.logger.Warn("token type mismatch",
			zap.String("expected", expectedType),
			zap.String("actual", claims.Type),
		)
		return nil, ErrInvalidToken
	}

	// 验证发行者
	if claims.Issuer != s.config.App.Name {
		s.logger.Warn("token issuer mismatch",
			zap.String("expected", s.config.App.Name),
			zap.String("actual", claims.Issuer),
		)
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshTokenPair 使用刷新令牌生成新的令牌对
// 这个方法可以让用户在访问令牌过期后继续使用应用，而无需重新登录
func (s *jwtService) RefreshTokenPair(refreshTokenString string) (*TokenPair, error) {
	// 验证刷新令牌
	claims, err := s.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("validate refresh token: %w", err)
	}

	// 构建用户对象用于生成新令牌
	// 注意：这里只使用令牌中的基本信息，实际生产环境可能需要从数据库重新获取用户信息
	// 以确保用户状态（如是否被禁用）是最新的
	user := &domain.User{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
		IsActive: true, // 这里假设从令牌中的用户是活跃的
	}

	// 生成新的令牌对
	tokenPair, err := s.GenerateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("generate new token pair: %w", err)
	}

	s.logger.Info("token pair refreshed",
		zap.Int64("user_id", claims.UserID),
		zap.String("username", claims.Username),
	)

	return tokenPair, nil
}
