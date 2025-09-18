// Package router 提供 HTTP 路由设置和中间件配置功能
package router

import (
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/api"
	"github.com/MorseWayne/spike_shop/internal/config"
	mw "github.com/MorseWayne/spike_shop/internal/middleware"
	"github.com/MorseWayne/spike_shop/internal/resp"
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

// HTTPRouter HTTP路由器实现
type HTTPRouter struct{}

// New 创建新的路由器实例
func New() Router {
	return &HTTPRouter{}
}

// Setup 设置路由和中间件
func (r *HTTPRouter) Setup(cfg *config.Config, deps *Dependencies, lg *zap.Logger) http.Handler {
	// 标准库 ServeMux 即可满足当前需求（后续可替换为 chi/gin）
	mux := http.NewServeMux()

	// 健康检查端点
	setupHealthRoutes(mux, cfg)

	// 用户认证相关路由
	authMiddleware := mw.AuthMiddleware(deps.JWTService, lg)
	setupAuthRoutes(mux, deps.UserHandler, authMiddleware)

	// 商品相关路由
	setupProductRoutes(mux, deps.ProductHandler, deps.InventoryHandler)

	// 库存操作路由
	setupInventoryRoutes(mux, deps.InventoryHandler, authMiddleware)

	// 管理员专用路由
	adminMiddleware := mw.RequireAdmin(lg)
	setupAdminRoutes(mux, deps.UserHandler, deps.ProductHandler, deps.InventoryHandler, authMiddleware, adminMiddleware)

	// 应用中间件链
	return setupMiddleware(mux, cfg, lg)
}

// setupHealthRoutes 设置健康检查路由
func setupHealthRoutes(mux *http.ServeMux, cfg *config.Config) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		reqID := mw.RequestIDFromContext(r.Context())
		data := map[string]any{
			"status":  "ok",
			"version": cfg.App.Version,
		}
		resp.OK(w, &data, reqID, "")
	})
}

// setupAuthRoutes 设置用户认证相关路由
func setupAuthRoutes(mux *http.ServeMux, userHandler *api.UserHandler, authMiddleware func(http.Handler) http.Handler) {
	// 无需认证的路由
	mux.HandleFunc("/api/v1/auth/register", userHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", userHandler.Login)
	mux.HandleFunc("/api/v1/auth/refresh", userHandler.RefreshToken)

	// 需要认证的路由
	mux.Handle("/api/v1/users/profile", authMiddleware(http.HandlerFunc(userHandler.GetProfile)))
}

// setupProductRoutes 设置商品相关路由（公开访问）
func setupProductRoutes(mux *http.ServeMux, productHandler *api.ProductHandler, inventoryHandler *api.InventoryHandler) {
	// 商品列表
	mux.HandleFunc("/api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			productHandler.ListProducts(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 商品搜索
	mux.HandleFunc("/api/v1/products/search", productHandler.SearchProducts)
	mux.Handle("/api/v1/products/with-inventory", http.HandlerFunc(productHandler.GetProductsWithInventory))

	// 商品详情和库存查询
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
}

// setupInventoryRoutes 设置库存操作路由（需要认证）
func setupInventoryRoutes(mux *http.ServeMux, inventoryHandler *api.InventoryHandler, authMiddleware func(http.Handler) http.Handler) {
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
}

// setupAdminRoutes 设置管理员专用路由（需要管理员权限）
func setupAdminRoutes(mux *http.ServeMux, userHandler *api.UserHandler, productHandler *api.ProductHandler, inventoryHandler *api.InventoryHandler, authMiddleware, adminMiddleware func(http.Handler) http.Handler) {
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
}

// setupMiddleware 设置中间件链
func setupMiddleware(mux *http.ServeMux, cfg *config.Config, lg *zap.Logger) http.Handler {
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

	return handler
}
