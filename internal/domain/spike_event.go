// Package domain 定义秒杀活动相关的业务领域模型和核心业务规则。
package domain

import (
	"time"
)

// SpikeEventStatus 定义秒杀活动状态类型
type SpikeEventStatus string

const (
	SpikeEventStatusPending   SpikeEventStatus = "pending"   // 待开始
	SpikeEventStatusActive    SpikeEventStatus = "active"    // 进行中
	SpikeEventStatusEnded     SpikeEventStatus = "ended"     // 已结束
	SpikeEventStatusCancelled SpikeEventStatus = "cancelled" // 已取消
)

// SpikeEvent 表示秒杀活动领域模型
type SpikeEvent struct {
	ID            int64            `json:"id"`
	ProductID     int64            `json:"product_id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	SpikePrice    float64          `json:"spike_price"`
	OriginalPrice float64          `json:"original_price"`
	SpikeStock    int64            `json:"spike_stock"`
	SoldCount     int64            `json:"sold_count"`
	StartAt       time.Time        `json:"start_at"`
	EndAt         time.Time        `json:"end_at"`
	Status        SpikeEventStatus `json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// IsActive 判断秒杀活动是否正在进行
func (s *SpikeEvent) IsActive() bool {
	now := time.Now()
	return s.Status == SpikeEventStatusActive && 
		   now.After(s.StartAt) && 
		   now.Before(s.EndAt)
}

// IsAvailable 判断秒杀活动是否可参与（有库存且活动中）
func (s *SpikeEvent) IsAvailable() bool {
	return s.IsActive() && s.SoldCount < s.SpikeStock
}

// GetRemainingStock 获取剩余库存
func (s *SpikeEvent) GetRemainingStock() int64 {
	remaining := s.SpikeStock - s.SoldCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetDiscountPercentage 获取折扣百分比
func (s *SpikeEvent) GetDiscountPercentage() float64 {
	if s.OriginalPrice <= 0 {
		return 0
	}
	return (s.OriginalPrice - s.SpikePrice) / s.OriginalPrice * 100
}

// CanStart 判断活动是否可以开始
func (s *SpikeEvent) CanStart() bool {
	return s.Status == SpikeEventStatusPending && time.Now().After(s.StartAt)
}

// CanEnd 判断活动是否可以结束
func (s *SpikeEvent) CanEnd() bool {
	return s.Status == SpikeEventStatusActive && 
		   (time.Now().After(s.EndAt) || s.SoldCount >= s.SpikeStock)
}

// CreateSpikeEventRequest 表示创建秒杀活动请求
type CreateSpikeEventRequest struct {
	ProductID     int64   `json:"product_id" binding:"required,gt=0"`
	Name          string  `json:"name" binding:"required,min=1,max=255"`
	Description   string  `json:"description"`
	SpikePrice    float64 `json:"spike_price" binding:"required,gt=0"`
	OriginalPrice float64 `json:"original_price" binding:"required,gt=0"`
	SpikeStock    int64   `json:"spike_stock" binding:"required,gt=0"`
	StartAt       string  `json:"start_at" binding:"required"`
	EndAt         string  `json:"end_at" binding:"required"`
}

// UpdateSpikeEventRequest 表示更新秒杀活动请求
type UpdateSpikeEventRequest struct {
	Name          *string           `json:"name"`
	Description   *string           `json:"description"`
	SpikePrice    *float64          `json:"spike_price"`
	OriginalPrice *float64          `json:"original_price"`
	SpikeStock    *int64            `json:"spike_stock"`
	StartAt       *string           `json:"start_at"`
	EndAt         *string           `json:"end_at"`
	Status        *SpikeEventStatus `json:"status"`
}

// SpikeEventListRequest 表示秒杀活动列表查询请求
type SpikeEventListRequest struct {
	Page      int               `json:"page"`       // 页码，从1开始
	PageSize  int               `json:"page_size"`  // 每页大小
	ProductID *int64            `json:"product_id"` // 商品ID过滤
	Status    *SpikeEventStatus `json:"status"`     // 状态过滤
	Active    *bool             `json:"active"`     // 是否只查询活跃的活动
	SortBy    *string           `json:"sort_by"`    // 排序字段: start_at, created_at, spike_price
	SortOrder *string           `json:"sort_order"` // 排序顺序: asc, desc
}

// SpikeEventListResponse 表示秒杀活动列表查询响应
type SpikeEventListResponse struct {
	Events   []*SpikeEvent `json:"events"`    // 秒杀活动列表
	Total    int64         `json:"total"`     // 总活动数
	Page     int           `json:"page"`      // 当前页码
	PageSize int           `json:"page_size"` // 每页大小
}

// SpikeEventWithProduct 表示带商品信息的秒杀活动
type SpikeEventWithProduct struct {
	*SpikeEvent
	Product *Product `json:"product"`
}
