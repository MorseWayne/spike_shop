# 里程碑0：工程初始化实现文档

## 概述

本文档详细记录了里程碑0的完整实现过程，主要涉及Go项目的初始化、目录结构设计、开发环境搭建以及基础CI/CD配置。

**实施时间**：项目初始阶段  
**目标**：建立规范的Go工程骨架，确保开发环境一致性  
**技术栈**：Go 1.21+ + Docker Compose + golangci-lint

## 项目架构设计

### 目录结构
```
spike_shop/
├── cmd/                    # 应用程序入口点
│   └── spike-server/       # 主服务器应用
│       ├── main.go         # 程序入口
│       └── main_test.go    # 主程序测试
├── internal/               # 私有应用代码库
│   ├── config/             # 配置管理
│   ├── logger/             # 日志组件
│   ├── middleware/         # HTTP中间件
│   ├── resp/               # 统一响应格式
│   ├── database/           # 数据库连接与迁移
│   ├── domain/             # 领域模型
│   ├── repo/               # 数据仓储层
│   ├── service/            # 业务服务层
│   └── api/                # API处理器层
├── migrations/             # 数据库迁移文件
├── configs/                # 配置文件模板
├── deploy/                 # 部署相关文件
├── docs/                   # 项目文档
│   └── trace/              # 开发跟踪文档
├── .env.example            # 环境变量示例
├── docker-compose.yml      # 开发环境编排
├── .golangci.yml           # 代码质量检查配置
├── go.mod                  # Go模块定义
├── go.sum                  # 依赖锁定文件
└── README.md               # 项目说明
```

### 设计原则

#### 1. **Clean Architecture**
- **cmd/**：应用程序的入口点，包含主函数和应用初始化逻辑
- **internal/**：私有代码库，外部无法引用，确保封装性
- **分层架构**：API层 → 服务层 → 仓储层 → 数据库层

#### 2. **依赖方向**
```
外层依赖内层，内层不依赖外层：
API层 → 服务层 → 仓储层 → 数据库
     ↘     ↘     ↘
      领域层（Domain）
```

#### 3. **配置管理**
- 环境变量优先原则
- `.env`文件作为本地开发便利性
- 配置验证和默认值机制

## 实施步骤

### 第一步：Go模块初始化

#### 1.1 创建Go模块
```bash
# 初始化Go模块
go mod init github.com/MorseWayne/spike_shop

# 设置Go版本要求
go 1.21
```

**关键决策**：
- 使用语义化版本管理
- 明确Go版本依赖（1.21+支持泛型）
- 采用标准的GitHub路径命名

#### 1.2 依赖管理策略
```go
// go.mod 核心依赖
require (
    github.com/joho/godotenv v1.4.0      // 环境变量加载
    go.uber.org/zap v1.24.0              // 结构化日志
    github.com/google/uuid v1.3.0         // UUID生成
    // 后续按需添加
)
```

### 第二步：目录结构创建

#### 2.1 标准Go项目布局
遵循[Standard Go Project Layout](https://github.com/golang-standards/project-layout)：

**cmd目录**：
- 每个应用一个子目录
- 包含main.go和相关测试
- 保持main函数简洁，业务逻辑放在internal

**internal目录**：
- 确保代码封装，外部无法引用
- 按功能领域分包
- 避免循环依赖

**其他目录**：
- `migrations/`：数据库迁移SQL文件
- `configs/`：配置模板和示例
- `deploy/`：容器化和部署文件
- `docs/`：项目文档

#### 2.2 包依赖关系
```
cmd/spike-server
├── internal/config      (配置加载)
├── internal/logger      (日志初始化)
├── internal/database    (数据库连接)
├── internal/middleware  (HTTP中间件)
├── internal/api         (路由处理)
└── internal/service     (业务逻辑)
    └── internal/repo    (数据访问)
        └── internal/domain (领域模型)
```

### 第三步：开发环境配置

#### 3.1 Docker Compose环境
**文件**：`docker-compose.yml`

```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: spike
      MYSQL_USER: spike
      MYSQL_PASSWORD: spike
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    command: >
      --default-authentication-plugin=mysql_native_password
      --character-set-server=utf8mb4
      --collation-server=utf8mb4_unicode_ci

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  rabbitmq:
    image: rabbitmq:3-management-alpine
    environment:
      RABBITMQ_DEFAULT_USER: spike
      RABBITMQ_DEFAULT_PASS: spike
    ports:
      - "5672:5672"
      - "15672:15672"
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq

volumes:
  mysql_data:
  redis_data:
  rabbitmq_data:
```

**设计特点**：
- **MySQL 8.0**：主数据库，支持JSON字段和现代SQL特性
- **Redis 7**：缓存和会话存储
- **RabbitMQ**：消息队列，支持延时队列
- **数据持久化**：使用Docker volumes确保数据不丢失
- **端口映射**：方便本地开发调试

#### 3.2 环境变量配置
**文件**：`.env.example`

```bash
# 应用配置
APP_NAME=spike-server
APP_ENV=dev
APP_PORT=8080
APP_VERSION=0.1.0
REQUEST_TIMEOUT_MS=5000
SHUTDOWN_TIMEOUT_MS=5000

# 日志配置
LOG_LEVEL=info
LOG_ENCODING=console

# 数据库配置
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=spike
MYSQL_PASSWORD=spike
MYSQL_DB=spike

# JWT配置
JWT_SECRET=change_me_in_production
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h

# CORS配置
CORS_ALLOWED_ORIGINS=*
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Authorization,Content-Type

# 迁移配置
MIGRATIONS_DIR=migrations
```

**配置原则**：
- **开发友好**：提供合理的默认值
- **安全意识**：生产环境必须修改的配置有明确标识
- **文档化**：每个配置项都有清晰含义
- **类型安全**：支持不同数据类型的配置项

### 第四步：代码质量保障

#### 4.1 Linting配置
**文件**：`.golangci.yml`

```yaml
run:
  timeout: 5m
  go: "1.21"

linters-settings:
  gofmt:
    simplify: true
  goimports:
    local-prefixes: github.com/MorseWayne/spike_shop
  govet:
    enable-all: true
  staticcheck:
    checks: ["all"]

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - unconvert
    - goconst
    - gocyclo
    - prealloc

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gocyclo
        - errcheck
        - dupl
        - gosec
```

**质量标准**：
- **代码格式**：强制使用gofmt和goimports
- **静态分析**：检查常见错误和性能问题
- **复杂度控制**：限制函数圈复杂度
- **测试豁免**：测试文件适当放宽规则

#### 4.2 基础CI配置概念
虽然本里程碑不包含CI实现，但确立了基本流程：

```yaml
# 概念性CI流程
name: CI
on: [push, pull_request]
jobs:
  test:
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: go mod download
      - run: golangci-lint run
      - run: go test ./... -v
      - run: go build ./cmd/spike-server
```

### 第五步：基础工具和脚本

#### 5.1 开发便利性工具
**概念性Makefile**：
```makefile
.PHONY: help build test lint clean docker-up docker-down

help:           ## 显示帮助信息
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build:          ## 构建应用
	go build -o bin/spike-server ./cmd/spike-server

test:           ## 运行测试
	go test ./... -v

lint:           ## 代码检查
	golangci-lint run

clean:          ## 清理构建文件
	rm -rf bin/

docker-up:      ## 启动开发环境
	docker compose up -d

docker-down:    ## 停止开发环境
	docker compose down
```

#### 5.2 Git配置
**文件**：`.gitignore`

```gitignore
# 二进制文件
bin/
*.exe
*.exe~
*.dll
*.so
*.dylib

# 测试覆盖率文件
*.out
coverage.html

# 依赖目录
vendor/

# 环境变量文件
.env
.env.local
.env.*.local

# IDE文件
.vscode/
.idea/
*.swp
*.swo
*~

# 系统文件
.DS_Store
Thumbs.db

# 日志文件
*.log

# 临时文件
tmp/
temp/
```

## 验收标准完成情况

### ✅ 基础验收标准

#### 1. **项目初始化成功**
```bash
# 测试命令
go mod tidy
go build ./cmd/spike-server
```
**结果**：编译成功，生成可执行文件

#### 2. **开发环境一键启动**
```bash
# 测试命令
docker compose up -d
```
**结果**：
- MySQL：端口3306可访问
- Redis：端口6379可访问
- RabbitMQ：Web管理界面15672可访问

#### 3. **代码质量检查**
```bash
# 测试命令
golangci-lint run
```
**结果**：通过所有linter检查，无警告错误

### ✅ 扩展验收标准

#### 4. **目录结构规范**
- ✅ 遵循Standard Go Project Layout
- ✅ 包依赖关系清晰，无循环依赖
- ✅ internal包确保代码封装

#### 5. **环境配置完整**
- ✅ 提供完整的`.env.example`
- ✅ 支持多环境配置（dev/test/prod）
- ✅ 配置验证机制

#### 6. **开发工具就绪**
- ✅ Docker Compose一键环境
- ✅ golangci-lint质量检查
- ✅ Git忽略文件配置

## 技术决策记录

### 1. **项目结构决策**

**决策**：采用Standard Go Project Layout  
**理由**：
- 行业标准，团队成员易于理解
- 清晰的代码组织和职责分离
- 便于CI/CD和工具集成

**替代方案**：平铺结构  
**拒绝原因**：不利于大型项目的代码组织

### 2. **依赖管理决策**

**决策**：最小化初始依赖，按需添加  
**理由**：
- 避免过度工程化
- 减少安全漏洞攻击面
- 便于理解和维护

**初期核心依赖**：
- `godotenv`：环境变量管理
- `zap`：高性能结构化日志
- `uuid`：唯一标识符生成

### 3. **开发环境决策**

**决策**：Docker Compose本地环境  
**理由**：
- 环境一致性，避免"在我机器上能跑"
- 简化外部依赖安装（MySQL/Redis/RabbitMQ）
- 易于版本控制和团队协作

**替代方案**：本地安装  
**拒绝原因**：环境差异和安装复杂度

### 4. **代码质量决策**

**决策**：集成golangci-lint  
**理由**：
- 统一代码风格和质量标准
- 自动检查常见错误和性能问题
- CI/CD集成友好

**配置原则**：
- 启用核心linters
- 测试文件适当放宽规则
- 项目特定配置（本地包前缀）

## 风险与应对

### 1. **环境兼容性风险**

**风险描述**：Windows/macOS/Linux环境差异  
**应对措施**：
- 使用Docker统一运行时环境
- 避免平台特定的代码和路径
- 文档明确环境要求

### 2. **依赖管理风险**

**风险描述**：依赖版本冲突和安全漏洞  
**应对措施**：
- 锁定依赖版本（go.sum）
- 定期更新依赖并测试
- 使用官方和知名的开源库

### 3. **开发效率风险**

**风险描述**：过度的工程化影响开发速度  
**应对措施**：
- 渐进式完善，避免一次性过度设计
- 关注核心功能实现
- 工具配置简单易用

## 后续里程碑支撑

### 为里程碑1提供基础
- ✅ 配置管理框架
- ✅ 日志组件基础
- ✅ HTTP服务框架
- ✅ 测试环境准备

### 为里程碑2提供基础
- ✅ 数据库连接框架
- ✅ 迁移管理机制
- ✅ API结构设计

### 为后续里程碑提供基础
- ✅ 缓存连接（Redis）
- ✅ 消息队列（RabbitMQ）
- ✅ 容器化准备
- ✅ 代码质量保障

## 总结

里程碑0成功建立了一个规范、可扩展的Go项目基础设施：

✅ **工程规范**：遵循Go社区最佳实践  
✅ **环境一致**：Docker Compose统一开发环境  
✅ **质量保障**：集成代码检查和测试框架  
✅ **扩展性**：为后续功能开发提供坚实基础  

这个基础设施为整个spike_shop项目的后续开发提供了坚实的工程支撑，确保了代码质量、开发效率和团队协作的一致性。
