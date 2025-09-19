// Package database 提供数据库连接与迁移功能。
package database

import (
	"database/sql"
	"fmt"

	// 使用下划线导入是Go语言的特殊语法，表示只执行包的初始化函数但不使用包中的标识符
	// MySQL驱动需要在程序启动时注册自己，而我们不需要直接调用它的函数
	// 后续通过sql.Open("mysql", dsn)时，database/sql包会自动查找已注册的mysql驱动
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/MorseWayne/spike_shop/internal/config"
)

// DB 封装数据库连接
type DB struct {
	*sql.DB
	logger *zap.Logger
	dsn    string
}

// New 创建数据库连接
func New(cfg *config.Config, logger *zap.Logger) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
	)

	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	logger.Info("database connected",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port),
		zap.String("database", cfg.Database.DBName),
	)

	return &DB{DB: sqlDB, logger: logger, dsn: dsn}, nil
}

// RunMigrations 使用 go-migrate 执行数据库迁移
// 数据库迁移是一种管理数据库结构变更的版本控制机制，通过SQL文件定义数据库模式变更
// 主要作用是：
// 1. 确保所有环境（开发、测试、生产）使用相同的数据库结构
// 2. 跟踪数据库结构的变更历史
// 3. 支持向前（应用新变更）和向后（回滚）操作
// 4. 多人协作开发时避免数据库结构不一致
//
// go-migrate 相比自定义实现的优势：
// 1. 成熟稳定的迁移管理，广泛使用
// 2. 支持多种数据库和迁移源
// 3. 提供完整的up/down迁移支持
// 4. 自动处理迁移版本冲突和错误恢复
// 5. 支持脏迁移检测和修复
func (db *DB) RunMigrations(migrationsDir string) error {
	// 迁移使用独立连接，避免错误时影响主连接
	migrateSQLDB, err := sql.Open("mysql", db.dsn)
	if err != nil {
		return fmt.Errorf("open database for migration: %w", err)
	}
	defer migrateSQLDB.Close()

	// 创建 MySQL 数据库驱动实例（基于独立连接）
	driver, err := mysql.WithInstance(migrateSQLDB, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("create mysql driver: %w", err)
	}

	// 创建 migrate 实例
	// 使用 file:// 协议指定迁移文件目录
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsDir),
		"mysql",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	// 获取当前迁移版本
	currentVersion, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("get current version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d, please check and fix manually", currentVersion)
	}

	db.logger.Info("current migration version", zap.Uint("version", currentVersion))

	// 执行所有待执行的迁移（up）
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			db.logger.Info("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("run migrations: %w", err)
	}

	// 获取新版本
	newVersion, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("get new version: %w", err)
	}

	db.logger.Info("migrations completed successfully",
		zap.Uint("from_version", currentVersion),
		zap.Uint("to_version", newVersion),
	)

	return nil
}

// MigrateDown 执行向下迁移（回滚）
// 注意：这个方法应该谨慎使用，特别是在生产环境中
func (db *DB) MigrateDown(migrationsDir string, steps int) error {
	migrateSQLDB, err := sql.Open("mysql", db.dsn)
	if err != nil {
		return fmt.Errorf("open database for migration: %w", err)
	}
	defer migrateSQLDB.Close()

	// 创建 MySQL 数据库驱动实例（基于独立连接）
	driver, err := mysql.WithInstance(migrateSQLDB, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("create mysql driver: %w", err)
	}

	// 创建 migrate 实例
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsDir),
		"mysql",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	// 获取当前版本
	currentVersion, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d", currentVersion)
	}

	db.logger.Info("starting migration rollback",
		zap.Uint("current_version", currentVersion),
		zap.Int("steps", steps),
	)

	// 执行向下迁移
	if err := m.Steps(-steps); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}

	// 获取新版本
	newVersion, _, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("get new version: %w", err)
	}

	db.logger.Info("migration rollback completed",
		zap.Uint("from_version", currentVersion),
		zap.Uint("to_version", newVersion),
	)

	return nil
}

// MigrateToVersion 迁移到指定版本
func (db *DB) MigrateToVersion(migrationsDir string, version uint) error {
	migrateSQLDB, err := sql.Open("mysql", db.dsn)
	if err != nil {
		return fmt.Errorf("open database for migration: %w", err)
	}
	defer migrateSQLDB.Close()

	// 创建 MySQL 数据库驱动实例（基于独立连接）
	driver, err := mysql.WithInstance(migrateSQLDB, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("create mysql driver: %w", err)
	}

	// 创建 migrate 实例
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsDir),
		"mysql",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	// 获取当前版本
	currentVersion, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("get current version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d", currentVersion)
	}

	db.logger.Info("migrating to specific version",
		zap.Uint("current_version", currentVersion),
		zap.Uint("target_version", version),
	)

	// 迁移到指定版本
	if err := m.Migrate(version); err != nil {
		if err == migrate.ErrNoChange {
			db.logger.Info("already at target version", zap.Uint("version", version))
			return nil
		}
		return fmt.Errorf("migrate to version %d: %w", version, err)
	}

	db.logger.Info("migration to version completed",
		zap.Uint("from_version", currentVersion),
		zap.Uint("to_version", version),
	)

	return nil
}

// ForceMigrationVersion 强制设置迁移版本状态
// 注意：这个方法应该非常谨慎使用，只在修复脏状态时使用
func (db *DB) ForceMigrationVersion(migrationsDir string, version uint) error {
	migrateSQLDB, err := sql.Open("mysql", db.dsn)
	if err != nil {
		return fmt.Errorf("open database for migration: %w", err)
	}
	defer migrateSQLDB.Close()

	// 创建 MySQL 数据库驱动实例（基于独立连接）
	driver, err := mysql.WithInstance(migrateSQLDB, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("create mysql driver: %w", err)
	}

	// 创建 migrate 实例
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsDir),
		"mysql",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	db.logger.Info("forcing migration version",
		zap.Uint("version", version),
	)

	// 强制设置版本（这会清除脏状态）
	if err := m.Force(int(version)); err != nil {
		return fmt.Errorf("force migration version: %w", err)
	}

	db.logger.Info("migration version forced successfully",
		zap.Uint("version", version),
	)

	return nil
}
