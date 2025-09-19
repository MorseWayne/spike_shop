// Package mq 提供RabbitMQ消费者实现
package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// MessageHandler 消息处理函数
type MessageHandler func(ctx context.Context, delivery amqp.Delivery) error

// Consumer RabbitMQ消费者
type Consumer struct {
	cm      *ConnectionManager
	config  *ConsumerConfig
	logger  *zap.Logger
	handler MessageHandler

	// 消费配置
	queueName   string
	consumerTag string
	autoAck     bool
	exclusive   bool
	noLocal     bool
	noWait      bool

	// 重试配置
	enableRetry      bool
	maxRetryAttempts int
	retryInterval    time.Duration

	// 死信队列配置
	enableDLX     bool
	dlxExchange   string
	dlxRoutingKey string

	// 并发控制
	concurrentConsumers int
	workers             []*ConsumerWorker

	// 状态管理
	running int32
	closed  int32

	// 统计信息
	processedCount int64
	failedCount    int64
	retriedCount   int64
}

// ConsumerWorker 消费者工作器
type ConsumerWorker struct {
	id       int
	consumer *Consumer
	ch       *amqp.Channel
	delivery <-chan amqp.Delivery
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewConsumer 创建消费者
func NewConsumer(cm *ConnectionManager, config *ConsumerConfig, logger *zap.Logger) *Consumer {
	if config == nil {
		config = &ConsumerConfig{
			PrefetchCount:       10,
			AutoAck:             false,
			EnableRetry:         true,
			MaxRetryAttempts:    3,
			RetryInterval:       1 * time.Second,
			EnableDLX:           true,
			DLXExchange:         "dlx",
			DLXRoutingKey:       "failed",
			ConsumeTimeout:      30 * time.Second,
			ConcurrentConsumers: 1,
		}
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Consumer{
		cm:                  cm,
		config:              config,
		logger:              logger,
		autoAck:             config.AutoAck,
		exclusive:           config.Exclusive,
		noLocal:             config.NoLocal,
		noWait:              config.NoWait,
		enableRetry:         config.EnableRetry,
		maxRetryAttempts:    config.MaxRetryAttempts,
		retryInterval:       config.RetryInterval,
		enableDLX:           config.EnableDLX,
		dlxExchange:         config.DLXExchange,
		dlxRoutingKey:       config.DLXRoutingKey,
		concurrentConsumers: config.ConcurrentConsumers,
	}
}

// SetHandler 设置消息处理函数
func (c *Consumer) SetHandler(handler MessageHandler) {
	c.handler = handler
}

// StartConsuming 开始消费消息
func (c *Consumer) StartConsuming(ctx context.Context, queueName string) error {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return fmt.Errorf("consumer is already running")
	}

	if c.handler == nil {
		atomic.StoreInt32(&c.running, 0)
		return fmt.Errorf("message handler is not set")
	}

	c.queueName = queueName
	c.consumerTag = fmt.Sprintf("consumer-%s-%d", queueName, time.Now().Unix())

	c.logger.Info("开始消费消息",
		zap.String("queue", queueName),
		zap.String("consumer_tag", c.consumerTag),
		zap.Int("concurrent_consumers", c.concurrentConsumers))

	// 启动多个消费者工作器
	c.workers = make([]*ConsumerWorker, c.concurrentConsumers)
	for i := 0; i < c.concurrentConsumers; i++ {
		worker, err := c.createWorker(ctx, i)
		if err != nil {
			// 清理已创建的工作器
			c.stopWorkers()
			atomic.StoreInt32(&c.running, 0)
			return fmt.Errorf("failed to create worker %d: %w", i, err)
		}
		c.workers[i] = worker
		go worker.run()
	}

	return nil
}

// StopConsuming 停止消费消息
func (c *Consumer) StopConsuming() error {
	if !atomic.CompareAndSwapInt32(&c.running, 1, 0) {
		return fmt.Errorf("consumer is not running")
	}

	c.logger.Info("停止消费消息", zap.String("queue", c.queueName))

	c.stopWorkers()
	return nil
}

// createWorker 创建消费者工作器
func (c *Consumer) createWorker(ctx context.Context, id int) (*ConsumerWorker, error) {
	ch, err := c.cm.GetChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	// 设置QoS
	if err := ch.Qos(c.config.PrefetchCount, c.config.PrefetchSize, false); err != nil {
		c.cm.ReturnChannel(ch)
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// 开始消费
	deliveryCh, err := ch.Consume(
		c.queueName,
		fmt.Sprintf("%s-%d", c.consumerTag, id),
		c.autoAck,
		c.exclusive,
		c.noLocal,
		c.noWait,
		nil,
	)
	if err != nil {
		c.cm.ReturnChannel(ch)
		return nil, fmt.Errorf("failed to start consuming: %w", err)
	}

	workerCtx, cancel := context.WithCancel(ctx)

	return &ConsumerWorker{
		id:       id,
		consumer: c,
		ch:       ch,
		delivery: deliveryCh,
		ctx:      workerCtx,
		cancel:   cancel,
		done:     make(chan struct{}),
	}, nil
}

// stopWorkers 停止所有工作器
func (c *Consumer) stopWorkers() {
	for _, worker := range c.workers {
		if worker != nil {
			worker.stop()
		}
	}

	// 等待所有工作器停止
	for _, worker := range c.workers {
		if worker != nil {
			<-worker.done
		}
	}
}

// run 工作器运行逻辑
func (w *ConsumerWorker) run() {
	defer close(w.done)
	defer w.consumer.cm.ReturnChannel(w.ch)

	w.consumer.logger.Info("消费者工作器启动",
		zap.Int("worker_id", w.id),
		zap.String("queue", w.consumer.queueName))

	for {
		select {
		case delivery, ok := <-w.delivery:
			if !ok {
				w.consumer.logger.Info("消费通道关闭", zap.Int("worker_id", w.id))
				return
			}

			w.processMessage(delivery)

		case <-w.ctx.Done():
			w.consumer.logger.Info("消费者工作器停止", zap.Int("worker_id", w.id))
			return
		}
	}
}

// processMessage 处理消息
func (w *ConsumerWorker) processMessage(delivery amqp.Delivery) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		w.consumer.logger.Debug("消息处理完成",
			zap.Int("worker_id", w.id),
			zap.String("message_id", delivery.MessageId),
			zap.Duration("duration", duration))
	}()

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(w.ctx, w.consumer.config.ConsumeTimeout)
	defer cancel()

	var err error
	retryCount := 0
	maxRetries := 0
	if w.consumer.enableRetry {
		maxRetries = w.consumer.maxRetryAttempts
	}

retryLoop:
	for {
		// 处理消息
		err = w.consumer.handler(ctx, delivery)
		if err == nil {
			// 处理成功
			if !w.consumer.autoAck {
				if ackErr := delivery.Ack(false); ackErr != nil {
					w.consumer.logger.Error("消息确认失败",
						zap.Error(ackErr),
						zap.String("message_id", delivery.MessageId))
				}
			}
			atomic.AddInt64(&w.consumer.processedCount, 1)
			return
		}

		// 处理失败
		w.consumer.logger.Error("消息处理失败",
			zap.Error(err),
			zap.String("message_id", delivery.MessageId),
			zap.Int("retry_count", retryCount),
			zap.Int("max_retries", maxRetries))

		// 检查是否需要重试
		if retryCount >= maxRetries {
			break
		}

		retryCount++
		atomic.AddInt64(&w.consumer.retriedCount, 1)

		// 等待重试间隔
		select {
		case <-time.After(w.consumer.retryInterval):
		case <-ctx.Done():
			break retryLoop
		}
	}

	// 最终处理失败
	atomic.AddInt64(&w.consumer.failedCount, 1)

	if !w.consumer.autoAck {
		if w.consumer.enableDLX {
			// 发送到死信队列
			if rejectErr := delivery.Nack(false, false); rejectErr != nil {
				w.consumer.logger.Error("消息拒绝失败",
					zap.Error(rejectErr),
					zap.String("message_id", delivery.MessageId))
			}
		} else {
			// 直接丢弃
			if rejectErr := delivery.Reject(false); rejectErr != nil {
				w.consumer.logger.Error("消息拒绝失败",
					zap.Error(rejectErr),
					zap.String("message_id", delivery.MessageId))
			}
		}
	}
}

// stop 停止工作器
func (w *ConsumerWorker) stop() {
	w.cancel()
}

// Close 关闭消费者
func (c *Consumer) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	if atomic.LoadInt32(&c.running) == 1 {
		c.StopConsuming()
	}

	return nil
}

// IsRunning 检查是否正在运行
func (c *Consumer) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

// IsClosed 检查是否已关闭
func (c *Consumer) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// GetStats 获取统计信息
func (c *Consumer) GetStats() ConsumerStats {
	return ConsumerStats{
		QueueName:           c.queueName,
		ConsumerTag:         c.consumerTag,
		ConcurrentConsumers: c.concurrentConsumers,
		ProcessedCount:      atomic.LoadInt64(&c.processedCount),
		FailedCount:         atomic.LoadInt64(&c.failedCount),
		RetriedCount:        atomic.LoadInt64(&c.retriedCount),
		Running:             c.IsRunning(),
		Closed:              c.IsClosed(),
	}
}

// ConsumerStats 消费者统计信息
type ConsumerStats struct {
	QueueName           string `json:"queue_name"`
	ConsumerTag         string `json:"consumer_tag"`
	ConcurrentConsumers int    `json:"concurrent_consumers"`
	ProcessedCount      int64  `json:"processed_count"`
	FailedCount         int64  `json:"failed_count"`
	RetriedCount        int64  `json:"retried_count"`
	Running             bool   `json:"running"`
	Closed              bool   `json:"closed"`
}

// JSONMessageHandler 通用JSON消息处理器
func JSONMessageHandler[T any](handler func(ctx context.Context, data T, delivery amqp.Delivery) error) MessageHandler {
	return func(ctx context.Context, delivery amqp.Delivery) error {
		var data T
		if err := json.Unmarshal(delivery.Body, &data); err != nil {
			return fmt.Errorf("failed to unmarshal JSON message: %w", err)
		}

		return handler(ctx, data, delivery)
	}
}

// RetryableHandler 可重试处理器包装器
func RetryableHandler(handler MessageHandler, isRetryable func(error) bool) MessageHandler {
	return func(ctx context.Context, delivery amqp.Delivery) error {
		err := handler(ctx, delivery)
		if err != nil && isRetryable != nil && !isRetryable(err) {
			// 不可重试的错误，包装为特殊错误类型
			return &NonRetryableError{Err: err}
		}
		return err
	}
}

// NonRetryableError 不可重试错误
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return fmt.Sprintf("non-retryable error: %v", e.Err)
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

// IsNonRetryableError 检查是否为不可重试错误
func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查上下文错误
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// 检查不可重试错误类型
	var nonRetryable *NonRetryableError
	return errors.As(err, &nonRetryable)
}
