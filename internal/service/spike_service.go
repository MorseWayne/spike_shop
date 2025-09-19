// Package service 实现秒杀业务逻辑服务层
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/cache"
	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/limiter"
	"github.com/MorseWayne/spike_shop/internal/mq"
	"github.com/MorseWayne/spike_shop/internal/repo"
)

// SpikeService 秒杀服务
type SpikeService struct {
	// 仓储层
	spikeEventRepo repo.SpikeEventRepository
	spikeOrderRepo repo.SpikeOrderRepository
	productRepo    repo.ProductRepository
	inventoryRepo  repo.InventoryRepository
	userRepo       repo.UserRepository

	// 缓存层
	spikeCache *cache.SpikeCache

	// 消息队列
	spikeProducer *mq.SpikeProducer

	// 限流器
	globalLimiter limiter.Limiter
	userLimiter   limiter.Limiter

	// 日志
	logger *zap.Logger

	// 配置
	config *SpikeServiceConfig
}

// SpikeServiceConfig 秒杀服务配置
type SpikeServiceConfig struct {
	// 订单过期时间
	OrderExpireTime time.Duration `json:"order_expire_time"`

	// 限流配置
	GlobalRateLimit int64         `json:"global_rate_limit"`
	UserRateLimit   int64         `json:"user_rate_limit"`
	RateLimitWindow time.Duration `json:"rate_limit_window"`

	// 库存预热配置
	StockWarmupEnabled bool          `json:"stock_warmup_enabled"`
	StockWarmupTime    time.Duration `json:"stock_warmup_time"`

	// 缓存配置
	StockCacheTTL  time.Duration `json:"stock_cache_ttl"`
	UserMarkTTL    time.Duration `json:"user_mark_ttl"`
	IdempotencyTTL time.Duration `json:"idempotency_ttl"`

	// 重试配置
	MaxRetryAttempts int           `json:"max_retry_attempts"`
	RetryInterval    time.Duration `json:"retry_interval"`
}

// DefaultSpikeServiceConfig 默认配置
func DefaultSpikeServiceConfig() *SpikeServiceConfig {
	return &SpikeServiceConfig{
		OrderExpireTime:    30 * time.Minute,
		GlobalRateLimit:    1000,
		UserRateLimit:      5,
		RateLimitWindow:    time.Minute,
		StockWarmupEnabled: true,
		StockWarmupTime:    5 * time.Minute,
		StockCacheTTL:      2 * time.Hour,
		UserMarkTTL:        24 * time.Hour,
		IdempotencyTTL:     24 * time.Hour,
		MaxRetryAttempts:   3,
		RetryInterval:      time.Second,
	}
}

// NewSpikeService 创建秒杀服务
func NewSpikeService(
	spikeEventRepo repo.SpikeEventRepository,
	spikeOrderRepo repo.SpikeOrderRepository,
	productRepo repo.ProductRepository,
	inventoryRepo repo.InventoryRepository,
	userRepo repo.UserRepository,
	spikeCache *cache.SpikeCache,
	spikeProducer *mq.SpikeProducer,
	globalLimiter limiter.Limiter,
	userLimiter limiter.Limiter,
	config *SpikeServiceConfig,
	logger *zap.Logger,
) *SpikeService {
	if config == nil {
		config = DefaultSpikeServiceConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SpikeService{
		spikeEventRepo: spikeEventRepo,
		spikeOrderRepo: spikeOrderRepo,
		productRepo:    productRepo,
		inventoryRepo:  inventoryRepo,
		userRepo:       userRepo,
		spikeCache:     spikeCache,
		spikeProducer:  spikeProducer,
		globalLimiter:  globalLimiter,
		userLimiter:    userLimiter,
		config:         config,
		logger:         logger,
	}
}

// ParticipateSpike 参与秒杀
func (s *SpikeService) ParticipateSpike(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64) (*domain.SpikeParticipationResponse, error) {
	// 生成追踪ID
	traceID := uuid.New().String()
	logger := s.logger.With(
		zap.String("trace_id", traceID),
		zap.Int64("user_id", userID),
		zap.Int64("spike_event_id", req.SpikeEventID),
		zap.Int64("quantity", req.Quantity),
		zap.String("idempotency_key", req.IdempotencyKey),
	)

	logger.Info("开始处理秒杀请求")

	// 1. 限流检查
	if err := s.checkRateLimit(ctx, userID); err != nil {
		logger.Warn("限流检查失败", zap.Error(err))
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: "请求过于频繁，请稍后重试",
		}, nil
	}

	// 2. 参数验证
	if err := s.validateSpikeRequest(req, userID); err != nil {
		logger.Warn("参数验证失败", zap.Error(err))
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// 3. 获取秒杀活动信息
	spikeEvent, err := s.getSpikeEventWithCache(ctx, req.SpikeEventID)
	if err != nil {
		logger.Error("获取秒杀活动失败", zap.Error(err))
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: "秒杀活动不存在或已结束",
		}, nil
	}

	// 4. 检查活动状态
	if !spikeEvent.IsActive() {
		logger.Warn("秒杀活动未开始或已结束")
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: "秒杀活动未开始或已结束",
		}, nil
	}

	// 5. 检查库存和售罄标记
	stockInfo, err := s.spikeCache.GetStockInfo(ctx, req.SpikeEventID)
	if err != nil {
		logger.Error("获取库存信息失败", zap.Error(err))
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: "系统繁忙，请稍后重试",
		}, nil
	}

	if stockInfo.SoldOut {
		logger.Info("商品已售罄")
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: "商品已售罄",
		}, nil
	}

	// 6. Redis原子性预减库存
	result, err := s.spikeCache.DecrementStock(ctx, req.SpikeEventID, userID, req.Quantity,
		s.config.UserMarkTTL, s.config.StockCacheTTL)
	if err != nil {
		logger.Error("预减库存失败", zap.Error(err))
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: "系统繁忙，请稍后重试",
		}, nil
	}

	if !result.Success {
		logger.Info("预减库存失败", zap.String("reason", result.Message))
		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: result.Message,
		}, nil
	}

	logger.Info("预减库存成功", zap.Int64("remaining_stock", result.RemainingStock))

	// 7. 发送异步消息进行DB落库
	if err := s.sendOrderCreatedMessage(ctx, req, userID, spikeEvent, traceID); err != nil {
		logger.Error("发送订单创建消息失败", zap.Error(err))

		// 恢复Redis库存
		if _, restoreErr := s.spikeCache.RestoreStock(ctx, req.SpikeEventID, userID, req.Quantity); restoreErr != nil {
			logger.Error("恢复Redis库存失败", zap.Error(restoreErr))
		}

		return &domain.SpikeParticipationResponse{
			Success: false,
			Message: "系统繁忙，请稍后重试",
		}, nil
	}

	logger.Info("秒杀请求处理成功")

	return &domain.SpikeParticipationResponse{
		Success: true,
		Message: "秒杀成功，请尽快完成支付",
	}, nil
}

// checkRateLimit 检查限流
func (s *SpikeService) checkRateLimit(ctx context.Context, userID int64) error {
	// 检查全局限流
	globalKey := "global"
	globalResult, err := s.globalLimiter.Allow(ctx, globalKey)
	if err != nil {
		return fmt.Errorf("global rate limit check failed: %w", err)
	}
	if !globalResult.Allowed {
		return fmt.Errorf("global rate limit exceeded")
	}

	// 检查用户限流
	userKey := fmt.Sprintf("user:%d", userID)
	userResult, err := s.userLimiter.Allow(ctx, userKey)
	if err != nil {
		return fmt.Errorf("user rate limit check failed: %w", err)
	}
	if !userResult.Allowed {
		return fmt.Errorf("user rate limit exceeded")
	}

	return nil
}

// validateSpikeRequest 验证秒杀请求
func (s *SpikeService) validateSpikeRequest(req *domain.SpikeParticipationRequest, userID int64) error {
	if req.SpikeEventID <= 0 {
		return fmt.Errorf("无效的秒杀活动ID")
	}
	if req.Quantity <= 0 || req.Quantity > 10 {
		return fmt.Errorf("购买数量必须在1-10之间")
	}
	if req.IdempotencyKey == "" {
		return fmt.Errorf("幂等键不能为空")
	}
	if userID <= 0 {
		return fmt.Errorf("用户未登录")
	}
	return nil
}

// getSpikeEventWithCache 获取秒杀活动信息（带缓存）
func (s *SpikeService) getSpikeEventWithCache(ctx context.Context, eventID int64) (*domain.SpikeEvent, error) {
	// 尝试从缓存获取
	var spikeEvent domain.SpikeEvent
	err := s.spikeCache.GetEventInfo(ctx, eventID, &spikeEvent)
	if err == nil {
		return &spikeEvent, nil
	}

	// 缓存未命中，从数据库获取
	event, err := s.spikeEventRepo.GetByID(eventID)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	if cacheErr := s.spikeCache.CacheEventInfo(ctx, eventID, event, s.config.StockCacheTTL); cacheErr != nil {
		s.logger.Warn("缓存秒杀活动信息失败", zap.Error(cacheErr))
	}

	return event, nil
}

// sendOrderCreatedMessage 发送订单创建消息
func (s *SpikeService) sendOrderCreatedMessage(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64, spikeEvent *domain.SpikeEvent, traceID string) error {
	expireAt := time.Now().Add(s.config.OrderExpireTime)

	data := &mq.SpikeOrderCreatedData{
		SpikeEventID:   req.SpikeEventID,
		UserID:         userID,
		ProductID:      spikeEvent.ProductID,
		Quantity:       req.Quantity,
		SpikePrice:     spikeEvent.SpikePrice,
		TotalAmount:    float64(req.Quantity) * spikeEvent.SpikePrice,
		IdempotencyKey: req.IdempotencyKey,
		ExpireAt:       expireAt,
		CreatedAt:      time.Now(),
	}

	return s.spikeProducer.PublishSpikeOrderCreated(ctx, data, traceID)
}

// GetSpikeEventDetail 获取秒杀活动详情
func (s *SpikeService) GetSpikeEventDetail(ctx context.Context, eventID int64) (*domain.SpikeEventWithProduct, error) {
	// 获取秒杀活动
	spikeEvent, err := s.getSpikeEventWithCache(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spike event: %w", err)
	}

	// 获取商品信息
	product, err := s.productRepo.GetByID(spikeEvent.ProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	// 获取实时库存信息
	stockInfo, err := s.spikeCache.GetStockInfo(ctx, eventID)
	if err != nil {
		s.logger.Warn("获取Redis库存信息失败", zap.Error(err))
		// 使用数据库库存信息
		stockInfo = &cache.StockInfo{
			Stock:   spikeEvent.SpikeStock - spikeEvent.SoldCount,
			SoldOut: spikeEvent.SoldCount >= spikeEvent.SpikeStock,
			Exists:  true,
		}
	}

	// 更新实时库存信息
	if stockInfo.Exists && stockInfo.Stock >= 0 {
		spikeEvent.SpikeStock = stockInfo.Stock
	}

	return &domain.SpikeEventWithProduct{
		SpikeEvent: spikeEvent,
		Product:    product,
	}, nil
}

// GetUserSpikeOrders 获取用户秒杀订单列表
func (s *SpikeService) GetUserSpikeOrders(ctx context.Context, userID int64, req *domain.SpikeOrderListRequest) (*domain.SpikeOrderListResponse, error) {
	req.UserID = &userID
	orders, total, err := s.spikeOrderRepo.List(req)
	if err != nil {
		return nil, err
	}

	return &domain.SpikeOrderListResponse{
		Orders:   orders,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetSpikeOrderDetail 获取秒杀订单详情
func (s *SpikeService) GetSpikeOrderDetail(ctx context.Context, orderID, userID int64) (*domain.SpikeOrderWithDetails, error) {
	// 获取秒杀订单
	spikeOrder, err := s.spikeOrderRepo.GetByID(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spike order: %w", err)
	}

	// 验证订单所有权
	if spikeOrder.UserID != userID {
		return nil, fmt.Errorf("订单不属于当前用户")
	}

	// 获取秒杀活动信息
	spikeEvent, err := s.spikeEventRepo.GetByID(spikeOrder.SpikeEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spike event: %w", err)
	}

	// 获取用户信息
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &domain.SpikeOrderWithDetails{
		SpikeOrder: spikeOrder,
		SpikeEvent: spikeEvent,
		User:       user,
	}, nil
}

// CancelSpikeOrder 取消秒杀订单
func (s *SpikeService) CancelSpikeOrder(ctx context.Context, orderID, userID int64, req *domain.CancelSpikeOrderRequest) error {
	// 获取秒杀订单
	spikeOrder, err := s.spikeOrderRepo.GetByID(orderID)
	if err != nil {
		return fmt.Errorf("failed to get spike order: %w", err)
	}

	// 验证订单所有权
	if spikeOrder.UserID != userID {
		return fmt.Errorf("订单不属于当前用户")
	}

	// 检查订单状态
	if !spikeOrder.CanCancel() {
		return fmt.Errorf("订单当前状态不允许取消")
	}

	// 获取秒杀活动信息
	spikeEvent, err := s.spikeEventRepo.GetByID(spikeOrder.SpikeEventID)
	if err != nil {
		return fmt.Errorf("failed to get spike event: %w", err)
	}

	// 发送订单取消消息
	traceID := uuid.New().String()
	data := &mq.SpikeOrderCancelledData{
		SpikeOrderID:   spikeOrder.ID,
		SpikeEventID:   spikeOrder.SpikeEventID,
		UserID:         userID,
		ProductID:      spikeEvent.ProductID,
		Quantity:       spikeOrder.Quantity,
		Reason:         req.Reason,
		CancelledAt:    time.Now(),
		IdempotencyKey: fmt.Sprintf("cancel_%d_%d", spikeOrder.ID, time.Now().Unix()),
	}

	if err := s.spikeProducer.PublishSpikeOrderCancelled(ctx, data, traceID); err != nil {
		return fmt.Errorf("failed to publish order cancelled message: %w", err)
	}

	// 更新订单状态
	if err := s.spikeOrderRepo.UpdateStatus(orderID, domain.SpikeOrderStatusCancelled); err != nil {
		s.logger.Error("更新订单状态失败", zap.Error(err))
		// 不返回错误，因为消息已经发送，消费者会处理库存恢复
	}

	s.logger.Info("秒杀订单取消成功",
		zap.Int64("order_id", orderID),
		zap.Int64("user_id", userID),
		zap.String("reason", req.Reason))

	return nil
}

// GetActiveEvents 获取活跃的秒杀活动列表
func (s *SpikeService) GetActiveEvents(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error) {
	// 设置查询条件为活跃状态
	active := true
	req.Active = &active

	events, total, err := s.spikeEventRepo.List(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get active events: %w", err)
	}

	// 更新实时库存信息
	for _, event := range events {
		stockInfo, err := s.spikeCache.GetStockInfo(ctx, event.ID)
		if err == nil && stockInfo.Exists && stockInfo.Stock >= 0 {
			event.SpikeStock = stockInfo.Stock
		}
	}

	return &domain.SpikeEventListResponse{
		Events:   events,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// WarmupStock 预热库存（在秒杀开始前调用）
func (s *SpikeService) WarmupStock(ctx context.Context, eventID int64) error {
	spikeEvent, err := s.spikeEventRepo.GetByID(eventID)
	if err != nil {
		return fmt.Errorf("failed to get spike event: %w", err)
	}

	// 预热Redis库存
	remainingStock := spikeEvent.SpikeStock - spikeEvent.SoldCount
	if remainingStock > 0 {
		if err := s.spikeCache.WarmupStock(ctx, eventID, remainingStock, s.config.StockCacheTTL); err != nil {
			return fmt.Errorf("failed to warmup stock: %w", err)
		}
		s.logger.Info("库存预热成功",
			zap.Int64("event_id", eventID),
			zap.Int64("stock", remainingStock))
	}

	return nil
}

// GetSpikeStats 获取秒杀统计信息
func (s *SpikeService) GetSpikeStats(ctx context.Context, eventID int64) (*SpikeStats, error) {
	// 获取秒杀活动
	spikeEvent, err := s.spikeEventRepo.GetByID(eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spike event: %w", err)
	}

	// 获取Redis库存信息
	stockInfo, err := s.spikeCache.GetStockInfo(ctx, eventID)
	if err != nil {
		s.logger.Warn("获取Redis库存信息失败", zap.Error(err))
	}

	// 获取订单统计
	orderStats := make(map[domain.SpikeOrderStatus]int64)
	for _, status := range []domain.SpikeOrderStatus{
		domain.SpikeOrderStatusPending,
		domain.SpikeOrderStatusPaid,
		domain.SpikeOrderStatusCancelled,
		domain.SpikeOrderStatusExpired,
	} {
		count, err := s.spikeOrderRepo.CountByStatus(status)
		if err != nil {
			s.logger.Warn("获取订单统计失败", zap.String("status", string(status)), zap.Error(err))
		} else {
			orderStats[status] = count
		}
	}

	stats := &SpikeStats{
		EventID:        eventID,
		TotalStock:     spikeEvent.SpikeStock,
		SoldCount:      spikeEvent.SoldCount,
		RemainingStock: spikeEvent.SpikeStock - spikeEvent.SoldCount,
		OrderStats:     orderStats,
		IsActive:       spikeEvent.IsActive(),
		StartAt:        spikeEvent.StartAt,
		EndAt:          spikeEvent.EndAt,
	}

	// 使用Redis库存信息更新统计
	if stockInfo != nil && stockInfo.Exists {
		stats.RemainingStock = stockInfo.Stock
		stats.SoldOut = stockInfo.SoldOut
	}

	return stats, nil
}

// SpikeStats 秒杀统计信息
type SpikeStats struct {
	EventID        int64                             `json:"event_id"`
	TotalStock     int64                             `json:"total_stock"`
	SoldCount      int64                             `json:"sold_count"`
	RemainingStock int64                             `json:"remaining_stock"`
	SoldOut        bool                              `json:"sold_out"`
	OrderStats     map[domain.SpikeOrderStatus]int64 `json:"order_stats"`
	IsActive       bool                              `json:"is_active"`
	StartAt        time.Time                         `json:"start_at"`
	EndAt          time.Time                         `json:"end_at"`
}
