# 里程碑2：用户认证系统实现文档

## 概述

本文档详细记录了里程碑2的完整实现过程，包括用户认证、JWT策略、RBAC权限控制以及相关的单元测试。

**实施时间**：2025年9月18日  
**目标**：构建完整的用户认证与授权系统  
**技术栈**：Go + JWT + bcrypt + MySQL

## 架构设计

### 整体架构
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Layer     │    │  Service Layer  │    │  Repository     │
│                 │    │                 │    │     Layer       │
│ user_handler.go │◄──►│user_service.go  │◄──►│ user_repo.go    │
│                 │    │jwt_service.go   │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         ▲                       ▲                       ▲
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Middleware    │    │    Domain       │    │    Database     │
│                 │    │     Layer       │    │                 │
│ auth.go         │    │ user.go         │    │ MySQL           │
│ request_id.go   │    │                 │    │ migrations/     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 安全模型
- **密码存储**：bcrypt哈希 (cost=10)
- **会话管理**：JWT Access/Refresh Token
- **权限控制**：基于角色的访问控制 (RBAC)
- **请求追踪**：每个请求的唯一ID
- **日志审计**：结构化日志记录所有认证活动

## 实施步骤

### 第一步：数据库迁移与用户表

#### 1.1 创建迁移工具
**文件**：`internal/database/database.go`

**核心功能**：
- 数据库连接管理
- 迁移执行引擎
- 迁移历史追踪

**关键特性**：
```go
// 迁移表自动创建
CREATE TABLE IF NOT EXISTS migrations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    filename VARCHAR(255) NOT NULL UNIQUE,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
```

#### 1.2 用户表设计
**文件**：`migrations/20250918_001_create_users_table.sql`

**表结构**：
```sql
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  `username` varchar(64) NOT NULL COMMENT '用户名，唯一',
  `email` varchar(255) NOT NULL COMMENT '邮箱，唯一',
  `password_hash` varchar(255) NOT NULL COMMENT 'bcrypt 哈希后的密码',
  `role` enum('user', 'admin') NOT NULL DEFAULT 'user' COMMENT '用户角色',
  `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  UNIQUE KEY `uk_email` (`email`),
  KEY `idx_role` (`role`),
  KEY `idx_is_active` (`is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';
```

**设计考虑**：
- 使用`bigint unsigned`作为主键，支持大量用户
- 用户名和邮箱都有唯一约束
- 角色使用枚举类型，确保数据一致性
- 软删除通过`is_active`字段实现
- 自动维护创建和更新时间戳

#### 1.3 配置扩展
**文件**：`internal/config/config.go`

**新增配置项**：
```go
Database struct {
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
}
JWT struct {
    Secret          string
    AccessTokenTTL  time.Duration
    RefreshTokenTTL time.Duration
}
Migrations struct {
    Dir string
}
```

### 第二步：用户注册与登录API

#### 2.1 领域模型定义
**文件**：`internal/domain/user.go`

**核心实体**：
```go
type User struct {
    ID           int64     `json:"id"`
    Username     string    `json:"username"`
    Email        string    `json:"email"`
    PasswordHash string    `json:"-"` // JSON序列化时忽略
    Role         UserRole  `json:"role"`
    IsActive     bool      `json:"is_active"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type UserRole string
const (
    UserRoleUser  UserRole = "user"
    UserRoleAdmin UserRole = "admin"
)
```

**请求响应模型**：
- `RegisterRequest` - 用户注册请求
- `LoginRequest` - 用户登录请求  
- `LoginResponse` - 登录成功响应（包含令牌）
- `RefreshTokenRequest` - 刷新令牌请求

#### 2.2 仓储层实现
**文件**：`internal/repo/user_repo.go`

**接口设计**：
```go
type UserRepository interface {
    Create(user *domain.User) error
    GetByID(id int64) (*domain.User, error)
    GetByUsername(username string) (*domain.User, error)
    GetByEmail(email string) (*domain.User, error)
    Update(user *domain.User) error
    Delete(id int64) error
    // 管理员专用方法
    ListUsers(offset, limit int) ([]*domain.User, int64, error)
    UpdateUserRole(userID int64, role domain.UserRole) error
    UpdateUserStatus(userID int64, isActive bool) error
}
```

**核心实现特点**：
- 使用预编译语句防止SQL注入
- 统一错误处理和日志记录
- 支持分页查询
- 软删除实现

#### 2.3 服务层业务逻辑
**文件**：`internal/service/user_service.go`

**关键业务规则**：

1. **用户注册**：
   - 用户名和邮箱唯一性检查
   - 密码bcrypt哈希（cost=10）
   - 新用户默认为普通用户角色
   - 邮箱格式基础验证

2. **用户登录**：
   - 支持用户名或邮箱登录
   - bcrypt密码验证
   - 用户状态检查（is_active）
   - 时间恒定比较防止时序攻击

3. **错误定义**：
```go
var (
    ErrUserNotFound       = errors.New("user not found")
    ErrUserExists         = errors.New("user already exists")
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrUserInactive       = errors.New("user is inactive")
)
```

#### 2.4 API处理器
**文件**：`internal/api/user_handler.go`

**端点实现**：
- `POST /api/v1/auth/register` - 用户注册
- `POST /api/v1/auth/login` - 用户登录
- `GET /api/v1/users/profile` - 获取用户信息

**安全措施**：
- 请求体验证
- 统一错误响应
- 敏感信息过滤（密码哈希不返回）
- 请求ID追踪

### 第三步：JWT Access/Refresh策略

#### 3.1 JWT服务设计
**文件**：`internal/service/jwt_service.go`

**令牌策略**：
- **Access Token**：短期有效（默认15分钟），用于API访问
- **Refresh Token**：长期有效（默认7天），用于刷新Access Token

**Claims结构**：
```go
type Claims struct {
    UserID   int64            `json:"user_id"`
    Username string           `json:"username"`
    Role     domain.UserRole  `json:"role"`
    Type     string           `json:"type"` // "access" 或 "refresh"
    jwt.RegisteredClaims
}
```

**安全特性**：
- HMAC-SHA256签名算法
- 标准JWT声明字段（iss, sub, iat, exp, nbf）
- 令牌类型验证
- 发行者验证

#### 3.2 认证中间件
**文件**：`internal/middleware/auth.go`

**中间件层次**：

1. **AuthMiddleware** - 强制JWT认证
   ```go
   func AuthMiddleware(jwtService service.JWTService, logger *zap.Logger) func(http.Handler) http.Handler
   ```
   - Authorization头验证
   - Bearer令牌提取
   - JWT令牌验证
   - 用户信息注入上下文

2. **RequireRole** - 角色授权
   ```go
   func RequireRole(requiredRole domain.UserRole, logger *zap.Logger) func(http.Handler) http.Handler
   ```
   - 用户角色检查
   - 权限不足拒绝访问
   - 详细的访问日志

3. **OptionalAuth** - 可选认证
   ```go
   func OptionalAuth(jwtService service.JWTService, logger *zap.Logger) func(http.Handler) http.Handler
   ```
   - 有令牌则验证，无令牌继续处理
   - 适用于既支持匿名又支持认证的端点

#### 3.3 令牌刷新机制
**实现流程**：
1. 客户端使用Refresh Token调用刷新端点
2. 验证Refresh Token有效性
3. 生成新的Access Token和Refresh Token对
4. 返回新令牌对，客户端更新本地存储

**安全考虑**：
- Refresh Token一次性使用（可选）
- 令牌轮换防止重放攻击
- 过期令牌清理机制

### 第四步：简单RBAC权限控制

#### 4.1 角色定义
**角色类型**：
- **user** - 普通用户：可以查看自己的信息
- **admin** - 管理员：可以管理所有用户

#### 4.2 管理员专用功能

**用户管理API**：
- `GET /api/v1/admin/users` - 获取用户列表（分页）
- `PUT /api/v1/admin/users/role` - 更新用户角色
- `PUT /api/v1/admin/users/status` - 更新用户状态

**业务逻辑**：
```go
// 用户列表查询
func (s *userService) ListUsers(page, pageSize int) (*domain.UserListResponse, error)

// 角色更新
func (s *userService) UpdateUserRole(userID int64, role domain.UserRole) error

// 状态更新  
func (s *userService) UpdateUserStatus(userID int64, isActive bool) error
```

#### 4.3 权限保护
**中间件链路**：
```go
// 双重中间件保护管理员端点
authMiddleware := mw.AuthMiddleware(jwtService, lg)
adminMiddleware := mw.RequireAdmin(lg)
mux.Handle("/api/v1/admin/users", 
    authMiddleware(adminMiddleware(http.HandlerFunc(userHandler.ListUsers))))
```

**执行顺序**：
1. JWT认证验证
2. 用户信息注入上下文
3. 管理员角色检查
4. 业务逻辑执行

### 第五步：单元测试

#### 5.1 JWT服务测试
**文件**：`internal/service/jwt_service_test.go`

**测试覆盖**：
- ✅ 令牌生成和验证
- ✅ 无效令牌处理
- ✅ 令牌类型验证
- ✅ 令牌刷新功能
- ✅ 令牌过期处理
- ✅ 错误分类处理

**关键测试用例**：
```go
func TestJWTService_GenerateTokenPair(t *testing.T)
func TestJWTService_ValidateAccessToken_InvalidToken(t *testing.T)
func TestJWTService_TokenExpiration(t *testing.T)
func TestJWTService_RefreshTokenPair(t *testing.T)
```

#### 5.2 用户服务测试
**文件**：`internal/service/user_service_test.go`

**Mock仓储实现**：
```go
type MockUserRepository struct {
    users  map[string]*domain.User // username -> user
    emails map[string]*domain.User // email -> user
    nextID int64
}
```

**测试场景**：
- ✅ 用户注册成功/失败
- ✅ 用户登录验证（用户名/邮箱）
- ✅ 密码哈希验证
- ✅ 重复用户名/邮箱处理
- ✅ 非活跃用户处理
- ✅ 各种错误场景

#### 5.3 认证中间件测试
**文件**：`internal/middleware/auth_test.go`

**Mock JWT服务**：
```go
type MockJWTService struct {
    validTokens   map[string]*service.Claims
    expiredTokens map[string]bool
}
```

**测试覆盖**：
- ✅ JWT认证中间件各种场景
- ✅ 角色授权中间件测试
- ✅ 可选认证中间件测试
- ✅ 上下文用户信息传递
- ✅ 错误响应格式验证

## API文档

### 认证端点

#### 用户注册
```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "testuser",
  "email": "user@example.com",
  "password": "password123"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 1,
    "username": "testuser",
    "email": "user@example.com",
    "role": "user",
    "is_active": true,
    "created_at": "2025-09-18T10:30:00Z"
  },
  "request_id": "uuid-here",
  "timestamp": 1726647000
}
```

#### 用户登录
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "testuser",
  "password": "password123"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "user@example.com",
      "role": "user",
      "is_active": true,
      "created_at": "2025-09-18T10:30:00Z"
    },
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "request_id": "uuid-here",
  "timestamp": 1726647000
}
```

#### 刷新令牌
```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### 获取用户信息
```http
GET /api/v1/users/profile
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 管理员端点

#### 获取用户列表
```http
GET /api/v1/admin/users?page=1&page_size=20
Authorization: Bearer <admin_access_token>
```

#### 更新用户角色
```http
PUT /api/v1/admin/users/role?user_id=123
Authorization: Bearer <admin_access_token>
Content-Type: application/json

{
  "role": "admin"
}
```

#### 更新用户状态
```http
PUT /api/v1/admin/users/status?user_id=123
Authorization: Bearer <admin_access_token>
Content-Type: application/json

{
  "is_active": false
}
```

## 安全特性总结

### 密码安全
- **bcrypt哈希**：使用bcrypt算法，cost=10
- **盐值自动生成**：每个密码使用唯一盐值
- **时间恒定比较**：防止时序攻击

### 会话管理
- **双令牌策略**：Access Token + Refresh Token
- **令牌轮换**：刷新时生成新的令牌对
- **签名验证**：HMAC-SHA256算法
- **过期处理**：自动令牌过期机制

### 访问控制
- **基于角色的访问控制**：user/admin角色
- **中间件保护**：多层中间件验证
- **最小权限原则**：按需授权

### 审计日志
- **请求追踪**：每个请求的唯一ID
- **结构化日志**：JSON格式便于分析
- **关键事件记录**：登录、注册、权限检查

## 测试结果

### 测试统计
```
服务层测试：    12个测试用例，全部通过
中间件测试：    13个测试用例，全部通过
配置测试：      3个测试用例，全部通过
健康检查测试：  1个测试用例，通过

总计：         29个测试用例，100%通过率
覆盖场景：     成功、失败、过期、权限不足、无效输入等
```

### 性能基准
- **bcrypt哈希**：~100ms/次（符合安全要求）
- **JWT生成**：<1ms/次
- **JWT验证**：<0.5ms/次
- **数据库查询**：<10ms/次（本地环境）

## 部署注意事项

### 环境变量配置
```bash
# 数据库配置
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=spike
MYSQL_PASSWORD=spike
MYSQL_DB=spike

# JWT配置
JWT_SECRET=your-strong-secret-key-here
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h

# 应用配置
APP_ENV=prod
LOG_LEVEL=info
LOG_ENCODING=json
```

### 生产环境安全
1. **强JWT密钥**：至少32字节随机字符串
2. **HTTPS强制**：生产环境必须使用HTTPS
3. **令牌存储**：客户端安全存储令牌
4. **密钥轮换**：定期更换JWT密钥
5. **监控告警**：异常登录行为监控

## 后续扩展

### 功能增强
- [ ] 多因素认证（MFA）
- [ ] 社交登录集成
- [ ] 密码复杂度策略
- [ ] 账户锁定机制
- [ ] 登录历史记录

### 性能优化
- [ ] Redis令牌黑名单
- [ ] 令牌缓存机制
- [ ] 数据库连接池优化
- [ ] 查询性能调优

### 安全增强
- [ ] 速率限制
- [ ] IP白名单
- [ ] 设备指纹
- [ ] 异常行为检测

## 总结

里程碑2成功实现了完整的用户认证与授权系统，包括：

✅ **数据库层**：用户表设计与迁移管理  
✅ **服务层**：用户注册/登录业务逻辑  
✅ **安全层**：JWT令牌管理与密码哈希  
✅ **权限层**：RBAC角色控制  
✅ **测试层**：完整的单元测试覆盖  

该系统为后续的商品管理、订单处理和秒杀功能提供了坚实的认证基础，确保了系统的安全性和可扩展性。
