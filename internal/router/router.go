// Package router 提供 HTTP 路由设置和中间件配置功能
package router

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/api"
	"github.com/MorseWayne/spike_shop/internal/config"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// Dependencies 包含路由设置所需的所有依赖
type Dependencies struct {
	UserHandler      *api.UserHandler
	ProductHandler   *api.ProductHandler
	InventoryHandler *api.InventoryHandler
	JWTService       service.JWTService
}

// Router 路由器接口
type Router interface {
	Setup(cfg *config.Config, deps *Dependencies, lg *zap.Logger) http.Handler
}

// GinRouter Gin路由器实现
type GinRouter struct {
	engine *gin.Engine
	deps   *Dependencies
	logger *zap.Logger
}

// New 创建新的路由器实例
func New() Router {
	return &GinRouter{}
}

// Setup 设置路由和中间件
func (r *GinRouter) Setup(cfg *config.Config, deps *Dependencies, lg *zap.Logger) http.Handler {
	// 根据环境设置 Gin 模式
	if cfg.App.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r.engine = gin.New()
	r.deps = deps
	r.logger = lg

	// 设置中间件
	r.setupMiddleware(cfg)

	// 设置路由
	r.setupRoutes()

	return r.engine
}

// setupMiddleware 设置 Gin 中间件
func (r *GinRouter) setupMiddleware(cfg *config.Config) {
	// 恢复中间件（从 panic 中恢复）
	r.engine.Use(gin.Recovery())

	// 日志中间件
	r.engine.Use(r.ginLogger())

	// CORS 中间件
	r.engine.Use(r.corsMiddleware(cfg))
}

// setupRoutes 设置所有路由
func (r *GinRouter) setupRoutes() {
	// 健康检查
	r.engine.GET("/healthz", r.healthCheck)

	// API v1 路由组
	v1 := r.engine.Group("/api/v1")
	{
		// 认证路由（无需认证）
		auth := v1.Group("/auth")
		{
			auth.POST("/register", r.wrapHandler(r.deps.UserHandler.Register))
			auth.POST("/login", r.wrapHandler(r.deps.UserHandler.Login))
			auth.POST("/refresh", r.wrapHandler(r.deps.UserHandler.RefreshToken))
		}

		// 用户路由（需要认证）
		users := v1.Group("/users")
		users.Use(r.authMiddleware())
		{
			users.GET("/profile", r.wrapHandler(r.deps.UserHandler.GetProfile))
		}

		// 商品路由（公开）
		products := v1.Group("/products")
		{
			products.GET("", r.wrapHandler(r.deps.ProductHandler.ListProducts))
			products.GET("/search", r.wrapHandler(r.deps.ProductHandler.SearchProducts))
			products.GET("/with-inventory", r.wrapHandler(r.deps.ProductHandler.GetProductsWithInventory))
			products.GET("/:id", r.wrapHandler(r.deps.ProductHandler.GetProduct))
			products.GET("/:id/inventory", r.wrapHandler(r.deps.InventoryHandler.GetInventoryByProductID))
			products.GET("/:id/inventory/check", r.wrapHandler(r.deps.InventoryHandler.CheckStockAvailability))
		}

		// 库存路由（需要认证）
		inventory := v1.Group("/inventory")
		inventory.Use(r.authMiddleware())
		{
			inventory.GET("", r.wrapHandler(r.deps.InventoryHandler.ListInventories))
			inventory.POST("/reserve", r.wrapHandler(r.deps.InventoryHandler.ReserveStock))
			inventory.POST("/release", r.wrapHandler(r.deps.InventoryHandler.ReleaseStock))
			inventory.POST("/consume", r.wrapHandler(r.deps.InventoryHandler.ConsumeStock))
		}

		// 管理员路由（需要认证+管理员权限）
		admin := v1.Group("/admin")
		admin.Use(r.authMiddleware(), r.adminMiddleware())
		{
			// 用户管理
			adminUsers := admin.Group("/users")
			{
				adminUsers.GET("", r.wrapHandler(r.deps.UserHandler.ListUsers))
				adminUsers.PUT("/role", r.wrapHandler(r.deps.UserHandler.UpdateUserRole))
				adminUsers.PUT("/status", r.wrapHandler(r.deps.UserHandler.UpdateUserStatus))
			}

			// 商品管理
			adminProducts := admin.Group("/products")
			{
				adminProducts.POST("", r.wrapHandler(r.deps.ProductHandler.CreateProduct))
				adminProducts.PUT("/:id", r.wrapHandler(r.deps.ProductHandler.UpdateProduct))
				adminProducts.DELETE("/:id", r.wrapHandler(r.deps.ProductHandler.DeleteProduct))
				adminProducts.GET("/stats", r.wrapHandler(r.deps.ProductHandler.GetProductStats))
				adminProducts.POST("/:id/inventory/adjust", r.wrapHandler(r.deps.InventoryHandler.AdjustStock))
			}

			// 库存管理
			adminInventory := admin.Group("/inventory")
			{
				adminInventory.POST("", r.wrapHandler(r.deps.InventoryHandler.CreateInventory))
				adminInventory.GET("/:id", r.wrapHandler(r.deps.InventoryHandler.GetInventory))
				adminInventory.PUT("/:id", r.wrapHandler(r.deps.InventoryHandler.UpdateInventory))
				adminInventory.GET("/alerts/low-stock", r.wrapHandler(r.deps.InventoryHandler.GetLowStockAlerts))
				adminInventory.GET("/stats", r.wrapHandler(r.deps.InventoryHandler.GetInventoryStats))
			}
		}
	}
}

// healthCheck 健康检查处理器
func (r *GinRouter) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "1.0.0", // 可以从配置中获取
	})
}

// wrapHandler 将标准的 http.HandlerFunc 包装为 gin.HandlerFunc
func (r *GinRouter) wrapHandler(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return gin.WrapF(handler)
}

// ginLogger 自定义 Gin 日志中间件
func (r *GinRouter) ginLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// 这里可以自定义日志格式，或者集成到现有的 zap logger
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format("02/Jan/2006:15:04:05 -0700"),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// corsMiddleware CORS 中间件
func (r *GinRouter) corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// authMiddleware 认证中间件
func (r *GinRouter) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 这里需要实现 JWT 认证逻辑
		// 由于现有的中间件是基于标准库的，我们需要适配
		// 暂时直接通过，实际项目中需要实现 JWT 验证
		c.Next()
	}
}

// adminMiddleware 管理员权限中间件
func (r *GinRouter) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 这里需要实现管理员权限检查逻辑
		// 暂时直接通过，实际项目中需要验证用户角色
		c.Next()
	}
}
