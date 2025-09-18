package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/MorseWayne/spike_shop/internal/api"
	"github.com/MorseWayne/spike_shop/internal/cache"
	"github.com/MorseWayne/spike_shop/internal/config"
	"github.com/MorseWayne/spike_shop/internal/database"
	"github.com/MorseWayne/spike_shop/internal/logger"
	mw "github.com/MorseWayne/spike_shop/internal/middleware"
	"github.com/MorseWayne/spike_shop/internal/repo"
	"github.com/MorseWayne/spike_shop/internal/resp"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// main 为应用入口：
// 1) 加载并校验配置；
// 2) 初始化结构化日志；
// 3) 初始化数据库连接并执行迁移；
// 4) 构建路由与中间件链；
// 5) 启动 HTTP 服务。
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	// init logger
	lg, err := logger.New(cfg.App.Env, cfg.Log.Level, cfg.Log.Encoding, cfg.App.Name, cfg.App.Version)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}

	// 初始化数据库连接
	db, err := database.New(cfg, lg)
	if err != nil {
		lg.Sugar().Fatalw("failed to initialize database", "err", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			lg.Sugar().Errorw("failed to close database connection", "err", err)
		}
	}()

	// 执行数据库迁移
	// 最佳实践：在应用启动时、HTTP服务器启动前执行数据库迁移
	// 这样可以确保在处理请求前，数据库结构已经完全准备好
	// 从环境变量获取迁移目录路径，如果未设置则使用默认值
	// 从配置中获取迁移目录路径
	migrationsDir := cfg.Migrations.Dir
	lg.Sugar().Infow("using migrations directory", "path", migrationsDir)

	if err := db.RunMigrations(migrationsDir); err != nil {
		lg.Sugar().Fatalw("failed to run database migrations", "err", err, "dir", migrationsDir)
	}

	// 初始化依赖注入链：仓储 -> 服务 -> API处理器
	userRepo := repo.NewUserRepository(db)
	userService := service.NewUserService(userRepo, lg)
	jwtService := service.NewJWTService(cfg, lg)
	userHandler := api.NewUserHandler(userService, jwtService, lg)

	// 初始化缓存
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

	// 标准库 ServeMux 即可满足当前需求（后续可替换为 chi/gin）
	mux := http.NewServeMux()
	// 健康检查端点
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		reqID := mw.RequestIDFromContext(r.Context())
		data := map[string]any{
			"status":  "ok",
			"version": cfg.App.Version,
		}
		resp.OK(w, &data, reqID, "")
	})

	// 用户认证相关API路由（无需认证）
	mux.HandleFunc("/api/v1/auth/register", userHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", userHandler.Login)
	mux.HandleFunc("/api/v1/auth/refresh", userHandler.RefreshToken)

	// 需要认证的API路由
	authMiddleware := mw.AuthMiddleware(jwtService, lg)
	mux.Handle("/api/v1/users/profile", authMiddleware(http.HandlerFunc(userHandler.GetProfile)))

	// 商品相关API路由
	// 公开访问（无需认证）
	mux.HandleFunc("/api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			productHandler.ListProducts(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/products/search", productHandler.SearchProducts)
	mux.Handle("/api/v1/products/with-inventory", http.HandlerFunc(productHandler.GetProductsWithInventory))

	// 商品详情和库存查询（无需认证）
	mux.HandleFunc("/api/v1/products/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/inventory") {
			inventoryHandler.GetInventoryByProductID(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/inventory/check") {
			inventoryHandler.CheckStockAvailability(w, r)
		} else {
			switch r.Method {
			case http.MethodGet:
				productHandler.GetProduct(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})

	// 库存操作（需要认证）
	mux.Handle("/api/v1/inventory/reserve", authMiddleware(http.HandlerFunc(inventoryHandler.ReserveStock)))
	mux.Handle("/api/v1/inventory/release", authMiddleware(http.HandlerFunc(inventoryHandler.ReleaseStock)))
	mux.Handle("/api/v1/inventory/consume", authMiddleware(http.HandlerFunc(inventoryHandler.ConsumeStock)))
	mux.Handle("/api/v1/inventory", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			inventoryHandler.ListInventories(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// 管理员专用API路由（需要管理员权限）
	adminMiddleware := mw.RequireAdmin(lg)

	// 用户管理
	mux.Handle("/api/v1/admin/users", authMiddleware(adminMiddleware(http.HandlerFunc(userHandler.ListUsers))))
	mux.Handle("/api/v1/admin/users/role", authMiddleware(adminMiddleware(http.HandlerFunc(userHandler.UpdateUserRole))))
	mux.Handle("/api/v1/admin/users/status", authMiddleware(adminMiddleware(http.HandlerFunc(userHandler.UpdateUserStatus))))

	// 商品管理
	mux.Handle("/api/v1/admin/products", authMiddleware(adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			productHandler.CreateProduct(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	mux.Handle("/api/v1/admin/products/", authMiddleware(adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/inventory/adjust") {
			inventoryHandler.AdjustStock(w, r)
		} else {
			switch r.Method {
			case http.MethodPut:
				productHandler.UpdateProduct(w, r)
			case http.MethodDelete:
				productHandler.DeleteProduct(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	}))))
	mux.Handle("/api/v1/admin/products/stats", authMiddleware(adminMiddleware(http.HandlerFunc(productHandler.GetProductStats))))

	// 库存管理
	mux.Handle("/api/v1/admin/inventory", authMiddleware(adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			inventoryHandler.CreateInventory(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	mux.Handle("/api/v1/admin/inventory/", authMiddleware(adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			inventoryHandler.GetInventory(w, r)
		case http.MethodPut:
			inventoryHandler.UpdateInventory(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	mux.Handle("/api/v1/admin/inventory/alerts/low-stock", authMiddleware(adminMiddleware(http.HandlerFunc(inventoryHandler.GetLowStockAlerts))))
	mux.Handle("/api/v1/admin/inventory/stats", authMiddleware(adminMiddleware(http.HandlerFunc(inventoryHandler.GetInventoryStats))))

	// 构建中间件链：请求进入时执行顺序为 access log → CORS → timeout → recovery → request ID
	// 响应返回时执行顺序为 request ID → recovery → timeout → CORS → access log
	handler := mw.RequestID(mux)
	handler = mw.Recovery(lg)(handler)
	handler = mw.Timeout(cfg.App.RequestTimeout)(handler)
	handler = mw.CORS(mw.CORSConfig{
		AllowedOrigins: cfg.CORS.AllowedOrigins,
		AllowedMethods: cfg.CORS.AllowedMethods,
		AllowedHeaders: cfg.CORS.AllowedHeaders,
	})(handler)
	handler = mw.AccessLog(lg)(handler)

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
