// Package main 提供数据库迁移管理的命令行工具
// 基于 go-migrate 库，支持向上迁移、向下迁移和版本管理
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/MorseWayne/spike_shop/internal/config"
	"github.com/MorseWayne/spike_shop/internal/database"
	"github.com/MorseWayne/spike_shop/internal/logger"
)

func main() {
	var (
		action = flag.String("action", "up", "Migration action: up, down, version, force")
		steps  = flag.Int("steps", 1, "Number of steps for down migration")
		target = flag.Uint("target", 0, "Target version for version or force migration")
	)
	flag.Parse()

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 初始化日志
	lg, err := logger.New(cfg.App.Env, cfg.Log.Level, cfg.Log.Encoding, "migrate", cfg.App.Version)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}

	// 连接数据库
	db, err := database.New(cfg, lg)
	if err != nil {
		lg.Sugar().Fatalw("failed to connect to database", "error", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			lg.Sugar().Errorw("failed to close database", "error", err)
		}
	}()

	migrationsDir := cfg.Migrations.Dir

	switch *action {
	case "up":
		lg.Info("running up migrations...")
		if err := db.RunMigrations(migrationsDir); err != nil {
			lg.Sugar().Fatalw("failed to run up migrations", "error", err)
		}
		lg.Info("up migrations completed successfully")

	case "down":
		lg.Sugar().Infow("running down migrations", "steps", *steps)
		if err := db.MigrateDown(migrationsDir, *steps); err != nil {
			lg.Sugar().Fatalw("failed to run down migrations", "error", err)
		}
		lg.Info("down migrations completed successfully")

	case "version":
		if *target == 0 {
			lg.Fatal("target version must be specified for version migration")
		}
		lg.Sugar().Infow("migrating to version", "target", *target)
		if err := db.MigrateToVersion(migrationsDir, *target); err != nil {
			lg.Sugar().Fatalw("failed to migrate to version", "error", err)
		}
		lg.Info("version migration completed successfully")

	case "force":
		// 对于force操作，允许版本0（表示重置到无迁移状态）
		lg.Sugar().Warnw("forcing migration version - this will clear dirty state", "target", *target)
		if err := db.ForceMigrationVersion(migrationsDir, *target); err != nil {
			lg.Sugar().Fatalw("failed to force migration version", "error", err)
		}
		lg.Info("migration version forced successfully")

	default:
		fmt.Printf("Usage: %s -action=[up|down|version|force] [options]\n", os.Args[0])
		fmt.Println("Options:")
		fmt.Println("  -action string")
		fmt.Println("        Migration action: up, down, version, force (default \"up\")")
		fmt.Println("  -steps int")
		fmt.Println("        Number of steps for down migration (default 1)")
		fmt.Println("  -target uint")
		fmt.Println("        Target version for version or force migration (default 0)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # Run all pending migrations")
		fmt.Println("  ./migrate -action=up")
		fmt.Println()
		fmt.Println("  # Rollback 1 migration")
		fmt.Println("  ./migrate -action=down -steps=1")
		fmt.Println()
		fmt.Println("  # Migrate to specific version")
		fmt.Println("  ./migrate -action=version -target=2")
		fmt.Println()
		fmt.Println("  # Force migration version (clear dirty state)")
		fmt.Println("  ./migrate -action=force -target=0")
		os.Exit(1)
	}
}
