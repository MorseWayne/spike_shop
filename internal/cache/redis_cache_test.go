package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRedisCache_Basic(t *testing.T) {
	// 注意：此测试需要运行Redis实例
	// 可以通过环境变量跳过
	if testing.Short() {
		t.Skip("Skipping Redis test in short mode")
	}

	// 尝试连接Redis
	cache, err := NewRedisCache("localhost:6379", "", 1) // 使用DB 1避免冲突
	if err != nil {
		t.Skipf("Skipping Redis test, cannot connect: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// 清理测试数据
	cache.FlushDB(ctx)

	t.Run("Set and Get", func(t *testing.T) {
		key := "test:key1"
		value := map[string]interface{}{
			"name": "test",
			"id":   123,
		}

		// 设置值
		err := cache.Set(ctx, key, value, 1*time.Minute)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// 获取值
		var result map[string]interface{}
		err = cache.Get(ctx, key, &result)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if result["name"] != "test" {
			t.Errorf("Expected name=test, got %v", result["name"])
		}
	})

	t.Run("Exists", func(t *testing.T) {
		key := "test:key2"

		// 检查不存在的键
		exists, err := cache.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Key should not exist")
		}

		// 设置键
		cache.Set(ctx, key, "value", 1*time.Minute)

		// 检查存在的键
		exists, err = cache.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("Key should exist")
		}
	})

	t.Run("SetNX", func(t *testing.T) {
		key := "test:key3"

		// 第一次设置应该成功
		success, err := cache.SetNX(ctx, key, "first", 1*time.Minute)
		if err != nil {
			t.Fatalf("SetNX failed: %v", err)
		}
		if !success {
			t.Error("First SetNX should succeed")
		}

		// 第二次设置应该失败
		success, err = cache.SetNX(ctx, key, "second", 1*time.Minute)
		if err != nil {
			t.Fatalf("SetNX failed: %v", err)
		}
		if success {
			t.Error("Second SetNX should fail")
		}

		// 验证值没有被覆盖
		var result string
		cache.Get(ctx, key, &result)
		if result != "first" {
			t.Errorf("Expected 'first', got %v", result)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		key := "test:key4"

		// 设置值
		cache.Set(ctx, key, "value", 1*time.Minute)

		// 删除值
		err := cache.Del(ctx, key)
		if err != nil {
			t.Fatalf("Del failed: %v", err)
		}

		// 验证已删除
		exists, _ := cache.Exists(ctx, key)
		if exists {
			t.Error("Key should be deleted")
		}
	})

	t.Run("TTL", func(t *testing.T) {
		key := "test:key5"

		// 设置带过期时间的值
		cache.Set(ctx, key, "value", 10*time.Second)

		// 检查TTL
		ttl, err := cache.TTL(ctx, key)
		if err != nil {
			t.Fatalf("TTL failed: %v", err)
		}

		if ttl <= 0 || ttl > 10*time.Second {
			t.Errorf("TTL should be between 0 and 10s, got %v", ttl)
		}
	})

	t.Run("Incr", func(t *testing.T) {
		key := "test:counter"

		// 递增不存在的键
		val, err := cache.Incr(ctx, key)
		if err != nil {
			t.Fatalf("Incr failed: %v", err)
		}
		if val != 1 {
			t.Errorf("Expected 1, got %d", val)
		}

		// 再次递增
		val, err = cache.Incr(ctx, key)
		if err != nil {
			t.Fatalf("Incr failed: %v", err)
		}
		if val != 2 {
			t.Errorf("Expected 2, got %d", val)
		}

		// 按指定值递增
		val, err = cache.IncrBy(ctx, key, 5)
		if err != nil {
			t.Fatalf("IncrBy failed: %v", err)
		}
		if val != 7 {
			t.Errorf("Expected 7, got %d", val)
		}
	})
}

func TestMemoryCache_Compatibility(t *testing.T) {
	// 测试内存缓存与Redis缓存的接口兼容性
	caches := []Cache{
		NewMemoryCache(),
		NewNullCache(),
	}

	// 如果Redis可用，也测试Redis缓存
	if redisCache, err := NewRedisCache("localhost:6379", "", 2); err == nil {
		caches = append(caches, redisCache)
		defer redisCache.Close()
		redisCache.FlushDB(context.Background())
	}

	for i, cache := range caches {
		t.Run(fmt.Sprintf("Cache_%d", i), func(t *testing.T) {
			ctx := context.Background()
			key := "test:compat"
			value := "test_value"

			// Set
			err := cache.Set(ctx, key, value, 1*time.Minute)
			if err != nil && i != 2 { // NullCache会返回nil
				t.Fatalf("Set failed: %v", err)
			}

			// Get (只有NullCache会失败)
			var result string
			err = cache.Get(ctx, key, &result)
			if i == 1 { // NullCache
				if err == nil {
					t.Error("NullCache Get should fail")
				}
			} else if err != nil {
				t.Fatalf("Get failed: %v", err)
			} else if result != value {
				t.Errorf("Expected %v, got %v", value, result)
			}

			// Exists
			exists, err := cache.Exists(ctx, key)
			if err != nil {
				t.Fatalf("Exists failed: %v", err)
			}
			if i == 1 { // NullCache
				if exists {
					t.Error("NullCache should always return false for Exists")
				}
			}

			// Delete
			err = cache.Del(ctx, key)
			if err != nil {
				t.Fatalf("Del failed: %v", err)
			}
		})
	}
}
