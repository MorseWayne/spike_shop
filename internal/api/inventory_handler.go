// Package api 提供库存相关的HTTP API处理器实现。
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

// InventoryHandler 库存相关的HTTP处理器
type InventoryHandler struct {
	inventoryService service.InventoryService
	logger           *zap.Logger
}

// NewInventoryHandler 创建库存处理器实例
func NewInventoryHandler(inventoryService service.InventoryService, logger *zap.Logger) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
		logger:           logger,
	}
}

// CreateInventory 创建库存记录
// POST /api/v1/inventory
// 需要管理员权限
func (h *InventoryHandler) CreateInventory(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体
	var req domain.CreateInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateCreateInventoryRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层创建库存
	inventory, err := h.inventoryService.CreateInventory(&req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "product not found", reqID, "")
			return
		}
		if strings.Contains(err.Error(), "already exists") {
			resp.Error(w, http.StatusConflict, resp.CodeInvalidParam, "inventory already exists for this product", reqID, "")
			return
		}

		h.logger.Error("create inventory failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "create inventory failed", reqID, "")
		return
	}

	resp.OK(w, inventory, reqID, "")
}

// GetInventory 获取库存详情
// GET /api/v1/inventory/{id}
func (h *InventoryHandler) GetInventory(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取库存ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid inventory ID", reqID, "")
		return
	}

	idStr := parts[4] // /api/v1/inventory/{id}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid inventory ID", reqID, "")
		return
	}

	// 调用服务层获取库存
	inventory, err := h.inventoryService.GetInventory(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "inventory not found", reqID, "")
			return
		}

		h.logger.Error("get inventory failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get inventory failed", reqID, "")
		return
	}

	resp.OK(w, inventory, reqID, "")
}

// GetInventoryByProductID 根据商品ID获取库存
// GET /api/v1/products/{product_id}/inventory
func (h *InventoryHandler) GetInventoryByProductID(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取商品ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	productIDStr := parts[4] // /api/v1/products/{product_id}/inventory
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	// 调用服务层获取库存
	inventory, err := h.inventoryService.GetInventoryByProductID(productID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "inventory not found", reqID, "")
			return
		}

		h.logger.Error("get inventory by product ID failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get inventory failed", reqID, "")
		return
	}

	resp.OK(w, inventory, reqID, "")
}

// UpdateInventory 更新库存
// PUT /api/v1/inventory/{id}
// 需要管理员权限
func (h *InventoryHandler) UpdateInventory(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取库存ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid inventory ID", reqID, "")
		return
	}

	idStr := parts[4] // /api/v1/inventory/{id}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid inventory ID", reqID, "")
		return
	}

	// 解析请求体
	var req domain.UpdateInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 调用服务层更新库存
	inventory, err := h.inventoryService.UpdateInventory(id, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "inventory not found", reqID, "")
			return
		}
		if strings.Contains(err.Error(), "version conflict") {
			resp.Error(w, http.StatusConflict, resp.CodeInvalidParam, "inventory has been modified by another request", reqID, "")
			return
		}

		h.logger.Error("update inventory failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "update inventory failed", reqID, "")
		return
	}

	resp.OK(w, inventory, reqID, "")
}

// ListInventories 获取库存列表
// GET /api/v1/inventory?page=1&page_size=20&product_id=1&low_stock=true&min_stock=10&max_stock=100&sort_by=stock&sort_order=asc
func (h *InventoryHandler) ListInventories(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析查询参数
	req := &domain.InventoryListRequest{}
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
	if productIDStr := query.Get("product_id"); productIDStr != "" {
		if productID, err := strconv.ParseInt(productIDStr, 10, 64); err == nil {
			req.ProductID = &productID
		}
	}

	if lowStockStr := query.Get("low_stock"); lowStockStr != "" {
		if lowStock, err := strconv.ParseBool(lowStockStr); err == nil {
			req.LowStock = &lowStock
		}
	}

	if minStockStr := query.Get("min_stock"); minStockStr != "" {
		if minStock, err := strconv.Atoi(minStockStr); err == nil {
			req.MinStock = &minStock
		}
	}

	if maxStockStr := query.Get("max_stock"); maxStockStr != "" {
		if maxStock, err := strconv.Atoi(maxStockStr); err == nil {
			req.MaxStock = &maxStock
		}
	}

	// 排序参数
	if sortBy := query.Get("sort_by"); sortBy != "" {
		req.SortBy = &sortBy
	}

	if sortOrder := query.Get("sort_order"); sortOrder != "" {
		req.SortOrder = &sortOrder
	}

	// 调用服务层获取库存列表
	result, err := h.inventoryService.ListInventories(req)
	if err != nil {
		h.logger.Error("list inventories failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "list inventories failed", reqID, "")
		return
	}

	resp.OK(w, result, reqID, "")
}

// GetLowStockAlerts 获取低库存警告
// GET /api/v1/inventory/alerts/low-stock
// 需要管理员权限
func (h *InventoryHandler) GetLowStockAlerts(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 调用服务层获取低库存警告
	alerts, err := h.inventoryService.GetLowStockAlerts()
	if err != nil {
		h.logger.Error("get low stock alerts failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get low stock alerts failed", reqID, "")
		return
	}

	resp.OK(w, &alerts, reqID, "")
}

// AdjustStock 调整库存
// POST /api/v1/products/{product_id}/inventory/adjust
// 需要管理员权限
func (h *InventoryHandler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取商品ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	productIDStr := parts[4] // /api/v1/products/{product_id}/inventory/adjust
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	// 解析请求体
	var req domain.StockAdjustmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateStockAdjustmentRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层调整库存
	err = h.inventoryService.AdjustStock(productID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "product not found", reqID, "")
			return
		}
		if strings.Contains(err.Error(), "negative stock") {
			resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "adjustment would result in negative stock", reqID, "")
			return
		}

		h.logger.Error("adjust stock failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "adjust stock failed", reqID, "")
		return
	}

	result := map[string]interface{}{"adjusted": true}
	resp.OK(w, &result, reqID, "")
}

// ReserveStock 预留库存
// POST /api/v1/inventory/reserve
func (h *InventoryHandler) ReserveStock(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体
	var req domain.ReserveStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateReserveStockRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层预留库存
	err := h.inventoryService.ReserveStock(&req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "product not found", reqID, "")
			return
		}
		if strings.Contains(err.Error(), "not available") {
			resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "product is not available for sale", reqID, "")
			return
		}
		if strings.Contains(err.Error(), "insufficient stock") {
			resp.Error(w, http.StatusConflict, resp.CodeInvalidParam, "insufficient stock", reqID, "")
			return
		}

		h.logger.Error("reserve stock failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "reserve stock failed", reqID, "")
		return
	}

	result := map[string]interface{}{"reserved": true}
	resp.OK(w, &result, reqID, "")
}

// ReleaseStock 释放库存
// POST /api/v1/inventory/release
func (h *InventoryHandler) ReleaseStock(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体
	var req domain.ReleaseStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateReleaseStockRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层释放库存
	err := h.inventoryService.ReleaseStock(&req)
	if err != nil {
		if strings.Contains(err.Error(), "insufficient reserved stock") {
			resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "insufficient reserved stock", reqID, "")
			return
		}

		h.logger.Error("release stock failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "release stock failed", reqID, "")
		return
	}

	result := map[string]interface{}{"released": true}
	resp.OK(w, &result, reqID, "")
}

// ConsumeStock 消费库存
// POST /api/v1/inventory/consume
func (h *InventoryHandler) ConsumeStock(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 解析请求体
	var req domain.ConsumeStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request body", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid request body", reqID, "")
		return
	}

	// 基本验证
	if err := h.validateConsumeStockRequest(&req); err != nil {
		h.logger.Warn("validation failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, err.Error(), reqID, "")
		return
	}

	// 调用服务层消费库存
	err := h.inventoryService.ConsumeStock(&req)
	if err != nil {
		if strings.Contains(err.Error(), "insufficient reserved stock") {
			resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "insufficient reserved stock", reqID, "")
			return
		}

		h.logger.Error("consume stock failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "consume stock failed", reqID, "")
		return
	}

	result := map[string]interface{}{"consumed": true}
	resp.OK(w, &result, reqID, "")
}

// GetInventoryStats 获取库存统计信息
// GET /api/v1/inventory/stats
// 需要管理员权限
func (h *InventoryHandler) GetInventoryStats(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 调用服务层获取统计信息
	stats, err := h.inventoryService.GetInventoryStats()
	if err != nil {
		h.logger.Error("get inventory stats failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "get inventory stats failed", reqID, "")
		return
	}

	resp.OK(w, stats, reqID, "")
}

// CheckStockAvailability 检查库存可用性
// GET /api/v1/products/{product_id}/inventory/check?quantity=10
func (h *InventoryHandler) CheckStockAvailability(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// 从URL路径中提取商品ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	productIDStr := parts[4] // /api/v1/products/{product_id}/inventory/check
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid product ID", reqID, "")
		return
	}

	// 从查询参数获取数量
	quantityStr := r.URL.Query().Get("quantity")
	if quantityStr == "" {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "quantity is required", reqID, "")
		return
	}

	quantity, err := strconv.Atoi(quantityStr)
	if err != nil || quantity <= 0 {
		resp.Error(w, http.StatusBadRequest, resp.CodeInvalidParam, "invalid quantity", reqID, "")
		return
	}

	// 调用服务层检查库存可用性
	available, err := h.inventoryService.CheckStockAvailability(productID, quantity)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Error(w, http.StatusNotFound, resp.CodeInvalidParam, "inventory not found", reqID, "")
			return
		}

		h.logger.Error("check stock availability failed", zap.String("request_id", reqID), zap.Error(err))
		resp.Error(w, http.StatusInternalServerError, resp.CodeInternalError, "check stock availability failed", reqID, "")
		return
	}

	result := map[string]interface{}{
		"product_id": productID,
		"quantity":   quantity,
		"available":  available,
	}
	resp.OK(w, &result, reqID, "")
}

// 验证函数

func (h *InventoryHandler) validateCreateInventoryRequest(req *domain.CreateInventoryRequest) error {
	if req.ProductID <= 0 {
		return errors.New("product_id is required")
	}

	if req.Stock < 0 {
		return errors.New("stock cannot be negative")
	}

	if req.ReorderPoint < 0 {
		return errors.New("reorder_point cannot be negative")
	}

	if req.MaxStock <= 0 {
		return errors.New("max_stock must be greater than 0")
	}

	if req.Stock > req.MaxStock {
		return errors.New("stock cannot exceed max_stock")
	}

	return nil
}

func (h *InventoryHandler) validateStockAdjustmentRequest(req *domain.StockAdjustmentRequest) error {
	if req.Quantity == 0 {
		return errors.New("quantity cannot be zero")
	}

	if req.Reason == "" {
		return errors.New("reason is required")
	}

	if req.Type != "in" && req.Type != "out" {
		return errors.New("type must be 'in' or 'out'")
	}

	return nil
}

func (h *InventoryHandler) validateReserveStockRequest(req *domain.ReserveStockRequest) error {
	if req.ProductID <= 0 {
		return errors.New("product_id is required")
	}

	if req.Quantity <= 0 {
		return errors.New("quantity must be greater than 0")
	}

	return nil
}

func (h *InventoryHandler) validateReleaseStockRequest(req *domain.ReleaseStockRequest) error {
	if req.ProductID <= 0 {
		return errors.New("product_id is required")
	}

	if req.Quantity <= 0 {
		return errors.New("quantity must be greater than 0")
	}

	return nil
}

func (h *InventoryHandler) validateConsumeStockRequest(req *domain.ConsumeStockRequest) error {
	if req.ProductID <= 0 {
		return errors.New("product_id is required")
	}

	if req.Quantity <= 0 {
		return errors.New("quantity must be greater than 0")
	}

	return nil
}
