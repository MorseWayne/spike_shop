// Package mq 提供秒杀相关的队列定义和服务
package mq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// 秒杀相关的交换机和队列常量
const (
	// 交换机
	SpikeExchange      = "spike.exchange"       // 秒杀主交换机
	SpikeDelayExchange = "spike.delay.exchange" // 延时交换机
	SpikeDLXExchange   = "spike.dlx.exchange"   // 死信交换机

	// 队列
	SpikeOrderQueue        = "spike.order.queue"         // 秒杀订单队列
	SpikeOrderDelayQueue   = "spike.order.delay.queue"   // 秒杀订单延时队列
	SpikeStockRestoreQueue = "spike.stock.restore.queue" // 库存恢复队列
	SpikeNotificationQueue = "spike.notification.queue"  // 通知队列
	SpikeDLXQueue          = "spike.dlx.queue"           // 死信队列

	// 路由键
	SpikeOrderCreatedRoutingKey      = "spike.order.created"
	SpikeOrderPaidRoutingKey         = "spike.order.paid"
	SpikeOrderExpiredRoutingKey      = "spike.order.expired"
	SpikeOrderCancelledRoutingKey    = "spike.order.cancelled"
	SpikeStockRestoreRoutingKey      = "spike.stock.restore"
	SpikeNotificationRoutingKey      = "notification.send"
	SpikeOrderConfirmationRoutingKey = "notification.order.confirmation"
)

// SpikeQueueManager 秒杀队列管理器
type SpikeQueueManager struct {
	cm     *ConnectionManager
	logger *zap.Logger
}

// NewSpikeQueueManager 创建秒杀队列管理器
func NewSpikeQueueManager(cm *ConnectionManager, logger *zap.Logger) *SpikeQueueManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SpikeQueueManager{
		cm:     cm,
		logger: logger,
	}
}

// SetupQueues 设置所有队列和交换机
func (qm *SpikeQueueManager) SetupQueues(ctx context.Context) error {
	ch, err := qm.cm.GetChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer qm.cm.ReturnChannel(ch)

	// 声明交换机
	if err := qm.declareExchanges(ch); err != nil {
		return fmt.Errorf("failed to declare exchanges: %w", err)
	}

	// 声明队列
	if err := qm.declareQueues(ch); err != nil {
		return fmt.Errorf("failed to declare queues: %w", err)
	}

	// 绑定队列
	if err := qm.bindQueues(ch); err != nil {
		return fmt.Errorf("failed to bind queues: %w", err)
	}

	qm.logger.Info("秒杀队列设置完成")
	return nil
}

// declareExchanges 声明交换机
func (qm *SpikeQueueManager) declareExchanges(ch *amqp.Channel) error {
	exchanges := []struct {
		name       string
		kind       string
		durable    bool
		autoDelete bool
		internal   bool
		noWait     bool
		args       amqp.Table
	}{
		{
			name:       SpikeExchange,
			kind:       "topic",
			durable:    true,
			autoDelete: false,
			internal:   false,
			noWait:     false,
			args:       nil,
		},
		{
			name:       SpikeDelayExchange,
			kind:       "x-delayed-message",
			durable:    true,
			autoDelete: false,
			internal:   false,
			noWait:     false,
			args:       amqp.Table{"x-delayed-type": "topic"},
		},
		{
			name:       SpikeDLXExchange,
			kind:       "topic",
			durable:    true,
			autoDelete: false,
			internal:   false,
			noWait:     false,
			args:       nil,
		},
	}

	for _, exchange := range exchanges {
		err := ch.ExchangeDeclare(
			exchange.name,
			exchange.kind,
			exchange.durable,
			exchange.autoDelete,
			exchange.internal,
			exchange.noWait,
			exchange.args,
		)
		if err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", exchange.name, err)
		}
		qm.logger.Debug("声明交换机", zap.String("exchange", exchange.name))
	}

	return nil
}

// declareQueues 声明队列
func (qm *SpikeQueueManager) declareQueues(ch *amqp.Channel) error {
	queues := []struct {
		name       string
		durable    bool
		autoDelete bool
		exclusive  bool
		noWait     bool
		args       amqp.Table
	}{
		{
			name:       SpikeOrderQueue,
			durable:    true,
			autoDelete: false,
			exclusive:  false,
			noWait:     false,
			args: amqp.Table{
				"x-dead-letter-exchange":    SpikeDLXExchange,
				"x-dead-letter-routing-key": "failed.order",
				"x-max-retries":             3,
			},
		},
		{
			name:       SpikeOrderDelayQueue,
			durable:    true,
			autoDelete: false,
			exclusive:  false,
			noWait:     false,
			args: amqp.Table{
				"x-message-ttl":             30 * 60 * 1000, // 30分钟TTL
				"x-dead-letter-exchange":    SpikeExchange,
				"x-dead-letter-routing-key": SpikeOrderExpiredRoutingKey,
			},
		},
		{
			name:       SpikeStockRestoreQueue,
			durable:    true,
			autoDelete: false,
			exclusive:  false,
			noWait:     false,
			args: amqp.Table{
				"x-dead-letter-exchange":    SpikeDLXExchange,
				"x-dead-letter-routing-key": "failed.stock",
			},
		},
		{
			name:       SpikeNotificationQueue,
			durable:    true,
			autoDelete: false,
			exclusive:  false,
			noWait:     false,
			args: amqp.Table{
				"x-dead-letter-exchange":    SpikeDLXExchange,
				"x-dead-letter-routing-key": "failed.notification",
			},
		},
		{
			name:       SpikeDLXQueue,
			durable:    true,
			autoDelete: false,
			exclusive:  false,
			noWait:     false,
			args:       nil,
		},
	}

	for _, queue := range queues {
		_, err := ch.QueueDeclare(
			queue.name,
			queue.durable,
			queue.autoDelete,
			queue.exclusive,
			queue.noWait,
			queue.args,
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue.name, err)
		}
		qm.logger.Debug("声明队列", zap.String("queue", queue.name))
	}

	return nil
}

// bindQueues 绑定队列
func (qm *SpikeQueueManager) bindQueues(ch *amqp.Channel) error {
	bindings := []struct {
		queue      string
		exchange   string
		routingKey string
		noWait     bool
		args       amqp.Table
	}{
		// 绑定秒杀订单队列
		{SpikeOrderQueue, SpikeExchange, SpikeOrderCreatedRoutingKey, false, nil},
		{SpikeOrderQueue, SpikeExchange, SpikeOrderPaidRoutingKey, false, nil},

		// 绑定库存恢复队列
		{SpikeStockRestoreQueue, SpikeExchange, SpikeOrderExpiredRoutingKey, false, nil},
		{SpikeStockRestoreQueue, SpikeExchange, SpikeOrderCancelledRoutingKey, false, nil},
		{SpikeStockRestoreQueue, SpikeExchange, SpikeStockRestoreRoutingKey, false, nil},

		// 绑定通知队列
		{SpikeNotificationQueue, SpikeExchange, SpikeNotificationRoutingKey, false, nil},
		{SpikeNotificationQueue, SpikeExchange, SpikeOrderConfirmationRoutingKey, false, nil},

		// 绑定死信队列
		{SpikeDLXQueue, SpikeDLXExchange, "failed.*", false, nil},

		// 绑定延时队列
		{SpikeOrderDelayQueue, SpikeDelayExchange, "delay.order.*", false, nil},
	}

	for _, binding := range bindings {
		err := ch.QueueBind(
			binding.queue,
			binding.routingKey,
			binding.exchange,
			binding.noWait,
			binding.args,
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w",
				binding.queue, binding.exchange, err)
		}
		qm.logger.Debug("绑定队列",
			zap.String("queue", binding.queue),
			zap.String("exchange", binding.exchange),
			zap.String("routing_key", binding.routingKey))
	}

	return nil
}

// GetQueueInfo 获取队列信息
func (qm *SpikeQueueManager) GetQueueInfo(ctx context.Context, queueName string) (*QueueInfo, error) {
	ch, err := qm.cm.GetChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	defer qm.cm.ReturnChannel(ch)

	queue, err := ch.QueueInspect(queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect queue %s: %w", queueName, err)
	}

	return &QueueInfo{
		Name:      queue.Name,
		Messages:  queue.Messages,
		Consumers: queue.Consumers,
	}, nil
}

// QueueInfo 队列信息
type QueueInfo struct {
	Name      string `json:"name"`
	Messages  int    `json:"messages"`
	Consumers int    `json:"consumers"`
}

// GetAllQueuesInfo 获取所有秒杀相关队列信息
func (qm *SpikeQueueManager) GetAllQueuesInfo(ctx context.Context) ([]*QueueInfo, error) {
	queueNames := []string{
		SpikeOrderQueue,
		SpikeOrderDelayQueue,
		SpikeStockRestoreQueue,
		SpikeNotificationQueue,
		SpikeDLXQueue,
	}

	var queuesInfo []*QueueInfo
	for _, queueName := range queueNames {
		info, err := qm.GetQueueInfo(ctx, queueName)
		if err != nil {
			qm.logger.Error("获取队列信息失败",
				zap.String("queue", queueName),
				zap.Error(err))
			continue
		}
		queuesInfo = append(queuesInfo, info)
	}

	return queuesInfo, nil
}

// PurgeQueue 清空队列
func (qm *SpikeQueueManager) PurgeQueue(ctx context.Context, queueName string) (int, error) {
	ch, err := qm.cm.GetChannel()
	if err != nil {
		return 0, fmt.Errorf("failed to get channel: %w", err)
	}
	defer qm.cm.ReturnChannel(ch)

	count, err := ch.QueuePurge(queueName, false)
	if err != nil {
		return 0, fmt.Errorf("failed to purge queue %s: %w", queueName, err)
	}

	qm.logger.Info("清空队列",
		zap.String("queue", queueName),
		zap.Int("purged_messages", count))

	return count, nil
}

// DeleteQueue 删除队列
func (qm *SpikeQueueManager) DeleteQueue(ctx context.Context, queueName string, ifUnused, ifEmpty bool) (int, error) {
	ch, err := qm.cm.GetChannel()
	if err != nil {
		return 0, fmt.Errorf("failed to get channel: %w", err)
	}
	defer qm.cm.ReturnChannel(ch)

	count, err := ch.QueueDelete(queueName, ifUnused, ifEmpty, false)
	if err != nil {
		return 0, fmt.Errorf("failed to delete queue %s: %w", queueName, err)
	}

	qm.logger.Info("删除队列",
		zap.String("queue", queueName),
		zap.Int("deleted_messages", count))

	return count, nil
}
