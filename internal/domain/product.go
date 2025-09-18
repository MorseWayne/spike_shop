// Package domain 定义商品相关的业务领域模型和核心业务规则。
package domain

import (
	"time"
)

// ProductStatus 定义商品状态类型
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"   // 正常销售
	ProductStatusInactive ProductStatus = "inactive" // 暂停销售
	ProductStatusDeleted  ProductStatus = "deleted"  // 已删除
)

// Product 表示商品领域模型
type Product struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Price       float64       `json:"price"`
	CategoryID  *int64        `json:"category_id"`
	Brand       string        `json:"brand"`
	SKU         string        `json:"sku"`
	Status      ProductStatus `json:"status"`
	Weight      *float64      `json:"weight"`
	ImageURL    string        `json:"image_url"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// IsAvailable 判断商品是否可售
func (p *Product) IsAvailable() bool {
	return p.Status == ProductStatusActive
}

// CreateProductRequest 表示创建商品请求
type CreateProductRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=255"`
	Description string   `json:"description"`
	Price       float64  `json:"price" binding:"required,gt=0"`
	CategoryID  *int64   `json:"category_id"`
	Brand       string   `json:"brand"`
	SKU         string   `json:"sku" binding:"required,min=1,max=100"`
	Weight      *float64 `json:"weight"`
	ImageURL    string   `json:"image_url"`
}

// UpdateProductRequest 表示更新商品请求
type UpdateProductRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Price       *float64       `json:"price"`
	CategoryID  *int64         `json:"category_id"`
	Brand       *string        `json:"brand"`
	Status      *ProductStatus `json:"status"`
	Weight      *float64       `json:"weight"`
	ImageURL    *string        `json:"image_url"`
}

// ProductListRequest 表示商品列表查询请求
type ProductListRequest struct {
	Page       int            `json:"page"`        // 页码，从1开始
	PageSize   int            `json:"page_size"`   // 每页大小
	Status     *ProductStatus `json:"status"`      // 商品状态过滤
	CategoryID *int64         `json:"category_id"` // 分类过滤
	Brand      *string        `json:"brand"`       // 品牌过滤
	Keyword    *string        `json:"keyword"`     // 关键词搜索
	SortBy     *string        `json:"sort_by"`     // 排序字段: price, created_at, name
	SortOrder  *string        `json:"sort_order"`  // 排序顺序: asc, desc
}

// ProductListResponse 表示商品列表查询响应
type ProductListResponse struct {
	Products []*Product `json:"products"`  // 商品列表
	Total    int64      `json:"total"`     // 总商品数
	Page     int        `json:"page"`      // 当前页码
	PageSize int        `json:"page_size"` // 每页大小
}

// ProductWithInventory 表示带库存信息的商品
type ProductWithInventory struct {
	*Product
	Inventory *Inventory `json:"inventory"`
}
