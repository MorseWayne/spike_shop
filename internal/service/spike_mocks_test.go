package service

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/MorseWayne/spike_shop/internal/cache"
	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/limiter"
	"github.com/MorseWayne/spike_shop/internal/mq"
)

// MockSpikeEventRepository 秒杀活动仓储模拟
type MockSpikeEventRepository struct {
	events map[int64]*domain.SpikeEvent
	nextID int64
	mu     sync.RWMutex
}

func NewMockSpikeEventRepository() *MockSpikeEventRepository {
	return &MockSpikeEventRepository{
		events: make(map[int64]*domain.SpikeEvent),
		nextID: 1,
	}
}

func (m *MockSpikeEventRepository) Create(event *domain.SpikeEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	event.ID = m.nextID
	m.nextID++
	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()

	m.events[event.ID] = event
	return nil
}

func (m *MockSpikeEventRepository) GetByID(id int64) (*domain.SpikeEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event, exists := m.events[id]
	if !exists {
		return nil, nil
	}
	return event, nil
}

func (m *MockSpikeEventRepository) GetByIDWithTx(tx *sql.Tx, id int64) (*domain.SpikeEvent, error) {
	return m.GetByID(id)
}

func (m *MockSpikeEventRepository) UpdateSoldCount(id int64, soldCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	event, exists := m.events[id]
	if !exists {
		return errors.New("event not found")
	}

	event.SoldCount = soldCount
	event.UpdatedAt = time.Now()
	return nil
}

func (m *MockSpikeEventRepository) List(req *domain.SpikeEventListRequest) ([]*domain.SpikeEvent, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []*domain.SpikeEvent
	for _, event := range m.events {
		// 简化筛选逻辑
		if req.Active != nil && *req.Active {
			if event.IsActive() {
				events = append(events, event)
			}
		} else {
			events = append(events, event)
		}
	}

	total := int64(len(events))

	// 简化分页逻辑
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start >= len(events) {
		return []*domain.SpikeEvent{}, total, nil
	}
	if end > len(events) {
		end = len(events)
	}

	return events[start:end], total, nil
}

// MockSpikeOrderRepository 秒杀订单仓储模拟
type MockSpikeOrderRepository struct {
	orders map[int64]*domain.SpikeOrder
	nextID int64
	mu     sync.RWMutex
}

func NewMockSpikeOrderRepository() *MockSpikeOrderRepository {
	return &MockSpikeOrderRepository{
		orders: make(map[int64]*domain.SpikeOrder),
		nextID: 1,
	}
}

func (m *MockSpikeOrderRepository) Create(order *domain.SpikeOrder) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order.ID = m.nextID
	m.nextID++
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	m.orders[order.ID] = order
	return nil
}

func (m *MockSpikeOrderRepository) GetByID(id int64) (*domain.SpikeOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	order, exists := m.orders[id]
	if !exists {
		return nil, nil
	}
	return order, nil
}

func (m *MockSpikeOrderRepository) GetByUserIDAndEventID(userID, eventID int64) (*domain.SpikeOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, order := range m.orders {
		if order.UserID == userID && order.SpikeEventID == eventID {
			return order, nil
		}
	}
	return nil, nil
}

func (m *MockSpikeOrderRepository) GetByUserIDAndEventIDWithTx(tx *sql.Tx, userID, eventID int64) (*domain.SpikeOrder, error) {
	return m.GetByUserIDAndEventID(userID, eventID)
}

func (m *MockSpikeOrderRepository) UpdateStatus(id int64, status domain.SpikeOrderStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orders[id]
	if !exists {
		return errors.New("order not found")
	}

	order.Status = status
	order.UpdatedAt = time.Now()
	return nil
}

func (m *MockSpikeOrderRepository) List(req *domain.SpikeOrderListRequest) ([]*domain.SpikeOrder, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var orders []*domain.SpikeOrder
	for _, order := range m.orders {
		// 简化筛选逻辑
		if req.UserID != nil && order.UserID != *req.UserID {
			continue
		}
		if req.Status != nil && order.Status != *req.Status {
			continue
		}
		orders = append(orders, order)
	}

	total := int64(len(orders))

	// 简化分页逻辑
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start >= len(orders) {
		return []*domain.SpikeOrder{}, total, nil
	}
	if end > len(orders) {
		end = len(orders)
	}

	return orders[start:end], total, nil
}

func (m *MockSpikeOrderRepository) CountByStatus(status domain.SpikeOrderStatus) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := int64(0)
	for _, order := range m.orders {
		if order.Status == status {
			count++
		}
	}
	return count, nil
}

// MockSpikeCache 秒杀缓存模拟
type MockSpikeCache struct {
	stockData     map[int64]int64 // eventID -> stock
	soldOutData   map[int64]bool  // eventID -> soldOut
	userMarkData  map[string]bool // userKey -> marked
	eventData     map[int64]interface{} // eventID -> event data
	processedData map[string]bool  // messageID -> processed
	mu            sync.RWMutex
}

func NewMockSpikeCache() *MockSpikeCache {
	return &MockSpikeCache{
		stockData:     make(map[int64]int64),
		soldOutData:   make(map[int64]bool),
		userMarkData:  make(map[string]bool),
		eventData:     make(map[int64]interface{}),
		processedData: make(map[string]bool),
	}
}

func (m *MockSpikeCache) DecrementStock(ctx context.Context, eventID, userID, quantity int64, userTTL, stockTTL time.Duration) (*cache.StockDecrementResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	userKey := cache.GetSpikeUserKey(userID, eventID)

	// 检查售罄标记
	if m.soldOutData[eventID] {
		return &cache.StockDecrementResult{
			Success:        false,
			Message:        "商品已售罄",
			RemainingStock: 0,
		}, nil
	}

	// 检查用户去重
	if m.userMarkData[userKey] {
		return &cache.StockDecrementResult{
			Success:        false,
			Message:        "用户已参与该活动",
			RemainingStock: m.stockData[eventID],
		}, nil
	}

	// 检查库存
	currentStock := m.stockData[eventID]
	if currentStock < quantity {
		m.soldOutData[eventID] = true
		return &cache.StockDecrementResult{
			Success:        false,
			Message:        "库存不足",
			RemainingStock: 0,
		}, nil
	}

	// 执行扣减
	newStock := currentStock - quantity
	m.stockData[eventID] = newStock
	m.userMarkData[userKey] = true

	if newStock == 0 {
		m.soldOutData[eventID] = true
	}

	return &cache.StockDecrementResult{
		Success:        true,
		Message:        "扣减成功",
		RemainingStock: newStock,
	}, nil
}

func (m *MockSpikeCache) RestoreStock(ctx context.Context, eventID, userID, quantity int64) (*cache.StockRestoreResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentStock := m.stockData[eventID]
	newStock := currentStock + quantity
	m.stockData[eventID] = newStock

	// 如果恢复库存后不再售罄，移除售罄标记
	if m.soldOutData[eventID] && newStock > 0 {
		m.soldOutData[eventID] = false
	}

	userKey := cache.GetSpikeUserKey(userID, eventID)
	delete(m.userMarkData, userKey)

	return &cache.StockRestoreResult{
		Success:        true,
		Message:        "库存恢复成功",
		RemainingStock: newStock,
	}, nil
}

func (m *MockSpikeCache) GetStockInfo(ctx context.Context, eventID int64) (*cache.StockInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &cache.StockInfo{
		Stock:   m.stockData[eventID],
		SoldOut: m.soldOutData[eventID],
		Exists:  true,
	}, nil
}

func (m *MockSpikeCache) WarmupStock(ctx context.Context, eventID, stock int64, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stockData[eventID] = stock
	m.soldOutData[eventID] = false
	return nil
}

func (m *MockSpikeCache) CacheEventInfo(ctx context.Context, eventID int64, event interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.eventData[eventID] = event
	return nil
}

func (m *MockSpikeCache) GetEventInfo(ctx context.Context, eventID int64, dest interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.eventData[eventID]
	if !exists {
		return errors.New("not found")
	}

	// 简化实现，直接赋值
	if event, ok := dest.(*domain.SpikeEvent); ok {
		if srcEvent, ok := data.(*domain.SpikeEvent); ok {
			*event = *srcEvent
		}
	}

	return nil
}

func (m *MockSpikeCache) CheckAndSetMessageProcessed(ctx context.Context, messageID, idempotencyKey string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := messageID + ":" + idempotencyKey
	if m.processedData[key] {
		return true, nil
	}

	m.processedData[key] = true
	return false, nil
}

// MockSpikeProducer 秒杀消息生产者模拟
type MockSpikeProducer struct {
	publishedMessages []interface{}
	shouldFail        bool
	mu                sync.Mutex
}

func NewMockSpikeProducer() *MockSpikeProducer {
	return &MockSpikeProducer{
		publishedMessages: make([]interface{}, 0),
	}
}

func (m *MockSpikeProducer) SetShouldFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
}

func (m *MockSpikeProducer) GetPublishedMessages() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]interface{}{}, m.publishedMessages...)
}

func (m *MockSpikeProducer) PublishSpikeOrderCreated(ctx context.Context, data *mq.SpikeOrderCreatedData, traceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return errors.New("mock publish failed")
	}

	m.publishedMessages = append(m.publishedMessages, data)
	return nil
}

func (m *MockSpikeProducer) PublishSpikeOrderCancelled(ctx context.Context, data *mq.SpikeOrderCancelledData, traceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return errors.New("mock publish failed")
	}

	m.publishedMessages = append(m.publishedMessages, data)
	return nil
}

func (m *MockSpikeProducer) PublishSpikeOrderExpired(ctx context.Context, data *mq.SpikeOrderExpiredData, traceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return errors.New("mock publish failed")
	}

	m.publishedMessages = append(m.publishedMessages, data)
	return nil
}

func (m *MockSpikeProducer) PublishStockRestore(ctx context.Context, data *mq.StockRestoreData, traceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return errors.New("mock publish failed")
	}

	m.publishedMessages = append(m.publishedMessages, data)
	return nil
}

// MockLimiter 限流器模拟
type MockLimiter struct {
	shouldAllow bool
	mu          sync.Mutex
}

func NewMockLimiter(shouldAllow bool) *MockLimiter {
	return &MockLimiter{
		shouldAllow: shouldAllow,
	}
}

func (m *MockLimiter) SetShouldAllow(allow bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldAllow = allow
}

func (m *MockLimiter) Allow(ctx context.Context, key string) (*limiter.LimitResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return &limiter.LimitResult{
		Allowed:       m.shouldAllow,
		Remaining:     100,
		RetryAfter:    time.Second,
		TotalRequests: 1,
	}, nil
}

func (m *MockLimiter) AllowN(ctx context.Context, key string, n int64) (*limiter.LimitResult, error) {
	return m.Allow(ctx, key)
}

func (m *MockLimiter) Reset(ctx context.Context, key string) error {
	return nil
}

func (m *MockLimiter) GetInfo(ctx context.Context, key string) (*limiter.LimitInfo, error) {
	return &limiter.LimitInfo{
		Limit:     100,
		Remaining: 99,
		Window:    time.Minute,
		ResetTime: time.Now().Add(time.Minute),
	}, nil
}

// MockRedisClient Redis客户端模拟
type MockRedisClient struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]interface{}),
	}
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd := redis.NewStringCmd(ctx, "get", key)
	if val, exists := m.data[key]; exists {
		if str, ok := val.(string); ok {
			cmd.SetVal(str)
		}
	} else {
		cmd.SetErr(redis.Nil)
	}
	return cmd
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
	cmd := redis.NewStatusCmd(ctx, "set", key, value)
	cmd.SetVal("OK")
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			delete(m.data, key)
			count++
		}
	}

	cmd := redis.NewIntCmd(ctx, "del")
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			count++
		}
	}

	cmd := redis.NewIntCmd(ctx, "exists")
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "ping")
	cmd.SetVal("PONG")
	return cmd
}

func (m *MockRedisClient) Close() error {
	return nil
}

// 实现redis.Cmdable接口的其他必需方法（简化实现）
func (m *MockRedisClient) Pipeline() redis.Pipeliner                                          { return nil }
func (m *MockRedisClient) Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) { return nil, nil }
func (m *MockRedisClient) TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) { return nil, nil }
func (m *MockRedisClient) TxPipeline() redis.Pipeliner                                       { return nil }
func (m *MockRedisClient) Command(ctx context.Context) *redis.CommandsInfoCmd                { return nil }
func (m *MockRedisClient) CommandList(ctx context.Context, filter *redis.FilterBy) *redis.StringSliceCmd { return nil }
func (m *MockRedisClient) CommandGetKeys(ctx context.Context, commands ...interface{}) *redis.StringSliceCmd { return nil }
func (m *MockRedisClient) CommandGetKeysAndFlags(ctx context.Context, commands ...interface{}) *redis.KeyFlagsCmd { return nil }
func (m *MockRedisClient) ClientGetName(ctx context.Context) *redis.StringCmd               { return nil }
func (m *MockRedisClient) Echo(ctx context.Context, message interface{}) *redis.StringCmd  { return nil }
func (m *MockRedisClient) Hello(ctx context.Context, ver int, username, password, clientName string) *redis.MapStringInterfaceCmd { return nil }
func (m *MockRedisClient) Select(ctx context.Context, index int) *redis.StatusCmd          { return nil }
func (m *MockRedisClient) SwapDB(ctx context.Context, index1, index2 int) *redis.StatusCmd { return nil }
func (m *MockRedisClient) ClientID(ctx context.Context) *redis.IntCmd                       { return nil }

// 其他必需方法的空实现...
func (m *MockRedisClient) SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return m.Set(ctx, key, value, expiration)
}
func (m *MockRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx, "setnx", key, value)
	cmd.SetVal(true)
	return cmd
}
func (m *MockRedisClient) GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd {
	return m.Get(ctx, key)
}
func (m *MockRedisClient) GetDel(ctx context.Context, key string) *redis.StringCmd {
	cmd := m.Get(ctx, key)
	m.Del(ctx, key)
	return cmd
}

// 更多方法的空实现，保持简单...
func (m *MockRedisClient) Append(ctx context.Context, key, value string) *redis.IntCmd { return nil }
func (m *MockRedisClient) Decr(ctx context.Context, key string) *redis.IntCmd          { return nil }
func (m *MockRedisClient) DecrBy(ctx context.Context, key string, decrement int64) *redis.IntCmd { return nil }
func (m *MockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd          { return redis.NewIntCmd(ctx, "incr", key) }
func (m *MockRedisClient) IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd { return redis.NewIntCmd(ctx, "incrby", key, value) }
func (m *MockRedisClient) IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd { return nil }
