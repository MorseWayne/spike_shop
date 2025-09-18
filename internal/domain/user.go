// Package domain 定义业务领域模型和核心业务规则。
// 领域模型是业务逻辑的核心，独立于外部依赖（数据库、HTTP等）。
package domain

import (
	"time"
)

// UserRole 定义用户角色类型
type UserRole string

const (
	UserRoleUser  UserRole = "user"  // 普通用户
	UserRoleAdmin UserRole = "admin" // 管理员
)

// User 表示用户领域模型
// 包含用户的基本信息和业务规则
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // JSON序列化时忽略密码哈希
	Role         UserRole  `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IsAdmin 判断用户是否为管理员
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// RegisterRequest 表示用户注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=72"`
}

// LoginRequest 表示用户登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 表示登录成功的响应
type LoginResponse struct {
	User         *User  `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenRequest 表示刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
