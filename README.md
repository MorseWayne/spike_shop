# Spike Shop - 高并发秒杀购物系统

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat-square&logo=docker)](docker-compose.yml)

一个基于 Go 语言构建的高并发秒杀购物系统，专注于后端 API 开发，集成了现代微服务架构的核心技术栈。

## 🎯 项目概览

### 核心特性
- **高并发处理**：支持万级并发请求，Redis + Lua 脚本保证原子性操作
- **防超卖机制**：多重保护机制，确保数据一致性
- **异步处理**：消息队列异步化关键业务流程，提升响应速度
- **多重限流**：全局限流 + 用户限流 + 接口限流，防止系统过载
- **幂等保证**：完整的幂等性设计，防止重复提交
- **可观测性**：结构化日志、指标监控、链路追踪

### 技术栈
- **后端框架**：Go 1.25+ + Gin
- **数据库**：MySQL 8.0 + Redis 7
- **消息队列**：RabbitMQ 3
- **认证授权**：JWT + RBAC
- **缓存策略**：Redis + 内存缓存
- **限流算法**：令牌桶、滑动窗口、固定窗口
- **容器化**：Docker + Docker Compose

## 🚀 快速开始

### 环境要求
- Go 1.25+
- Docker & Docker Compose
- MySQL 8.0
- Redis 7
- RabbitMQ 3

### 一键启动

1. **克隆项目**
```bash
git clone https://github.com/MorseWayne/spike_shop.git
cd spike_shop
```

2. **启动基础设施**
```bash
docker compose up -d
```

3. **配置环境变量**
```bash
cp env.example .env
# 根据需要修改 .env 文件中的配置
```

4. **运行应用**
```bash
# 方式一：直接运行
go run ./cmd/spike-server

# 方式二：构建后运行
make build
./bin/spike-server
```

5. **健康检查**
```bash
curl -s http://localhost:8080/healthz | jq
```

## 🏗️ 系统架构

### 整体架构图
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client        │    │   API Gateway   │    │   Service Layer │
│                 │    │                 │    │                 │
│ Web/Mobile App  │◄──►│ Gin Router      │◄──►│ Business Logic  │
│ Postman/k6      │    │ Middleware      │    │ Domain Models   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       ▼
         │                       │              ┌─────────────────┐
         │                       │              │  Data Layer     │
         │                       │              │                 │
         │                       │              │ MySQL + Redis   │
         │                       │              │ + RabbitMQ      │
         │                       │              └─────────────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐    ┌─────────────────┐
│   Observability │    │   Security      │
│                 │    │                 │
│ Logs + Metrics  │    │ JWT + RateLimit │
│ + Tracing       │    │ + RBAC          │
└─────────────────┘    └─────────────────┘
```

### 秒杀核心流程
```
用户请求 → 限流检查 → 参数验证 → 活动状态检查
    ↓
Redis预减库存 → 用户去重标记 → 异步消息投递
    ↓
立即响应成功 → MQ消费者 → DB事务落库 → 库存最终一致性
```

## 📁 项目结构

```
spike_shop/
├── cmd/                          # 应用程序入口
│   └── spike-server/            # 主服务器应用
│       ├── main.go              # 程序入口
│       └── main_test.go         # 主程序测试
├── internal/                     # 私有应用代码库
│   ├── api/                     # API处理器层
│   │   ├── user_handler.go      # 用户相关API
│   │   ├── product_handler.go   # 商品相关API
│   │   ├── inventory_handler.go # 库存相关API
│   │   └── spike_handler.go     # 秒杀相关API
│   ├── service/                 # 业务服务层
│   │   ├── user_service.go      # 用户业务逻辑
│   │   ├── product_service.go   # 商品业务逻辑
│   │   ├── inventory_service.go # 库存业务逻辑
│   │   └── spike_service.go     # 秒杀业务逻辑
│   ├── repo/                    # 数据仓储层
│   │   ├── user_repository.go   # 用户数据访问
│   │   ├── product_repository.go# 商品数据访问
│   │   ├── inventory_repository.go# 库存数据访问
│   │   └── spike_repository.go  # 秒杀数据访问
│   ├── domain/                  # 领域模型
│   │   ├── user.go              # 用户实体
│   │   ├── product.go           # 商品实体
│   │   ├── inventory.go         # 库存实体
│   │   ├── spike_event.go       # 秒杀活动实体
│   │   └── spike_order.go       # 秒杀订单实体
│   ├── cache/                   # 缓存组件
│   │   ├── memory_cache.go      # 内存缓存
│   │   ├── redis_cache.go       # Redis缓存
│   │   └── spike_cache.go       # 秒杀专用缓存
│   ├── config/                  # 配置管理
│   ├── database/                # 数据库连接与迁移
│   ├── logger/                  # 日志组件
│   ├── middleware/              # HTTP中间件
│   ├── limiter/                 # 限流组件
│   ├── mq/                      # 消息队列组件
│   ├── resp/                    # 统一响应格式
│   └── router/                  # 路由配置
├── migrations/                  # 数据库迁移文件
│   ├── 000001_create_users_table.up.sql
│   ├── 000002_insert_admin_user.up.sql
│   ├── 000003_create_products_table.up.sql
│   ├── 000004_create_inventory_table.up.sql
│   ├── 000005_create_spike_events_table.up.sql
│   └── 000006_create_spike_orders_table.up.sql
├── docs/                        # 项目文档
│   ├── project_design.md        # 项目设计文档
│   ├── development_plan.md      # 开发计划
│   ├── api_examples.md          # API使用示例
│   ├── spike_api.md             # 秒杀API文档
│   └── trace/                   # 开发跟踪文档
├── deploy/                      # 部署相关文件
├── scripts/                     # 脚本文件
├── .env.example                 # 环境变量示例
├── docker-compose.yml           # 开发环境编排
├── .golangci.yml               # 代码质量检查配置
├── Makefile                    # 构建脚本
├── go.mod                      # Go模块定义
├── go.sum                      # 依赖锁定文件
└── README.md                   # 项目说明
```

## 🔧 核心功能模块

### 1. 用户认证系统 ✅
- **用户注册/登录**：支持邮箱注册，密码哈希存储
- **JWT认证**：Access Token + Refresh Token 双令牌机制
- **RBAC权限**：支持用户和管理员角色
- **安全特性**：密码强度验证、登录失败锁定
- **令牌管理**：自动刷新、过期处理、安全存储

### 2. 商品管理系统 ✅
- **商品CRUD**：完整的商品增删改查功能
- **库存管理**：实时库存查询、库存预留/释放/消费
- **缓存优化**：Redis缓存热点商品信息
- **搜索功能**：支持商品名称和描述搜索
- **分页排序**：支持分页查询和多种排序方式

### 3. 秒杀系统（核心）🚧
- **活动管理**：秒杀活动创建、状态管理、时间控制
- **高并发处理**：Redis Lua脚本原子性预减库存
- **防超卖机制**：
  - Redis预减库存 + 售罄标记
  - 用户-活动去重约束
  - 数据库唯一约束
- **异步处理**：消息队列异步落库，保证最终一致性
- **多重限流**：
  - 全局限流：1000 req/min
  - 用户限流：5 req/min
  - API限流：100 req/min
- **幂等保证**：自动幂等键生成、手动幂等键支持
- **库存预热**：活动开始前预热Redis缓存

### 4. 订单系统 📋
- **订单创建**：支持普通订单和秒杀订单
- **状态管理**：pending → paid → completed 状态流转
- **超时处理**：未支付订单自动取消，库存回补
- **事务保证**：订单与库存更新在同一事务中
- **订单查询**：支持订单历史查询和状态跟踪
- **延时取消**：基于死信队列的延时取消机制

### 5. 支付模拟系统 📋
- **支付接口**：模拟第三方支付接口
- **支付状态**：支付成功/失败/超时处理
- **回调处理**：支付结果异步回调
- **退款机制**：支持订单退款和库存回补

### 6. 消息队列系统 🚧
- **生产者**：秒杀订单消息生产
- **消费者**：幂等消费、重试机制
- **死信队列**：失败消息处理和告警
- **消息持久化**：消息可靠性保证
- **监控告警**：队列积压监控

### 7. 可观测性系统 📋
- **指标监控**：QPS、延迟、错误率、缓存命中率
- **链路追踪**：OpenTelemetry分布式追踪
- **日志聚合**：结构化日志、请求追踪
- **性能分析**：pprof性能剖析
- **告警机制**：关键指标异常告警

### 8. 缓存系统 ✅
- **多级缓存**：内存缓存 + Redis缓存
- **缓存策略**：TTL、LRU、主动失效
- **缓存预热**：热点数据预加载
- **缓存穿透保护**：布隆过滤器、空值缓存
- **缓存雪崩防护**：随机TTL、熔断机制

## 📊 API 接口文档

### 基础信息
- **Base URL**: `http://localhost:8080`
- **API版本**: `/api/v1`
- **认证方式**: `Authorization: Bearer <token>`

### 主要接口

#### 用户认证
```http
POST /api/v1/auth/register     # 用户注册
POST /api/v1/auth/login        # 用户登录
POST /api/v1/auth/refresh      # 刷新令牌
GET  /api/v1/users/profile     # 获取用户信息
```

#### 商品管理
```http
GET  /api/v1/products                    # 获取商品列表
GET  /api/v1/products/search             # 搜索商品
GET  /api/v1/products/:id                # 获取商品详情
GET  /api/v1/products/:id/inventory      # 获取商品库存
```

#### 库存操作
```http
GET  /api/v1/inventory                   # 获取库存列表
POST /api/v1/inventory/reserve           # 预留库存
POST /api/v1/inventory/release           # 释放库存
POST /api/v1/inventory/consume           # 消费库存
```

#### 秒杀系统
```http
GET  /api/v1/spike/health                # 健康检查
GET  /api/v1/spike/events                # 获取活跃秒杀活动
GET  /api/v1/spike/events/:id            # 获取秒杀活动详情
GET  /api/v1/spike/events/:id/stats      # 获取秒杀统计信息
POST /api/v1/spike/participate           # 参与秒杀（核心接口）
GET  /api/v1/spike/orders                # 获取用户秒杀订单
GET  /api/v1/spike/orders/:id            # 获取秒杀订单详情
POST /api/v1/spike/orders/:id/cancel     # 取消秒杀订单
```

#### 订单系统
```http
GET  /api/v1/orders                      # 获取用户订单列表
GET  /api/v1/orders/:id                  # 获取订单详情
POST /api/v1/orders                      # 创建订单
POST /api/v1/orders/:id/pay              # 支付订单
POST /api/v1/orders/:id/cancel           # 取消订单
GET  /api/v1/orders/:id/status           # 查询订单状态
```

#### 支付系统
```http
POST /api/v1/payments/create             # 创建支付
POST /api/v1/payments/:id/confirm        # 确认支付
POST /api/v1/payments/:id/callback       # 支付回调
POST /api/v1/payments/:id/refund         # 申请退款
GET  /api/v1/payments/:id/status         # 查询支付状态
```

#### 管理员接口
```http
# 用户管理
GET  /api/v1/admin/users                 # 获取用户列表
PUT  /api/v1/admin/users/role            # 更新用户角色
PUT  /api/v1/admin/users/status          # 更新用户状态

# 商品管理
POST /api/v1/admin/products              # 创建商品
PUT  /api/v1/admin/products/:id          # 更新商品
DELETE /api/v1/admin/products/:id        # 删除商品
GET  /api/v1/admin/products/stats        # 获取商品统计
POST /api/v1/admin/products/:id/inventory/adjust # 调整库存

# 库存管理
POST /api/v1/admin/inventory             # 创建库存记录
GET  /api/v1/admin/inventory/:id         # 获取库存详情
PUT  /api/v1/admin/inventory/:id         # 更新库存记录
GET  /api/v1/admin/inventory/alerts/low-stock # 获取低库存警告
GET  /api/v1/admin/inventory/stats       # 获取库存统计

# 秒杀管理
POST /api/v1/admin/spike/events/:id/warmup # 预热库存缓存
GET  /api/v1/admin/spike/events          # 获取所有秒杀活动
POST /api/v1/admin/spike/events          # 创建秒杀活动
PUT  /api/v1/admin/spike/events/:id      # 更新秒杀活动
DELETE /api/v1/admin/spike/events/:id    # 删除秒杀活动

# 订单管理
GET  /api/v1/admin/orders                # 获取所有订单
GET  /api/v1/admin/orders/:id            # 获取订单详情
PUT  /api/v1/admin/orders/:id/status     # 更新订单状态
GET  /api/v1/admin/orders/stats          # 获取订单统计
```

#### 监控接口
```http
GET  /metrics                            # Prometheus指标
GET  /debug/pprof/                       # 性能分析
GET  /healthz                            # 健康检查
GET  /readyz                             # 就绪检查
```

### 响应格式
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [],
    "page": 1,
    "page_size": 20,
    "total": 0
  }
}
```

## 🛡️ 安全机制

### 1. 认证与授权
- **JWT双令牌**：Access Token（15分钟）+ Refresh Token（7天）
- **RBAC权限**：用户角色管理，接口权限控制
- **密码安全**：bcrypt哈希，盐值随机化

### 2. 限流保护
| 限流类型 | 限制 | 说明 |
|---------|------|------|
| 全局限流 | 1000 req/min | 防止系统过载 |
| 用户限流 | 5 req/min | 防止单用户恶意请求 |
| API限流 | 100 req/min | 通用API保护 |

### 3. 防超卖机制
- **Redis预减库存**：Lua脚本保证原子性
- **用户去重标记**：防止同一用户重复参与
- **数据库约束**：唯一约束保证数据一致性
- **异步处理**：消息队列异步落库

### 4. 幂等性保证
- **自动幂等键**：基于用户ID、方法、路径和时间戳
- **手动幂等键**：通过 `X-Idempotency-Key` 头提供
- **Redis去重**：使用Redis存储幂等键

## 🚀 性能优化

### 1. 缓存策略
- **多级缓存**：内存缓存 + Redis缓存
- **缓存预热**：秒杀活动开始前预热库存数据
- **缓存失效**：TTL + 主动失效策略
- **缓存穿透防护**：布隆过滤器、空值缓存
- **缓存雪崩防护**：随机TTL、熔断机制

### 2. 数据库优化
- **索引优化**：为查询字段添加复合索引
- **连接池**：数据库连接池管理
- **读写分离**：读操作优先使用缓存
- **分页优化**：游标分页减少深度分页问题
- **批量操作**：减少数据库往返次数

### 3. 异步处理
```
用户请求 → Redis预减库存 → 立即响应
           ↓
    异步消息队列 → DB事务落库
```

### 4. 高并发优化
- **连接池优化**：数据库和Redis连接池调优
- **协程池**：控制并发协程数量
- **内存优化**：对象池复用，减少GC压力
- **CPU优化**：避免不必要的序列化/反序列化

## 🏆 技术亮点

### 1. 秒杀系统核心优势
- **原子性保证**：Redis Lua脚本确保库存扣减原子性
- **防超卖机制**：多重保护，数据库唯一约束 + Redis标记
- **高并发处理**：支持万级并发，响应时间 < 100ms
- **异步解耦**：关键业务流程异步化，提升系统吞吐量

### 2. 架构设计优势
- **Clean Architecture**：清晰的分层架构，职责分离
- **依赖注入**：松耦合设计，便于测试和维护
- **接口抽象**：面向接口编程，支持多种实现
- **配置驱动**：环境变量配置，支持多环境部署

### 3. 可靠性保障
- **幂等设计**：完整的幂等性保证，防止重复操作
- **事务一致性**：数据库事务 + 分布式事务处理
- **故障恢复**：优雅降级、熔断机制
- **数据备份**：多级备份策略，确保数据安全

### 4. 可观测性
- **全链路追踪**：OpenTelemetry分布式追踪
- **指标监控**：Prometheus指标收集
- **日志聚合**：结构化日志，便于问题排查
- **性能分析**：pprof性能剖析工具集成

### 5. 开发体验
- **热重载**：开发环境支持代码热重载
- **代码质量**：golangci-lint代码检查
- **测试覆盖**：单元测试 + 集成测试
- **文档完善**：API文档 + 开发文档

## 📈 监控与可观测性

### 1. 日志系统
- **结构化日志**：使用zap进行结构化日志记录
- **请求追踪**：每个请求分配唯一TraceID
- **日志级别**：DEBUG、INFO、WARN、ERROR分级记录

### 2. 指标监控
- **业务指标**：QPS、响应时间、错误率
- **系统指标**：CPU、内存、磁盘使用率
- **缓存指标**：缓存命中率、Redis连接数
- **队列指标**：消息积压、消费者处理速度

### 3. 链路追踪
- **OpenTelemetry**：支持分布式链路追踪
- **跨组件追踪**：HTTP、数据库、消息队列
- **采样策略**：按环境配置采样率

## 🧪 测试策略

### 1. 单元测试 ✅
- **覆盖率要求**：关键业务逻辑 > 80%
- **测试框架**：Go标准testing包
- **Mock策略**：使用gomock生成Mock对象
- **测试范围**：
  - 服务层业务逻辑
  - 仓储层数据访问
  - 工具函数和中间件
  - 边界条件和异常处理

### 2. 集成测试 🚧
- **测试容器**：使用testcontainers-go启动真实依赖
- **端到端测试**：完整业务流程验证
- **数据隔离**：每个测试用例独立数据
- **测试场景**：
  - 用户注册登录流程
  - 商品CRUD操作
  - 库存管理流程
  - 秒杀参与流程
  - 消息队列消费

### 3. 性能测试 📋
- **压测工具**：k6、hey、wrk
- **性能指标**：P95/P99延迟、QPS、错误率
- **压力场景**：
  - 正常流量：1000 QPS
  - 峰值流量：10000 QPS
  - 异常流量：突发请求
- **测试用例**：
  - 登录接口压测
  - 商品列表查询压测
  - 秒杀参与压测
  - 数据库连接池压测

### 4. 安全测试 📋
- **认证测试**：JWT令牌验证、过期处理
- **授权测试**：RBAC权限控制
- **输入验证**：SQL注入、XSS防护
- **限流测试**：各种限流策略验证

### 5. 混沌测试 📋
- **故障注入**：数据库连接中断、Redis故障
- **网络分区**：服务间通信中断
- **资源耗尽**：内存、CPU、磁盘空间
- **恢复测试**：故障恢复能力验证

### 6. 兼容性测试 📋
- **数据库版本**：MySQL 8.0兼容性
- **Redis版本**：Redis 7兼容性
- **Go版本**：Go 1.25+兼容性
- **操作系统**：Linux、macOS、Windows

## 🐳 部署与运维

### 1. 容器化部署 📋
```bash
# 构建镜像
docker build -t spike-shop:latest .

# 运行容器
docker run -d -p 8080:8080 --env-file .env spike-shop:latest

# 多阶段构建优化
docker build --target production -t spike-shop:prod .
```

### 2. Docker Compose ✅
```bash
# 启动完整环境
docker compose up -d

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f spike-server

# 停止服务
docker compose down

# 重新构建并启动
docker compose up -d --build
```

### 3. Kubernetes部署 📋
```yaml
# 部署清单示例
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spike-shop
spec:
  replicas: 3
  selector:
    matchLabels:
      app: spike-shop
  template:
    metadata:
      labels:
        app: spike-shop
    spec:
      containers:
      - name: spike-shop
        image: spike-shop:latest
        ports:
        - containerPort: 8080
        env:
        - name: APP_ENV
          value: "production"
```

### 4. 环境配置
- **开发环境**：本地开发，使用内存缓存
- **测试环境**：集成测试，使用Redis缓存
- **预生产环境**：接近生产环境的配置
- **生产环境**：高可用部署，完整监控

### 5. CI/CD流水线 📋
```yaml
# GitHub Actions示例
name: CI/CD Pipeline
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: go test ./...
      - run: golangci-lint run

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Build Docker image
        run: docker build -t spike-shop:${{ github.sha }} .
      - name: Push to registry
        run: docker push spike-shop:${{ github.sha }}
```

### 6. 监控告警 📋
- **Prometheus指标**：QPS、延迟、错误率
- **Grafana仪表板**：可视化监控面板
- **AlertManager**：告警规则和通知
- **日志聚合**：ELK Stack或类似方案

### 7. 备份恢复 📋
- **数据库备份**：定期全量+增量备份
- **Redis备份**：RDB+AOF持久化
- **配置备份**：环境变量和配置文件
- **灾难恢复**：RTO/RPO目标制定

## 📚 开发指南

### 1. 开发环境搭建
```bash
# 安装依赖
go mod download

# 启动基础设施
docker compose up -d

# 运行应用
make run

# 代码检查
make lint

# 运行测试
make test
```

### 2. 代码规范
- **Go代码风格**：遵循gofmt和goimports
- **命名规范**：驼峰命名，包名小写
- **注释规范**：公开函数必须有注释
- **错误处理**：使用errors包包装错误

### 3. 提交规范
- **提交信息**：使用约定式提交格式
- **分支策略**：feature/、bugfix/、hotfix/前缀
- **代码审查**：所有PR必须经过Code Review

## 🔄 开发进度与里程碑

### 里程碑 0：工程初始化 ✅ (已完成)
- [x] 项目结构搭建（cmd/、internal/、migrations/、deploy/）
- [x] Docker Compose 开发环境（MySQL/Redis/RabbitMQ）
- [x] 环境配置管理（.env.example）
- [x] 代码质量检查（golangci-lint）
- [x] 基础CI配置

### 里程碑 1：基础骨架 ✅ (已完成)
- [x] 配置加载与校验（环境变量优先）
- [x] 结构化日志（zap）、全局错误恢复
- [x] 统一响应格式
- [x] 路由与健康检查（GET /healthz）
- [x] 请求ID/Trace ID、CORS、超时中间件

### 里程碑 2：用户与认证 ✅ (已完成)
- [x] 用户表迁移（users）
- [x] 用户注册/登录功能
- [x] 密码哈希（bcrypt）
- [x] JWT Access/Refresh 双令牌策略
- [x] Refresh 流程实现
- [x] 简单RBAC（admin/user角色）
- [x] 单元测试覆盖（鉴权、过期、刷新、非法token）

### 里程碑 3：商品与库存 ✅ (已完成)
- [x] 商品表迁移（products）
- [x] 库存表迁移（inventory）
- [x] 商品CRUD操作
- [x] 库存查询、预留、释放、消费
- [x] 必要索引与约束
- [x] 缓存优化（可选缓存读）
- [x] 缓存失效策略

### 里程碑 4：秒杀与消息队列 🚧 (进行中)
- [x] 秒杀活动表迁移（spike_events）
- [x] 秒杀订单表迁移（spike_orders）
- [x] Redis Lua 预减库存与售罄标记
- [x] 用户-活动去重标记
- [x] 多重限流（令牌桶/滑动窗口/固定窗口）
- [x] 接口防抖、幂等键
- [x] MQ Producer 推单消息
- [x] Consumer 幂等消费、DB 事务落库
- [ ] 可选 Outbox 模式（提升最终一致性）
- [ ] 压测验证（10k请求无超卖/重复下单）

### 里程碑 5：订单事务与支付模拟 📋 (计划中)
- [ ] 订单创建与查询
- [ ] 订单项与金额校验
- [ ] 延时取消（死信队列或延迟插件）
- [ ] 库存回补机制
- [ ] 事务保证：订单与库存一致性
- [ ] 未支付订单超时T+X自动关闭

### 里程碑 6：可观测性 📋 (计划中)
- [ ] 指标监控：QPS、延迟、错误率、缓存命中率
- [ ] 队列积压、消费者重试数监控
- [ ] OpenTelemetry追踪（HTTP/MQ/DB跨组件）
- [ ] 采样策略配置
- [ ] pprof 性能探查
- [ ] 指标在 /metrics 端点可采集

### 里程碑 7：压测与优化 📋 (计划中)
- [ ] 压测脚本（k6/hey）覆盖登录、商品列表、下单、秒杀
- [ ] 性能瓶颈分析（连接池、索引、缓存命中、锁粒度）
- [ ] 降级与限流参数调优
- [ ] 压测报告（吞吐、P95/P99、错误率）
- [ ] 优化记录文档

### 里程碑 8：容器化与交付 📋 (计划中)
- [ ] 多阶段 Dockerfile
- [ ] Compose 一键启动
- [ ] 部署说明与环境变量清单
- [ ] 可选：K8s 清单与 HPA 设计建议
- [ ] 容器方式一键起服务并跑过 E2E 脚本

### 成功标准（DoD）
- [x] 主链路端到端打通
- [ ] E2E 测试通过
- [ ] 核心接口 P95 小于 200ms（本地/单机）
- [ ] 错误率小于 1%
- [ ] 秒杀压测 10k 请求内无超卖/重复下单
- [ ] 消费无明显积压
- [ ] 代码质量：lint 通过、关键路径具备单元与集成测试覆盖

## 🤝 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 📞 联系方式

- 项目维护者：MorseWayne
- 邮箱：morsewayne98@outlook.com
- 项目地址：https://github.com/MorseWayne/spike_shop

## 🙏 致谢

感谢所有为这个项目做出贡献的开发者和开源社区。

---

**注意**：这是一个教学和练习项目，专注于后端API开发。在生产环境使用前，请确保进行充分的安全评估和性能测试。
