// Package mq 提供RabbitMQ通道池管理
package mq

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ChannelPool 通道池
type ChannelPool struct {
	maxSize  int
	channels chan *amqp.Channel
	cm       *ConnectionManager
	closed   int32

	// 统计信息
	created   int64
	reused    int64
	discarded int64
}

// NewChannelPool 创建通道池
func NewChannelPool(maxSize int, cm *ConnectionManager) *ChannelPool {
	return &ChannelPool{
		maxSize:  maxSize,
		channels: make(chan *amqp.Channel, maxSize),
		cm:       cm,
	}
}

// Get 获取通道
func (cp *ChannelPool) Get() (*amqp.Channel, error) {
	if atomic.LoadInt32(&cp.closed) == 1 {
		return nil, fmt.Errorf("channel pool is closed")
	}

	// 尝试从池中获取通道
	select {
	case ch := <-cp.channels:
		if ch != nil && !ch.IsClosed() {
			atomic.AddInt64(&cp.reused, 1)
			return ch, nil
		}
		// 通道已关闭，丢弃
		atomic.AddInt64(&cp.discarded, 1)
	default:
		// 池中没有可用通道
	}

	// 创建新通道
	conn := cp.cm.GetConnection()
	if conn == nil || conn.IsClosed() {
		return nil, fmt.Errorf("connection is not available")
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	atomic.AddInt64(&cp.created, 1)
	return ch, nil
}

// Return 归还通道
func (cp *ChannelPool) Return(ch *amqp.Channel) {
	if atomic.LoadInt32(&cp.closed) == 1 {
		if ch != nil && !ch.IsClosed() {
			ch.Close()
		}
		return
	}

	if ch == nil || ch.IsClosed() {
		atomic.AddInt64(&cp.discarded, 1)
		return
	}

	// 尝试归还到池中
	select {
	case cp.channels <- ch:
		// 成功归还
	default:
		// 池已满，关闭通道
		ch.Close()
		atomic.AddInt64(&cp.discarded, 1)
	}
}

// Close 关闭通道池
func (cp *ChannelPool) Close() {
	if !atomic.CompareAndSwapInt32(&cp.closed, 0, 1) {
		return
	}

	// 关闭所有池中的通道
	close(cp.channels)
	for ch := range cp.channels {
		if ch != nil && !ch.IsClosed() {
			ch.Close()
		}
	}
}

// GetStats 获取通道池统计信息
func (cp *ChannelPool) GetStats() ChannelPoolStats {
	return ChannelPoolStats{
		MaxSize:   cp.maxSize,
		Available: len(cp.channels),
		Created:   atomic.LoadInt64(&cp.created),
		Reused:    atomic.LoadInt64(&cp.reused),
		Discarded: atomic.LoadInt64(&cp.discarded),
		Closed:    atomic.LoadInt32(&cp.closed) == 1,
	}
}

// ChannelPoolStats 通道池统计信息
type ChannelPoolStats struct {
	MaxSize   int   `json:"max_size"`
	Available int   `json:"available"`
	Created   int64 `json:"created"`
	Reused    int64 `json:"reused"`
	Discarded int64 `json:"discarded"`
	Closed    bool  `json:"closed"`
}

// ChannelWrapper 通道包装器，提供自动归还功能
type ChannelWrapper struct {
	*amqp.Channel
	pool     *ChannelPool
	returned int32
}

// NewChannelWrapper 创建通道包装器
func NewChannelWrapper(ch *amqp.Channel, pool *ChannelPool) *ChannelWrapper {
	return &ChannelWrapper{
		Channel: ch,
		pool:    pool,
	}
}

// Close 关闭通道（实际是归还到池）
func (cw *ChannelWrapper) Close() error {
	if atomic.CompareAndSwapInt32(&cw.returned, 0, 1) {
		cw.pool.Return(cw.Channel)
	}
	return nil
}

// ForceClose 强制关闭通道
func (cw *ChannelWrapper) ForceClose() error {
	if atomic.CompareAndSwapInt32(&cw.returned, 0, 1) {
		if cw.Channel != nil && !cw.Channel.IsClosed() {
			return cw.Channel.Close()
		}
	}
	return nil
}

// ManagedChannel 托管通道，自动处理错误和重连
type ManagedChannel struct {
	cm         *ConnectionManager
	onError    func(error)
	onRecreate func(*amqp.Channel)

	ch     *amqp.Channel
	mutex  sync.RWMutex
	closed int32
}

// NewManagedChannel 创建托管通道
func NewManagedChannel(cm *ConnectionManager) *ManagedChannel {
	return &ManagedChannel{
		cm: cm,
	}
}

// SetEventCallbacks 设置事件回调
func (mc *ManagedChannel) SetEventCallbacks(
	onError func(error),
	onRecreate func(*amqp.Channel)) {
	mc.onError = onError
	mc.onRecreate = onRecreate
}

// GetChannel 获取通道
func (mc *ManagedChannel) GetChannel() (*amqp.Channel, error) {
	if atomic.LoadInt32(&mc.closed) == 1 {
		return nil, fmt.Errorf("managed channel is closed")
	}

	mc.mutex.RLock()
	ch := mc.ch
	mc.mutex.RUnlock()

	if ch != nil && !ch.IsClosed() {
		return ch, nil
	}

	// 重新创建通道
	return mc.recreateChannel()
}

// recreateChannel 重新创建通道
func (mc *ManagedChannel) recreateChannel() (*amqp.Channel, error) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// 双重检查
	if mc.ch != nil && !mc.ch.IsClosed() {
		return mc.ch, nil
	}

	// 创建新通道
	ch, err := mc.cm.GetChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to create managed channel: %w", err)
	}

	// 监听通道关闭事件
	closeCh := make(chan *amqp.Error, 1)
	ch.NotifyClose(closeCh)

	go func() {
		err := <-closeCh
		if err != nil && mc.onError != nil {
			mc.onError(err)
		}

		// 自动重新创建（如果没有关闭）
		if atomic.LoadInt32(&mc.closed) == 0 {
			if newCh, createErr := mc.recreateChannel(); createErr == nil && mc.onRecreate != nil {
				mc.onRecreate(newCh)
			}
		}
	}()

	mc.ch = ch
	return ch, nil
}

// Close 关闭托管通道
func (mc *ManagedChannel) Close() error {
	if !atomic.CompareAndSwapInt32(&mc.closed, 0, 1) {
		return nil
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if mc.ch != nil && !mc.ch.IsClosed() {
		mc.cm.ReturnChannel(mc.ch)
	}

	return nil
}

// WithChannel 使用通道执行操作（自动处理获取和归还）
func (cp *ChannelPool) WithChannel(fn func(*amqp.Channel) error) error {
	ch, err := cp.Get()
	if err != nil {
		return err
	}
	defer cp.Return(ch)

	return fn(ch)
}

// WithChannelTimeout 带超时的通道操作
func (cp *ChannelPool) WithChannelTimeout(timeout time.Duration, fn func(*amqp.Channel) error) error {
	done := make(chan error, 1)

	go func() {
		done <- cp.WithChannel(fn)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("channel operation timeout after %v", timeout)
	}
}
