// Package mq 提供秒杀消息消费者服务
package mq

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/cache"
	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/repo"
)

// SpikeConsumer 秒杀消息消费者
type SpikeConsumer struct {
	cm     *ConnectionManager
	logger *zap.Logger

	// 仓储层
	spikeEventRepo repo.SpikeEventRepository
	spikeOrderRepo repo.SpikeOrderRepository
	inventoryRepo  repo.InventoryRepository

	// 缓存层
	spikeCache *cache.SpikeCache

	// 消费者实例
	consumers map[string]*Consumer

	// 数据库连接
	db *sql.DB
}

// NewSpikeConsumer 创建秒杀消息消费者
func NewSpikeConsumer(
	cm *ConnectionManager,
	db *sql.DB,
	spikeEventRepo repo.SpikeEventRepository,
	spikeOrderRepo repo.SpikeOrderRepository,
	inventoryRepo repo.InventoryRepository,
	spikeCache *cache.SpikeCache,
	logger *zap.Logger,
) *SpikeConsumer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SpikeConsumer{
		cm:             cm,
		db:             db,
		spikeEventRepo: spikeEventRepo,
		spikeOrderRepo: spikeOrderRepo,
		inventoryRepo:  inventoryRepo,
		spikeCache:     spikeCache,
		logger:         logger,
		consumers:      make(map[string]*Consumer),
	}
}

// StartConsumers 启动所有消费者
func (sc *SpikeConsumer) StartConsumers(ctx context.Context) error {
	// 启动秒杀订单消费者
	if err := sc.startOrderConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start order consumer: %w", err)
	}

	// 启动库存恢复消费者
	if err := sc.startStockRestoreConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start stock restore consumer: %w", err)
	}

	// 启动通知消费者
	if err := sc.startNotificationConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start notification consumer: %w", err)
	}

	sc.logger.Info("所有秒杀消费者启动成功")
	return nil
}

// startOrderConsumer 启动订单消费者
func (sc *SpikeConsumer) startOrderConsumer(ctx context.Context) error {
	config := &ConsumerConfig{
		PrefetchCount:       5,
		AutoAck:             false,
		EnableRetry:         true,
		MaxRetryAttempts:    3,
		RetryInterval:       2 * time.Second,
		EnableDLX:           true,
		DLXExchange:         SpikeDLXExchange,
		DLXRoutingKey:       "failed.order",
		ConsumeTimeout:      30 * time.Second,
		ConcurrentConsumers: 2,
	}

	consumer := NewConsumer(sc.cm, config, sc.logger)
	consumer.SetHandler(sc.handleOrderMessage)

	if err := consumer.StartConsuming(ctx, SpikeOrderQueue); err != nil {
		return err
	}

	sc.consumers["order"] = consumer
	return nil
}

// startStockRestoreConsumer 启动库存恢复消费者
func (sc *SpikeConsumer) startStockRestoreConsumer(ctx context.Context) error {
	config := &ConsumerConfig{
		PrefetchCount:       10,
		AutoAck:             false,
		EnableRetry:         true,
		MaxRetryAttempts:    5,
		RetryInterval:       1 * time.Second,
		EnableDLX:           true,
		DLXExchange:         SpikeDLXExchange,
		DLXRoutingKey:       "failed.stock",
		ConsumeTimeout:      15 * time.Second,
		ConcurrentConsumers: 3,
	}

	consumer := NewConsumer(sc.cm, config, sc.logger)
	consumer.SetHandler(sc.handleStockRestoreMessage)

	if err := consumer.StartConsuming(ctx, SpikeStockRestoreQueue); err != nil {
		return err
	}

	sc.consumers["stock"] = consumer
	return nil
}

// startNotificationConsumer 启动通知消费者
func (sc *SpikeConsumer) startNotificationConsumer(ctx context.Context) error {
	config := &ConsumerConfig{
		PrefetchCount:       20,
		AutoAck:             false,
		EnableRetry:         true,
		MaxRetryAttempts:    2,
		RetryInterval:       500 * time.Millisecond,
		EnableDLX:           true,
		DLXExchange:         SpikeDLXExchange,
		DLXRoutingKey:       "failed.notification",
		ConsumeTimeout:      10 * time.Second,
		ConcurrentConsumers: 1,
	}

	consumer := NewConsumer(sc.cm, config, sc.logger)
	consumer.SetHandler(sc.handleNotificationMessage)

	if err := consumer.StartConsuming(ctx, SpikeNotificationQueue); err != nil {
		return err
	}

	sc.consumers["notification"] = consumer
	return nil
}

// handleOrderMessage 处理订单消息
func (sc *SpikeConsumer) handleOrderMessage(ctx context.Context, delivery amqp.Delivery) error {
	// 解析消息
	var message SpikeMessage
	if err := message.FromJSON(delivery.Body); err != nil {
		sc.logger.Error("解析订单消息失败", zap.Error(err), zap.ByteString("body", delivery.Body))
		return &NonRetryableError{Err: fmt.Errorf("invalid message format: %w", err)}
	}

	sc.logger.Info("处理订单消息",
		zap.String("message_id", message.ID),
		zap.String("message_type", string(message.Type)),
		zap.String("trace_id", message.TraceID))

	// 根据消息类型分发处理
	switch message.Type {
	case MessageTypeSpikeOrderCreated:
		return sc.handleSpikeOrderCreated(ctx, &message)
	case MessageTypeSpikeOrderPaid:
		return sc.handleSpikeOrderPaid(ctx, &message)
	default:
		sc.logger.Warn("未知的订单消息类型", zap.String("type", string(message.Type)))
		return &NonRetryableError{Err: fmt.Errorf("unknown message type: %s", message.Type)}
	}
}

// handleSpikeOrderCreated 处理秒杀订单创建消息
func (sc *SpikeConsumer) handleSpikeOrderCreated(ctx context.Context, message *SpikeMessage) error {
	var data SpikeOrderCreatedData
	if err := message.GetDataAs(&data); err != nil {
		return &NonRetryableError{Err: fmt.Errorf("failed to parse spike order created data: %w", err)}
	}

	// 幂等性检查
	if err := sc.checkIdempotency(ctx, data.IdempotencyKey, message.ID); err != nil {
		if err == ErrDuplicateMessage {
			sc.logger.Info("重复消息，跳过处理",
				zap.String("idempotency_key", data.IdempotencyKey),
				zap.String("message_id", message.ID))
			return nil // 重复消息，直接返回成功
		}
		return err
	}

	// 开始数据库事务
	tx, err := sc.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 验证秒杀活动是否有效
	spikeEvent, err := sc.spikeEventRepo.GetByID(data.SpikeEventID)
	if err != nil {
		return fmt.Errorf("failed to get spike event: %w", err)
	}

	if !spikeEvent.IsActive() {
		return &NonRetryableError{Err: fmt.Errorf("spike event %d is not active", data.SpikeEventID)}
	}

	// 检查是否有足够库存
	if spikeEvent.SoldCount+data.Quantity > spikeEvent.SpikeStock {
		sc.logger.Warn("库存不足，恢复Redis库存",
			zap.Int64("spike_event_id", data.SpikeEventID),
			zap.Int64("sold_count", spikeEvent.SoldCount),
			zap.Int64("spike_stock", spikeEvent.SpikeStock),
			zap.Int64("requested_quantity", data.Quantity))

		// 恢复Redis库存
		_, err := sc.spikeCache.RestoreStock(ctx, data.SpikeEventID, data.UserID, data.Quantity)
		if err != nil {
			sc.logger.Error("恢复Redis库存失败", zap.Error(err))
		}

		return &NonRetryableError{Err: fmt.Errorf("insufficient stock")}
	}

	// 更新秒杀活动已售数量
	spikeEvent.SoldCount += data.Quantity
	if err := sc.spikeEventRepo.UpdateSoldCount(spikeEvent.ID, spikeEvent.SoldCount); err != nil {
		return fmt.Errorf("failed to update sold count: %w", err)
	}

	// 创建秒杀订单记录
	spikeOrder := &domain.SpikeOrder{
		SpikeEventID:   data.SpikeEventID,
		UserID:         data.UserID,
		Quantity:       data.Quantity,
		SpikePrice:     data.SpikePrice,
		TotalAmount:    data.TotalAmount,
		Status:         domain.SpikeOrderStatusPending,
		IdempotencyKey: data.IdempotencyKey,
		ExpireAt:       &data.ExpireAt,
		CreatedAt:      data.CreatedAt,
	}

	if err := sc.spikeOrderRepo.Create(spikeOrder); err != nil {
		return fmt.Errorf("failed to create spike order: %w", err)
	}

	// 消费库存
	if err := sc.inventoryRepo.ConsumeStock(data.ProductID, int(data.Quantity)); err != nil {
		return fmt.Errorf("failed to consume inventory: %w", err)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 标记幂等键处理完成
	if err := sc.markIdempotencyProcessed(ctx, data.IdempotencyKey, message.ID); err != nil {
		sc.logger.Error("标记幂等键处理完成失败", zap.Error(err))
	}

	sc.logger.Info("秒杀订单创建成功",
		zap.Int64("spike_order_id", spikeOrder.ID),
		zap.Int64("spike_event_id", data.SpikeEventID),
		zap.Int64("user_id", data.UserID),
		zap.String("idempotency_key", data.IdempotencyKey))

	return nil
}

// handleSpikeOrderPaid 处理秒杀订单支付消息
func (sc *SpikeConsumer) handleSpikeOrderPaid(ctx context.Context, message *SpikeMessage) error {
	var data SpikeOrderPaidData
	if err := message.GetDataAs(&data); err != nil {
		return &NonRetryableError{Err: fmt.Errorf("failed to parse spike order paid data: %w", err)}
	}

	// 开始数据库事务
	tx, err := sc.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 更新秒杀订单状态
	if err := sc.spikeOrderRepo.UpdatePaymentInfo(data.SpikeOrderID, data.PaidAt); err != nil {
		return fmt.Errorf("failed to update spike order payment info: %w", err)
	}

	// 关联普通订单ID
	if data.OrderID > 0 {
		if err := sc.spikeOrderRepo.UpdateOrderID(data.SpikeOrderID, data.OrderID); err != nil {
			return fmt.Errorf("failed to update order id: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	sc.logger.Info("秒杀订单支付处理成功",
		zap.Int64("spike_order_id", data.SpikeOrderID),
		zap.Int64("order_id", data.OrderID),
		zap.Int64("user_id", data.UserID),
		zap.String("payment_method", data.PaymentMethod))

	return nil
}

// handleStockRestoreMessage 处理库存恢复消息
func (sc *SpikeConsumer) handleStockRestoreMessage(ctx context.Context, delivery amqp.Delivery) error {
	// 解析消息
	var message SpikeMessage
	if err := message.FromJSON(delivery.Body); err != nil {
		sc.logger.Error("解析库存恢复消息失败", zap.Error(err))
		return &NonRetryableError{Err: fmt.Errorf("invalid message format: %w", err)}
	}

	sc.logger.Info("处理库存恢复消息",
		zap.String("message_id", message.ID),
		zap.String("message_type", string(message.Type)),
		zap.String("trace_id", message.TraceID))

	// 根据消息类型处理
	switch message.Type {
	case MessageTypeSpikeOrderExpired:
		return sc.handleSpikeOrderExpired(ctx, &message)
	case MessageTypeSpikeOrderCancelled:
		return sc.handleSpikeOrderCancelled(ctx, &message)
	case MessageTypeStockRestore:
		return sc.handleStockRestore(ctx, &message)
	default:
		sc.logger.Warn("未知的库存恢复消息类型", zap.String("type", string(message.Type)))
		return &NonRetryableError{Err: fmt.Errorf("unknown message type: %s", message.Type)}
	}
}

// handleSpikeOrderExpired 处理秒杀订单过期
func (sc *SpikeConsumer) handleSpikeOrderExpired(ctx context.Context, message *SpikeMessage) error {
	var data SpikeOrderExpiredData
	if err := message.GetDataAs(&data); err != nil {
		return &NonRetryableError{Err: fmt.Errorf("failed to parse spike order expired data: %w", err)}
	}

	// 幂等性检查
	if err := sc.checkIdempotency(ctx, data.IdempotencyKey, message.ID); err != nil {
		if err == ErrDuplicateMessage {
			return nil
		}
		return err
	}

	return sc.processStockRestore(ctx, data.SpikeEventID, data.UserID, data.ProductID,
		data.Quantity, "order_expired", data.SpikeOrderID, data.IdempotencyKey, message.ID)
}

// handleSpikeOrderCancelled 处理秒杀订单取消
func (sc *SpikeConsumer) handleSpikeOrderCancelled(ctx context.Context, message *SpikeMessage) error {
	var data SpikeOrderCancelledData
	if err := message.GetDataAs(&data); err != nil {
		return &NonRetryableError{Err: fmt.Errorf("failed to parse spike order cancelled data: %w", err)}
	}

	// 幂等性检查
	if err := sc.checkIdempotency(ctx, data.IdempotencyKey, message.ID); err != nil {
		if err == ErrDuplicateMessage {
			return nil
		}
		return err
	}

	return sc.processStockRestore(ctx, data.SpikeEventID, data.UserID, data.ProductID,
		data.Quantity, data.Reason, data.SpikeOrderID, data.IdempotencyKey, message.ID)
}

// handleStockRestore 处理库存恢复
func (sc *SpikeConsumer) handleStockRestore(ctx context.Context, message *SpikeMessage) error {
	var data StockRestoreData
	if err := message.GetDataAs(&data); err != nil {
		return &NonRetryableError{Err: fmt.Errorf("failed to parse stock restore data: %w", err)}
	}

	// 幂等性检查
	if err := sc.checkIdempotency(ctx, data.IdempotencyKey, message.ID); err != nil {
		if err == ErrDuplicateMessage {
			return nil
		}
		return err
	}

	return sc.processStockRestore(ctx, data.SpikeEventID, data.UserID, data.ProductID,
		data.Quantity, data.Reason, data.SourceOrderID, data.IdempotencyKey, message.ID)
}

// processStockRestore 处理库存恢复的通用方法
func (sc *SpikeConsumer) processStockRestore(ctx context.Context, spikeEventID, userID, productID, quantity int64,
	reason string, sourceOrderID int64, idempotencyKey, messageID string) error {

	// 开始数据库事务
	tx, err := sc.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 恢复秒杀活动库存
	spikeEvent, err := sc.spikeEventRepo.GetByID(spikeEventID)
	if err != nil {
		return fmt.Errorf("failed to get spike event: %w", err)
	}

	if spikeEvent.SoldCount >= quantity {
		spikeEvent.SoldCount -= quantity
		if err := sc.spikeEventRepo.UpdateSoldCount(spikeEvent.ID, spikeEvent.SoldCount); err != nil {
			return fmt.Errorf("failed to update sold count: %w", err)
		}
	}

	// 恢复商品库存
	if err := sc.inventoryRepo.AdjustStock(productID, int(quantity), reason); err != nil {
		return fmt.Errorf("failed to restore inventory: %w", err)
	}

	// 恢复Redis库存
	restoredStock, err := sc.spikeCache.RestoreStock(ctx, spikeEventID, userID, quantity)
	if err != nil {
		sc.logger.Error("恢复Redis库存失败", zap.Error(err))
		// Redis操作失败不影响数据库事务，只记录错误
	} else {
		sc.logger.Info("恢复Redis库存成功",
			zap.Int64("spike_event_id", spikeEventID),
			zap.Int64("restored_stock", restoredStock))
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 标记幂等键处理完成
	if err := sc.markIdempotencyProcessed(ctx, idempotencyKey, messageID); err != nil {
		sc.logger.Error("标记幂等键处理完成失败", zap.Error(err))
	}

	sc.logger.Info("库存恢复处理成功",
		zap.Int64("spike_event_id", spikeEventID),
		zap.Int64("product_id", productID),
		zap.Int64("user_id", userID),
		zap.Int64("quantity", quantity),
		zap.String("reason", reason),
		zap.Int64("source_order_id", sourceOrderID))

	return nil
}

// handleNotificationMessage 处理通知消息
func (sc *SpikeConsumer) handleNotificationMessage(ctx context.Context, delivery amqp.Delivery) error {
	// 解析消息
	var message SpikeMessage
	if err := message.FromJSON(delivery.Body); err != nil {
		sc.logger.Error("解析通知消息失败", zap.Error(err))
		return &NonRetryableError{Err: fmt.Errorf("invalid message format: %w", err)}
	}

	sc.logger.Info("处理通知消息",
		zap.String("message_id", message.ID),
		zap.String("message_type", string(message.Type)),
		zap.String("trace_id", message.TraceID))

	var data NotificationData
	if err := message.GetDataAs(&data); err != nil {
		return &NonRetryableError{Err: fmt.Errorf("failed to parse notification data: %w", err)}
	}

	// 这里可以集成各种通知渠道（邮件、短信、推送等）
	// 暂时只记录日志
	sc.logger.Info("发送通知",
		zap.Int64("user_id", data.UserID),
		zap.String("type", data.Type),
		zap.String("title", data.Title),
		zap.String("content", data.Content),
		zap.String("priority", data.Priority),
		zap.Strings("channels", data.Channels))

	// TODO: 实际的通知发送逻辑
	// - 邮件通知
	// - 短信通知
	// - App推送通知
	// - 站内信通知

	return nil
}

// ErrDuplicateMessage 重复消息错误
var ErrDuplicateMessage = fmt.Errorf("duplicate message")

// checkIdempotency 检查幂等性
func (sc *SpikeConsumer) checkIdempotency(ctx context.Context, idempotencyKey, messageID string) error {
	// 使用Redis检查幂等键
	processed := fmt.Sprintf("processed:%s", idempotencyKey)
	exists, err := sc.spikeCache.SetIdempotencyKey(ctx, processed, messageID, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check idempotency: %w", err)
	}

	if !exists {
		// 键已存在，说明消息已处理
		return ErrDuplicateMessage
	}

	return nil
}

// markIdempotencyProcessed 标记幂等键处理完成
func (sc *SpikeConsumer) markIdempotencyProcessed(ctx context.Context, idempotencyKey, messageID string) error {
	completed := fmt.Sprintf("completed:%s", idempotencyKey)
	_, err := sc.spikeCache.SetIdempotencyKey(ctx, completed, messageID, 24*time.Hour)
	return err
}

// StopConsumers 停止所有消费者
func (sc *SpikeConsumer) StopConsumers() error {
	for name, consumer := range sc.consumers {
		if err := consumer.StopConsuming(); err != nil {
			sc.logger.Error("停止消费者失败",
				zap.String("consumer", name),
				zap.Error(err))
		} else {
			sc.logger.Info("消费者停止成功", zap.String("consumer", name))
		}
	}
	return nil
}

// GetConsumerStats 获取所有消费者统计信息
func (sc *SpikeConsumer) GetConsumerStats() map[string]ConsumerStats {
	stats := make(map[string]ConsumerStats)
	for name, consumer := range sc.consumers {
		stats[name] = consumer.GetStats()
	}
	return stats
}
