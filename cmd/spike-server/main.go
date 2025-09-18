package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/MorseWayne/spike_shop/internal/api"
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
	userHandler := api.NewUserHandler(userService, lg)

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

	// 用户认证相关API路由
	mux.HandleFunc("/api/v1/auth/register", userHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", userHandler.Login)
	mux.HandleFunc("/api/v1/users/profile", userHandler.GetProfile)

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
