# API 文档与使用示例

本文档展示了商品与库存管理API的完整路由结构和使用示例。

## 🗺️ API 路由结构概览

我们的 API 采用 RESTful 设计风格，使用 Gin 框架实现。以下是完整的路由树：

```
Base URL: http://localhost:8080

/healthz                                    # 健康检查 (GET)

/api/v1/
├── auth/                                   # 🔐 用户认证 (公开)
│   ├── POST   /register                    # 用户注册
│   ├── POST   /login                       # 用户登录
│   └── POST   /refresh                     # 刷新令牌
│
├── users/                                  # 👤 用户管理 (需认证)
│   └── GET    /profile                     # 获取用户信息
│
├── products/                               # 📦 商品管理 (公开)
│   ├── GET    /                            # 获取商品列表
│   ├── GET    /search                      # 搜索商品
│   ├── GET    /with-inventory             # 获取带库存的商品列表
│   ├── GET    /:id                        # 获取商品详情
│   ├── GET    /:id/inventory              # 获取商品库存
│   └── GET    /:id/inventory/check        # 检查库存可用性
│
├── inventory/                              # 📋 库存操作 (需认证)
│   ├── GET    /                           # 获取库存列表
│   ├── POST   /reserve                    # 预留库存
│   ├── POST   /release                    # 释放库存
│   └── POST   /consume                    # 消费库存
│
└── admin/                                  # 🛡️ 管理员专用 (需认证+管理员权限)
    ├── users/                              # 用户管理
    │   ├── GET    /                        # 获取用户列表
    │   ├── PUT    /role                    # 更新用户角色
    │   └── PUT    /status                  # 更新用户状态
    │
    ├── products/                           # 商品管理
    │   ├── POST   /                        # 创建商品
    │   ├── PUT    /:id                     # 更新商品
    │   ├── DELETE /:id                     # 删除商品
    │   ├── GET    /stats                   # 获取商品统计
    │   └── POST   /:id/inventory/adjust    # 调整库存
    │
    └── inventory/                          # 库存管理
        ├── POST   /                        # 创建库存记录
        ├── GET    /:id                     # 获取库存详情
        ├── PUT    /:id                     # 更新库存记录
        ├── GET    /alerts/low-stock        # 获取低库存警告
        └── GET    /stats                   # 获取库存统计
```

## 🔑 权限说明

| 权限级别 | 说明 | 标识 |
|---------|------|------|
| 🌍 公开 | 无需认证，任何人都可访问 | 无标识 |
| 🔐 需认证 | 需要有效的 JWT 令牌 | `Authorization: Bearer <token>` |
| 🛡️ 管理员 | 需要认证 + 管理员角色 | 认证 + `role: admin` |

## 📝 HTTP 方法说明

| 方法 | 用途 | 示例 |
|------|------|------|
| `GET` | 获取资源 | 查询商品列表、获取用户信息 |
| `POST` | 创建资源或执行操作 | 创建商品、预留库存 |
| `PUT` | 更新资源 | 更新商品信息、修改用户角色 |
| `DELETE` | 删除资源 | 删除商品 |

## 🚀 快速开始

### 前置条件

1. 启动服务：`./spike-server`
2. 确保数据库迁移已执行
3. 获取管理员JWT令牌（通过用户认证API）

### 基本使用流程

1. **用户注册/登录** → 获取 JWT 令牌
2. **浏览商品** → 公开 API，无需认证
3. **库存操作** → 需要用户认证
4. **管理功能** → 需要管理员权限

## 🏗️ 技术架构

### 路由框架：Gin
- **高性能**：基于 httprouter，性能极优
- **中间件支持**：内置 Recovery、Logger、CORS 等
- **路由组**：支持路由分组和嵌套中间件
- **参数绑定**：自动解析 URL 参数和请求体

### 路由组织结构
```go
// 路由组示例
v1 := engine.Group("/api/v1")
{
    // 认证路由组
    auth := v1.Group("/auth")
    {
        auth.POST("/register", registerHandler)
        auth.POST("/login", loginHandler)
    }
    
    // 需要认证的路由组
    protected := v1.Group("/users")
    protected.Use(authMiddleware())
    {
        protected.GET("/profile", profileHandler)
    }
}
```

### 中间件链
1. **Recovery** - Panic 恢复
2. **Logger** - 访问日志
3. **CORS** - 跨域支持
4. **Auth** - JWT 认证（特定路由）
5. **Admin** - 管理员权限（管理路由）

## 📋 API 详细示例

## 商品管理 API

### 1. 创建商品（管理员）

```bash
# POST /api/v1/admin/products
curl -X POST http://localhost:8080/api/v1/admin/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "name": "iPhone 15 Pro Max",
    "description": "苹果最新旗舰手机，512GB存储",
    "price": 9999.00,
    "sku": "IPHONE-15-PRO-MAX-512GB",
    "brand": "Apple",
    "weight": 0.221,
    "image_url": "https://example.com/iphone15.jpg"
  }'
```

### 2. 获取商品列表（公开）

```bash
# GET /api/v1/products
curl "http://localhost:8080/api/v1/products?page=1&page_size=10&status=active"
```

### 3. 搜索商品（公开）

```bash
# GET /api/v1/products/search
curl "http://localhost:8080/api/v1/products/search?keyword=iPhone&page=1&page_size=5"
```

### 4. 获取商品详情（公开）

```bash
# GET /api/v1/products/{id}
curl "http://localhost:8080/api/v1/products/1"
```

### 5. 更新商品（管理员）

```bash
# PUT /api/v1/admin/products/{id}
curl -X PUT http://localhost:8080/api/v1/admin/products/1 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "name": "iPhone 15 Pro Max（更新版）",
    "price": 8999.00
  }'
```

### 6. 删除商品（管理员）

```bash
# DELETE /api/v1/admin/products/{id}
curl -X DELETE http://localhost:8080/api/v1/admin/products/1 \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN"
```

### 7. 获取商品统计（管理员）

```bash
# GET /api/v1/admin/products/stats
curl -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  "http://localhost:8080/api/v1/admin/products/stats"
```

## 库存管理 API

### 1. 创建库存记录（管理员）

```bash
# POST /api/v1/admin/inventory
curl -X POST http://localhost:8080/api/v1/admin/inventory \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "product_id": 1,
    "stock": 100,
    "reorder_point": 10,
    "max_stock": 1000
  }'
```

### 2. 获取商品库存（公开）

```bash
# GET /api/v1/products/{product_id}/inventory
curl "http://localhost:8080/api/v1/products/1/inventory"
```

### 3. 检查库存可用性（公开）

```bash
# GET /api/v1/products/{product_id}/inventory/check
curl "http://localhost:8080/api/v1/products/1/inventory/check?quantity=5"
```

### 4. 获取库存列表（需要认证）

```bash
# GET /api/v1/inventory
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v1/inventory?page=1&page_size=10"
```

### 5. 调整库存（管理员）

```bash
# POST /api/v1/admin/products/{product_id}/inventory/adjust
curl -X POST http://localhost:8080/api/v1/admin/products/1/inventory/adjust \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "quantity": 50,
    "reason": "补充库存",
    "type": "in"
  }'
```

### 6. 预留库存（需要认证）

```bash
# POST /api/v1/inventory/reserve
curl -X POST http://localhost:8080/api/v1/inventory/reserve \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "product_id": 1,
    "quantity": 2
  }'
```

### 7. 释放库存（需要认证）

```bash
# POST /api/v1/inventory/release
curl -X POST http://localhost:8080/api/v1/inventory/release \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "product_id": 1,
    "quantity": 1
  }'
```

### 8. 消费库存（需要认证）

```bash
# POST /api/v1/inventory/consume
curl -X POST http://localhost:8080/api/v1/inventory/consume \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "product_id": 1,
    "quantity": 1
  }'
```

### 9. 获取低库存警告（管理员）

```bash
# GET /api/v1/admin/inventory/alerts/low-stock
curl -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  "http://localhost:8080/api/v1/admin/inventory/alerts/low-stock"
```

### 10. 获取库存统计（管理员）

```bash
# GET /api/v1/admin/inventory/stats
curl -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  "http://localhost:8080/api/v1/admin/inventory/stats"
```

## 批量操作

### 获取带库存信息的商品列表

```bash
# POST /api/v1/products/with-inventory
curl -X POST http://localhost:8080/api/v1/products/with-inventory \
  -H "Content-Type: application/json" \
  -d '{
    "product_ids": [1, 2, 3]
  }'
```

## 响应格式

所有API响应都遵循统一格式：

```json
{
  "code": 0,
  "message": "OK",
  "data": {...},
  "request_id": "uuid-string",
  "timestamp": 1640995200
}
```

## 错误处理

常见错误码：
- `0`: 成功
- `10000`: 内部服务器错误
- `10001`: 参数错误
- `10002`: 请求超时

HTTP状态码：
- `200`: 成功
- `400`: 请求参数错误
- `401`: 未认证
- `403`: 权限不足
- `404`: 资源不存在
- `409`: 资源冲突（如SKU重复）
- `500`: 服务器内部错误

## 缓存策略

### 缓存类型
- **内存缓存** (`CACHE_TYPE=memory`)：适合单实例部署，重启后数据丢失
- **Redis缓存** (`CACHE_TYPE=redis`)：适合多实例部署，数据持久化
- **禁用缓存** (`CACHE_ENABLED=false`)：适合开发调试

### 缓存TTL
- 商品详情缓存：默认5分钟（`CACHE_TTL=5m`）
- 库存信息缓存：2.5分钟（商品TTL的一半，因为变化频繁）
- 写操作会自动清除相关缓存

### 环境变量配置
```bash
# 启用Redis缓存
CACHE_ENABLED=true
CACHE_TYPE=redis
CACHE_TTL=5m
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your_password
REDIS_DB=0

# 启用内存缓存
CACHE_ENABLED=true
CACHE_TYPE=memory
CACHE_TTL=5m

# 禁用缓存
CACHE_ENABLED=false
```

### Redis高级功能
- 支持单实例和集群模式
- 连接池优化
- 自动重连和故障转移
- 管道操作支持
- Lua脚本支持

## 性能优化

1. **分页查询**：建议每页不超过100条记录
2. **批量操作**：使用批量API减少网络请求
3. **索引优化**：已为常用查询字段添加索引
4. **缓存策略**：读多写少的数据启用缓存
5. **乐观锁**：库存更新使用版本号防止并发冲突

## 🔧 开发工具

### 路由调试
Gin 框架提供了路由调试功能，启动时会打印所有注册的路由：

```
[GIN-debug] POST   /api/v1/auth/register     --> handler
[GIN-debug] POST   /api/v1/auth/login        --> handler
[GIN-debug] GET    /api/v1/products          --> handler
[GIN-debug] GET    /api/v1/products/:id      --> handler
...
```

### API 测试工具推荐
1. **Postman** - 图形化 API 测试工具
2. **Insomnia** - 轻量级 REST 客户端
3. **curl** - 命令行工具（本文档示例）
4. **httpie** - 用户友好的命令行工具

### 环境切换
```bash
# 开发环境 - 显示详细路由信息
APP_ENV=dev ./spike-server

# 生产环境 - 静默模式
APP_ENV=prod ./spike-server
```

## 📚 相关文档

- [项目初始化文档](./trace/01_milestone_0_project_initialization.md)
- [用户认证文档](./trace/03_milestone_2_user_authentication.md)
- [数据库迁移文档](../migrations/)
- [配置文件说明](../internal/config/)
