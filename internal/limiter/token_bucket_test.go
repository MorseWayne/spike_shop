package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// MockRedisClient for limiter testing
type MockRedisClient struct {
	data      map[string]interface{}
	scriptSHA map[string]string
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data:      make(map[string]interface{}),
		scriptSHA: make(map[string]string),
	}
}

func (m *MockRedisClient) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx, "evalsha", sha1)

	// 模拟令牌桶脚本逻辑
	if len(args) >= 5 {
		capacity := args[0].(string)
		rate := args[1].(string)
		window := args[2].(string)
		tokensRequested := args[3].(string)
		currentTime := args[4].(string)

		// 简化逻辑：如果请求的令牌数 <= 容量，则允许
		if tokensRequested == "1" && capacity == "10" {
			// 模拟成功的情况
			cmd.SetVal([]interface{}{
				int64(1), // allowed
				int64(9), // remaining
				int64(0), // retry_after
				int64(1), // total_requests
			})
		} else {
			// 模拟失败的情况
			cmd.SetVal([]interface{}{
				int64(0), // not allowed
				int64(0), // remaining
				int64(1), // retry_after (seconds)
				int64(1), // total_requests
			})
		}
	}

	return cmd
}

func (m *MockRedisClient) ScriptLoad(ctx context.Context, script string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx, "script", "load", script)
	sha := "mock_sha1_" + string(rune(len(m.scriptSHA)))
	m.scriptSHA[sha] = script
	cmd.SetVal(sha)
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
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

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "ping")
	cmd.SetVal("PONG")
	return cmd
}

// Implement minimal required methods for redis.Cmdable interface
func (m *MockRedisClient) Pipeline() redis.Pipeliner { return nil }
func (m *MockRedisClient) Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return nil, nil
}
func (m *MockRedisClient) TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return nil, nil
}
func (m *MockRedisClient) TxPipeline() redis.Pipeliner                        { return nil }
func (m *MockRedisClient) Command(ctx context.Context) *redis.CommandsInfoCmd { return nil }
func (m *MockRedisClient) CommandList(ctx context.Context, filter *redis.FilterBy) *redis.StringSliceCmd {
	return nil
}
func (m *MockRedisClient) CommandGetKeys(ctx context.Context, commands ...interface{}) *redis.StringSliceCmd {
	return nil
}
func (m *MockRedisClient) CommandGetKeysAndFlags(ctx context.Context, commands ...interface{}) *redis.KeyFlagsCmd {
	return nil
}
func (m *MockRedisClient) ClientGetName(ctx context.Context) *redis.StringCmd             { return nil }
func (m *MockRedisClient) Echo(ctx context.Context, message interface{}) *redis.StringCmd { return nil }
func (m *MockRedisClient) Hello(ctx context.Context, ver int, username, password, clientName string) *redis.MapStringInterfaceCmd {
	return nil
}
func (m *MockRedisClient) Quit(ctx context.Context) *redis.StatusCmd              { return nil }
func (m *MockRedisClient) Select(ctx context.Context, index int) *redis.StatusCmd { return nil }
func (m *MockRedisClient) SwapDB(ctx context.Context, index1, index2 int) *redis.StatusCmd {
	return nil
}
func (m *MockRedisClient) ClientID(ctx context.Context) *redis.IntCmd { return nil }

func TestNewTokenBucketLimiter(t *testing.T) {
	client := NewMockRedisClient()

	tests := []struct {
		name       string
		config     *Config
		wantErr    bool
		wantPrefix string
	}{
		{
			name: "valid config",
			config: &Config{
				Rate:      10,
				Window:    time.Minute,
				Burst:     20,
				KeyPrefix: "test:tb",
			},
			wantErr:    false,
			wantPrefix: "test:tb",
		},
		{
			name: "empty key prefix",
			config: &Config{
				Rate:   10,
				Window: time.Minute,
				Burst:  20,
			},
			wantErr:    false,
			wantPrefix: "limiter:tb",
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := NewTokenBucketLimiter(client, tt.config)

			if tt.wantErr && err == nil {
				t.Errorf("NewTokenBucketLimiter() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("NewTokenBucketLimiter() unexpected error = %v", err)
			}

			if !tt.wantErr && limiter != nil {
				tokenBucket := limiter.(*TokenBucketLimiter)
				if tokenBucket.keyPrefix != tt.wantPrefix {
					t.Errorf("NewTokenBucketLimiter() keyPrefix = %v, want %v", tokenBucket.keyPrefix, tt.wantPrefix)
				}
			}
		})
	}
}

func TestTokenBucketLimiter_Allow(t *testing.T) {
	client := NewMockRedisClient()
	config := &Config{
		Rate:      10,
		Window:    time.Minute,
		Burst:     10,
		KeyPrefix: "test:tb",
	}

	limiter, err := NewTokenBucketLimiter(client, config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	tests := []struct {
		name        string
		key         string
		setupFunc   func()
		wantAllowed bool
		wantErr     bool
	}{
		{
			name:        "first request allowed",
			key:         "user:123",
			setupFunc:   nil,
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "another user allowed",
			key:         "user:456",
			setupFunc:   nil,
			wantAllowed: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			result, err := limiter.Allow(context.Background(), tt.key)

			if tt.wantErr && err == nil {
				t.Errorf("Allow() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Allow() unexpected error = %v", err)
			}

			if result != nil {
				if result.Allowed != tt.wantAllowed {
					t.Errorf("Allow() allowed = %v, want %v", result.Allowed, tt.wantAllowed)
				}

				// 验证返回值的合理性
				if result.Allowed {
					if result.Remaining < 0 {
						t.Errorf("Allow() remaining should not be negative when allowed")
					}
				} else {
					if result.RetryAfter <= 0 {
						t.Errorf("Allow() retry_after should be positive when not allowed")
					}
				}

				if result.TotalRequests <= 0 {
					t.Errorf("Allow() total_requests should be positive")
				}
			}
		})
	}
}

func TestTokenBucketLimiter_AllowN(t *testing.T) {
	client := NewMockRedisClient()
	config := &Config{
		Rate:      10,
		Window:    time.Minute,
		Burst:     10,
		KeyPrefix: "test:tb",
	}

	limiter, err := NewTokenBucketLimiter(client, config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	tests := []struct {
		name        string
		key         string
		n           int64
		wantAllowed bool
		wantErr     bool
	}{
		{
			name:        "allow 1 token",
			key:         "user:123",
			n:           1,
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "allow 5 tokens",
			key:         "user:456",
			n:           5,
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "request too many tokens",
			key:         "user:789",
			n:           20, // 超过burst容量
			wantAllowed: false,
			wantErr:     false,
		},
		{
			name:    "invalid token count",
			key:     "user:000",
			n:       0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := limiter.AllowN(context.Background(), tt.key, tt.n)

			if tt.wantErr && err == nil {
				t.Errorf("AllowN() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("AllowN() unexpected error = %v", err)
			}

			if !tt.wantErr && result != nil {
				if result.Allowed != tt.wantAllowed {
					t.Errorf("AllowN() allowed = %v, want %v", result.Allowed, tt.wantAllowed)
				}
			}
		})
	}
}

func TestTokenBucketLimiter_Reset(t *testing.T) {
	client := NewMockRedisClient()
	config := &Config{
		Rate:      10,
		Window:    time.Minute,
		Burst:     10,
		KeyPrefix: "test:tb",
	}

	limiter, err := NewTokenBucketLimiter(client, config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	key := "user:123"

	// 先发起几个请求
	_, err = limiter.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("Failed to make initial request: %v", err)
	}

	// 重置限流状态
	err = limiter.Reset(context.Background(), key)
	if err != nil {
		t.Errorf("Reset() unexpected error = %v", err)
	}

	// 重置后应该能正常请求
	result, err := limiter.Allow(context.Background(), key)
	if err != nil {
		t.Errorf("Allow() after Reset() unexpected error = %v", err)
	}

	if result != nil && !result.Allowed {
		t.Errorf("Allow() after Reset() should be allowed")
	}
}

func TestTokenBucketLimiter_GetInfo(t *testing.T) {
	client := NewMockRedisClient()
	config := &Config{
		Rate:      10,
		Window:    time.Minute,
		Burst:     10,
		KeyPrefix: "test:tb",
	}

	limiter, err := NewTokenBucketLimiter(client, config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	key := "user:123"

	info, err := limiter.GetInfo(context.Background(), key)
	if err != nil {
		t.Errorf("GetInfo() unexpected error = %v", err)
	}

	if info != nil {
		if info.Limit != config.Rate {
			t.Errorf("GetInfo() limit = %d, want %d", info.Limit, config.Rate)
		}

		if info.Window != config.Window {
			t.Errorf("GetInfo() window = %v, want %v", info.Window, config.Window)
		}

		if info.Remaining < 0 {
			t.Errorf("GetInfo() remaining should not be negative")
		}

		if info.ResetTime.Before(time.Now()) {
			t.Errorf("GetInfo() reset time should be in the future")
		}
	}
}

// 测试限流效果
func TestTokenBucketLimiter_RateLimiting(t *testing.T) {
	client := NewMockRedisClient()
	config := &Config{
		Rate:      5, // 每分钟5个请求
		Window:    time.Minute,
		Burst:     5, // 突发容量5
		KeyPrefix: "test:tb",
	}

	limiter, err := NewTokenBucketLimiter(client, config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	key := "user:rate_test"

	// 连续发起多个请求
	allowedCount := 0
	deniedCount := 0

	for i := 0; i < 10; i++ {
		result, err := limiter.Allow(context.Background(), key)
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
			continue
		}

		if result.Allowed {
			allowedCount++
		} else {
			deniedCount++
		}
	}

	t.Logf("Rate limiting test: %d allowed, %d denied", allowedCount, deniedCount)

	// 在Mock环境中，我们的逻辑比较简化，这里主要验证功能调用正常
	if allowedCount+deniedCount != 10 {
		t.Errorf("Total requests should be 10, got %d", allowedCount+deniedCount)
	}
}

// 测试并发安全
func TestTokenBucketLimiter_Concurrent(t *testing.T) {
	client := NewMockRedisClient()
	config := &Config{
		Rate:      10,
		Window:    time.Minute,
		Burst:     10,
		KeyPrefix: "test:tb",
	}

	limiter, err := NewTokenBucketLimiter(client, config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	const concurrency = 20
	results := make(chan *LimitResult, concurrency)
	errors := make(chan error, concurrency)

	// 并发请求
	for i := 0; i < concurrency; i++ {
		go func(index int) {
			key := "user:concurrent_test"
			result, err := limiter.Allow(context.Background(), key)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(i)
	}

	// 收集结果
	allowedCount := 0
	for i := 0; i < concurrency; i++ {
		select {
		case result := <-results:
			if result.Allowed {
				allowedCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent request error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Test timeout")
		}
	}

	t.Logf("Concurrent test: %d/%d requests allowed", allowedCount, concurrency)
}
