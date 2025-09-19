// Package cache 提供秒杀相关的Redis缓存操作和Lua脚本
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SpikeCache 秒杀缓存服务
type SpikeCache struct {
	client redis.Cmdable
}

// NewSpikeCache 创建秒杀缓存实例
func NewSpikeCache(client redis.Cmdable) *SpikeCache {
	return &SpikeCache{
		client: client,
	}
}

// Redis Key 模板常量
const (
	// 秒杀活动库存Key: spike:stock:{event_id}
	SpikeStockKeyTemplate = "spike:stock:%d"

	// 秒杀活动售罄标记Key: spike:sold_out:{event_id}
	SpikeSoldOutKeyTemplate = "spike:sold_out:%d"

	// 用户参与秒杀去重Key: spike:user:{user_id}:{event_id}
	SpikeUserKeyTemplate = "spike:user:%d:%d"

	// 秒杀活动信息缓存Key: spike:event:{event_id}
	SpikeEventKeyTemplate = "spike:event:%d"

	// 幂等键缓存Key: spike:idempotency:{key}
	SpikeIdempotencyKeyTemplate = "spike:idempotency:%s"
)

// Lua脚本：原子性预减库存
const luaDecrementStock = `
-- KEYS[1]: 库存key (spike:stock:{event_id})
-- KEYS[2]: 售罄标记key (spike:sold_out:{event_id})
-- KEYS[3]: 用户去重key (spike:user:{user_id}:{event_id})
-- ARGV[1]: 减少的数量
-- ARGV[2]: 用户去重TTL（秒）
-- ARGV[3]: 售罄标记TTL（秒）

-- 检查是否已售罄
if redis.call('EXISTS', KEYS[2]) == 1 then
    return {-1, 'sold_out'}  -- 商品已售罄
end

-- 检查用户是否已经参与
if redis.call('EXISTS', KEYS[3]) == 1 then
    return {-2, 'duplicate_user'}  -- 用户重复参与
end

-- 获取当前库存
local current_stock = redis.call('GET', KEYS[1])
if current_stock == false then
    return {-3, 'stock_not_found'}  -- 库存不存在
end

current_stock = tonumber(current_stock)
local decrement = tonumber(ARGV[1])

-- 检查库存是否足够
if current_stock < decrement then
    -- 库存不足，设置售罄标记
    redis.call('SETEX', KEYS[2], tonumber(ARGV[3]), '1')
    return {-4, 'insufficient_stock'}
end

-- 减少库存
local new_stock = redis.call('DECRBY', KEYS[1], decrement)

-- 设置用户去重标记
redis.call('SETEX', KEYS[3], tonumber(ARGV[2]), '1')

-- 如果库存为0，设置售罄标记
if new_stock <= 0 then
    redis.call('SETEX', KEYS[2], tonumber(ARGV[3]), '1')
end

return {new_stock, 'success'}
`

// Lua脚本：批量检查库存状态
const luaCheckStockBatch = `
-- KEYS: 多个库存key
-- 返回: 每个key对应的库存数量，不存在返回-1

local result = {}
for i = 1, #KEYS do
    local stock = redis.call('GET', KEYS[i])
    if stock == false then
        result[i] = -1
    else
        result[i] = tonumber(stock)
    end
end
return result
`

// Lua脚本：恢复库存（用于订单取消/过期）
const luaRestoreStock = `
-- KEYS[1]: 库存key
-- KEYS[2]: 售罄标记key
-- KEYS[3]: 用户去重key
-- ARGV[1]: 恢复的数量

-- 增加库存
local new_stock = redis.call('INCRBY', KEYS[1], tonumber(ARGV[1]))

-- 删除售罄标记（如果存在）
redis.call('DEL', KEYS[2])

-- 删除用户去重标记（如果存在）
redis.call('DEL', KEYS[3])

return new_stock
`

// DecrementStockResult 预减库存结果
type DecrementStockResult struct {
	Success        bool   `json:"success"`
	RemainingStock int64  `json:"remaining_stock"`
	Message        string `json:"message"`
}

// 生成Redis Key的辅助函数
func (s *SpikeCache) getStockKey(eventID int64) string {
	return fmt.Sprintf(SpikeStockKeyTemplate, eventID)
}

func (s *SpikeCache) getSoldOutKey(eventID int64) string {
	return fmt.Sprintf(SpikeSoldOutKeyTemplate, eventID)
}

func (s *SpikeCache) getUserKey(userID, eventID int64) string {
	return fmt.Sprintf(SpikeUserKeyTemplate, userID, eventID)
}

func (s *SpikeCache) getEventKey(eventID int64) string {
	return fmt.Sprintf(SpikeEventKeyTemplate, eventID)
}

func (s *SpikeCache) getIdempotencyKey(key string) string {
	return fmt.Sprintf(SpikeIdempotencyKeyTemplate, key)
}

// InitStock 初始化秒杀活动库存
func (s *SpikeCache) InitStock(ctx context.Context, eventID int64, stock int64, ttl time.Duration) error {
	key := s.getStockKey(eventID)

	err := s.client.Set(ctx, key, stock, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to init spike stock: %w", err)
	}

	return nil
}

// GetStock 获取当前库存
func (s *SpikeCache) GetStock(ctx context.Context, eventID int64) (int64, error) {
	key := s.getStockKey(eventID)

	result := s.client.Get(ctx, key)
	if result.Err() == redis.Nil {
		return -1, nil // 库存不存在
	}
	if result.Err() != nil {
		return 0, fmt.Errorf("failed to get spike stock: %w", result.Err())
	}

	stock, err := result.Int64()
	if err != nil {
		return 0, fmt.Errorf("failed to parse stock value: %w", err)
	}

	return stock, nil
}

// IsSoldOut 检查是否已售罄
func (s *SpikeCache) IsSoldOut(ctx context.Context, eventID int64) (bool, error) {
	key := s.getSoldOutKey(eventID)

	result := s.client.Exists(ctx, key)
	if result.Err() != nil {
		return false, fmt.Errorf("failed to check sold out status: %w", result.Err())
	}

	return result.Val() > 0, nil
}

// IsUserParticipated 检查用户是否已参与
func (s *SpikeCache) IsUserParticipated(ctx context.Context, userID, eventID int64) (bool, error) {
	key := s.getUserKey(userID, eventID)

	result := s.client.Exists(ctx, key)
	if result.Err() != nil {
		return false, fmt.Errorf("failed to check user participation: %w", result.Err())
	}

	return result.Val() > 0, nil
}

// DecrementStock 原子性预减库存（核心方法）
func (s *SpikeCache) DecrementStock(ctx context.Context, eventID, userID, quantity int64, userTTL, soldOutTTL time.Duration) (*DecrementStockResult, error) {
	stockKey := s.getStockKey(eventID)
	soldOutKey := s.getSoldOutKey(eventID)
	userKey := s.getUserKey(userID, eventID)

	// 执行Lua脚本
	result := s.client.Eval(ctx, luaDecrementStock,
		[]string{stockKey, soldOutKey, userKey},
		quantity, int(userTTL.Seconds()), int(soldOutTTL.Seconds()))

	if result.Err() != nil {
		return nil, fmt.Errorf("failed to execute decrement stock script: %w", result.Err())
	}

	// 解析结果
	values, ok := result.Val().([]interface{})
	if !ok || len(values) != 2 {
		return nil, fmt.Errorf("unexpected script result format")
	}

	stockValue, ok := values[0].(int64)
	if !ok {
		return nil, fmt.Errorf("unexpected stock value type")
	}

	_, ok = values[1].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected message type")
	}

	switch stockValue {
	case -1:
		return &DecrementStockResult{
			Success:        false,
			RemainingStock: 0,
			Message:        "商品已售罄",
		}, nil
	case -2:
		return &DecrementStockResult{
			Success:        false,
			RemainingStock: 0,
			Message:        "用户重复参与",
		}, nil
	case -3:
		return &DecrementStockResult{
			Success:        false,
			RemainingStock: 0,
			Message:        "库存信息不存在",
		}, nil
	case -4:
		return &DecrementStockResult{
			Success:        false,
			RemainingStock: 0,
			Message:        "库存不足",
		}, nil
	default:
		return &DecrementStockResult{
			Success:        true,
			RemainingStock: stockValue,
			Message:        "预减库存成功",
		}, nil
	}
}

// RestoreStock 恢复库存（用于订单取消/过期）
func (s *SpikeCache) RestoreStock(ctx context.Context, eventID, userID, quantity int64) (int64, error) {
	stockKey := s.getStockKey(eventID)
	soldOutKey := s.getSoldOutKey(eventID)
	userKey := s.getUserKey(userID, eventID)

	result := s.client.Eval(ctx, luaRestoreStock,
		[]string{stockKey, soldOutKey, userKey},
		quantity)

	if result.Err() != nil {
		return 0, fmt.Errorf("failed to execute restore stock script: %w", result.Err())
	}

	newStock, ok := result.Val().(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected script result type")
	}

	return newStock, nil
}

// BatchCheckStock 批量检查多个活动的库存状态
func (s *SpikeCache) BatchCheckStock(ctx context.Context, eventIDs []int64) (map[int64]int64, error) {
	if len(eventIDs) == 0 {
		return make(map[int64]int64), nil
	}

	keys := make([]string, len(eventIDs))
	for i, eventID := range eventIDs {
		keys[i] = s.getStockKey(eventID)
	}

	result := s.client.Eval(ctx, luaCheckStockBatch, keys)
	if result.Err() != nil {
		return nil, fmt.Errorf("failed to execute batch check stock script: %w", result.Err())
	}

	values, ok := result.Val().([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected script result format")
	}

	stockMap := make(map[int64]int64)
	for i, value := range values {
		stock, ok := value.(int64)
		if !ok {
			return nil, fmt.Errorf("unexpected stock value type at index %d", i)
		}
		stockMap[eventIDs[i]] = stock
	}

	return stockMap, nil
}

// SetUserParticipation 设置用户参与标记
func (s *SpikeCache) SetUserParticipation(ctx context.Context, userID, eventID int64, ttl time.Duration) error {
	key := s.getUserKey(userID, eventID)

	err := s.client.Set(ctx, key, "1", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set user participation: %w", err)
	}

	return nil
}

// RemoveUserParticipation 移除用户参与标记（用于订单取消）
func (s *SpikeCache) RemoveUserParticipation(ctx context.Context, userID, eventID int64) error {
	key := s.getUserKey(userID, eventID)

	err := s.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to remove user participation: %w", err)
	}

	return nil
}

// SetIdempotencyKey 设置幂等键
func (s *SpikeCache) SetIdempotencyKey(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	cacheKey := s.getIdempotencyKey(key)

	result := s.client.SetNX(ctx, cacheKey, value, ttl)
	if result.Err() != nil {
		return false, fmt.Errorf("failed to set idempotency key: %w", result.Err())
	}

	return result.Val(), nil
}

// GetIdempotencyKey 获取幂等键对应的值
func (s *SpikeCache) GetIdempotencyKey(ctx context.Context, key string, dest interface{}) error {
	cacheKey := s.getIdempotencyKey(key)

	result := s.client.Get(ctx, cacheKey)
	if result.Err() == redis.Nil {
		return fmt.Errorf("idempotency key not found")
	}
	if result.Err() != nil {
		return fmt.Errorf("failed to get idempotency key: %w", result.Err())
	}

	return result.Scan(dest)
}

// DeleteIdempotencyKey 删除幂等键
func (s *SpikeCache) DeleteIdempotencyKey(ctx context.Context, key string) error {
	cacheKey := s.getIdempotencyKey(key)

	err := s.client.Del(ctx, cacheKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete idempotency key: %w", err)
	}

	return nil
}

// CacheEventInfo 缓存秒杀活动信息
func (s *SpikeCache) CacheEventInfo(ctx context.Context, eventID int64, eventData interface{}, ttl time.Duration) error {
	key := s.getEventKey(eventID)

	err := s.client.Set(ctx, key, eventData, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to cache event info: %w", err)
	}

	return nil
}

// GetEventInfo 获取缓存的秒杀活动信息
func (s *SpikeCache) GetEventInfo(ctx context.Context, eventID int64, dest interface{}) error {
	key := s.getEventKey(eventID)

	result := s.client.Get(ctx, key)
	if result.Err() == redis.Nil {
		return fmt.Errorf("event info not found")
	}
	if result.Err() != nil {
		return fmt.Errorf("failed to get event info: %w", result.Err())
	}

	return result.Scan(dest)
}

// WarmupStock 预热库存（在秒杀开始前调用）
func (s *SpikeCache) WarmupStock(ctx context.Context, eventID int64, stock int64, ttl time.Duration) error {
	// 预热库存
	if err := s.InitStock(ctx, eventID, stock, ttl); err != nil {
		return fmt.Errorf("failed to warmup stock: %w", err)
	}

	// 清除可能存在的售罄标记
	soldOutKey := s.getSoldOutKey(eventID)
	if err := s.client.Del(ctx, soldOutKey).Err(); err != nil {
		return fmt.Errorf("failed to clear sold out flag: %w", err)
	}

	return nil
}

// GetStockInfo 获取库存综合信息
type StockInfo struct {
	Stock   int64 `json:"stock"`
	SoldOut bool  `json:"sold_out"`
	Exists  bool  `json:"exists"`
}

func (s *SpikeCache) GetStockInfo(ctx context.Context, eventID int64) (*StockInfo, error) {
	stockKey := s.getStockKey(eventID)
	soldOutKey := s.getSoldOutKey(eventID)

	// 使用Pipeline批量执行
	pipe := s.client.Pipeline()
	stockCmd := pipe.Get(ctx, stockKey)
	soldOutCmd := pipe.Exists(ctx, soldOutKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	info := &StockInfo{}

	// 处理库存信息
	if stockCmd.Err() == redis.Nil {
		info.Stock = -1
		info.Exists = false
	} else if stockCmd.Err() != nil {
		return nil, fmt.Errorf("failed to get stock: %w", stockCmd.Err())
	} else {
		stock, err := stockCmd.Int64()
		if err != nil {
			return nil, fmt.Errorf("failed to parse stock: %w", err)
		}
		info.Stock = stock
		info.Exists = true
	}

	// 处理售罄标记
	if soldOutCmd.Err() != nil {
		return nil, fmt.Errorf("failed to check sold out status: %w", soldOutCmd.Err())
	}
	info.SoldOut = soldOutCmd.Val() > 0

	return info, nil
}
