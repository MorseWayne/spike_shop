// Package api 提供秒杀相关的HTTP API处理器
package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/resp"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// SpikeServiceInterface 定义秒杀服务接口
type SpikeServiceInterface interface {
	ParticipateSpike(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64) (*domain.SpikeParticipationResponse, error)
	GetSpikeEventDetail(ctx context.Context, eventID int64) (*domain.SpikeEventWithProduct, error)
	GetUserSpikeOrders(ctx context.Context, userID int64, req *domain.SpikeOrderListRequest) (*domain.SpikeOrderListResponse, error)
	GetSpikeOrderDetail(ctx context.Context, orderID, userID int64) (*domain.SpikeOrderWithDetails, error)
	CancelSpikeOrder(ctx context.Context, orderID, userID int64, req *domain.CancelSpikeOrderRequest) error
	GetActiveEvents(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error)
	WarmupStock(ctx context.Context, eventID int64) error
	GetSpikeStats(ctx context.Context, eventID int64) (*service.SpikeStats, error)
}

// SpikeHandler 秒杀API处理器
type SpikeHandler struct {
	spikeService SpikeServiceInterface
	logger       *zap.Logger
}

// NewSpikeHandler 创建秒杀API处理器
func NewSpikeHandler(spikeService SpikeServiceInterface, logger *zap.Logger) *SpikeHandler {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SpikeHandler{
		spikeService: spikeService,
		logger:       logger,
	}
}

// ParticipateSpike 参与秒杀
// @Summary 参与秒杀
// @Description 用户参与秒杀活动
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param request body domain.SpikeParticipationRequest true "秒杀参与请求"
// @Success 200 {object} resp.Response[domain.SpikeParticipationResponse] "成功"
// @Failure 400 {object} resp.Response[any] "请求参数错误"
// @Failure 401 {object} resp.Response[any] "未授权"
// @Failure 429 {object} resp.Response[any] "请求过于频繁"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/spike/participate [post]
// @Security Bearer
func (h *SpikeHandler) ParticipateSpike(c *gin.Context) {
	var req domain.SpikeParticipationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("参数绑定失败", zap.Error(err))
		resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
			"请求参数格式错误", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 获取用户ID
	userID := h.getCurrentUserID(c)
	if userID == 0 {
		resp.Error(c.Writer, http.StatusUnauthorized, resp.CodeInvalidParam,
			"用户未登录", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 记录请求日志
	h.logger.Info("处理秒杀参与请求",
		zap.Int64("user_id", userID),
		zap.Int64("spike_event_id", req.SpikeEventID),
		zap.Int64("quantity", req.Quantity),
		zap.String("idempotency_key", req.IdempotencyKey))

	// 调用服务层
	result, err := h.spikeService.ParticipateSpike(c.Request.Context(), &req, userID)
	if err != nil {
		h.logger.Error("秒杀参与失败", zap.Error(err))
		resp.Error(c.Writer, http.StatusInternalServerError, resp.CodeInternalError,
			"系统繁忙，请稍后重试", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 返回结果
	resp.WriteJSON(c.Writer, http.StatusOK, resp.CodeOK, "success", result,
		h.getRequestID(c), h.getTraceID(c))
}

// GetSpikeEventDetail 获取秒杀活动详情
// @Summary 获取秒杀活动详情
// @Description 获取指定秒杀活动的详细信息，包含实时库存
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param id path int true "秒杀活动ID"
// @Success 200 {object} resp.Response[domain.SpikeEventWithProduct] "成功"
// @Failure 400 {object} resp.Response[any] "请求参数错误"
// @Failure 404 {object} resp.Response[any] "活动不存在"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/spike/events/{id} [get]
func (h *SpikeHandler) GetSpikeEventDetail(c *gin.Context) {
	// 解析活动ID
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil || eventID <= 0 {
		resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
			"无效的活动ID", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 调用服务层
	eventDetail, err := h.spikeService.GetSpikeEventDetail(c.Request.Context(), eventID)
	if err != nil {
		h.logger.Error("获取秒杀活动详情失败", zap.Int64("event_id", eventID), zap.Error(err))
		resp.Error(c.Writer, http.StatusNotFound, resp.CodeInvalidParam,
			"秒杀活动不存在", h.getRequestID(c), h.getTraceID(c))
		return
	}

	resp.WriteJSON(c.Writer, http.StatusOK, resp.CodeOK, "success", eventDetail,
		h.getRequestID(c), h.getTraceID(c))
}

// GetActiveEvents 获取活跃的秒杀活动列表
// @Summary 获取活跃秒杀活动列表
// @Description 获取当前活跃的秒杀活动列表，支持分页
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(20)
// @Param sort_by query string false "排序字段" Enums(start_at, created_at, spike_price)
// @Param sort_order query string false "排序方向" Enums(asc, desc) default(desc)
// @Success 200 {object} resp.Response[domain.SpikeEventListResponse] "成功"
// @Failure 400 {object} resp.Response[any] "请求参数错误"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/spike/events [get]
func (h *SpikeHandler) GetActiveEvents(c *gin.Context) {
	// 解析查询参数
	req := &domain.SpikeEventListRequest{
		Page:     1,
		PageSize: 20,
	}

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			req.Page = page
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 && pageSize <= 100 {
			req.PageSize = pageSize
		}
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		req.SortBy = &sortBy
	}

	if sortOrder := c.Query("sort_order"); sortOrder != "" {
		req.SortOrder = &sortOrder
	}

	// 调用服务层
	events, err := h.spikeService.GetActiveEvents(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("获取活跃秒杀活动失败", zap.Error(err))
		resp.Error(c.Writer, http.StatusInternalServerError, resp.CodeInternalError,
			"获取活动列表失败", h.getRequestID(c), h.getTraceID(c))
		return
	}

	resp.WriteJSON(c.Writer, http.StatusOK, resp.CodeOK, "success", events,
		h.getRequestID(c), h.getTraceID(c))
}

// GetUserSpikeOrders 获取用户秒杀订单列表
// @Summary 获取用户秒杀订单列表
// @Description 获取当前用户的秒杀订单列表，支持分页和状态过滤
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(20)
// @Param status query string false "订单状态" Enums(pending, paid, cancelled, expired)
// @Param sort_by query string false "排序字段" Enums(created_at, total_amount)
// @Param sort_order query string false "排序方向" Enums(asc, desc) default(desc)
// @Success 200 {object} resp.Response[domain.SpikeOrderListResponse] "成功"
// @Failure 401 {object} resp.Response[any] "未授权"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/spike/orders [get]
// @Security Bearer
func (h *SpikeHandler) GetUserSpikeOrders(c *gin.Context) {
	// 获取用户ID
	userID := h.getCurrentUserID(c)
	if userID == 0 {
		resp.Error(c.Writer, http.StatusUnauthorized, resp.CodeInvalidParam,
			"用户未登录", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 解析查询参数
	req := &domain.SpikeOrderListRequest{
		Page:     1,
		PageSize: 20,
	}

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			req.Page = page
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 && pageSize <= 100 {
			req.PageSize = pageSize
		}
	}

	if status := c.Query("status"); status != "" {
		orderStatus := domain.SpikeOrderStatus(status)
		req.Status = &orderStatus
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		req.SortBy = &sortBy
	}

	if sortOrder := c.Query("sort_order"); sortOrder != "" {
		req.SortOrder = &sortOrder
	}

	// 调用服务层
	orders, err := h.spikeService.GetUserSpikeOrders(c.Request.Context(), userID, req)
	if err != nil {
		h.logger.Error("获取用户秒杀订单失败", zap.Int64("user_id", userID), zap.Error(err))
		resp.Error(c.Writer, http.StatusInternalServerError, resp.CodeInternalError,
			"获取订单列表失败", h.getRequestID(c), h.getTraceID(c))
		return
	}

	resp.WriteJSON(c.Writer, http.StatusOK, resp.CodeOK, "success", orders,
		h.getRequestID(c), h.getTraceID(c))
}

// GetSpikeOrderDetail 获取秒杀订单详情
// @Summary 获取秒杀订单详情
// @Description 获取指定秒杀订单的详细信息
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Success 200 {object} resp.Response[domain.SpikeOrderWithDetails] "成功"
// @Failure 400 {object} resp.Response[any] "请求参数错误"
// @Failure 401 {object} resp.Response[any] "未授权"
// @Failure 403 {object} resp.Response[any] "无权限访问"
// @Failure 404 {object} resp.Response[any] "订单不存在"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/spike/orders/{id} [get]
// @Security Bearer
func (h *SpikeHandler) GetSpikeOrderDetail(c *gin.Context) {
	// 获取用户ID
	userID := h.getCurrentUserID(c)
	if userID == 0 {
		resp.Error(c.Writer, http.StatusUnauthorized, resp.CodeInvalidParam,
			"用户未登录", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 解析订单ID
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil || orderID <= 0 {
		resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
			"无效的订单ID", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 调用服务层
	orderDetail, err := h.spikeService.GetSpikeOrderDetail(c.Request.Context(), orderID, userID)
	if err != nil {
		h.logger.Error("获取秒杀订单详情失败",
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", userID),
			zap.Error(err))

		if err.Error() == "订单不属于当前用户" {
			resp.Error(c.Writer, http.StatusForbidden, resp.CodeInvalidParam,
				"无权限访问该订单", h.getRequestID(c), h.getTraceID(c))
		} else {
			resp.Error(c.Writer, http.StatusNotFound, resp.CodeInvalidParam,
				"订单不存在", h.getRequestID(c), h.getTraceID(c))
		}
		return
	}

	resp.WriteJSON(c.Writer, http.StatusOK, resp.CodeOK, "success", orderDetail,
		h.getRequestID(c), h.getTraceID(c))
}

// CancelSpikeOrder 取消秒杀订单
// @Summary 取消秒杀订单
// @Description 取消指定的秒杀订单，会异步恢复库存
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Param request body domain.CancelSpikeOrderRequest true "取消订单请求"
// @Success 200 {object} resp.Response[any] "成功"
// @Failure 400 {object} resp.Response[any] "请求参数错误"
// @Failure 401 {object} resp.Response[any] "未授权"
// @Failure 403 {object} resp.Response[any] "无权限访问"
// @Failure 404 {object} resp.Response[any] "订单不存在"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/spike/orders/{id}/cancel [post]
// @Security Bearer
func (h *SpikeHandler) CancelSpikeOrder(c *gin.Context) {
	// 获取用户ID
	userID := h.getCurrentUserID(c)
	if userID == 0 {
		resp.Error(c.Writer, http.StatusUnauthorized, resp.CodeInvalidParam,
			"用户未登录", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 解析订单ID
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil || orderID <= 0 {
		resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
			"无效的订单ID", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 解析请求体
	var req domain.CancelSpikeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("参数绑定失败", zap.Error(err))
		resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
			"请求参数格式错误", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 调用服务层
	err = h.spikeService.CancelSpikeOrder(c.Request.Context(), orderID, userID, &req)
	if err != nil {
		h.logger.Error("取消秒杀订单失败",
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", userID),
			zap.Error(err))

		if err.Error() == "订单不属于当前用户" {
			resp.Error(c.Writer, http.StatusForbidden, resp.CodeInvalidParam,
				"无权限操作该订单", h.getRequestID(c), h.getTraceID(c))
		} else if err.Error() == "订单当前状态不允许取消" {
			resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
				"订单当前状态不允许取消", h.getRequestID(c), h.getTraceID(c))
		} else {
			resp.Error(c.Writer, http.StatusInternalServerError, resp.CodeInternalError,
				"取消订单失败", h.getRequestID(c), h.getTraceID(c))
		}
		return
	}

	resp.WriteJSON[any](c.Writer, http.StatusOK, resp.CodeOK, "订单取消成功", nil,
		h.getRequestID(c), h.getTraceID(c))
}

// GetSpikeStats 获取秒杀统计信息
// @Summary 获取秒杀统计信息
// @Description 获取指定秒杀活动的统计信息，包含库存、订单等数据
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param id path int true "秒杀活动ID"
// @Success 200 {object} resp.Response[service.SpikeStats] "成功"
// @Failure 400 {object} resp.Response[any] "请求参数错误"
// @Failure 404 {object} resp.Response[any] "活动不存在"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/spike/events/{id}/stats [get]
func (h *SpikeHandler) GetSpikeStats(c *gin.Context) {
	// 解析活动ID
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil || eventID <= 0 {
		resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
			"无效的活动ID", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 调用服务层
	stats, err := h.spikeService.GetSpikeStats(c.Request.Context(), eventID)
	if err != nil {
		h.logger.Error("获取秒杀统计信息失败", zap.Int64("event_id", eventID), zap.Error(err))
		resp.Error(c.Writer, http.StatusNotFound, resp.CodeInvalidParam,
			"秒杀活动不存在", h.getRequestID(c), h.getTraceID(c))
		return
	}

	resp.WriteJSON(c.Writer, http.StatusOK, resp.CodeOK, "success", stats,
		h.getRequestID(c), h.getTraceID(c))
}

// WarmupStock 预热库存（管理员接口）
// @Summary 预热库存
// @Description 预热指定秒杀活动的库存到Redis缓存中
// @Tags 秒杀管理
// @Accept json
// @Produce json
// @Param id path int true "秒杀活动ID"
// @Success 200 {object} resp.Response[any] "成功"
// @Failure 400 {object} resp.Response[any] "请求参数错误"
// @Failure 401 {object} resp.Response[any] "未授权"
// @Failure 403 {object} resp.Response[any] "权限不足"
// @Failure 404 {object} resp.Response[any] "活动不存在"
// @Failure 500 {object} resp.Response[any] "服务器内部错误"
// @Router /api/v1/admin/spike/events/{id}/warmup [post]
// @Security Bearer
func (h *SpikeHandler) WarmupStock(c *gin.Context) {
	// 检查管理员权限
	if !h.isAdmin(c) {
		resp.Error(c.Writer, http.StatusForbidden, resp.CodeInvalidParam,
			"权限不足", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 解析活动ID
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil || eventID <= 0 {
		resp.Error(c.Writer, http.StatusBadRequest, resp.CodeInvalidParam,
			"无效的活动ID", h.getRequestID(c), h.getTraceID(c))
		return
	}

	// 调用服务层
	err = h.spikeService.WarmupStock(c.Request.Context(), eventID)
	if err != nil {
		h.logger.Error("预热库存失败", zap.Int64("event_id", eventID), zap.Error(err))
		resp.Error(c.Writer, http.StatusInternalServerError, resp.CodeInternalError,
			"预热库存失败", h.getRequestID(c), h.getTraceID(c))
		return
	}

	h.logger.Info("库存预热成功", zap.Int64("event_id", eventID))
	resp.WriteJSON[any](c.Writer, http.StatusOK, resp.CodeOK, "库存预热成功", nil,
		h.getRequestID(c), h.getTraceID(c))
}

// 辅助方法

// getCurrentUserID 获取当前用户ID
func (h *SpikeHandler) getCurrentUserID(c *gin.Context) int64 {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(int64); ok {
			return id
		}
	}
	return 0
}

// isAdmin 检查是否为管理员
func (h *SpikeHandler) isAdmin(c *gin.Context) bool {
	if role, exists := c.Get("user_role"); exists {
		if userRole, ok := role.(string); ok {
			return userRole == "admin"
		}
	}
	return false
}

// getRequestID 获取请求ID
func (h *SpikeHandler) getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// getTraceID 获取追踪ID
func (h *SpikeHandler) getTraceID(c *gin.Context) string {
	if traceID, exists := c.Get("trace_id"); exists {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}

// HealthCheck 健康检查接口
// @Summary 健康检查
// @Description 检查秒杀服务的健康状态
// @Tags 系统
// @Accept json
// @Produce json
// @Success 200 {object} resp.Response[map[string]interface{}] "成功"
// @Router /api/v1/spike/health [get]
func (h *SpikeHandler) HealthCheck(c *gin.Context) {
	healthInfo := map[string]interface{}{
		"service":   "spike-service",
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "v1.0.0",
	}

	resp.WriteJSON(c.Writer, http.StatusOK, resp.CodeOK, "healthy", &healthInfo,
		h.getRequestID(c), h.getTraceID(c))
}
