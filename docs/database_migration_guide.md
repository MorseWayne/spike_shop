# 数据库迁移优化与故障排除指南

## 目录

1. [概述](#概述)
2. [优化内容](#优化内容)
3. [新增功能](#新增功能)
4. [迁移文件规范](#迁移文件规范)
5. [使用方式](#使用方式)
6. [故障排除](#故障排除)
7. [最佳实践](#最佳实践)
8. [性能考虑](#性能考虑)
9. [总结](#总结)

## 概述

本指南记录了将自定义数据库迁移实现优化为使用成熟的 [go-migrate](https://github.com/golang-migrate/migrate) 库的完整过程，包括优化内容、遇到的问题以及解决方案。

### 优化目标

- 提升迁移系统的稳定性、功能性和可维护性
- 支持完整的双向迁移（up/down）
- 提供健壮的错误处理和恢复机制
- 实现企业级的版本管理能力

## 优化内容

### 1. 依赖更新

```go
// 新增依赖
github.com/golang-migrate/migrate/v4 v4.19.0
github.com/golang-migrate/migrate/v4/database/mysql
github.com/golang-migrate/migrate/v4/source/file
```

### 2. 迁移文件格式变更

**旧格式（自定义实现）:**
```
migrations/
├── 20250918_001_create_users_table.sql
├── 20250918_002_create_products_table.sql
└── 20250918_003_create_inventory_table.sql
```

**新格式（go-migrate 标准）:**
```
migrations/
├── 000001_create_users_table.up.sql
├── 000001_create_users_table.down.sql
├── 000002_insert_admin_user.up.sql
├── 000002_insert_admin_user.down.sql
├── 000003_create_products_table.up.sql
├── 000003_create_products_table.down.sql
├── 000004_create_inventory_table.up.sql
└── 000004_create_inventory_table.down.sql
```

### 3. 核心功能重构

#### 原始实现的问题
- 手动管理迁移状态表（~130行代码）
- 简单的 SQL 解析容易出错
- 缺乏回滚支持
- 无版本冲突检测
- 错误处理不够健壮

#### go-migrate 的优势
- **成熟稳定**: 被广泛使用的开源库
- **多数据库支持**: MySQL、PostgreSQL、SQLite 等
- **完整的up/down支持**: 支持迁移和回滚
- **版本管理**: 自动处理版本冲突和脏状态
- **错误恢复**: 处理失败的迁移
- **原子性**: 确保迁移的事务性

## 新增功能

### 1. 向上迁移 (Up)
```go
func (db *DB) RunMigrations(migrationsDir string) error
```
- 执行所有待执行的迁移
- 自动跟踪迁移版本
- 支持增量迁移

### 2. 向下迁移 (Down/Rollback)
```go
func (db *DB) MigrateDown(migrationsDir string, steps int) error
```
- 回滚指定步数的迁移
- 谨慎使用，特别是生产环境

### 3. 版本迁移
```go
func (db *DB) MigrateToVersion(migrationsDir string, version uint) error
```
- 迁移到指定版本
- 支持向前或向后迁移

### 4. 强制清理功能 🆕
```go
func (db *DB) ForceMigrationVersion(migrationsDir string, version uint) error
```
- 强制设置迁移版本状态
- 用于清理脏状态
- **注意**: 非常谨慎使用，只在修复时使用

## 迁移文件规范

### 命名规范
- 版本号: 6位数字 `000001`, `000002`, `000003`
- 描述: 下划线分隔的描述性名称
- 格式: `{version}_{description}.{direction}.sql`

### Up 文件示例
```sql
-- 000001_create_users_table.up.sql
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  `username` varchar(64) NOT NULL COMMENT '用户名，唯一',
  `email` varchar(255) NOT NULL COMMENT '邮箱，唯一',
  -- ... 其他字段
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

### Down 文件示例
```sql
-- 000001_create_users_table.down.sql
DROP TABLE IF EXISTS `users`;
```

### ⚠️ 重要约束
- **一个文件一条语句**: 避免在单个文件中使用多条SQL语句
- **DDL与DML分离**: 表结构变更和数据变更应分别放在不同迁移中

## 使用方式

### 1. 应用启动时自动迁移
```go
// 在 main.go 中
if err := db.RunMigrations(migrationsDir); err != nil {
    lg.Sugar().Fatalw("failed to run database migrations", "err", err)
}
```

### 2. 命令行工具管理

```bash
# 执行所有待执行迁移
./migrate -action=up

# 回滚1个迁移
./migrate -action=down -steps=1

# 迁移到指定版本
./migrate -action=version -target=2

# 强制设置版本（清理脏状态）
./migrate -action=force -target=0

# 查看帮助
./migrate
```

## 故障排除

### 脏状态问题 (Dirty State)

#### 问题描述
```
database is in dirty state at version X, please check and fix manually
```

#### 问题原因
1. **脏状态产生**: 之前的迁移执行失败，导致数据库处于不一致状态
2. **SQL语法问题**: 单个迁移文件包含多条SQL语句，go-migrate 无法正确处理
3. **版本冲突**: 旧的迁移格式与新的 go-migrate 格式不兼容

#### 解决步骤

**步骤1: 分析问题**
```bash
# 检查迁移状态
mysql -u spike -pspike spike -e "SELECT * FROM schema_migrations;"
```

**步骤2: 强制清理脏状态**
```bash
./migrate -action=force -target=0
```

**步骤3: 重新执行迁移**
```bash
./migrate -action=up
```

**步骤4: 验证结果**
```bash
# 检查数据库表
mysql -u spike -pspike spike -e "SHOW TABLES;"

# 检查迁移状态
mysql -u spike -pspike spike -e "SELECT * FROM schema_migrations;"

# 测试应用启动
make run
```

### 常见SQL语法问题

#### 问题: 多语句错误
```sql
-- ❌ 错误格式
CREATE TABLE users (...);
INSERT INTO users (...);  -- 这会导致语法错误
```

#### 解决: 分离为多个迁移
```sql
-- ✅ 000001_create_users_table.up.sql
CREATE TABLE users (...);

-- ✅ 000002_insert_admin_user.up.sql  
INSERT INTO users (...);
```

### 版本冲突
```
migration version conflict
```

#### 解决方案
- 检查迁移文件版本号
- 确保团队协作时版本号不冲突
- 重新编号冲突的迁移文件

## 最佳实践

### 1. 迁移文件设计原则
- **一个文件一个逻辑单元**: 不要在单个迁移文件中混合DDL和DML
- **原子性**: 每个迁移应该是原子的，要么全成功要么全失败
- **可回滚**: 每个up迁移都应该有对应的down迁移
- **向前兼容**: 新迁移不应破坏现有功能

### 2. 版本管理
- 使用6位数字作为版本号: `000001`, `000002`, `000003`
- 描述性的迁移名称: `create_users_table`, `add_user_index`
- 按时间顺序递增版本号

### 3. 生产环境注意事项
- **备份数据**: 迁移前务必备份数据库
- **测试验证**: 在开发/测试环境充分验证
- **谨慎回滚**: 生产环境尽量避免down迁移
- **监控日志**: 密切关注迁移执行日志

### 4. 脏状态预防
- **测试环境验证**: 在生产环境前充分测试迁移
- **CI/CD集成**: 将迁移测试集成到CI/CD流程
- **监控日志**: 密切关注迁移执行日志

### 5. 故障处理流程
- **立即停止**: 发现脏状态立即停止应用
- **分析原因**: 查看日志了解失败原因
- **谨慎修复**: 使用force命令前确保理解后果
- **验证修复**: 修复后全面验证数据完整性

## 性能考虑

### 启动时间影响
- **首次运行**: 迁移可能增加应用启动时间
- **后续启动**: 无新迁移时影响很小
- **优化建议**: 可考虑异步迁移机制

### 资源占用
- **内存**: 大型迁移可能消耗较多内存
- **CPU**: DDL操作可能占用CPU资源
- **锁定**: DDL操作可能短暂锁定表

### 优化建议
- **批量操作**: 大量数据迁移考虑分批处理
- **索引策略**: 先删除索引，迁移后重建
- **维护窗口**: 大型迁移安排在维护窗口执行

## 成功验证

修复完成后，应用可以正常启动：
```
2025-09-18T18:17:53.456Z  info  current migration version  {"version": 4}
2025-09-18T18:17:53.457Z  info  no new migrations to apply
2025-09-18T18:17:53.457Z  info  server starting  {"addr": ":8080"}
```

数据库状态正常：
- ✅ 所有表结构正确创建
- ✅ 迁移版本状态正常（version=4, dirty=0）
- ✅ 管理员用户成功创建
- ✅ 应用服务正常启动

## 总结

### 主要成果

使用 go-migrate 替换自定义迁移实现带来了显著提升：

1. **可靠性提升**
   - 成熟的错误处理和状态管理
   - 自动脏状态检测和恢复机制
   - 事务性保证迁移完整性

2. **功能性增强**
   - 完整的up/down双向迁移支持
   - 灵活的版本控制和回滚能力
   - 强制清理功能处理异常情况

3. **可维护性改善**
   - 标准化的迁移格式和命名规范
   - 清晰的命令行工具接口
   - 详细的日志和错误信息

4. **可扩展性提升**
   - 支持多种数据库和迁移源
   - 易于集成到CI/CD流程
   - 适应团队协作开发需求

### 解决的关键问题

1. **脏状态清理**: 通过 `force` 命令安全清理脏状态
2. **SQL语法兼容**: 分离复杂SQL为多个简单迁移
3. **工具完善**: 提供完整的迁移管理命令行工具
4. **错误恢复**: 健壮的错误处理和恢复机制

现在项目具备了企业级的数据库迁移管理能力，支持向前迁移、回滚、版本控制和错误恢复，为后续开发提供了稳固的基础。
