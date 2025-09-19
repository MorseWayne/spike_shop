package service

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

func TestSpikeService_ParticipateSpike(t *testing.T) {
	// 准备测试数据
	spikeEventRepo := NewMockSpikeEventRepository()
	spikeOrderRepo := NewMockSpikeOrderRepository()
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	userRepo := NewMockUserRepository()
	spikeCache := NewMockSpikeCache()
	spikeProducer := NewMockSpikeProducer()
	globalLimiter := NewMockLimiter(true)
	userLimiter := NewMockLimiter(true)
	logger := zap.NewNop()

	// 创建测试用户
	user := &domain.User{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     domain.UserRoleUser,
		IsActive: true,
	}
	userRepo.Create(user)

	// 创建测试商品
	product := &domain.Product{
		Name:        "Test Product",
		Description: "Test Description",
		Price:       100.0,
		SKU:         "TEST001",
		Brand:       "TestBrand",
		Status:      domain.ProductStatusActive,
	}
	productRepo.Create(product)

	// 创建测试库存
	inventory := &domain.Inventory{
		ProductID:    product.ID,
		Stock:        1000,
		ReorderPoint: 10,
		MaxStock:     2000,
	}
	inventoryRepo.Create(inventory)

	// 创建测试秒杀活动
	now := time.Now()
	spikeEvent := &domain.SpikeEvent{
		ProductID:     product.ID,
		Name:          "Test Spike Event",
		StartAt:       now.Add(-time.Hour), // 1小时前开始
		EndAt:         now.Add(time.Hour),  // 1小时后结束
		OriginalPrice: 100.0,
		SpikePrice:    50.0,
		SpikeStock:    100,
		SoldCount:     0,
		Status:        domain.SpikeEventStatusActive,
	}
	spikeEventRepo.Create(spikeEvent)

	// 预热缓存库存
	spikeCache.WarmupStock(context.Background(), spikeEvent.ID, spikeEvent.GetRemainingStock(), time.Hour)

	// 创建服务
	service := NewSpikeService(
		spikeEventRepo,
		spikeOrderRepo,
		productRepo,
		inventoryRepo,
		userRepo,
		spikeCache,
		spikeProducer,
		globalLimiter,
		userLimiter,
		DefaultSpikeServiceConfig(),
		logger,
	)

	tests := []struct {
		name        string
		userID      int64
		request     *domain.SpikeParticipationRequest
		setupFunc   func()
		wantErr     bool
		wantSuccess bool
	}{
		{
			name:   "successful participation",
			userID: user.ID,
			request: &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       1,
				IdempotencyKey: "test_key_1",
			},
			setupFunc: func() {
				// 重置限流器状态
				globalLimiter.SetShouldAllow(true)
				userLimiter.SetShouldAllow(true)
			},
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:   "rate limited - global limiter",
			userID: user.ID,
			request: &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       1,
				IdempotencyKey: "test_key_2",
			},
			setupFunc: func() {
				globalLimiter.SetShouldAllow(false)
				userLimiter.SetShouldAllow(true)
			},
			wantErr:     false,
			wantSuccess: false,
		},
		{
			name:   "rate limited - user limiter",
			userID: user.ID,
			request: &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       1,
				IdempotencyKey: "test_key_3",
			},
			setupFunc: func() {
				globalLimiter.SetShouldAllow(true)
				userLimiter.SetShouldAllow(false)
			},
			wantErr:     false,
			wantSuccess: false,
		},
		{
			name:   "invalid quantity - zero",
			userID: user.ID,
			request: &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       0,
				IdempotencyKey: "test_key_4",
			},
			setupFunc: func() {
				globalLimiter.SetShouldAllow(true)
				userLimiter.SetShouldAllow(true)
			},
			wantErr:     false,
			wantSuccess: false,
		},
		{
			name:   "invalid quantity - too large",
			userID: user.ID,
			request: &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       15,
				IdempotencyKey: "test_key_5",
			},
			setupFunc: func() {
				globalLimiter.SetShouldAllow(true)
				userLimiter.SetShouldAllow(true)
			},
			wantErr:     false,
			wantSuccess: false,
		},
		{
			name:   "empty idempotency key",
			userID: user.ID,
			request: &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       1,
				IdempotencyKey: "",
			},
			setupFunc: func() {
				globalLimiter.SetShouldAllow(true)
				userLimiter.SetShouldAllow(true)
			},
			wantErr:     false,
			wantSuccess: false,
		},
		{
			name:   "invalid user id",
			userID: 0,
			request: &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       1,
				IdempotencyKey: "test_key_6",
			},
			setupFunc: func() {
				globalLimiter.SetShouldAllow(true)
				userLimiter.SetShouldAllow(true)
			},
			wantErr:     false,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			result, err := service.ParticipateSpike(context.Background(), tt.request, tt.userID)

			if tt.wantErr && err == nil {
				t.Errorf("ParticipateSpike() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ParticipateSpike() unexpected error = %v", err)
			}

			if result != nil && result.Success != tt.wantSuccess {
				t.Errorf("ParticipateSpike() success = %v, want %v", result.Success, tt.wantSuccess)
			}
		})
	}
}

func TestSpikeService_GetSpikeEventDetail(t *testing.T) {
	// 准备测试数据
	spikeEventRepo := NewMockSpikeEventRepository()
	productRepo := newMockProductRepository()
	spikeCache := NewMockSpikeCache()
	logger := zap.NewNop()

	// 创建测试商品
	product := &domain.Product{
		Name:        "Test Product",
		Description: "Test Description",
		Price:       100.0,
		SKU:         "TEST001",
		Brand:       "TestBrand",
		Status:      domain.ProductStatusActive,
	}
	productRepo.Create(product)

	// 创建测试秒杀活动
	now := time.Now()
	spikeEvent := &domain.SpikeEvent{
		ProductID:     product.ID,
		Name:          "Test Spike Event",
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(time.Hour),
		OriginalPrice: 100.0,
		SpikePrice:    50.0,
		SpikeStock:    100,
		SoldCount:     10,
		Status:        domain.SpikeEventStatusActive,
	}
	spikeEventRepo.Create(spikeEvent)

	// 设置缓存数据
	spikeCache.WarmupStock(context.Background(), spikeEvent.ID, 85, time.Hour) // 缓存中显示85库存

	service := NewSpikeService(
		spikeEventRepo,
		nil,
		productRepo,
		nil,
		nil,
		spikeCache,
		nil,
		nil,
		nil,
		DefaultSpikeServiceConfig(),
		logger,
	)

	tests := []struct {
		name      string
		eventID   int64
		wantErr   bool
		wantStock int64
	}{
		{
			name:      "existing event with cache",
			eventID:   spikeEvent.ID,
			wantErr:   false,
			wantStock: 85, // 应该使用缓存中的库存
		},
		{
			name:    "non-existing event",
			eventID: 999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetSpikeEventDetail(context.Background(), tt.eventID)

			if tt.wantErr && err == nil {
				t.Errorf("GetSpikeEventDetail() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("GetSpikeEventDetail() unexpected error = %v", err)
			}

			if !tt.wantErr && result != nil {
				if result.SpikeEvent.GetRemainingStock() != tt.wantStock {
					t.Errorf("GetSpikeEventDetail() stock = %d, want %d", result.SpikeEvent.GetRemainingStock(), tt.wantStock)
				}
				if result.Product == nil {
					t.Errorf("GetSpikeEventDetail() product should not be nil")
				}
			}
		})
	}
}

func TestSpikeService_GetActiveEvents(t *testing.T) {
	spikeEventRepo := NewMockSpikeEventRepository()
	spikeCache := NewMockSpikeCache()
	logger := zap.NewNop()

	// 创建测试活动
	now := time.Now()

	// 活跃活动
	activeEvent := &domain.SpikeEvent{
		ProductID:     1,
		Name:          "Active Event",
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(time.Hour),
		OriginalPrice: 100.0,
		SpikePrice:    50.0,
		SpikeStock:    100,
		SoldCount:     20,
		Status:        domain.SpikeEventStatusActive,
	}
	spikeEventRepo.Create(activeEvent)

	// 未开始活动
	pendingEvent := &domain.SpikeEvent{
		ProductID:     2,
		Name:          "Pending Event",
		StartAt:       now.Add(time.Hour),
		EndAt:         now.Add(2 * time.Hour),
		OriginalPrice: 200.0,
		SpikePrice:    100.0,
		SpikeStock:    50,
		SoldCount:     0,
		Status:        domain.SpikeEventStatusPending,
	}
	spikeEventRepo.Create(pendingEvent)

	service := NewSpikeService(
		spikeEventRepo,
		nil,
		nil,
		nil,
		nil,
		spikeCache,
		nil,
		nil,
		nil,
		DefaultSpikeServiceConfig(),
		logger,
	)

	req := &domain.SpikeEventListRequest{
		Page:     1,
		PageSize: 10,
	}

	result, err := service.GetActiveEvents(context.Background(), req)
	if err != nil {
		t.Errorf("GetActiveEvents() unexpected error = %v", err)
	}

	if result == nil {
		t.Fatal("GetActiveEvents() result should not be nil")
	}

	// 应该只返回活跃的活动
	if len(result.Events) != 1 {
		t.Errorf("GetActiveEvents() expected 1 active event, got %d", len(result.Events))
	}

	if len(result.Events) > 0 && result.Events[0].ID != activeEvent.ID {
		t.Errorf("GetActiveEvents() expected active event ID %d, got %d", activeEvent.ID, result.Events[0].ID)
	}
}

func TestSpikeService_CancelSpikeOrder(t *testing.T) {
	spikeOrderRepo := NewMockSpikeOrderRepository()
	spikeEventRepo := NewMockSpikeEventRepository()
	spikeProducer := NewMockSpikeProducer()
	logger := zap.NewNop()

	// 创建测试秒杀活动
	spikeEvent := &domain.SpikeEvent{
		ProductID:     1,
		Name:          "Test Event",
		StartAt:       time.Now().Add(-time.Hour),
		EndAt:         time.Now().Add(time.Hour),
		OriginalPrice: 100.0,
		SpikePrice:    50.0,
		SpikeStock:    100,
		SoldCount:     20,
		Status:        domain.SpikeEventStatusActive,
	}
	spikeEventRepo.Create(spikeEvent)

	// 创建测试订单
	spikeOrder := &domain.SpikeOrder{
		SpikeEventID:   spikeEvent.ID,
		UserID:         1,
		Quantity:       1,
		SpikePrice:     50.0,
		TotalAmount:    50.0,
		Status:         domain.SpikeOrderStatusPending,
		IdempotencyKey: "test_order_1",
	}
	spikeOrderRepo.Create(spikeOrder)

	service := NewSpikeService(
		spikeEventRepo,
		spikeOrderRepo,
		nil,
		nil,
		nil,
		nil,
		spikeProducer,
		nil,
		nil,
		DefaultSpikeServiceConfig(),
		logger,
	)

	tests := []struct {
		name    string
		orderID int64
		userID  int64
		request *domain.CancelSpikeOrderRequest
		wantErr bool
	}{
		{
			name:    "successful cancellation",
			orderID: spikeOrder.ID,
			userID:  spikeOrder.UserID,
			request: &domain.CancelSpikeOrderRequest{
				Reason: "不想要了",
			},
			wantErr: false,
		},
		{
			name:    "order not found",
			orderID: 999,
			userID:  1,
			request: &domain.CancelSpikeOrderRequest{
				Reason: "不想要了",
			},
			wantErr: true,
		},
		{
			name:    "wrong user",
			orderID: spikeOrder.ID,
			userID:  999,
			request: &domain.CancelSpikeOrderRequest{
				Reason: "不想要了",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.CancelSpikeOrder(context.Background(), tt.orderID, tt.userID, tt.request)

			if tt.wantErr && err == nil {
				t.Errorf("CancelSpikeOrder() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("CancelSpikeOrder() unexpected error = %v", err)
			}

			// 验证消息是否发送
			if !tt.wantErr {
				messages := spikeProducer.GetPublishedMessages()
				if len(messages) == 0 {
					t.Errorf("CancelSpikeOrder() expected message to be sent but none found")
				}
			}
		})
	}
}

func TestSpikeService_GetSpikeStats(t *testing.T) {
	spikeEventRepo := NewMockSpikeEventRepository()
	spikeOrderRepo := NewMockSpikeOrderRepository()
	spikeCache := NewMockSpikeCache()
	logger := zap.NewNop()

	// 创建测试活动
	spikeEvent := &domain.SpikeEvent{
		ProductID:     1,
		Name:          "Test Event",
		StartAt:       time.Now().Add(-time.Hour),
		EndAt:         time.Now().Add(time.Hour),
		OriginalPrice: 100.0,
		SpikePrice:    50.0,
		SpikeStock:    100,
		SoldCount:     30,
		Status:        domain.SpikeEventStatusActive,
	}
	spikeEventRepo.Create(spikeEvent)

	// 创建测试订单
	pendingOrder := &domain.SpikeOrder{
		SpikeEventID: spikeEvent.ID,
		UserID:       1,
		Status:       domain.SpikeOrderStatusPending,
	}
	paidOrder := &domain.SpikeOrder{
		SpikeEventID: spikeEvent.ID,
		UserID:       2,
		Status:       domain.SpikeOrderStatusPaid,
	}
	spikeOrderRepo.Create(pendingOrder)
	spikeOrderRepo.Create(paidOrder)

	// 设置缓存库存
	spikeCache.WarmupStock(context.Background(), spikeEvent.ID, 65, time.Hour)

	service := NewSpikeService(
		spikeEventRepo,
		spikeOrderRepo,
		nil,
		nil,
		nil,
		spikeCache,
		nil,
		nil,
		nil,
		DefaultSpikeServiceConfig(),
		logger,
	)

	stats, err := service.GetSpikeStats(context.Background(), spikeEvent.ID)
	if err != nil {
		t.Errorf("GetSpikeStats() unexpected error = %v", err)
	}

	if stats == nil {
		t.Fatal("GetSpikeStats() result should not be nil")
	}

	// 验证统计数据
	if stats.EventID != spikeEvent.ID {
		t.Errorf("GetSpikeStats() event ID = %d, want %d", stats.EventID, spikeEvent.ID)
	}

	if stats.TotalStock != spikeEvent.SpikeStock {
		t.Errorf("GetSpikeStats() total stock = %d, want %d", stats.TotalStock, spikeEvent.SpikeStock)
	}

	// 应该使用缓存中的库存数据
	if stats.RemainingStock != 65 {
		t.Errorf("GetSpikeStats() remaining stock = %d, want 65", stats.RemainingStock)
	}

	if !stats.IsActive {
		t.Errorf("GetSpikeStats() is active = %v, want true", stats.IsActive)
	}

	// 验证订单统计
	if stats.OrderStats[domain.SpikeOrderStatusPending] != 1 {
		t.Errorf("GetSpikeStats() pending orders = %d, want 1", stats.OrderStats[domain.SpikeOrderStatusPending])
	}

	if stats.OrderStats[domain.SpikeOrderStatusPaid] != 1 {
		t.Errorf("GetSpikeStats() paid orders = %d, want 1", stats.OrderStats[domain.SpikeOrderStatusPaid])
	}
}

func TestSpikeService_WarmupStock(t *testing.T) {
	spikeEventRepo := NewMockSpikeEventRepository()
	spikeCache := NewMockSpikeCache()
	logger := zap.NewNop()

	// 创建测试活动
	spikeEvent := &domain.SpikeEvent{
		ProductID:     1,
		Name:          "Test Event",
		StartAt:       time.Now().Add(time.Hour),
		EndAt:         time.Now().Add(2 * time.Hour),
		OriginalPrice: 100.0,
		SpikePrice:    50.0,
		SpikeStock:    100,
		SoldCount:     10,
		Status:        domain.SpikeEventStatusPending,
	}
	spikeEventRepo.Create(spikeEvent)

	service := NewSpikeService(
		spikeEventRepo,
		nil,
		nil,
		nil,
		nil,
		spikeCache,
		nil,
		nil,
		nil,
		DefaultSpikeServiceConfig(),
		logger,
	)

	err := service.WarmupStock(context.Background(), spikeEvent.ID)
	if err != nil {
		t.Errorf("WarmupStock() unexpected error = %v", err)
	}

	// 验证缓存中的库存
	stockInfo, err := spikeCache.GetStockInfo(context.Background(), spikeEvent.ID)
	if err != nil {
		t.Errorf("GetStockInfo() unexpected error = %v", err)
	}

	expectedStock := spikeEvent.GetRemainingStock()
	if stockInfo.Stock != expectedStock {
		t.Errorf("WarmupStock() cached stock = %d, want %d", stockInfo.Stock, expectedStock)
	}

	if stockInfo.SoldOut {
		t.Errorf("WarmupStock() sold out should be false")
	}
}

// 测试并发安全性
func TestSpikeService_ConcurrentParticipation(t *testing.T) {
	spikeEventRepo := NewMockSpikeEventRepository()
	spikeOrderRepo := NewMockSpikeOrderRepository()
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	userRepo := NewMockUserRepository()
	spikeCache := NewMockSpikeCache()
	spikeProducer := NewMockSpikeProducer()
	globalLimiter := NewMockLimiter(true)
	userLimiter := NewMockLimiter(true)
	logger := zap.NewNop()

	// 准备测试数据
	user := &domain.User{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     domain.UserRoleUser,
		IsActive: true,
	}
	userRepo.Create(user)

	product := &domain.Product{
		Name:   "Test Product",
		SKU:    "TEST001",
		Status: domain.ProductStatusActive,
	}
	productRepo.Create(product)

	spikeEvent := &domain.SpikeEvent{
		ProductID:     product.ID,
		Name:          "Test Event",
		StartAt:       time.Now().Add(-time.Hour),
		EndAt:         time.Now().Add(time.Hour),
		OriginalPrice: 100.0,
		SpikePrice:    50.0,
		SpikeStock:    10, // 少量库存
		SoldCount:     0,
		Status:        domain.SpikeEventStatusActive,
	}
	spikeEventRepo.Create(spikeEvent)

	// 只预热10个库存
	spikeCache.WarmupStock(context.Background(), spikeEvent.ID, 10, time.Hour)

	service := NewSpikeService(
		spikeEventRepo,
		spikeOrderRepo,
		productRepo,
		inventoryRepo,
		userRepo,
		spikeCache,
		spikeProducer,
		globalLimiter,
		userLimiter,
		DefaultSpikeServiceConfig(),
		logger,
	)

	// 创建多个不同的用户
	users := make([]*domain.User, 20)
	for i := 0; i < 20; i++ {
		user := &domain.User{
			Username: "user" + string(rune('0'+i)),
			Email:    "user" + string(rune('0'+i)) + "@example.com",
			Role:     domain.UserRoleUser,
			IsActive: true,
		}
		userRepo.Create(user)
		users[i] = user
	}

	// 并发请求
	const concurrency = 20
	results := make(chan *domain.SpikeParticipationResponse, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(userIndex int) {
			req := &domain.SpikeParticipationRequest{
				SpikeEventID:   spikeEvent.ID,
				Quantity:       1,
				IdempotencyKey: "concurrent_test_" + string(rune('0'+userIndex)),
			}

			result, err := service.ParticipateSpike(context.Background(), req, users[userIndex].ID)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(i)
	}

	// 收集结果
	successCount := 0
	for i := 0; i < concurrency; i++ {
		select {
		case result := <-results:
			if result.Success {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent participation error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Test timeout")
		}
	}

	// 验证只有10个成功（等于库存数量）
	if successCount > 10 {
		t.Errorf("Too many successful participations: %d, expected max 10", successCount)
	}

	t.Logf("Successful participations: %d/20", successCount)
}
