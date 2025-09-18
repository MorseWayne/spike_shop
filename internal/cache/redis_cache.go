// Package cache 提供Redis缓存实现
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache Redis缓存实现
type RedisCache struct {
	client redis.Cmdable // 使用接口，支持单实例和集群
}

// NewRedisCache 创建Redis缓存实例
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,     // Redis地址
		Password: password, // 密码
		DB:       db,       // 数据库编号

		// 连接池配置
		PoolSize:     10, // 连接池大小
		MinIdleConns: 5,  // 最小空闲连接数
		MaxIdleConns: 10, // 最大空闲连接数

		// 超时配置
		DialTimeout:  5 * time.Second, // 连接超时
		ReadTimeout:  3 * time.Second, // 读超时
		WriteTimeout: 3 * time.Second, // 写超时

		// 重试配置
		MaxRetries:      3,                      // 最大重试次数
		MinRetryBackoff: 8 * time.Millisecond,   // 最小重试间隔
		MaxRetryBackoff: 512 * time.Millisecond, // 最大重试间隔
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Get 获取缓存值
func (r *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key not found")
		}
		return fmt.Errorf("failed to get key %s: %w", key, err)
	}

	return json.Unmarshal([]byte(val), dest)
}

// Set 设置缓存值
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	err = r.client.Set(ctx, key, data, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}

// Del 删除缓存值
func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	err := r.client.Del(ctx, keys...).Err()
	if err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}

	return nil
}

// Exists 检查键是否存在
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return count > 0, nil
}

// SetNX 仅当键不存在时设置
func (r *RedisCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}

	success, err := r.client.SetNX(ctx, key, data, expiration).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return success, nil
}

// Ping 检查连接
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close 关闭连接
func (r *RedisCache) Close() error {
	if wrapper, ok := r.client.(*RedisClientWrapper); ok {
		return wrapper.Close()
	}
	if client, ok := r.client.(*redis.Client); ok {
		return client.Close()
	}
	return nil
}

// 扩展功能

// Incr 原子递增
func (r *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// IncrBy 原子递增指定值
func (r *RedisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

// Expire 设置过期时间
func (r *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// TTL 获取剩余过期时间
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Keys 根据模式获取键列表（谨慎使用，可能影响性能）
func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

// FlushDB 清空当前数据库（谨慎使用）
func (r *RedisCache) FlushDB(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

// Pipeline 管道操作，提高批量操作性能
func (r *RedisCache) Pipeline(ctx context.Context, fn func(pipe redis.Pipeliner) error) error {
	pipe := r.client.Pipeline()

	if err := fn(pipe); err != nil {
		return err
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Lua脚本支持，用于原子操作
func (r *RedisCache) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.EvalSha(ctx, sha1, keys, args...).Result()
}

func (r *RedisCache) ScriptLoad(ctx context.Context, script string) (string, error) {
	return r.client.ScriptLoad(ctx, script).Result()
}

// 集群模式支持（如果使用Redis集群）
func NewRedisClusterCache(addrs []string, password string) (*RedisCache, error) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    addrs,
		Password: password,

		// 连接池配置
		PoolSize:     10,
		MinIdleConns: 5,
		MaxIdleConns: 10,

		// 超时配置
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		// 重试配置
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis cluster: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// RedisClientWrapper 包装Redis客户端，提供统一的Close方法
type RedisClientWrapper struct {
	redis.Cmdable
	closer func() error
}

func (w *RedisClientWrapper) Close() error {
	if w.closer != nil {
		return w.closer()
	}
	return nil
}
