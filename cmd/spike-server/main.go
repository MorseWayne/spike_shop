package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/api"
	"github.com/MorseWayne/spike_shop/internal/cache"
	"github.com/MorseWayne/spike_shop/internal/config"
	"github.com/MorseWayne/spike_shop/internal/database"
	"github.com/MorseWayne/spike_shop/internal/logger"
	"github.com/MorseWayne/spike_shop/internal/repo"
	"github.com/MorseWayne/spike_shop/internal/router"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// initConfigAndLogger 初始化配置和日志器
func initConfigAndLogger() (*config.Config, *zap.Logger, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %v", err)
	}

	// init logger
	lg, err := logger.New(cfg.App.Env, cfg.Log.Level, cfg.Log.Encoding, cfg.App.Name, cfg.App.Version)
	if err != nil {
		return nil, nil, fmt.Errorf("init logger: %v", err)
	}

	return cfg, lg, nil
}

// initDatabase 初始化数据库连接并执行迁移
func initDatabase(cfg *config.Config, lg *zap.Logger) (*database.DB, error) {
	// 初始化数据库连接
	db, err := database.New(cfg, lg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	// 执行数据库迁移
	// 最佳实践：在应用启动时、HTTP服务器启动前执行数据库迁移
	// 这样可以确保在处理请求前，数据库结构已经完全准备好
	// 从配置中获取迁移目录路径
	migrationsDir := cfg.Migrations.Dir
	lg.Sugar().Infow("using migrations directory", "path", migrationsDir)

	if err := db.RunMigrations(migrationsDir); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %v", err)
	}

	return db, nil
}

// initCache 初始化缓存实例
func initCache(cfg *config.Config, lg *zap.Logger) cache.Cache {
	var cacheInstance cache.Cache
	if cfg.Cache.Enabled {
		switch cfg.Cache.Type {
		case "redis":
			redisAddr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)
			redisCache, err := cache.NewRedisCache(redisAddr, cfg.Redis.Password, cfg.Redis.DB)
			if err != nil {
				lg.Sugar().Warnw("failed to connect to Redis, falling back to memory cache", "error", err)
				cacheInstance = cache.NewMemoryCache()
				lg.Sugar().Infow("cache enabled", "type", "memory (fallback)", "ttl", cfg.Cache.TTL)
			} else {
				cacheInstance = redisCache
				lg.Sugar().Infow("cache enabled", "type", "redis", "addr", redisAddr, "ttl", cfg.Cache.TTL)
			}
		case "memory":
			cacheInstance = cache.NewMemoryCache()
			lg.Sugar().Infow("cache enabled", "type", "memory", "ttl", cfg.Cache.TTL)
		default:
			lg.Sugar().Warnw("unknown cache type, using memory cache", "type", cfg.Cache.Type)
			cacheInstance = cache.NewMemoryCache()
			lg.Sugar().Infow("cache enabled", "type", "memory (default)", "ttl", cfg.Cache.TTL)
		}
	} else {
		cacheInstance = cache.NewNullCache()
		lg.Sugar().Infow("cache disabled")
	}
	return cacheInstance
}

// initDependencies 初始化应用依赖（仓储、服务、处理器）
func initDependencies(cfg *config.Config, db *database.DB, cacheInstance cache.Cache, lg *zap.Logger) *router.Dependencies {
	// 初始化依赖注入链：仓储 -> 服务 -> API处理器
	userRepo := repo.NewUserRepository(db)
	userService := service.NewUserService(userRepo, lg)
	jwtService := service.NewJWTService(cfg, lg)
	userHandler := api.NewUserHandler(userService, jwtService, lg)

	// 商品和库存相关
	baseProductRepo := repo.NewProductRepository(db.DB)
	baseInventoryRepo := repo.NewInventoryRepository(db.DB)

	// 可选缓存装饰器
	var productRepo repo.ProductRepository
	var inventoryRepo repo.InventoryRepository

	if cfg.Cache.Enabled {
		productRepo = repo.NewCachedProductRepository(baseProductRepo, cacheInstance, cfg.Cache.TTL)
		inventoryRepo = repo.NewCachedInventoryRepository(baseInventoryRepo, cacheInstance, cfg.Cache.TTL)
	} else {
		productRepo = baseProductRepo
		inventoryRepo = baseInventoryRepo
	}

	productService := service.NewProductService(productRepo, inventoryRepo)
	inventoryService := service.NewInventoryService(inventoryRepo, productRepo)
	productHandler := api.NewProductHandler(productService, lg)
	inventoryHandler := api.NewInventoryHandler(inventoryService, lg)

	return &router.Dependencies{
		UserHandler:      userHandler,
		ProductHandler:   productHandler,
		InventoryHandler: inventoryHandler,
		JWTService:       jwtService,
	}
}

// startServer 启动服务器并处理优雅关闭
func startServer(cfg *config.Config, handler http.Handler, lg *zap.Logger) {
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	lg.Sugar().Infow("server starting", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	// 启动服务器（异步）
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- srv.ListenAndServe()
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	select {
	case err := <-serverErrCh:
		if err != nil && err != http.ErrServerClosed {
			lg.Sugar().Fatalw("server error", "err", err)
		}
	case <-quit:
		lg.Sugar().Infow("shutdown signal received")
	}

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		lg.Sugar().Errorw("server shutdown error", "err", err)
	}
	lg.Sugar().Infow("server exited")
}

// main 为应用入口，协调各个组件的初始化和启动
func main() {
	// 1) 加载配置和初始化日志
	cfg, lg, err := initConfigAndLogger()
	if err != nil {
		log.Fatalf("failed to initialize config and logger: %v", err)
	}

	// 2) 初始化数据库连接并执行迁移
	db, err := initDatabase(cfg, lg)
	if err != nil {
		lg.Sugar().Fatalw("failed to initialize database", "err", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			lg.Sugar().Errorw("failed to close database connection", "err", err)
		}
	}()

	// 3) 初始化缓存
	cacheInstance := initCache(cfg, lg)

	// 4) 初始化应用依赖（仓储、服务、处理器）
	deps := initDependencies(cfg, db, cacheInstance, lg)

	// 5) 设置路由和中间件
	r := router.New()
	handler := r.Setup(cfg, deps, lg)

	// 6) 启动 HTTP 服务器
	startServer(cfg, handler, lg)
}
