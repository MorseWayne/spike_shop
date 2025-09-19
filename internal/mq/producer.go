// Package mq 提供RabbitMQ生产者实现
package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Producer RabbitMQ生产者
type Producer struct {
	cm     *ConnectionManager
	config *ProducerConfig
	logger *zap.Logger

	// 发布确认
	confirmMode    bool
	confirmTimeout time.Duration

	// 批量发送
	batchMode     bool
	batchSize     int
	batchTimeout  time.Duration
	batchMessages chan *BatchMessage

	// 统计信息
	publishedCount int64
	confirmedCount int64
	failedCount    int64

	// 状态管理
	closed bool
	mutex  sync.RWMutex
}

// BatchMessage 批量消息
type BatchMessage struct {
	Exchange   string
	RoutingKey string
	Publishing amqp.Publishing
	ResultCh   chan error
}

// PublishOptions 发布选项
type PublishOptions struct {
	Exchange   string
	RoutingKey string
	Mandatory  bool
	Immediate  bool
	Headers    map[string]interface{}
	Priority   uint8
	Expiration string
	MessageID  string
	Timestamp  time.Time
	Type       string
	UserID     string
	AppID      string
}

// NewProducer 创建生产者
func NewProducer(cm *ConnectionManager, config *ProducerConfig, logger *zap.Logger) *Producer {
	if config == nil {
		config = &ProducerConfig{
			EnableConfirm:    true,
			ConfirmTimeout:   5 * time.Second,
			EnableRetry:      true,
			MaxRetryAttempts: 3,
			RetryInterval:    1 * time.Second,
			EnableBatch:      false,
			BatchSize:        100,
			BatchTimeout:     1 * time.Second,
			PublishTimeout:   10 * time.Second,
		}
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	p := &Producer{
		cm:             cm,
		config:         config,
		logger:         logger,
		confirmMode:    config.EnableConfirm,
		confirmTimeout: config.ConfirmTimeout,
		batchMode:      config.EnableBatch,
		batchSize:      config.BatchSize,
		batchTimeout:   config.BatchTimeout,
	}

	if p.batchMode {
		p.batchMessages = make(chan *BatchMessage, config.BatchSize*2)
		go p.batchProcessor()
	}

	return p
}

// Publish 发布消息
func (p *Producer) Publish(ctx context.Context, exchange, routingKey string, body []byte, options *PublishOptions) error {
	if p.isClosed() {
		return fmt.Errorf("producer is closed")
	}

	publishing := p.buildPublishing(body, options)

	if p.batchMode {
		return p.publishBatch(ctx, exchange, routingKey, publishing)
	}

	return p.publishDirect(ctx, exchange, routingKey, publishing, options)
}

// PublishJSON 发布JSON消息
func (p *Producer) PublishJSON(ctx context.Context, exchange, routingKey string, data interface{}, options *PublishOptions) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if options == nil {
		options = &PublishOptions{}
	}
	if options.Headers == nil {
		options.Headers = make(map[string]interface{})
	}
	options.Headers["content-type"] = "application/json"

	return p.Publish(ctx, exchange, routingKey, body, options)
}

// publishDirect 直接发布消息
func (p *Producer) publishDirect(ctx context.Context, exchange, routingKey string, publishing amqp.Publishing, options *PublishOptions) error {
	var lastErr error
	maxAttempts := 1
	if p.config.EnableRetry {
		maxAttempts = p.config.MaxRetryAttempts + 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := p.publishOnce(ctx, exchange, routingKey, publishing, options)
		if err == nil {
			return nil
		}

		lastErr = err
		p.logger.Warn("消息发布失败",
			zap.String("exchange", exchange),
			zap.String("routing_key", routingKey),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", maxAttempts),
			zap.Error(err))

		// 如果是最后一次尝试，直接返回错误
		if attempt == maxAttempts {
			break
		}

		// 等待重试间隔
		select {
		case <-time.After(p.config.RetryInterval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	p.failedCount++
	return fmt.Errorf("failed to publish message after %d attempts: %w", maxAttempts, lastErr)
}

// publishOnce 单次发布消息
func (p *Producer) publishOnce(ctx context.Context, exchange, routingKey string, publishing amqp.Publishing, options *PublishOptions) error {
	ch, err := p.cm.GetChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer p.cm.ReturnChannel(ch)

	// 设置发布确认模式
	var confirmCh chan amqp.Confirmation
	if p.confirmMode {
		if err := ch.Confirm(false); err != nil {
			return fmt.Errorf("failed to set confirm mode: %w", err)
		}
		confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	}

	// 设置超时
	publishCtx, cancel := context.WithTimeout(ctx, p.config.PublishTimeout)
	defer cancel()

	// 发布消息
	mandatory := false
	immediate := false
	if options != nil {
		mandatory = options.Mandatory
		immediate = options.Immediate
	}

	err = ch.PublishWithContext(publishCtx, exchange, routingKey, mandatory, immediate, publishing)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	p.publishedCount++

	// 等待发布确认
	if p.confirmMode {
		select {
		case confirmation := <-confirmCh:
			if confirmation.Ack {
				p.confirmedCount++
				return nil
			}
			return fmt.Errorf("message was nacked by broker")
		case <-time.After(p.confirmTimeout):
			return fmt.Errorf("publish confirmation timeout")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// publishBatch 批量发布消息
func (p *Producer) publishBatch(ctx context.Context, exchange, routingKey string, publishing amqp.Publishing) error {
	resultCh := make(chan error, 1)

	batchMsg := &BatchMessage{
		Exchange:   exchange,
		RoutingKey: routingKey,
		Publishing: publishing,
		ResultCh:   resultCh,
	}

	select {
	case p.batchMessages <- batchMsg:
		// 等待结果
		select {
		case err := <-resultCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

// batchProcessor 批量处理器
func (p *Producer) batchProcessor() {
	ticker := time.NewTicker(p.batchTimeout)
	defer ticker.Stop()

	var batch []*BatchMessage

	for {
		select {
		case msg := <-p.batchMessages:
			if msg == nil {
				// 处理剩余消息并退出
				if len(batch) > 0 {
					p.processBatch(batch)
				}
				return
			}

			batch = append(batch, msg)

			// 达到批量大小，立即处理
			if len(batch) >= p.batchSize {
				p.processBatch(batch)
				batch = batch[:0] // 重置切片
			}

		case <-ticker.C:
			// 超时，处理当前批次
			if len(batch) > 0 {
				p.processBatch(batch)
				batch = batch[:0] // 重置切片
			}
		}
	}
}

// processBatch 处理批量消息
func (p *Producer) processBatch(batch []*BatchMessage) {
	if len(batch) == 0 {
		return
	}

	ch, err := p.cm.GetChannel()
	if err != nil {
		// 通知所有消息失败
		for _, msg := range batch {
			msg.ResultCh <- fmt.Errorf("failed to get channel: %w", err)
		}
		return
	}
	defer p.cm.ReturnChannel(ch)

	// 设置发布确认模式
	var confirmCh chan amqp.Confirmation
	if p.confirmMode {
		if err := ch.Confirm(false); err != nil {
			for _, msg := range batch {
				msg.ResultCh <- fmt.Errorf("failed to set confirm mode: %w", err)
			}
			return
		}
		confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, len(batch)))
	}

	// 批量发布
	ctx, cancel := context.WithTimeout(context.Background(), p.config.PublishTimeout)
	defer cancel()

	for _, msg := range batch {
		err := ch.PublishWithContext(ctx, msg.Exchange, msg.RoutingKey, false, false, msg.Publishing)
		if err != nil {
			msg.ResultCh <- fmt.Errorf("failed to publish message: %w", err)
			p.failedCount++
		} else {
			p.publishedCount++
		}
	}

	// 等待确认
	if p.confirmMode {
		confirmCount := 0
		timeout := time.After(p.confirmTimeout)

		for confirmCount < len(batch) {
			select {
			case confirmation := <-confirmCh:
				if confirmation.Ack {
					p.confirmedCount++
				}
				confirmCount++

				// 通知对应的消息
				if int(confirmation.DeliveryTag) <= len(batch) {
					idx := confirmation.DeliveryTag - 1
					if confirmation.Ack {
						batch[idx].ResultCh <- nil
					} else {
						batch[idx].ResultCh <- fmt.Errorf("message was nacked by broker")
						p.failedCount++
					}
				}

			case <-timeout:
				// 超时，通知剩余消息
				for i := confirmCount; i < len(batch); i++ {
					batch[i].ResultCh <- fmt.Errorf("publish confirmation timeout")
					p.failedCount++
				}
				return

			case <-ctx.Done():
				// 上下文取消
				for i := confirmCount; i < len(batch); i++ {
					batch[i].ResultCh <- ctx.Err()
					p.failedCount++
				}
				return
			}
		}
	} else {
		// 没有确认模式，直接通知成功
		for _, msg := range batch {
			msg.ResultCh <- nil
		}
	}
}

// buildPublishing 构建发布消息
func (p *Producer) buildPublishing(body []byte, options *PublishOptions) amqp.Publishing {
	publishing := amqp.Publishing{
		Body:        body,
		ContentType: "application/octet-stream",
		Timestamp:   time.Now(),
	}

	if options != nil {
		if options.Headers != nil {
			publishing.Headers = options.Headers
		}
		if options.Priority > 0 {
			publishing.Priority = options.Priority
		}
		if options.Expiration != "" {
			publishing.Expiration = options.Expiration
		}
		if options.MessageID != "" {
			publishing.MessageId = options.MessageID
		}
		if !options.Timestamp.IsZero() {
			publishing.Timestamp = options.Timestamp
		}
		if options.Type != "" {
			publishing.Type = options.Type
		}
		if options.UserID != "" {
			publishing.UserId = options.UserID
		}
		if options.AppID != "" {
			publishing.AppId = options.AppID
		}

		// 从Headers中提取content-type
		if ct, ok := options.Headers["content-type"].(string); ok {
			publishing.ContentType = ct
		}
	}

	return publishing
}

// Close 关闭生产者
func (p *Producer) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	if p.batchMode && p.batchMessages != nil {
		close(p.batchMessages)
	}

	return nil
}

// isClosed 检查是否已关闭
func (p *Producer) isClosed() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.closed
}

// GetStats 获取统计信息
func (p *Producer) GetStats() ProducerStats {
	return ProducerStats{
		PublishedCount: p.publishedCount,
		ConfirmedCount: p.confirmedCount,
		FailedCount:    p.failedCount,
		BatchMode:      p.batchMode,
		ConfirmMode:    p.confirmMode,
	}
}

// ProducerStats 生产者统计信息
type ProducerStats struct {
	PublishedCount int64 `json:"published_count"`
	ConfirmedCount int64 `json:"confirmed_count"`
	FailedCount    int64 `json:"failed_count"`
	BatchMode      bool  `json:"batch_mode"`
	ConfirmMode    bool  `json:"confirm_mode"`
}
