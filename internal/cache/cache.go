// Package cache 提供缓存抽象和Redis实现
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Cache 定义缓存操作接口
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Ping(ctx context.Context) error
	Close() error
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled bool
	TTL     time.Duration
}

// MemoryCache 内存缓存实现（用于开发和测试）
type MemoryCache struct {
	data map[string]*memoryCacheItem
}

type memoryCacheItem struct {
	value      []byte
	expiration time.Time
}

// NewMemoryCache 创建内存缓存实例
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]*memoryCacheItem),
	}
}

// Get 获取缓存值
func (m *MemoryCache) Get(ctx context.Context, key string, dest interface{}) error {
	item, exists := m.data[key]
	if !exists {
		return fmt.Errorf("key not found")
	}

	// 检查是否过期
	if time.Now().After(item.expiration) {
		delete(m.data, key)
		return fmt.Errorf("key expired")
	}

	return json.Unmarshal(item.value, dest)
}

// Set 设置缓存值
func (m *MemoryCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	m.data[key] = &memoryCacheItem{
		value:      data,
		expiration: time.Now().Add(expiration),
	}

	return nil
}

// Del 删除缓存值
func (m *MemoryCache) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// Exists 检查键是否存在
func (m *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	item, exists := m.data[key]
	if !exists {
		return false, nil
	}

	// 检查是否过期
	if time.Now().After(item.expiration) {
		delete(m.data, key)
		return false, nil
	}

	return true, nil
}

// SetNX 仅当键不存在时设置
func (m *MemoryCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	exists, err := m.Exists(ctx, key)
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	return true, m.Set(ctx, key, value, expiration)
}

// Ping 检查连接
func (m *MemoryCache) Ping(ctx context.Context) error {
	return nil
}

// Close 关闭缓存
func (m *MemoryCache) Close() error {
	m.data = make(map[string]*memoryCacheItem)
	return nil
}

// NullCache 空缓存实现（禁用缓存时使用）
type NullCache struct{}

// NewNullCache 创建空缓存实例
func NewNullCache() *NullCache {
	return &NullCache{}
}

func (n *NullCache) Get(ctx context.Context, key string, dest interface{}) error {
	return fmt.Errorf("cache disabled")
}

func (n *NullCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return nil // 不做任何操作
}

func (n *NullCache) Del(ctx context.Context, keys ...string) error {
	return nil
}

func (n *NullCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (n *NullCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return false, nil
}

func (n *NullCache) Ping(ctx context.Context) error {
	return nil
}

func (n *NullCache) Close() error {
	return nil
}
