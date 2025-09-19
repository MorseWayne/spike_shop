// Package mq 提供秒杀消息生产者服务
package mq

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SpikeProducer 秒杀消息生产者
type SpikeProducer struct {
	producer *Producer
	qm       *SpikeQueueManager
	logger   *zap.Logger
}

// NewSpikeProducer 创建秒杀消息生产者
func NewSpikeProducer(cm *ConnectionManager, config *ProducerConfig, logger *zap.Logger) (*SpikeProducer, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 创建基础生产者
	producer := NewProducer(cm, config, logger)

	// 创建队列管理器
	queueManager := NewSpikeQueueManager(cm, logger)

	return &SpikeProducer{
		producer: producer,
		qm:       queueManager,
		logger:   logger,
	}, nil
}

// PublishSpikeOrderCreated 发布秒杀订单创建消息
func (sp *SpikeProducer) PublishSpikeOrderCreated(ctx context.Context, data *SpikeOrderCreatedData, traceID string) error {
	message := CreateSpikeOrderCreatedMessage(data, traceID)

	return sp.publishMessage(ctx, message, SpikeExchange, &PublishOptions{
		MessageID: message.ID,
		Type:      string(message.Type),
		Timestamp: message.Timestamp,
		Headers: map[string]interface{}{
			"content-type":    "application/json",
			"trace-id":        traceID,
			"spike-event-id":  data.SpikeEventID,
			"user-id":         data.UserID,
			"idempotency-key": data.IdempotencyKey,
		},
		Priority: 5, // 高优先级
	})
}

// PublishSpikeOrderPaid 发布秒杀订单支付消息
func (sp *SpikeProducer) PublishSpikeOrderPaid(ctx context.Context, data *SpikeOrderPaidData, traceID string) error {
	message := CreateSpikeOrderPaidMessage(data, traceID)

	return sp.publishMessage(ctx, message, SpikeExchange, &PublishOptions{
		MessageID: message.ID,
		Type:      string(message.Type),
		Timestamp: message.Timestamp,
		Headers: map[string]interface{}{
			"content-type":   "application/json",
			"trace-id":       traceID,
			"spike-order-id": data.SpikeOrderID,
			"user-id":        data.UserID,
			"payment-method": data.PaymentMethod,
		},
		Priority: 8, // 最高优先级
	})
}

// PublishSpikeOrderExpired 发布秒杀订单过期消息
func (sp *SpikeProducer) PublishSpikeOrderExpired(ctx context.Context, data *SpikeOrderExpiredData, traceID string) error {
	message := CreateSpikeOrderExpiredMessage(data, traceID)

	return sp.publishMessage(ctx, message, SpikeExchange, &PublishOptions{
		MessageID: message.ID,
		Type:      string(message.Type),
		Timestamp: message.Timestamp,
		Headers: map[string]interface{}{
			"content-type":    "application/json",
			"trace-id":        traceID,
			"spike-event-id":  data.SpikeEventID,
			"user-id":         data.UserID,
			"idempotency-key": data.IdempotencyKey,
		},
		Priority: 6, // 较高优先级
	})
}

// PublishSpikeOrderCancelled 发布秒杀订单取消消息
func (sp *SpikeProducer) PublishSpikeOrderCancelled(ctx context.Context, data *SpikeOrderCancelledData, traceID string) error {
	message := CreateSpikeOrderCancelledMessage(data, traceID)

	return sp.publishMessage(ctx, message, SpikeExchange, &PublishOptions{
		MessageID: message.ID,
		Type:      string(message.Type),
		Timestamp: message.Timestamp,
		Headers: map[string]interface{}{
			"content-type":    "application/json",
			"trace-id":        traceID,
			"spike-event-id":  data.SpikeEventID,
			"user-id":         data.UserID,
			"idempotency-key": data.IdempotencyKey,
			"cancel-reason":   data.Reason,
		},
		Priority: 6, // 较高优先级
	})
}

// PublishStockRestore 发布库存恢复消息
func (sp *SpikeProducer) PublishStockRestore(ctx context.Context, data *StockRestoreData, traceID string) error {
	message := CreateStockRestoreMessage(data, traceID)

	return sp.publishMessage(ctx, message, SpikeExchange, &PublishOptions{
		MessageID: message.ID,
		Type:      string(message.Type),
		Timestamp: message.Timestamp,
		Headers: map[string]interface{}{
			"content-type":    "application/json",
			"trace-id":        traceID,
			"spike-event-id":  data.SpikeEventID,
			"product-id":      data.ProductID,
			"user-id":         data.UserID,
			"idempotency-key": data.IdempotencyKey,
			"restore-reason":  data.Reason,
		},
		Priority: 7, // 高优先级（库存恢复很重要）
	})
}

// PublishNotification 发布通知消息
func (sp *SpikeProducer) PublishNotification(ctx context.Context, data *NotificationData, traceID string) error {
	message := CreateNotificationMessage(data, traceID)

	priority := uint8(3) // 默认优先级
	switch data.Priority {
	case "high":
		priority = 7
	case "normal":
		priority = 5
	case "low":
		priority = 3
	}

	options := &PublishOptions{
		MessageID: message.ID,
		Type:      string(message.Type),
		Timestamp: message.Timestamp,
		Headers: map[string]interface{}{
			"content-type":       "application/json",
			"trace-id":           traceID,
			"user-id":            data.UserID,
			"notification-type":  data.Type,
			"notification-title": data.Title,
			"priority":           data.Priority,
		},
		Priority: priority,
	}

	// 设置过期时间
	if data.ExpireAt != nil {
		expiration := time.Until(*data.ExpireAt)
		if expiration > 0 {
			options.Expiration = fmt.Sprintf("%d", int64(expiration/time.Millisecond))
		}
	}

	return sp.publishMessage(ctx, message, SpikeExchange, options)
}

// PublishDelayedMessage 发布延时消息
func (sp *SpikeProducer) PublishDelayedMessage(ctx context.Context, message *SpikeMessage, delay time.Duration) error {
	messageBytes, err := message.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	options := &PublishOptions{
		MessageID: message.ID,
		Type:      string(message.Type),
		Timestamp: message.Timestamp,
		Headers: map[string]interface{}{
			"content-type": "application/json",
			"trace-id":     message.TraceID,
			"x-delay":      int64(delay / time.Millisecond), // 延时时间（毫秒）
		},
	}

	return sp.producer.Publish(ctx, SpikeDelayExchange, message.GetRouterKey(), messageBytes, options)
}

// PublishBatch 批量发布消息
func (sp *SpikeProducer) PublishBatch(ctx context.Context, messages []*SpikeMessage) error {
	for _, message := range messages {
		if err := sp.publishMessage(ctx, message, SpikeExchange, nil); err != nil {
			sp.logger.Error("批量发布消息失败",
				zap.String("message_id", message.ID),
				zap.String("message_type", string(message.Type)),
				zap.Error(err))
			return err
		}
	}
	return nil
}

// publishMessage 发布消息的通用方法
func (sp *SpikeProducer) publishMessage(ctx context.Context, message *SpikeMessage, exchange string, options *PublishOptions) error {
	messageBytes, err := message.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	routingKey := message.GetRouterKey()

	// 设置默认选项
	if options == nil {
		options = &PublishOptions{}
	}
	if options.Headers == nil {
		options.Headers = make(map[string]interface{})
	}

	// 添加消息默认头信息
	options.Headers["message-id"] = message.ID
	options.Headers["message-type"] = string(message.Type)
	options.Headers["message-version"] = message.Version
	options.Headers["message-source"] = message.Source
	options.Headers["content-type"] = "application/json"

	// 记录发布日志
	sp.logger.Info("发布秒杀消息",
		zap.String("message_id", message.ID),
		zap.String("message_type", string(message.Type)),
		zap.String("exchange", exchange),
		zap.String("routing_key", routingKey),
		zap.String("trace_id", message.TraceID))

	// 发布消息
	return sp.producer.Publish(ctx, exchange, routingKey, messageBytes, options)
}

// SetupInfrastructure 设置基础设施（队列、交换机等）
func (sp *SpikeProducer) SetupInfrastructure(ctx context.Context) error {
	return sp.qm.SetupQueues(ctx)
}

// GetStats 获取生产者统计信息
func (sp *SpikeProducer) GetStats() ProducerStats {
	return sp.producer.GetStats()
}

// GetQueueInfo 获取队列信息
func (sp *SpikeProducer) GetQueueInfo(ctx context.Context, queueName string) (*QueueInfo, error) {
	return sp.qm.GetQueueInfo(ctx, queueName)
}

// GetAllQueuesInfo 获取所有队列信息
func (sp *SpikeProducer) GetAllQueuesInfo(ctx context.Context) ([]*QueueInfo, error) {
	return sp.qm.GetAllQueuesInfo(ctx)
}

// Close 关闭生产者
func (sp *SpikeProducer) Close() error {
	return sp.producer.Close()
}

// SpikeProducerWithRetry 带重试的秒杀生产者包装器
type SpikeProducerWithRetry struct {
	*SpikeProducer
	maxRetries    int
	retryInterval time.Duration
}

// NewSpikeProducerWithRetry 创建带重试的秒杀生产者
func NewSpikeProducerWithRetry(producer *SpikeProducer, maxRetries int, retryInterval time.Duration) *SpikeProducerWithRetry {
	return &SpikeProducerWithRetry{
		SpikeProducer: producer,
		maxRetries:    maxRetries,
		retryInterval: retryInterval,
	}
}

// PublishWithRetry 带重试的消息发布
func (spr *SpikeProducerWithRetry) PublishWithRetry(ctx context.Context, publishFunc func() error) error {
	var lastErr error

	for attempt := 0; attempt <= spr.maxRetries; attempt++ {
		err := publishFunc()
		if err == nil {
			return nil
		}

		lastErr = err
		spr.logger.Warn("消息发布失败，准备重试",
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", spr.maxRetries+1),
			zap.Error(err))

		if attempt < spr.maxRetries {
			select {
			case <-time.After(spr.retryInterval):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("failed to publish message after %d attempts: %w", spr.maxRetries+1, lastErr)
}

// PublishSpikeOrderCreatedWithRetry 带重试的秒杀订单创建消息发布
func (spr *SpikeProducerWithRetry) PublishSpikeOrderCreatedWithRetry(ctx context.Context, data *SpikeOrderCreatedData, traceID string) error {
	return spr.PublishWithRetry(ctx, func() error {
		return spr.PublishSpikeOrderCreated(ctx, data, traceID)
	})
}
