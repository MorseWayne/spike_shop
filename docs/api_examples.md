# API 使用示例

本文档展示了商品与库存管理API的使用示例。

## 前置条件

1. 启动服务：`./spike-server`
2. 确保数据库迁移已执行
3. 获取管理员JWT令牌（通过用户认证API）

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
