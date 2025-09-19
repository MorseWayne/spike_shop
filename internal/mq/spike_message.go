// Package mq 提供秒杀相关的消息定义和处理
package mq

import (
	"encoding/json"
	"fmt"
	"time"
)

// MessageType 消息类型
type MessageType string

const (
	// 秒杀订单相关消息
	MessageTypeSpikeOrderCreated   MessageType = "spike_order_created"   // 秒杀订单创建
	MessageTypeSpikeOrderPaid      MessageType = "spike_order_paid"      // 秒杀订单支付
	MessageTypeSpikeOrderExpired   MessageType = "spike_order_expired"   // 秒杀订单过期
	MessageTypeSpikeOrderCancelled MessageType = "spike_order_cancelled" // 秒杀订单取消

	// 库存相关消息
	MessageTypeStockRestore MessageType = "stock_restore" // 库存恢复
	MessageTypeStockWarning MessageType = "stock_warning" // 库存预警

	// 通知相关消息
	MessageTypeNotification      MessageType = "notification"       // 用户通知
	MessageTypeOrderConfirmation MessageType = "order_confirmation" // 订单确认通知
)

// SpikeMessage 秒杀消息基础结构
type SpikeMessage struct {
	// 消息基础信息
	ID        string      `json:"id"`        // 消息唯一ID
	Type      MessageType `json:"type"`      // 消息类型
	Version   string      `json:"version"`   // 消息版本
	Timestamp time.Time   `json:"timestamp"` // 消息时间戳
	Source    string      `json:"source"`    // 消息源
	TraceID   string      `json:"trace_id"`  // 链路追踪ID

	// 重试相关
	RetryCount int `json:"retry_count"` // 重试次数
	MaxRetries int `json:"max_retries"` // 最大重试次数

	// 业务数据
	Data interface{} `json:"data"` // 消息数据

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"` // 额外元数据
}

// SpikeOrderCreatedData 秒杀订单创建消息数据
type SpikeOrderCreatedData struct {
	SpikeOrderID   int64     `json:"spike_order_id"`  // 秒杀订单ID
	SpikeEventID   int64     `json:"spike_event_id"`  // 秒杀活动ID
	UserID         int64     `json:"user_id"`         // 用户ID
	ProductID      int64     `json:"product_id"`      // 商品ID
	Quantity       int64     `json:"quantity"`        // 购买数量
	SpikePrice     float64   `json:"spike_price"`     // 秒杀价格
	TotalAmount    float64   `json:"total_amount"`    // 总金额
	IdempotencyKey string    `json:"idempotency_key"` // 幂等键
	ExpireAt       time.Time `json:"expire_at"`       // 过期时间
	CreatedAt      time.Time `json:"created_at"`      // 创建时间
}

// SpikeOrderPaidData 秒杀订单支付消息数据
type SpikeOrderPaidData struct {
	SpikeOrderID  int64     `json:"spike_order_id"` // 秒杀订单ID
	OrderID       int64     `json:"order_id"`       // 普通订单ID
	UserID        int64     `json:"user_id"`        // 用户ID
	PaymentMethod string    `json:"payment_method"` // 支付方式
	PaidAmount    float64   `json:"paid_amount"`    // 支付金额
	PaidAt        time.Time `json:"paid_at"`        // 支付时间
	TransactionID string    `json:"transaction_id"` // 交易ID
}

// SpikeOrderExpiredData 秒杀订单过期消息数据
type SpikeOrderExpiredData struct {
	SpikeOrderID   int64     `json:"spike_order_id"`  // 秒杀订单ID
	SpikeEventID   int64     `json:"spike_event_id"`  // 秒杀活动ID
	UserID         int64     `json:"user_id"`         // 用户ID
	ProductID      int64     `json:"product_id"`      // 商品ID
	Quantity       int64     `json:"quantity"`        // 需要恢复的库存数量
	ExpiredAt      time.Time `json:"expired_at"`      // 过期时间
	IdempotencyKey string    `json:"idempotency_key"` // 幂等键
}

// SpikeOrderCancelledData 秒杀订单取消消息数据
type SpikeOrderCancelledData struct {
	SpikeOrderID   int64     `json:"spike_order_id"`  // 秒杀订单ID
	SpikeEventID   int64     `json:"spike_event_id"`  // 秒杀活动ID
	UserID         int64     `json:"user_id"`         // 用户ID
	ProductID      int64     `json:"product_id"`      // 商品ID
	Quantity       int64     `json:"quantity"`        // 需要恢复的库存数量
	Reason         string    `json:"reason"`          // 取消原因
	CancelledAt    time.Time `json:"cancelled_at"`    // 取消时间
	IdempotencyKey string    `json:"idempotency_key"` // 幂等键
}

// StockRestoreData 库存恢复消息数据
type StockRestoreData struct {
	SpikeEventID   int64     `json:"spike_event_id"`  // 秒杀活动ID
	ProductID      int64     `json:"product_id"`      // 商品ID
	UserID         int64     `json:"user_id"`         // 用户ID
	Quantity       int64     `json:"quantity"`        // 恢复数量
	Reason         string    `json:"reason"`          // 恢复原因
	SourceOrderID  int64     `json:"source_order_id"` // 来源订单ID
	IdempotencyKey string    `json:"idempotency_key"` // 幂等键
	RestoreAt      time.Time `json:"restore_at"`      // 恢复时间
}

// NotificationData 通知消息数据
type NotificationData struct {
	UserID      int64                  `json:"user_id"`      // 用户ID
	Type        string                 `json:"type"`         // 通知类型
	Title       string                 `json:"title"`        // 通知标题
	Content     string                 `json:"content"`      // 通知内容
	Data        map[string]interface{} `json:"data"`         // 额外数据
	Priority    string                 `json:"priority"`     // 优先级: low, normal, high
	Channels    []string               `json:"channels"`     // 通知渠道: email, sms, push
	ScheduledAt *time.Time             `json:"scheduled_at"` // 定时发送时间
	ExpireAt    *time.Time             `json:"expire_at"`    // 过期时间
}

// ToJSON 将消息转换为JSON字节数组
func (m *SpikeMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON 从JSON字节数组解析消息
func (m *SpikeMessage) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}

// GetDataAs 获取指定类型的数据
func (m *SpikeMessage) GetDataAs(target interface{}) error {
	dataBytes, err := json.Marshal(m.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(dataBytes, target)
}

// CanRetry 判断消息是否可以重试
func (m *SpikeMessage) CanRetry() bool {
	return m.RetryCount < m.MaxRetries
}

// IncRetryCount 增加重试次数
func (m *SpikeMessage) IncRetryCount() {
	m.RetryCount++
}

// IsExpired 判断消息是否过期（基于业务逻辑）
func (m *SpikeMessage) IsExpired() bool {
	// 消息超过1小时认为过期
	return time.Since(m.Timestamp) > time.Hour
}

// GetRouterKey 获取路由键
func (m *SpikeMessage) GetRouterKey() string {
	switch m.Type {
	case MessageTypeSpikeOrderCreated:
		return "spike.order.created"
	case MessageTypeSpikeOrderPaid:
		return "spike.order.paid"
	case MessageTypeSpikeOrderExpired:
		return "spike.order.expired"
	case MessageTypeSpikeOrderCancelled:
		return "spike.order.cancelled"
	case MessageTypeStockRestore:
		return "spike.stock.restore"
	case MessageTypeStockWarning:
		return "spike.stock.warning"
	case MessageTypeNotification:
		return "notification.send"
	case MessageTypeOrderConfirmation:
		return "notification.order.confirmation"
	default:
		return "spike.general"
	}
}

// SpikeMessageBuilder 消息构建器
type SpikeMessageBuilder struct {
	message *SpikeMessage
}

// NewSpikeMessageBuilder 创建消息构建器
func NewSpikeMessageBuilder() *SpikeMessageBuilder {
	return &SpikeMessageBuilder{
		message: &SpikeMessage{
			Version:    "1.0",
			Timestamp:  time.Now(),
			Source:     "spike-service",
			RetryCount: 0,
			MaxRetries: 3,
			Metadata:   make(map[string]interface{}),
		},
	}
}

// WithID 设置消息ID
func (b *SpikeMessageBuilder) WithID(id string) *SpikeMessageBuilder {
	b.message.ID = id
	return b
}

// WithType 设置消息类型
func (b *SpikeMessageBuilder) WithType(msgType MessageType) *SpikeMessageBuilder {
	b.message.Type = msgType
	return b
}

// WithTraceID 设置追踪ID
func (b *SpikeMessageBuilder) WithTraceID(traceID string) *SpikeMessageBuilder {
	b.message.TraceID = traceID
	return b
}

// WithData 设置消息数据
func (b *SpikeMessageBuilder) WithData(data interface{}) *SpikeMessageBuilder {
	b.message.Data = data
	return b
}

// WithMaxRetries 设置最大重试次数
func (b *SpikeMessageBuilder) WithMaxRetries(maxRetries int) *SpikeMessageBuilder {
	b.message.MaxRetries = maxRetries
	return b
}

// WithMetadata 添加元数据
func (b *SpikeMessageBuilder) WithMetadata(key string, value interface{}) *SpikeMessageBuilder {
	b.message.Metadata[key] = value
	return b
}

// Build 构建消息
func (b *SpikeMessageBuilder) Build() *SpikeMessage {
	return b.message
}

// CreateSpikeOrderCreatedMessage 创建秒杀订单创建消息
func CreateSpikeOrderCreatedMessage(data *SpikeOrderCreatedData, traceID string) *SpikeMessage {
	return NewSpikeMessageBuilder().
		WithID(generateMessageID()).
		WithType(MessageTypeSpikeOrderCreated).
		WithTraceID(traceID).
		WithData(data).
		WithMetadata("user_id", data.UserID).
		WithMetadata("spike_event_id", data.SpikeEventID).
		Build()
}

// CreateSpikeOrderPaidMessage 创建秒杀订单支付消息
func CreateSpikeOrderPaidMessage(data *SpikeOrderPaidData, traceID string) *SpikeMessage {
	return NewSpikeMessageBuilder().
		WithID(generateMessageID()).
		WithType(MessageTypeSpikeOrderPaid).
		WithTraceID(traceID).
		WithData(data).
		WithMetadata("user_id", data.UserID).
		WithMetadata("spike_order_id", data.SpikeOrderID).
		Build()
}

// CreateSpikeOrderExpiredMessage 创建秒杀订单过期消息
func CreateSpikeOrderExpiredMessage(data *SpikeOrderExpiredData, traceID string) *SpikeMessage {
	return NewSpikeMessageBuilder().
		WithID(generateMessageID()).
		WithType(MessageTypeSpikeOrderExpired).
		WithTraceID(traceID).
		WithData(data).
		WithMetadata("user_id", data.UserID).
		WithMetadata("spike_event_id", data.SpikeEventID).
		Build()
}

// CreateSpikeOrderCancelledMessage 创建秒杀订单取消消息
func CreateSpikeOrderCancelledMessage(data *SpikeOrderCancelledData, traceID string) *SpikeMessage {
	return NewSpikeMessageBuilder().
		WithID(generateMessageID()).
		WithType(MessageTypeSpikeOrderCancelled).
		WithTraceID(traceID).
		WithData(data).
		WithMetadata("user_id", data.UserID).
		WithMetadata("spike_event_id", data.SpikeEventID).
		Build()
}

// CreateStockRestoreMessage 创建库存恢复消息
func CreateStockRestoreMessage(data *StockRestoreData, traceID string) *SpikeMessage {
	return NewSpikeMessageBuilder().
		WithID(generateMessageID()).
		WithType(MessageTypeStockRestore).
		WithTraceID(traceID).
		WithData(data).
		WithMetadata("spike_event_id", data.SpikeEventID).
		WithMetadata("product_id", data.ProductID).
		Build()
}

// CreateNotificationMessage 创建通知消息
func CreateNotificationMessage(data *NotificationData, traceID string) *SpikeMessage {
	return NewSpikeMessageBuilder().
		WithID(generateMessageID()).
		WithType(MessageTypeNotification).
		WithTraceID(traceID).
		WithData(data).
		WithMetadata("user_id", data.UserID).
		WithMetadata("notification_type", data.Type).
		Build()
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	// 使用时间戳 + 随机数生成消息ID
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000000)
}
