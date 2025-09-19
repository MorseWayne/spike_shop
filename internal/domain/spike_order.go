// Package domain 定义秒杀订单相关的业务领域模型和核心业务规则。
package domain

import (
	"time"
)

// SpikeOrderStatus 定义秒杀订单状态类型
type SpikeOrderStatus string

const (
	SpikeOrderStatusPending   SpikeOrderStatus = "pending"   // 待支付
	SpikeOrderStatusPaid      SpikeOrderStatus = "paid"      // 已支付
	SpikeOrderStatusCancelled SpikeOrderStatus = "cancelled" // 已取消
	SpikeOrderStatusExpired   SpikeOrderStatus = "expired"   // 已过期
)

// SpikeOrder 表示秒杀订单领域模型
type SpikeOrder struct {
	ID             int64            `json:"id"`
	SpikeEventID   int64            `json:"spike_event_id"`
	UserID         int64            `json:"user_id"`
	OrderID        *int64           `json:"order_id"`
	Quantity       int64            `json:"quantity"`
	SpikePrice     float64          `json:"spike_price"`
	TotalAmount    float64          `json:"total_amount"`
	Status         SpikeOrderStatus `json:"status"`
	IdempotencyKey string           `json:"idempotency_key"`
	ExpireAt       *time.Time       `json:"expire_at"`
	PaidAt         *time.Time       `json:"paid_at"`
	CancelledAt    *time.Time       `json:"cancelled_at"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// IsPending 判断订单是否为待支付状态
func (s *SpikeOrder) IsPending() bool {
	return s.Status == SpikeOrderStatusPending
}

// IsPaid 判断订单是否已支付
func (s *SpikeOrder) IsPaid() bool {
	return s.Status == SpikeOrderStatusPaid
}

// IsCancelled 判断订单是否已取消
func (s *SpikeOrder) IsCancelled() bool {
	return s.Status == SpikeOrderStatusCancelled
}

// IsExpired 判断订单是否已过期
func (s *SpikeOrder) IsExpired() bool {
	if s.Status == SpikeOrderStatusExpired {
		return true
	}
	if s.ExpireAt != nil && time.Now().After(*s.ExpireAt) {
		return true
	}
	return false
}

// CanPay 判断订单是否可以支付
func (s *SpikeOrder) CanPay() bool {
	return s.IsPending() && !s.IsExpired()
}

// CanCancel 判断订单是否可以取消
func (s *SpikeOrder) CanCancel() bool {
	return s.IsPending() || s.Status == SpikeOrderStatusExpired
}

// GetRemainingTime 获取订单剩余时间（秒）
func (s *SpikeOrder) GetRemainingTime() int64 {
	if s.ExpireAt == nil {
		return 0
	}
	remaining := s.ExpireAt.Unix() - time.Now().Unix()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CreateSpikeOrderRequest 表示创建秒杀订单请求
type CreateSpikeOrderRequest struct {
	SpikeEventID   int64  `json:"spike_event_id" binding:"required,gt=0"`
	Quantity       int64  `json:"quantity" binding:"required,gt=0,lte=10"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,min=1,max=64"`
}

// PaySpikeOrderRequest 表示支付秒杀订单请求
type PaySpikeOrderRequest struct {
	PaymentMethod string `json:"payment_method" binding:"required"`
	PaymentInfo   string `json:"payment_info"`
}

// CancelSpikeOrderRequest 表示取消秒杀订单请求
type CancelSpikeOrderRequest struct {
	Reason string `json:"reason"`
}

// SpikeOrderListRequest 表示秒杀订单列表查询请求
type SpikeOrderListRequest struct {
	Page         int               `json:"page"`           // 页码，从1开始
	PageSize     int               `json:"page_size"`      // 每页大小
	UserID       *int64            `json:"user_id"`        // 用户ID过滤
	SpikeEventID *int64            `json:"spike_event_id"` // 秒杀活动ID过滤
	Status       *SpikeOrderStatus `json:"status"`         // 状态过滤
	SortBy       *string           `json:"sort_by"`        // 排序字段: created_at, total_amount
	SortOrder    *string           `json:"sort_order"`     // 排序顺序: asc, desc
}

// SpikeOrderListResponse 表示秒杀订单列表查询响应
type SpikeOrderListResponse struct {
	Orders   []*SpikeOrder `json:"orders"`    // 秒杀订单列表
	Total    int64         `json:"total"`     // 总订单数
	Page     int           `json:"page"`      // 当前页码
	PageSize int           `json:"page_size"` // 每页大小
}

// SpikeOrderWithDetails 表示带详细信息的秒杀订单
type SpikeOrderWithDetails struct {
	*SpikeOrder
	SpikeEvent *SpikeEvent `json:"spike_event"`
	User       *User       `json:"user"`
}

// SpikeParticipationRequest 表示参与秒杀请求
type SpikeParticipationRequest struct {
	SpikeEventID   int64  `json:"spike_event_id" binding:"required,gt=0"`
	Quantity       int64  `json:"quantity" binding:"required,gt=0,lte=10"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,min=1,max=64"`
}

// SpikeParticipationResponse 表示参与秒杀响应
type SpikeParticipationResponse struct {
	Success     bool        `json:"success"`
	Message     string      `json:"message"`
	SpikeOrder  *SpikeOrder `json:"spike_order,omitempty"`
	QueueToken  string      `json:"queue_token,omitempty"`  // 排队令牌
	QueueLength int64       `json:"queue_length,omitempty"` // 排队长度
}
