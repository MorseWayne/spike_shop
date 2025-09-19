// Package mq 提供RabbitMQ连接管理和连接池
package mq

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// ConnectionState 连接状态
type ConnectionState int32

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ConnectionManager RabbitMQ连接管理器
type ConnectionManager struct {
	config *Config
	logger *zap.Logger

	// 连接管理
	conn      *amqp.Connection
	connMutex sync.RWMutex
	state     int32 // 使用atomic操作

	// 连接池
	channelPool *ChannelPool

	// 重连管理
	reconnectCh    chan struct{}
	stopCh         chan struct{}
	reconnectCount int32

	// 健康检查
	healthCheckInterval time.Duration
	lastHealthCheck     time.Time

	// 事件回调
	onConnected    func()
	onDisconnected func(error)
	onReconnected  func()
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(config *Config, logger *zap.Logger) *ConnectionManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	cm := &ConnectionManager{
		config:              config,
		logger:              logger,
		state:               int32(StateDisconnected),
		reconnectCh:         make(chan struct{}, 1),
		stopCh:              make(chan struct{}),
		healthCheckInterval: 30 * time.Second,
	}

	// 创建通道池
	cm.channelPool = NewChannelPool(config.MaxChannels, cm)

	return cm
}

// Connect 建立连接
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&cm.state, int32(StateDisconnected), int32(StateConnecting)) {
		return fmt.Errorf("connection is already in progress or connected")
	}

	cm.logger.Info("连接RabbitMQ", zap.String("url", cm.config.GetConnectionURL()))

	// 创建连接配置
	connConfig := amqp.Config{
		Heartbeat: cm.config.HeartbeatInterval,
		Locale:    "en_US",
	}

	// 设置TLS配置
	if cm.config.UseTLS {
		tlsConfig, err := cm.config.GetTLSConfig()
		if err != nil {
			atomic.StoreInt32(&cm.state, int32(StateDisconnected))
			return fmt.Errorf("failed to get TLS config: %w", err)
		}
		connConfig.TLSClientConfig = tlsConfig
	}

	// 建立连接
	conn, err := amqp.DialConfig(cm.config.GetConnectionURL(), connConfig)
	if err != nil {
		atomic.StoreInt32(&cm.state, int32(StateDisconnected))
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	cm.connMutex.Lock()
	cm.conn = conn
	cm.connMutex.Unlock()

	atomic.StoreInt32(&cm.state, int32(StateConnected))
	cm.lastHealthCheck = time.Now()

	cm.logger.Info("RabbitMQ连接成功")

	// 启动监控goroutine
	go cm.monitorConnection()
	go cm.healthCheck()

	// 触发连接回调
	if cm.onConnected != nil {
		cm.onConnected()
	}

	return nil
}

// GetConnection 获取连接
func (cm *ConnectionManager) GetConnection() *amqp.Connection {
	cm.connMutex.RLock()
	defer cm.connMutex.RUnlock()
	return cm.conn
}

// GetChannel 获取通道
func (cm *ConnectionManager) GetChannel() (*amqp.Channel, error) {
	return cm.channelPool.Get()
}

// ReturnChannel 归还通道
func (cm *ConnectionManager) ReturnChannel(ch *amqp.Channel) {
	cm.channelPool.Return(ch)
}

// IsConnected 检查是否已连接
func (cm *ConnectionManager) IsConnected() bool {
	state := atomic.LoadInt32(&cm.state)
	return state == int32(StateConnected)
}

// GetState 获取连接状态
func (cm *ConnectionManager) GetState() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&cm.state))
}

// Close 关闭连接
func (cm *ConnectionManager) Close() error {
	if !atomic.CompareAndSwapInt32(&cm.state, int32(StateConnected), int32(StateClosed)) &&
		!atomic.CompareAndSwapInt32(&cm.state, int32(StateDisconnected), int32(StateClosed)) &&
		!atomic.CompareAndSwapInt32(&cm.state, int32(StateReconnecting), int32(StateClosed)) {
		return nil // 已经关闭或正在关闭
	}

	cm.logger.Info("关闭RabbitMQ连接")

	// 停止监控
	close(cm.stopCh)

	// 关闭通道池
	cm.channelPool.Close()

	// 关闭连接
	cm.connMutex.Lock()
	if cm.conn != nil {
		err := cm.conn.Close()
		cm.conn = nil
		cm.connMutex.Unlock()
		return err
	}
	cm.connMutex.Unlock()

	return nil
}

// monitorConnection 监控连接状态
func (cm *ConnectionManager) monitorConnection() {
	cm.connMutex.RLock()
	conn := cm.conn
	cm.connMutex.RUnlock()

	if conn == nil {
		return
	}

	// 监听连接关闭事件
	closeCh := make(chan *amqp.Error, 1)
	conn.NotifyClose(closeCh)

	select {
	case err := <-closeCh:
		if err != nil {
			cm.logger.Error("RabbitMQ连接意外关闭", zap.Error(err))
			cm.handleDisconnection(err)
		}
	case <-cm.stopCh:
		return
	}
}

// handleDisconnection 处理连接断开
func (cm *ConnectionManager) handleDisconnection(err error) {
	if !atomic.CompareAndSwapInt32(&cm.state, int32(StateConnected), int32(StateReconnecting)) {
		return // 已经在重连或已关闭
	}

	cm.logger.Warn("RabbitMQ连接断开，开始重连", zap.Error(err))

	// 触发断开回调
	if cm.onDisconnected != nil {
		cm.onDisconnected(err)
	}

	// 启动重连
	if cm.config.EnableReconnect {
		go cm.reconnect()
	}
}

// reconnect 重连逻辑
func (cm *ConnectionManager) reconnect() {
	defer func() {
		if r := recover(); r != nil {
			cm.logger.Error("重连过程发生panic", zap.Any("panic", r))
		}
	}()

	attempts := 0
	maxAttempts := cm.config.MaxReconnectAttempts

	for {
		select {
		case <-cm.stopCh:
			return
		default:
		}

		attempts++
		atomic.AddInt32(&cm.reconnectCount, 1)

		cm.logger.Info("尝试重连RabbitMQ",
			zap.Int("attempt", attempts),
			zap.Int("max_attempts", maxAttempts))

		// 清理旧连接
		cm.connMutex.Lock()
		if cm.conn != nil {
			cm.conn.Close()
			cm.conn = nil
		}
		cm.connMutex.Unlock()

		// 尝试重新连接
		ctx, cancel := context.WithTimeout(context.Background(), cm.config.ConnectionTimeout)
		err := cm.connectInternal(ctx)
		cancel()

		if err == nil {
			cm.logger.Info("RabbitMQ重连成功", zap.Int("attempts", attempts))

			// 触发重连回调
			if cm.onReconnected != nil {
				cm.onReconnected()
			}

			// 重新启动监控
			go cm.monitorConnection()
			return
		}

		cm.logger.Error("RabbitMQ重连失败",
			zap.Error(err),
			zap.Int("attempt", attempts))

		// 检查是否达到最大重试次数
		if maxAttempts > 0 && attempts >= maxAttempts {
			cm.logger.Error("RabbitMQ重连失败，达到最大重试次数",
				zap.Int("max_attempts", maxAttempts))
			atomic.StoreInt32(&cm.state, int32(StateDisconnected))
			return
		}

		// 等待重连间隔
		select {
		case <-time.After(cm.config.ReconnectInterval):
		case <-cm.stopCh:
			return
		}
	}
}

// connectInternal 内部连接方法
func (cm *ConnectionManager) connectInternal(ctx context.Context) error {
	connConfig := amqp.Config{
		Heartbeat: cm.config.HeartbeatInterval,
		Locale:    "en_US",
	}

	if cm.config.UseTLS {
		tlsConfig, err := cm.config.GetTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to get TLS config: %w", err)
		}
		connConfig.TLSClientConfig = tlsConfig
	}

	conn, err := amqp.DialConfig(cm.config.GetConnectionURL(), connConfig)
	if err != nil {
		return err
	}

	cm.connMutex.Lock()
	cm.conn = conn
	cm.connMutex.Unlock()

	atomic.StoreInt32(&cm.state, int32(StateConnected))
	cm.lastHealthCheck = time.Now()

	return nil
}

// healthCheck 健康检查
func (cm *ConnectionManager) healthCheck() {
	ticker := time.NewTicker(cm.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if cm.IsConnected() {
				if err := cm.pingConnection(); err != nil {
					cm.logger.Error("健康检查失败", zap.Error(err))
					cm.handleDisconnection(err)
					return
				}
				cm.lastHealthCheck = time.Now()
			}
		case <-cm.stopCh:
			return
		}
	}
}

// pingConnection 测试连接
func (cm *ConnectionManager) pingConnection() error {
	cm.connMutex.RLock()
	conn := cm.conn
	cm.connMutex.RUnlock()

	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("connection is closed")
	}

	// 通过创建和关闭临时通道来测试连接
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel for health check: %w", err)
	}
	defer ch.Close()

	return nil
}

// SetEventCallbacks 设置事件回调
func (cm *ConnectionManager) SetEventCallbacks(
	onConnected func(),
	onDisconnected func(error),
	onReconnected func()) {
	cm.onConnected = onConnected
	cm.onDisconnected = onDisconnected
	cm.onReconnected = onReconnected
}

// GetStats 获取连接统计信息
func (cm *ConnectionManager) GetStats() ConnectionStats {
	return ConnectionStats{
		State:            cm.GetState(),
		ReconnectCount:   atomic.LoadInt32(&cm.reconnectCount),
		LastHealthCheck:  cm.lastHealthCheck,
		ChannelPoolStats: cm.channelPool.GetStats(),
	}
}

// ConnectionStats 连接统计信息
type ConnectionStats struct {
	State            ConnectionState  `json:"state"`
	ReconnectCount   int32            `json:"reconnect_count"`
	LastHealthCheck  time.Time        `json:"last_health_check"`
	ChannelPoolStats ChannelPoolStats `json:"channel_pool_stats"`
}
