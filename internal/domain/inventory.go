// Package domain 定义库存相关的业务领域模型和核心业务规则。
package domain

import (
	"errors"
	"time"
)

// Inventory 表示库存领域模型
type Inventory struct {
	ID            int64     `json:"id"`
	ProductID     int64     `json:"product_id"`
	Stock         int       `json:"stock"`          // 当前可用库存
	ReservedStock int       `json:"reserved_stock"` // 预留库存(购物车/未支付订单)
	SoldStock     int       `json:"sold_stock"`     // 已售库存
	ReorderPoint  int       `json:"reorder_point"`  // 补货提醒点
	MaxStock      int       `json:"max_stock"`      // 最大库存限制
	Version       int       `json:"version"`        // 乐观锁版本号
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AvailableStock 返回真实可售库存数量
func (i *Inventory) AvailableStock() int {
	return i.Stock - i.ReservedStock
}

// IsLowStock 判断是否低库存
func (i *Inventory) IsLowStock() bool {
	return i.Stock <= i.ReorderPoint
}

// CanReserve 判断是否可以预留指定数量的库存
func (i *Inventory) CanReserve(quantity int) bool {
	return i.AvailableStock() >= quantity
}

// Reserve 预留库存
func (i *Inventory) Reserve(quantity int) error {
	if !i.CanReserve(quantity) {
		return errors.New("insufficient stock")
	}
	i.ReservedStock += quantity
	return nil
}

// Release 释放预留库存
func (i *Inventory) Release(quantity int) error {
	if i.ReservedStock < quantity {
		return errors.New("insufficient reserved stock")
	}
	i.ReservedStock -= quantity
	return nil
}

// Consume 消费库存(从预留转为已售)
func (i *Inventory) Consume(quantity int) error {
	if i.ReservedStock < quantity {
		return errors.New("insufficient reserved stock")
	}
	i.ReservedStock -= quantity
	i.Stock -= quantity
	i.SoldStock += quantity
	return nil
}

// Restock 补充库存
func (i *Inventory) Restock(quantity int) error {
	if i.Stock+quantity > i.MaxStock {
		return errors.New("exceeds maximum stock limit")
	}
	i.Stock += quantity
	return nil
}

// CreateInventoryRequest 表示创建库存请求
type CreateInventoryRequest struct {
	ProductID    int64 `json:"product_id" binding:"required"`
	Stock        int   `json:"stock" binding:"min=0"`
	ReorderPoint int   `json:"reorder_point" binding:"min=0"`
	MaxStock     int   `json:"max_stock" binding:"required,gt=0"`
}

// UpdateInventoryRequest 表示更新库存请求
type UpdateInventoryRequest struct {
	Stock        *int `json:"stock"`
	ReorderPoint *int `json:"reorder_point"`
	MaxStock     *int `json:"max_stock"`
}

// StockAdjustmentRequest 表示库存调整请求
type StockAdjustmentRequest struct {
	Quantity int    `json:"quantity" binding:"required"`          // 调整数量，正数为增加，负数为减少
	Reason   string `json:"reason" binding:"required,min=1"`      // 调整原因
	Type     string `json:"type" binding:"required,oneof=in out"` // 调整类型: in(入库) out(出库)
}

// ReserveStockRequest 表示预留库存请求
type ReserveStockRequest struct {
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int   `json:"quantity" binding:"required,gt=0"`
}

// ReleaseStockRequest 表示释放库存请求
type ReleaseStockRequest struct {
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int   `json:"quantity" binding:"required,gt=0"`
}

// ConsumeStockRequest 表示消费库存请求
type ConsumeStockRequest struct {
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int   `json:"quantity" binding:"required,gt=0"`
}

// InventoryListRequest 表示库存列表查询请求
type InventoryListRequest struct {
	Page      int     `json:"page"`       // 页码，从1开始
	PageSize  int     `json:"page_size"`  // 每页大小
	ProductID *int64  `json:"product_id"` // 商品ID过滤
	LowStock  *bool   `json:"low_stock"`  // 是否只显示低库存
	MinStock  *int    `json:"min_stock"`  // 最小库存过滤
	MaxStock  *int    `json:"max_stock"`  // 最大库存过滤
	SortBy    *string `json:"sort_by"`    // 排序字段: stock, updated_at
	SortOrder *string `json:"sort_order"` // 排序顺序: asc, desc
}

// InventoryListResponse 表示库存列表查询响应
type InventoryListResponse struct {
	Inventories []*Inventory `json:"inventories"` // 库存列表
	Total       int64        `json:"total"`       // 总库存记录数
	Page        int          `json:"page"`        // 当前页码
	PageSize    int          `json:"page_size"`   // 每页大小
}

// StockMovement 表示库存变动记录
type StockMovement struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	Type      string    `json:"type"`     // 变动类型: in, out, reserve, release, consume
	Quantity  int       `json:"quantity"` // 变动数量
	Reason    string    `json:"reason"`   // 变动原因
	UserID    *int64    `json:"user_id"`  // 操作用户ID
	CreatedAt time.Time `json:"created_at"`
}
