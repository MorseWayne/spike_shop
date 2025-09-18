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
type HTTPRouter struct {
	deps *Dependencies
}

// New 创建新的路由器实例
func New() Router {
	return &HTTPRouter{}
}

// Setup 设置路由和中间件
func (r *HTTPRouter) Setup(cfg *config.Config, deps *Dependencies, lg *zap.Logger) http.Handler {
	r.deps = deps

	// 标准库 ServeMux 即可满足当前需求（后续可替换为 chi/gin）
	mux := http.NewServeMux()

	// 健康检查端点
	r.setupHealthRoutes(mux, cfg)

	// 用户认证相关路由
	authMiddleware := mw.AuthMiddleware(deps.JWTService, lg)
	r.setupAuthRoutes(mux, authMiddleware)

	// 商品相关路由
	r.setupProductRoutes(mux)

	// 库存操作路由
	r.setupInventoryRoutes(mux, authMiddleware)

	// 管理员专用路由
	adminMiddleware := mw.RequireAdmin(lg)
	r.setupAdminRoutes(mux, authMiddleware, adminMiddleware)

	// 应用中间件链
	return setupMiddleware(mux, cfg, lg)
}

// 商品相关的处理器方法
func (r *HTTPRouter) handleProducts(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.deps.ProductHandler.ListProducts(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *HTTPRouter) handleProductDetail(w http.ResponseWriter, req *http.Request) {
	if strings.HasSuffix(req.URL.Path, "/inventory") {
		r.deps.InventoryHandler.GetInventoryByProductID(w, req)
	} else if strings.HasSuffix(req.URL.Path, "/inventory/check") {
		r.deps.InventoryHandler.CheckStockAvailability(w, req)
	} else {
		switch req.Method {
		case http.MethodGet:
			r.deps.ProductHandler.GetProduct(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// 库存相关的处理器方法
func (r *HTTPRouter) handleInventory(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.deps.InventoryHandler.ListInventories(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// 管理员商品相关的处理器方法
func (r *HTTPRouter) handleAdminProducts(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		r.deps.ProductHandler.CreateProduct(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *HTTPRouter) handleAdminProductDetail(w http.ResponseWriter, req *http.Request) {
	if strings.HasSuffix(req.URL.Path, "/inventory/adjust") {
		r.deps.InventoryHandler.AdjustStock(w, req)
	} else {
		switch req.Method {
		case http.MethodPut:
			r.deps.ProductHandler.UpdateProduct(w, req)
		case http.MethodDelete:
			r.deps.ProductHandler.DeleteProduct(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// 管理员库存相关的处理器方法
func (r *HTTPRouter) handleAdminInventory(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		r.deps.InventoryHandler.CreateInventory(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *HTTPRouter) handleAdminInventoryDetail(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.deps.InventoryHandler.GetInventory(w, req)
	case http.MethodPut:
		r.deps.InventoryHandler.UpdateInventory(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// setupHealthRoutes 设置健康检查路由
func (r *HTTPRouter) setupHealthRoutes(mux *http.ServeMux, cfg *config.Config) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		reqID := mw.RequestIDFromContext(req.Context())
		data := map[string]any{
			"status":  "ok",
			"version": cfg.App.Version,
		}
		resp.OK(w, &data, reqID, "")
	})
}

// setupAuthRoutes 设置用户认证相关路由
func (r *HTTPRouter) setupAuthRoutes(mux *http.ServeMux, authMiddleware func(http.Handler) http.Handler) {
	// 无需认证的路由
	mux.HandleFunc("/api/v1/auth/register", r.deps.UserHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", r.deps.UserHandler.Login)
	mux.HandleFunc("/api/v1/auth/refresh", r.deps.UserHandler.RefreshToken)

	// 需要认证的路由
	mux.Handle("/api/v1/users/profile", authMiddleware(http.HandlerFunc(r.deps.UserHandler.GetProfile)))
}

// setupProductRoutes 设置商品相关路由（公开访问）
func (r *HTTPRouter) setupProductRoutes(mux *http.ServeMux) {
	// 商品列表
	mux.HandleFunc("/api/v1/products", r.handleProducts)

	// 商品搜索
	mux.HandleFunc("/api/v1/products/search", r.deps.ProductHandler.SearchProducts)
	mux.Handle("/api/v1/products/with-inventory", http.HandlerFunc(r.deps.ProductHandler.GetProductsWithInventory))

	// 商品详情和库存查询
	mux.HandleFunc("/api/v1/products/", r.handleProductDetail)
}

// setupInventoryRoutes 设置库存操作路由（需要认证）
func (r *HTTPRouter) setupInventoryRoutes(mux *http.ServeMux, authMiddleware func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/inventory/reserve", authMiddleware(http.HandlerFunc(r.deps.InventoryHandler.ReserveStock)))
	mux.Handle("/api/v1/inventory/release", authMiddleware(http.HandlerFunc(r.deps.InventoryHandler.ReleaseStock)))
	mux.Handle("/api/v1/inventory/consume", authMiddleware(http.HandlerFunc(r.deps.InventoryHandler.ConsumeStock)))
	mux.Handle("/api/v1/inventory", authMiddleware(http.HandlerFunc(r.handleInventory)))
}

// setupAdminRoutes 设置管理员专用路由（需要管理员权限）
func (r *HTTPRouter) setupAdminRoutes(mux *http.ServeMux, authMiddleware, adminMiddleware func(http.Handler) http.Handler) {
	// 用户管理
	mux.Handle("/api/v1/admin/users", authMiddleware(adminMiddleware(http.HandlerFunc(r.deps.UserHandler.ListUsers))))
	mux.Handle("/api/v1/admin/users/role", authMiddleware(adminMiddleware(http.HandlerFunc(r.deps.UserHandler.UpdateUserRole))))
	mux.Handle("/api/v1/admin/users/status", authMiddleware(adminMiddleware(http.HandlerFunc(r.deps.UserHandler.UpdateUserStatus))))

	// 商品管理
	mux.Handle("/api/v1/admin/products", authMiddleware(adminMiddleware(http.HandlerFunc(r.handleAdminProducts))))
	mux.Handle("/api/v1/admin/products/", authMiddleware(adminMiddleware(http.HandlerFunc(r.handleAdminProductDetail))))
	mux.Handle("/api/v1/admin/products/stats", authMiddleware(adminMiddleware(http.HandlerFunc(r.deps.ProductHandler.GetProductStats))))

	// 库存管理
	mux.Handle("/api/v1/admin/inventory", authMiddleware(adminMiddleware(http.HandlerFunc(r.handleAdminInventory))))
	mux.Handle("/api/v1/admin/inventory/", authMiddleware(adminMiddleware(http.HandlerFunc(r.handleAdminInventoryDetail))))
	mux.Handle("/api/v1/admin/inventory/alerts/low-stock", authMiddleware(adminMiddleware(http.HandlerFunc(r.deps.InventoryHandler.GetLowStockAlerts))))
	mux.Handle("/api/v1/admin/inventory/stats", authMiddleware(adminMiddleware(http.HandlerFunc(r.deps.InventoryHandler.GetInventoryStats))))
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
