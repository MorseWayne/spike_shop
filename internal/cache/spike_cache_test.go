package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Helper函数
func GetSpikeStockKey(eventID int64) string {
	return fmt.Sprintf("spike:stock:%d", eventID)
}

func GetSpikeSoldOutKey(eventID int64) string {
	return fmt.Sprintf("spike:sold_out:%d", eventID)
}

func GetSpikeUserKey(userID, eventID int64) string {
	return fmt.Sprintf("spike:user:%d:%d", userID, eventID)
}

func GetSpikeEventKey(eventID int64) string {
	return fmt.Sprintf("spike:event:%d", eventID)
}

// SimpleMockClient 简化的Mock Redis客户端，只实现必要的方法
type SimpleMockClient struct {
	data map[string]interface{}
}

func NewSimpleMockClient() *SimpleMockClient {
	return &SimpleMockClient{
		data: make(map[string]interface{}),
	}
}

func (m *SimpleMockClient) Get(ctx context.Context, key string) *redis.StringCmd {
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

func (m *SimpleMockClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.data[key] = value
	cmd := redis.NewStatusCmd(ctx, "set", key, value)
	cmd.SetVal("OK")
	return cmd
}

func (m *SimpleMockClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx, "setnx", key, value)
	_, exists := m.data[key]
	if !exists {
		m.data[key] = value
		cmd.SetVal(true)
	} else {
		cmd.SetVal(false)
	}
	return cmd
}

func (m *SimpleMockClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
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

func (m *SimpleMockClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
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

func (m *SimpleMockClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx, "eval", script)
	// 模拟不同脚本的返回值
	if len(args) > 0 {
		// 默认返回成功结果 [剩余库存, 成功标志, 消息]
		result := []interface{}{int64(99), int64(1), "操作成功"}
		cmd.SetVal(result)
	} else {
		// 批量检查结果
		result := []interface{}{int64(100), int64(0)} // [库存, 售罄标记]
		cmd.SetVal(result)
	}
	return cmd
}

// 实现Pipeline（简化版）
func (m *SimpleMockClient) Pipeline() redis.Pipeliner {
	return &MockPipeliner{}
}

// MockPipeliner 简化的Pipeline实现
type MockPipeliner struct{}

func (p *MockPipeliner) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx, "get", key)
	cmd.SetVal("1") // 默认值
	return cmd
}

func (p *MockPipeliner) Exec(ctx context.Context) ([]redis.Cmder, error) {
	return []redis.Cmder{}, nil
}

func (p *MockPipeliner) Len() int { return 0 }

// 为了满足接口要求而添加的空方法（仅为了编译通过）
func (m *SimpleMockClient) Pipeline() redis.Pipeliner { return &MockPipeliner{} }
func (m *SimpleMockClient) Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return nil, nil
}
func (m *SimpleMockClient) TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return nil, nil
}
func (m *SimpleMockClient) TxPipeline() redis.Pipeliner { return &MockPipeliner{} }

// 测试实际需要的函数
func TestSpikeCache_WarmupStock(t *testing.T) {
	// 使用简化的测试实现
	client := NewSimpleMockClient()
	spikeCache := &SpikeCache{client: client}

	err := spikeCache.WarmupStock(context.Background(), 1, 100, time.Hour)
	if err != nil {
		t.Errorf("WarmupStock() error = %v", err)
	}
}

func TestSpikeCache_GetStockInfo(t *testing.T) {
	client := NewSimpleMockClient()
	spikeCache := &SpikeCache{client: client}

	// 预设库存数据
	client.Set(context.Background(), GetSpikeStockKey(1), "100", time.Hour)

	stockInfo, err := spikeCache.GetStockInfo(context.Background(), 1)
	if err != nil {
		t.Errorf("GetStockInfo() error = %v", err)
	}

	if stockInfo.Stock != 100 {
		t.Errorf("GetStockInfo() stock = %d, want 100", stockInfo.Stock)
	}
}

func TestSpikeCache_IsSoldOut(t *testing.T) {
	client := NewSimpleMockClient()
	spikeCache := &SpikeCache{client: client}

	// 测试未售罄的情况
	soldOut, err := spikeCache.IsSoldOut(context.Background(), 1)
	if err != nil {
		t.Errorf("IsSoldOut() error = %v", err)
	}

	if soldOut {
		t.Errorf("IsSoldOut() = %v, want false", soldOut)
	}
}

func TestSpikeCache_MarkSoldOut(t *testing.T) {
	client := NewSimpleMockClient()
	spikeCache := &SpikeCache{client: client}

	err := spikeCache.MarkSoldOut(context.Background(), 1, time.Hour)
	if err != nil {
		t.Errorf("MarkSoldOut() error = %v", err)
	}
}

func TestSpikeCache_IsMessageProcessed(t *testing.T) {
	client := NewSimpleMockClient()
	spikeCache := &SpikeCache{client: client}

	// 第一次检查，应该返回false（未处理）
	processed, err := spikeCache.IsMessageProcessed(context.Background(), "msg_123")
	if err != nil {
		t.Errorf("IsMessageProcessed() error = %v", err)
	}

	if processed {
		t.Errorf("IsMessageProcessed() first call should return false")
	}
}
