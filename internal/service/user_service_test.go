package service

import (
	"errors"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

// MockUserRepository 是用于测试的用户仓储模拟实现
type MockUserRepository struct {
	users  map[string]*domain.User // username -> user
	emails map[string]*domain.User // email -> user
	nextID int64
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:  make(map[string]*domain.User),
		emails: make(map[string]*domain.User),
		nextID: 1,
	}
}

func (m *MockUserRepository) Create(user *domain.User) error {
	// 检查用户名是否已存在
	if _, exists := m.users[user.Username]; exists {
		return errors.New("username already exists")
	}

	// 检查邮箱是否已存在
	if _, exists := m.emails[user.Email]; exists {
		return errors.New("email already exists")
	}

	user.ID = m.nextID
	m.nextID++

	m.users[user.Username] = user
	m.emails[user.Email] = user
	return nil
}

func (m *MockUserRepository) GetByID(id int64) (*domain.User, error) {
	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, nil
}

func (m *MockUserRepository) GetByUsername(username string) (*domain.User, error) {
	user, exists := m.users[username]
	if !exists {
		return nil, nil
	}
	return user, nil
}

func (m *MockUserRepository) GetByEmail(email string) (*domain.User, error) {
	user, exists := m.emails[email]
	if !exists {
		return nil, nil
	}
	return user, nil
}

func (m *MockUserRepository) Update(user *domain.User) error {
	// 简化实现，实际项目中需要更复杂的逻辑
	return nil
}

func (m *MockUserRepository) Delete(id int64) error {
	// 简化实现
	return nil
}

func (m *MockUserRepository) ListUsers(offset, limit int) ([]*domain.User, int64, error) {
	var users []*domain.User
	for _, user := range m.users {
		users = append(users, user)
	}

	total := int64(len(users))

	// 简化的分页逻辑
	start := offset
	end := offset + limit
	if start > len(users) {
		return []*domain.User{}, total, nil
	}
	if end > len(users) {
		end = len(users)
	}

	return users[start:end], total, nil
}

func (m *MockUserRepository) UpdateUserRole(userID int64, role domain.UserRole) error {
	for _, user := range m.users {
		if user.ID == userID {
			user.Role = role
			return nil
		}
	}
	return errors.New("user not found")
}

func (m *MockUserRepository) UpdateUserStatus(userID int64, isActive bool) error {
	for _, user := range m.users {
		if user.ID == userID {
			user.IsActive = isActive
			return nil
		}
	}
	return errors.New("user not found")
}

func createTestUserService() UserService {
	mockRepo := NewMockUserRepository()
	logger := zap.NewNop()
	return NewUserService(mockRepo, logger)
}

func TestUserService_Register_Success(t *testing.T) {
	userService := createTestUserService()

	req := &domain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	user, err := userService.Register(req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if user.Username != req.Username {
		t.Errorf("Expected username %s, got %s", req.Username, user.Username)
	}

	if user.Email != req.Email {
		t.Errorf("Expected email %s, got %s", req.Email, user.Email)
	}

	if user.Role != domain.UserRoleUser {
		t.Errorf("Expected role %s, got %s", domain.UserRoleUser, user.Role)
	}

	if !user.IsActive {
		t.Error("Expected user to be active")
	}

	// 验证密码是否正确哈希
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		t.Error("Password hash verification failed")
	}
}

func TestUserService_Register_DuplicateUsername(t *testing.T) {
	userService := createTestUserService()

	// 注册第一个用户
	req1 := &domain.RegisterRequest{
		Username: "testuser",
		Email:    "test1@example.com",
		Password: "password123",
	}

	_, err := userService.Register(req1)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// 尝试注册相同用户名
	req2 := &domain.RegisterRequest{
		Username: "testuser", // 相同用户名
		Email:    "test2@example.com",
		Password: "password456",
	}

	_, err = userService.Register(req2)
	if err == nil {
		t.Error("Expected registration to fail with duplicate username")
	}

	if !errors.Is(err, ErrUserExists) {
		t.Errorf("Expected ErrUserExists, got %v", err)
	}
}

func TestUserService_Register_DuplicateEmail(t *testing.T) {
	userService := createTestUserService()

	// 注册第一个用户
	req1 := &domain.RegisterRequest{
		Username: "testuser1",
		Email:    "test@example.com",
		Password: "password123",
	}

	_, err := userService.Register(req1)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// 尝试注册相同邮箱
	req2 := &domain.RegisterRequest{
		Username: "testuser2",
		Email:    "test@example.com", // 相同邮箱
		Password: "password456",
	}

	_, err = userService.Register(req2)
	if err == nil {
		t.Error("Expected registration to fail with duplicate email")
	}

	if !errors.Is(err, ErrUserExists) {
		t.Errorf("Expected ErrUserExists, got %v", err)
	}
}

func TestUserService_Login_Success(t *testing.T) {
	userService := createTestUserService()

	// 先注册用户
	registerReq := &domain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	_, err := userService.Register(registerReq)
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}

	// 测试用户名登录
	loginReq := &domain.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}

	user, err := userService.Login(loginReq)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", user.Username)
	}

	// 测试邮箱登录
	loginReq.Username = "test@example.com"
	user, err = userService.Login(loginReq)
	if err != nil {
		t.Fatalf("Email login failed: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", user.Email)
	}
}

func TestUserService_Login_InvalidCredentials(t *testing.T) {
	userService := createTestUserService()

	// 先注册用户
	registerReq := &domain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	_, err := userService.Register(registerReq)
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}

	testCases := []struct {
		name     string
		username string
		password string
		expected error
	}{
		{"wrong password", "testuser", "wrongpassword", ErrInvalidCredentials},
		{"non-existent user", "nonexistent", "password123", ErrUserNotFound},
		{"wrong email", "wrong@example.com", "password123", ErrUserNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loginReq := &domain.LoginRequest{
				Username: tc.username,
				Password: tc.password,
			}

			_, err := userService.Login(loginReq)
			if err == nil {
				t.Error("Expected login to fail")
			}

			if !errors.Is(err, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, err)
			}
		})
	}
}

func TestUserService_Login_InactiveUser(t *testing.T) {
	// 这个测试需要直接操作mock仓储来设置用户为非活跃状态
	mockRepo := NewMockUserRepository()
	logger := zap.NewNop()
	userService := NewUserService(mockRepo, logger)

	// 先注册用户
	registerReq := &domain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	user, err := userService.Register(registerReq)
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}

	// 直接设置用户为非活跃
	user.IsActive = false

	// 尝试登录
	loginReq := &domain.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}

	_, err = userService.Login(loginReq)
	if err == nil {
		t.Error("Expected login to fail for inactive user")
	}

	if !errors.Is(err, ErrUserInactive) {
		t.Errorf("Expected ErrUserInactive, got %v", err)
	}
}

func TestUserService_GetUserByID(t *testing.T) {
	userService := createTestUserService()

	// 先注册用户
	registerReq := &domain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	registeredUser, err := userService.Register(registerReq)
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}

	// 通过ID获取用户
	user, err := userService.GetUserByID(registeredUser.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if user.ID != registeredUser.ID {
		t.Errorf("Expected ID %d, got %d", registeredUser.ID, user.ID)
	}

	// 测试不存在的用户
	_, err = userService.GetUserByID(999)
	if err == nil {
		t.Error("Expected GetUserByID to fail for non-existent user")
	}

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}
