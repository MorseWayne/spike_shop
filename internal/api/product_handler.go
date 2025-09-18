// Package api 提供商品相关的HTTP API处理器实现。
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/middleware"
	"github.com/MorseWayne/spike_shop/internal/resp"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// ProductHandler 商品相关的HTTP处理器
type ProductHandler struct {
	productService service.ProductService
	logger         *zap.Logger
}

// NewProductHandler 创建商品处理器实例
func NewProductHandler(productService service.ProductService, logger *zap.Logger) *ProductHandler {
	return &ProductHandler{
		productService: productService,
		logger:         logger,
	}
}

// CreateProduct 创建商品
// POST /api/v1/products
// 需要管理员权限
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体
	var req domain.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateCreateProductRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层创建商品
	product, err := h.productService.CreateProduct(&req)
	if err != nil {
		if strings.Contains(err.Error(), "SKU already exists") {
			resp.Error(w, http.StatusConflict, resp.CodeInvalidParam, "SKU already exists", reqID, "")
			return
		}

		h.logger.Error("create product failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "create product failed", reqID, "")
		return
	}

	resp.OK(w, product, reqID, "")
}

// GetProduct 获取商品详情
// GET /api/v1/products/{id}
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取商品ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	idStr := parts[4] // /api/v1/products/{id}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	// 调用服务层获取商品
	product, err := h.productService.GetProduct(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "product not found", reqID, "")
			return
		}

		h.logger.Error("get product failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get product failed", reqID, "")
		return
	}

	resp.OK(w, product, reqID, "")
}

// UpdateProduct 更新商品
// PUT /api/v1/products/{id}
// 需要管理员权限
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取商品ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	idStr := parts[4] // /api/v1/products/{id}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	// 解析请求体
	var req domain.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 调用服务层更新商品
	product, err := h.productService.UpdateProduct(id, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "product not found", reqID, "")
			return
		}

		h.logger.Error("update product failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "update product failed", reqID, "")
		return
	}

	resp.OK(w, product, reqID, "")
}

// DeleteProduct 删除商品
// DELETE /api/v1/products/{id}
// 需要管理员权限
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取商品ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	idStr := parts[4] // /api/v1/products/{id}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	// 调用服务层删除商品
	err = h.productService.DeleteProduct(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "product not found", reqID, "")
			return
		}
		if strings.Contains(err.Error(), "existing stock") {
			resp.Error(w, http.StatusConflict, resp.CodeInvalidParam, "cannot delete product with existing stock", reqID, "")
			return
		}

		h.logger.Error("delete product failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "delete product failed", reqID, "")
		return
	}

	result := map[string]interface{}{"deleted": true}
	resp.OK(w, &result, reqID, "")
}

// ListProducts 获取商品列表
// GET /api/v1/products?page=1&page_size=20&status=active&category_id=1&brand=Apple&keyword=iPhone&sort_by=price&sort_order=asc
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析查询参数
	req := &domain.ProductListRequest{}
	query := r.URL.Query()

	// 分页参数
	if pageStr := query.Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			req.Page = page
		} else {
			req.Page = 1
		}
	} else {
		req.Page = 1
	}

	if pageSizeStr := query.Get("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 && pageSize <= 100 {
			req.PageSize = pageSize
		} else {
			req.PageSize = 20
		}
	} else {
		req.PageSize = 20
	}

	// 过滤参数
	if status := query.Get("status"); status != "" {
		productStatus := domain.ProductStatus(status)
		req.Status = &productStatus
	}

	if categoryIDStr := query.Get("category_id"); categoryIDStr != "" {
		if categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64); err == nil {
			req.CategoryID = &categoryID
		}
	}

	if brand := query.Get("brand"); brand != "" {
		req.Brand = &brand
	}

	if keyword := query.Get("keyword"); keyword != "" {
		req.Keyword = &keyword
	}

	// 排序参数
	if sortBy := query.Get("sort_by"); sortBy != "" {
		req.SortBy = &sortBy
	}

	if sortOrder := query.Get("sort_order"); sortOrder != "" {
		req.SortOrder = &sortOrder
	}

	// 调用服务层获取商品列表
	result, err := h.productService.ListProducts(req)
	if err != nil {
		h.logger.Error("list products failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "list products failed", reqID, "")
		return
	}

	resp.OK(w, result, reqID, "")
}

// SearchProducts 搜索商品
// GET /api/v1/products/search?keyword=iPhone&page=1&page_size=20
func (h *ProductHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析查询参数
	query := r.URL.Query()
	keyword := query.Get("keyword")
	if keyword == "" {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "keyword is required", reqID, "")
		return
	}

	page := 1
	pageSize := 20

	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := query.Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// 调用服务层搜索商品
	result, err := h.productService.SearchProducts(keyword, page, pageSize)
	if err != nil {
		h.logger.Error("search products failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "search products failed", reqID, "")
		return
	}

	resp.OK(w, result, reqID, "")
}

// GetProductsWithInventory 获取带库存信息的商品列表
// POST /api/v1/products/with-inventory
func (h *ProductHandler) GetProductsWithInventory(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体（商品ID列表）
	var req struct {
		ProductIDs []int64 `json:"product_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	if len(req.ProductIDs) == 0 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "product_ids is required", reqID, "")
		return
	}

	if len(req.ProductIDs) > 100 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "too many product IDs (max 100)", reqID, "")
		return
	}

	// 调用服务层获取带库存信息的商品
	result, err := h.productService.GetProductsWithInventory(req.ProductIDs)
	if err != nil {
		h.logger.Error("get products with inventory failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get products with inventory failed", reqID, "")
		return
	}

	resp.OK(w, &result, reqID, "")
}

// GetProductStats 获取商品统计信息
// GET /api/v1/products/stats
// 需要管理员权限
func (h *ProductHandler) GetProductStats(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 调用服务层获取统计信息
	stats, err := h.productService.GetProductStats()
	if err != nil {
		h.logger.Error("get product stats failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get product stats failed", reqID, "")
		return
	}

	resp.OK(w, stats, reqID, "")
}

// validateCreateProductRequest 验证创建商品请求
func (h *ProductHandler) validateCreateProductRequest(req *domain.CreateProductRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}

	if len(req.Name) > 255 {
		return errors.New("name too long (max 255 characters)")
	}

	if req.Price <= 0 {
		return errors.New("price must be greater than 0")
	}

	if req.SKU == "" {
		return errors.New("SKU is required")
	}

	if len(req.SKU) > 100 {
		return errors.New("SKU too long (max 100 characters)")
	}

	if req.Weight != nil && *req.Weight < 0 {
		return errors.New("weight cannot be negative")
	}

	return nil
}
