// Package repo 提供数据访问层实现，负责与数据库交互。
// 仓储模式（Repository Pattern）将数据访问逻辑与业务逻辑分离，
// 使得业务逻辑不依赖于具体的数据存储实现。
package repo

import (
	"database/sql"
	"fmt"

	"github.com/MorseWayne/spike_shop/internal/database"
	"github.com/MorseWayne/spike_shop/internal/domain"
)

// UserRepository 定义用户数据访问接口
// 使用接口可以方便单元测试时进行模拟（mock）
type UserRepository interface {
	Create(user *domain.User) error
	GetByID(id int64) (*domain.User, error)
	GetByUsername(username string) (*domain.User, error)
	GetByEmail(email string) (*domain.User, error)
	Update(user *domain.User) error
	Delete(id int64) error
}

// userRepo 是 UserRepository 接口的数据库实现
type userRepo struct {
	db *database.DB
}

// NewUserRepository 创建用户仓储实例
func NewUserRepository(db *database.DB) UserRepository {
	return &userRepo{db: db}
}

// Create 创建新用户
// 注意：这里不处理密码哈希，密码哈希应该在服务层处理
func (r *userRepo) Create(user *domain.User) error {
	query := `
		INSERT INTO users (username, email, password_hash, role, is_active)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		user.Username,
		user.Email,
		user.PasswordHash,
		string(user.Role),
		user.IsActive,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	// 获取新插入记录的ID
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	user.ID = id
	return nil
}

// GetByID 根据ID查询用户
func (r *userRepo) GetByID(id int64) (*domain.User, error) {
	user := &domain.User{}
	query := `
		SELECT id, username, email, password_hash, role, is_active, created_at, updated_at
		FROM users WHERE id = ?
	`

	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 用户不存在
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return user, nil
}

// GetByUsername 根据用户名查询用户
func (r *userRepo) GetByUsername(username string) (*domain.User, error) {
	user := &domain.User{}
	query := `
		SELECT id, username, email, password_hash, role, is_active, created_at, updated_at
		FROM users WHERE username = ?
	`

	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 用户不存在
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}

	return user, nil
}

// GetByEmail 根据邮箱查询用户
func (r *userRepo) GetByEmail(email string) (*domain.User, error) {
	user := &domain.User{}
	query := `
		SELECT id, username, email, password_hash, role, is_active, created_at, updated_at
		FROM users WHERE email = ?
	`

	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 用户不存在
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return user, nil
}

// Update 更新用户信息
func (r *userRepo) Update(user *domain.User) error {
	query := `
		UPDATE users 
		SET username = ?, email = ?, password_hash = ?, role = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		user.Username,
		user.Email,
		user.PasswordHash,
		string(user.Role),
		user.IsActive,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

// Delete 删除用户（软删除，设置is_active为false）
func (r *userRepo) Delete(id int64) error {
	query := `UPDATE users SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}
