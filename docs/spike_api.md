# 秒杀系统 API 文档

本文档详细介绍了秒杀系统的所有API接口，包括使用示例、参数说明和响应格式。

## 🎯 秒杀系统概述

秒杀系统是一个高并发、高可用的限时抢购系统，具备以下核心特性：

- **高并发处理**：支持万级并发请求
- **防超卖机制**：Redis原子性操作 + 数据库最终一致性
- **多重限流**：全局限流 + 用户限流 + 接口限流
- **异步处理**：关键业务流程异步化，提高响应速度
- **幂等保证**：防止重复提交和数据不一致

## 🗺️ 秒杀 API 路由结构

```
Base URL: http://localhost:8080

/api/v1/spike/
├── GET    /health                           # ⚡ 健康检查
├── GET    /events                           # 🌍 获取活跃秒杀活动列表
├── GET    /events/{id}                      # 🌍 获取秒杀活动详情
├── GET    /events/{id}/stats                # 🌍 获取秒杀统计信息
├── POST   /participate                      # 🔐 参与秒杀 (核心接口)
├── GET    /orders                           # 🔐 获取用户秒杀订单列表
├── GET    /orders/{id}                      # 🔐 获取秒杀订单详情
└── POST   /orders/{id}/cancel               # 🔐 取消秒杀订单

/api/v1/admin/spike/
└── POST   /events/{id}/warmup               # 🛡️ 预热库存缓存
```

## 🔑 权限级别说明

| 权限 | 说明 | 标识 |
|------|------|------|
| 🌍 公开 | 无需认证，任何人都可访问 | 无标识 |
| 🔐 认证 | 需要有效的JWT令牌 | `Authorization: Bearer <token>` |
| 🛡️ 管理员 | 需要认证 + 管理员角色 | 认证 + `role: admin` |
| ⚡ 系统 | 系统健康检查 | 无认证要求 |

## 📋 API 详细文档

### 1. 健康检查

检查秒杀服务的运行状态。

```http
GET /api/v1/spike/health
```

**响应示例：**
```json
{
  "code": 0,
  "message": "healthy",
  "data": {
    "service": "spike-service",
    "status": "healthy",
    "timestamp": 1640995200,
    "version": "v1.0.0"
  },
  "request_id": "req-12345",
  "timestamp": 1640995200
}
```

### 2. 获取活跃秒杀活动列表 🌍

获取当前正在进行或即将开始的秒杀活动列表。

```http
GET /api/v1/spike/events?page=1&page_size=20&sort_by=start_at&sort_order=desc
```

**查询参数：**
- `page` (int, 可选): 页码，默认1
- `page_size` (int, 可选): 每页大小，默认20，最大100
- `sort_by` (string, 可选): 排序字段 (start_at, created_at, spike_price)
- `sort_order` (string, 可选): 排序方向 (asc, desc)，默认desc

**请求示例：**
```bash
curl "http://localhost:8080/api/v1/spike/events?page=1&page_size=10&sort_by=start_at"
```

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "events": [
      {
        "id": 1,
        "product_id": 100,
        "title": "iPhone 15 Pro Max 秒杀",
        "start_time": "2024-01-01T10:00:00Z",
        "end_time": "2024-01-01T12:00:00Z",
        "original_price": 9999.00,
        "spike_price": 7999.00,
        "total_stock": 1000,
        "available_stock": 856,
        "sold_count": 144,
        "status": "active",
        "created_at": "2023-12-25T00:00:00Z",
        "updated_at": "2024-01-01T10:30:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

### 3. 获取秒杀活动详情 🌍

获取指定秒杀活动的详细信息，包含商品信息和实时库存。

```http
GET /api/v1/spike/events/{id}
```

**路径参数：**
- `id` (int): 秒杀活动ID

**请求示例：**
```bash
curl "http://localhost:8080/api/v1/spike/events/1"
```

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "spike_event": {
      "id": 1,
      "product_id": 100,
      "title": "iPhone 15 Pro Max 秒杀",
      "start_time": "2024-01-01T10:00:00Z",
      "end_time": "2024-01-01T12:00:00Z",
      "original_price": 9999.00,
      "spike_price": 7999.00,
      "total_stock": 1000,
      "available_stock": 856,
      "sold_count": 144,
      "status": "active"
    },
    "product": {
      "id": 100,
      "name": "iPhone 15 Pro Max",
      "description": "苹果最新旗舰手机",
      "price": 9999.00,
      "sku": "IPHONE-15-PRO-MAX",
      "brand": "Apple",
      "image_url": "https://example.com/iphone15.jpg"
    }
  }
}
```

### 4. 获取秒杀统计信息 🌍

获取指定秒杀活动的详细统计数据。

```http
GET /api/v1/spike/events/{id}/stats
```

**请求示例：**
```bash
curl "http://localhost:8080/api/v1/spike/events/1/stats"
```

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "event_id": 1,
    "total_stock": 1000,
    "sold_count": 144,
    "remaining_stock": 856,
    "sold_out": false,
    "order_stats": {
      "pending": 20,
      "paid": 100,
      "cancelled": 15,
      "expired": 9
    },
    "is_active": true,
    "start_at": "2024-01-01T10:00:00Z",
    "end_at": "2024-01-01T12:00:00Z"
  }
}
```

### 5. 参与秒杀 🔐 [核心接口]

用户参与秒杀活动，这是系统的核心接口，具有最高的安全性和性能要求。

```http
POST /api/v1/spike/participate
Content-Type: application/json
Authorization: Bearer <your_jwt_token>
X-Idempotency-Key: <unique_key>
```

**请求头：**
- `Authorization`: JWT认证令牌 (必需)
- `X-Idempotency-Key`: 幂等键，防止重复提交 (可选，系统会自动生成)

**请求体：**
```json
{
  "spike_event_id": 1,
  "quantity": 1,
  "idempotency_key": "user123_event1_20240101103000"
}
```

**参数说明：**
- `spike_event_id` (int): 秒杀活动ID
- `quantity` (int): 购买数量，范围1-10
- `idempotency_key` (string): 幂等键，防止重复提交

**请求示例：**
```bash
curl -X POST http://localhost:8080/api/v1/spike/participate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-Idempotency-Key: user123_event1_$(date +%s)" \
  -d '{
    "spike_event_id": 1,
    "quantity": 1,
    "idempotency_key": "user123_event1_20240101103000"
  }'
```

**成功响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "message": "秒杀成功，请尽快完成支付"
  }
}
```

**失败响应示例：**
```json
{
  "code": 0,
  "message": "success", 
  "data": {
    "success": false,
    "message": "商品已售罄"
  }
}
```

### 6. 获取用户秒杀订单列表 🔐

获取当前用户的秒杀订单列表，支持状态过滤和分页。

```http
GET /api/v1/spike/orders?page=1&page_size=20&status=pending&sort_by=created_at&sort_order=desc
```

**查询参数：**
- `page` (int, 可选): 页码，默认1
- `page_size` (int, 可选): 每页大小，默认20
- `status` (string, 可选): 订单状态过滤 (pending, paid, cancelled, expired)
- `sort_by` (string, 可选): 排序字段 (created_at, total_amount)
- `sort_order` (string, 可选): 排序方向 (asc, desc)

**请求示例：**
```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  "http://localhost:8080/api/v1/spike/orders?status=pending&page=1"
```

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "orders": [
      {
        "id": 1001,
        "spike_event_id": 1,
        "user_id": 123,
        "order_id": null,
        "product_id": 100,
        "quantity": 1,
        "spike_price": 7999.00,
        "total_amount": 7999.00,
        "status": "pending",
        "idempotency_key": "user123_event1_20240101103000",
        "created_at": "2024-01-01T10:30:00Z",
        "updated_at": "2024-01-01T10:30:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

### 7. 获取秒杀订单详情 🔐

获取指定秒杀订单的详细信息，包含活动和用户信息。

```http
GET /api/v1/spike/orders/{id}
```

**路径参数：**
- `id` (int): 秒杀订单ID

**请求示例：**
```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  "http://localhost:8080/api/v1/spike/orders/1001"
```

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "spike_order": {
      "id": 1001,
      "spike_event_id": 1,
      "user_id": 123,
      "product_id": 100,
      "quantity": 1,
      "spike_price": 7999.00,
      "total_amount": 7999.00,
      "status": "pending",
      "created_at": "2024-01-01T10:30:00Z"
    },
    "spike_event": {
      "id": 1,
      "title": "iPhone 15 Pro Max 秒杀",
      "start_time": "2024-01-01T10:00:00Z",
      "end_time": "2024-01-01T12:00:00Z"
    },
    "user": {
      "id": 123,
      "username": "user123",
      "email": "user123@example.com"
    }
  }
}
```

### 8. 取消秒杀订单 🔐

取消指定的秒杀订单，会异步恢复库存。

```http
POST /api/v1/spike/orders/{id}/cancel
Content-Type: application/json
Authorization: Bearer <your_jwt_token>
```

**路径参数：**
- `id` (int): 秒杀订单ID

**请求体：**
```json
{
  "reason": "不想要了"
}
```

**请求示例：**
```bash
curl -X POST http://localhost:8080/api/v1/spike/orders/1001/cancel \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "reason": "不想要了"
  }'
```

**响应示例：**
```json
{
  "code": 0,
  "message": "订单取消成功",
  "data": null
}
```

### 9. 预热库存缓存 🛡️ (管理员)

将指定秒杀活动的库存数据预热到Redis缓存中，提高秒杀时的响应速度。

```http
POST /api/v1/admin/spike/events/{id}/warmup
Authorization: Bearer <admin_jwt_token>
```

**路径参数：**
- `id` (int): 秒杀活动ID

**请求示例：**
```bash
curl -X POST http://localhost:8080/api/v1/admin/spike/events/1/warmup \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN"
```

**响应示例：**
```json
{
  "code": 0,
  "message": "库存预热成功",
  "data": null
}
```

## 🛡️ 安全机制

### 1. 多重限流保护

| 限流类型 | 限制 | 说明 |
|---------|------|------|
| 全局限流 | 1000 req/min | 防止系统过载 |
| 用户限流 | 5 req/min | 防止单用户恶意请求 |
| API限流 | 100 req/min | 通用API保护 |

### 2. 幂等性保证

- **自动幂等键生成**：基于用户ID、方法、路径和时间戳
- **手动幂等键**：通过 `X-Idempotency-Key` 头提供
- **Redis去重**：使用Redis存储幂等键，防止重复处理

### 3. 防超卖机制

- **Redis预减库存**：使用Lua脚本保证原子性
- **异步DB落库**：通过消息队列异步处理
- **用户去重标记**：防止同一用户重复参与

## 🚀 性能优化

### 1. 缓存策略

- **活动信息缓存**：2小时TTL
- **库存信息缓存**：实时更新
- **用户标记缓存**：24小时TTL

### 2. 异步处理

```
用户请求 → Redis预减库存 → 立即响应
           ↓
    异步消息队列 → DB事务落库
```

### 3. 数据库优化

- **索引优化**：为查询字段添加复合索引
- **读写分离**：读操作使用缓存优先
- **批量操作**：减少数据库连接开销

## 📊 监控指标

### 关键指标

- **QPS**: 每秒请求数
- **RT**: 平均响应时间  
- **成功率**: 请求成功率
- **库存命中率**: 缓存命中率
- **消息积压**: MQ消息堆积数量

### 业务指标

- **参与人数**: 实际参与秒杀的用户数
- **转化率**: 下单成功率
- **支付率**: 订单支付完成率
- **库存准确率**: 库存数据一致性

## 🔧 错误码

| 错误码 | HTTP状态 | 说明 |
|-------|---------|------|
| 0 | 200 | 成功 |
| 10000 | 500 | 服务器内部错误 |
| 10001 | 400 | 请求参数错误 |
| 10002 | 408 | 请求超时 |
| 20001 | 401 | 未认证 |
| 20002 | 403 | 权限不足 |
| 20003 | 404 | 资源不存在 |
| 20004 | 429 | 请求过于频繁 |
| 30001 | 400 | 秒杀活动不存在 |
| 30002 | 400 | 秒杀活动未开始 |
| 30003 | 400 | 秒杀活动已结束 |
| 30004 | 400 | 商品已售罄 |
| 30005 | 400 | 用户已参与该活动 |
| 30006 | 400 | 购买数量超出限制 |

## 🧪 测试用例

### 压力测试场景

```bash
# 1000并发用户同时参与秒杀
ab -n 10000 -c 1000 -H "Authorization: Bearer TOKEN" \
   -p spike_request.json \
   http://localhost:8080/api/v1/spike/participate

# spike_request.json 内容
{
  "spike_event_id": 1,
  "quantity": 1,
  "idempotency_key": "test_$(date +%s%N)"
}
```

### 功能测试脚本

```bash
#!/bin/bash
# 完整的秒杀流程测试

# 1. 获取活动列表
curl "http://localhost:8080/api/v1/spike/events"

# 2. 参与秒杀
curl -X POST http://localhost:8080/api/v1/spike/participate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"spike_event_id":1,"quantity":1}'

# 3. 查看订单
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/spike/orders"

# 4. 取消订单
curl -X POST http://localhost:8080/api/v1/spike/orders/1001/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"reason":"测试取消"}'
```

## 📚 相关文档

- [秒杀系统设计文档](./project_design.md)
- [数据库设计](../migrations/)
- [开发环境搭建](./local_dev_guide.md)
- [部署指南](./build.md)
