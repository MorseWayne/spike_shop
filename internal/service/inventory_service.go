// Package service 实现库存业务逻辑层，负责库存管理和业务规则。
package service

import (
	"errors"
	"fmt"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/repo"
)

// InventoryService 定义库存业务逻辑接口
type InventoryService interface {
	// 库存管理
	CreateInventory(req *domain.CreateInventoryRequest) (*domain.Inventory, error)
	GetInventory(id int64) (*domain.Inventory, error)
	GetInventoryByProductID(productID int64) (*domain.Inventory, error)
	UpdateInventory(id int64, req *domain.UpdateInventoryRequest) (*domain.Inventory, error)
	DeleteInventory(id int64) error

	// 库存查询
	ListInventories(req *domain.InventoryListRequest) (*domain.InventoryListResponse, error)
	GetLowStockAlerts() ([]*LowStockAlert, error)

	// 库存操作
	AdjustStock(productID int64, req *domain.StockAdjustmentRequest) error
	ReserveStock(req *domain.ReserveStockRequest) error
	ReleaseStock(req *domain.ReleaseStockRequest) error
	ConsumeStock(req *domain.ConsumeStockRequest) error
	RestockProduct(productID int64, quantity int, reason string) error

	// 批量操作
	BatchReserveStock(requests []*domain.ReserveStockRequest) error
	BatchReleaseStock(requests []*domain.ReleaseStockRequest) error
	BatchConsumeStock(requests []*domain.ConsumeStockRequest) error

	// 统计查询
	GetInventoryStats() (*InventoryStats, error)
	CheckStockAvailability(productID int64, quantity int) (bool, error)
}

// LowStockAlert 低库存警告
type LowStockAlert struct {
	ProductID     int64   `json:"product_id"`
	ProductName   string  `json:"product_name"`
	ProductSKU    string  `json:"product_sku"`
	CurrentStock  int     `json:"current_stock"`
	ReorderPoint  int     `json:"reorder_point"`
	StockShortage int     `json:"stock_shortage"`
	ProductPrice  float64 `json:"product_price"`
}

// InventoryStats 库存统计信息
type InventoryStats struct {
	TotalProducts      int64   `json:"total_products"`
	LowStockProducts   int     `json:"low_stock_products"`
	OutOfStockProducts int     `json:"out_of_stock_products"`
	TotalStockValue    float64 `json:"total_stock_value"`
	TotalStock         int64   `json:"total_stock"`
	TotalReservedStock int64   `json:"total_reserved_stock"`
}

// inventoryService 实现InventoryService接口
type inventoryService struct {
	inventoryRepo repo.InventoryRepository
	productRepo   repo.ProductRepository
}

// NewInventoryService 创建库存服务实例
func NewInventoryService(inventoryRepo repo.InventoryRepository, productRepo repo.ProductRepository) InventoryService {
	return &inventoryService{
		inventoryRepo: inventoryRepo,
		productRepo:   productRepo,
	}
}

// CreateInventory 创建库存记录
func (s *inventoryService) CreateInventory(req *domain.CreateInventoryRequest) (*domain.Inventory, error) {
	// 验证商品是否存在
	product, err := s.productRepo.GetByID(req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	if product == nil {
		return nil, errors.New("product not found")
	}

	// 检查是否已存在库存记录
	existing, err := s.inventoryRepo.GetByProductID(req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing inventory: %w", err)
	}
	if existing != nil {
		return nil, errors.New("inventory already exists for this product")
	}

	// 验证库存数据
	if req.Stock < 0 {
		return nil, errors.New("stock cannot be negative")
	}
	if req.MaxStock <= 0 {
		return nil, errors.New("max stock must be greater than 0")
	}
	if req.Stock > req.MaxStock {
		return nil, errors.New("stock cannot exceed max stock")
	}

	// 创建库存记录
	inventory := &domain.Inventory{
		ProductID:     req.ProductID,
		Stock:         req.Stock,
		ReservedStock: 0,
		SoldStock:     0,
		ReorderPoint:  req.ReorderPoint,
		MaxStock:      req.MaxStock,
		Version:       0,
	}

	err = s.inventoryRepo.Create(inventory)
	if err != nil {
		return nil, fmt.Errorf("failed to create inventory: %w", err)
	}

	return inventory, nil
}

// GetInventory 获取库存详情
func (s *inventoryService) GetInventory(id int64) (*domain.Inventory, error) {
	inventory, err := s.inventoryRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return nil, errors.New("inventory not found")
	}

	return inventory, nil
}

// GetInventoryByProductID 根据商品ID获取库存
func (s *inventoryService) GetInventoryByProductID(productID int64) (*domain.Inventory, error) {
	inventory, err := s.inventoryRepo.GetByProductID(productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory by product ID: %w", err)
	}
	if inventory == nil {
		return nil, errors.New("inventory not found")
	}

	return inventory, nil
}

// UpdateInventory 更新库存
func (s *inventoryService) UpdateInventory(id int64, req *domain.UpdateInventoryRequest) (*domain.Inventory, error) {
	// 获取现有库存记录
	inventory, err := s.inventoryRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return nil, errors.New("inventory not found")
	}

	// 更新字段
	if req.Stock != nil {
		if *req.Stock < 0 {
			return nil, errors.New("stock cannot be negative")
		}
		if *req.Stock < inventory.ReservedStock {
			return nil, errors.New("stock cannot be less than reserved stock")
		}
		inventory.Stock = *req.Stock
	}
	if req.ReorderPoint != nil {
		if *req.ReorderPoint < 0 {
			return nil, errors.New("reorder point cannot be negative")
		}
		inventory.ReorderPoint = *req.ReorderPoint
	}
	if req.MaxStock != nil {
		if *req.MaxStock <= 0 {
			return nil, errors.New("max stock must be greater than 0")
		}
		if inventory.Stock > *req.MaxStock {
			return nil, errors.New("current stock exceeds new max stock")
		}
		inventory.MaxStock = *req.MaxStock
	}

	// 保存更新
	err = s.inventoryRepo.UpdateWithVersion(inventory)
	if err != nil {
		return nil, fmt.Errorf("failed to update inventory: %w", err)
	}

	return inventory, nil
}

// DeleteInventory 删除库存记录
func (s *inventoryService) DeleteInventory(id int64) error {
	// 检查库存是否存在
	inventory, err := s.inventoryRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return errors.New("inventory not found")
	}

	// 检查是否有剩余库存或预留库存
	if inventory.Stock > 0 || inventory.ReservedStock > 0 {
		return errors.New("cannot delete inventory with remaining stock")
	}

	return s.inventoryRepo.Delete(id)
}

// ListInventories 获取库存列表
func (s *inventoryService) ListInventories(req *domain.InventoryListRequest) (*domain.InventoryListResponse, error) {
	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 查询库存列表
	inventories, total, err := s.inventoryRepo.List(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventories: %w", err)
	}

	return &domain.InventoryListResponse{
		Inventories: inventories,
		Total:       total,
		Page:        req.Page,
		PageSize:    req.PageSize,
	}, nil
}

// GetLowStockAlerts 获取低库存警告
func (s *inventoryService) GetLowStockAlerts() ([]*LowStockAlert, error) {
	// 获取低库存商品
	lowStockInventories, err := s.inventoryRepo.GetLowStockProducts()
	if err != nil {
		return nil, fmt.Errorf("failed to get low stock products: %w", err)
	}

	if len(lowStockInventories) == 0 {
		return []*LowStockAlert{}, nil
	}

	// 获取商品信息
	var productIDs []int64
	for _, inv := range lowStockInventories {
		productIDs = append(productIDs, inv.ProductID)
	}

	products, err := s.productRepo.GetByIDs(productIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}

	// 构建商品映射
	productMap := make(map[int64]*domain.Product)
	for _, product := range products {
		productMap[product.ID] = product
	}

	// 构建警告列表
	var alerts []*LowStockAlert
	for _, inv := range lowStockInventories {
		product := productMap[inv.ProductID]
		if product == nil {
			continue
		}

		alert := &LowStockAlert{
			ProductID:     inv.ProductID,
			ProductName:   product.Name,
			ProductSKU:    product.SKU,
			CurrentStock:  inv.Stock,
			ReorderPoint:  inv.ReorderPoint,
			StockShortage: inv.ReorderPoint - inv.Stock,
			ProductPrice:  product.Price,
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// AdjustStock 调整库存
func (s *inventoryService) AdjustStock(productID int64, req *domain.StockAdjustmentRequest) error {
	// 验证商品存在
	_, err := s.productRepo.GetByID(productID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	// 验证调整类型和数量
	if req.Type == "out" && req.Quantity > 0 {
		req.Quantity = -req.Quantity // 出库转为负数
	} else if req.Type == "in" && req.Quantity < 0 {
		req.Quantity = -req.Quantity // 入库转为正数
	}

	// 执行库存调整
	err = s.inventoryRepo.AdjustStock(productID, req.Quantity, req.Reason)
	if err != nil {
		return fmt.Errorf("failed to adjust stock: %w", err)
	}

	return nil
}

// ReserveStock 预留库存
func (s *inventoryService) ReserveStock(req *domain.ReserveStockRequest) error {
	// 验证商品存在且可售
	product, err := s.productRepo.GetByID(req.ProductID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}
	if product == nil {
		return errors.New("product not found")
	}
	if !product.IsAvailable() {
		return errors.New("product is not available for sale")
	}

	// 预留库存
	err = s.inventoryRepo.ReserveStock(req.ProductID, req.Quantity)
	if err != nil {
		return fmt.Errorf("failed to reserve stock: %w", err)
	}

	return nil
}

// ReleaseStock 释放库存
func (s *inventoryService) ReleaseStock(req *domain.ReleaseStockRequest) error {
	err := s.inventoryRepo.ReleaseStock(req.ProductID, req.Quantity)
	if err != nil {
		return fmt.Errorf("failed to release stock: %w", err)
	}

	return nil
}

// ConsumeStock 消费库存
func (s *inventoryService) ConsumeStock(req *domain.ConsumeStockRequest) error {
	err := s.inventoryRepo.ConsumeStock(req.ProductID, req.Quantity)
	if err != nil {
		return fmt.Errorf("failed to consume stock: %w", err)
	}

	return nil
}

// RestockProduct 补充库存
func (s *inventoryService) RestockProduct(productID int64, quantity int, reason string) error {
	if quantity <= 0 {
		return errors.New("restock quantity must be positive")
	}

	// 获取库存记录
	inventory, err := s.inventoryRepo.GetByProductID(productID)
	if err != nil {
		return fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return errors.New("inventory not found")
	}

	// 检查是否超过最大库存限制
	if inventory.Stock+quantity > inventory.MaxStock {
		return fmt.Errorf("restock would exceed max stock limit (%d)", inventory.MaxStock)
	}

	// 执行补货
	err = s.inventoryRepo.AdjustStock(productID, quantity, reason)
	if err != nil {
		return fmt.Errorf("failed to restock: %w", err)
	}

	return nil
}

// BatchReserveStock 批量预留库存
func (s *inventoryService) BatchReserveStock(requests []*domain.ReserveStockRequest) error {
	var updates []repo.StockUpdate
	for _, req := range requests {
		updates = append(updates, repo.StockUpdate{
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Type:      "reserve",
		})
	}

	return s.inventoryRepo.BatchUpdateStock(updates)
}

// BatchReleaseStock 批量释放库存
func (s *inventoryService) BatchReleaseStock(requests []*domain.ReleaseStockRequest) error {
	var updates []repo.StockUpdate
	for _, req := range requests {
		updates = append(updates, repo.StockUpdate{
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Type:      "release",
		})
	}

	return s.inventoryRepo.BatchUpdateStock(updates)
}

// BatchConsumeStock 批量消费库存
func (s *inventoryService) BatchConsumeStock(requests []*domain.ConsumeStockRequest) error {
	var updates []repo.StockUpdate
	for _, req := range requests {
		updates = append(updates, repo.StockUpdate{
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Type:      "consume",
		})
	}

	return s.inventoryRepo.BatchUpdateStock(updates)
}

// GetInventoryStats 获取库存统计信息
func (s *inventoryService) GetInventoryStats() (*InventoryStats, error) {
	// 获取所有库存记录
	req := &domain.InventoryListRequest{
		Page:     1,
		PageSize: 1000, // 简化处理，实际应该分页处理
	}

	inventories, total, err := s.inventoryRepo.List(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventories: %w", err)
	}

	// 统计数据
	var lowStockCount, outOfStockCount int
	var totalStock, totalReservedStock int64

	for _, inv := range inventories {
		totalStock += int64(inv.Stock)
		totalReservedStock += int64(inv.ReservedStock)

		if inv.Stock == 0 {
			outOfStockCount++
		} else if inv.IsLowStock() {
			lowStockCount++
		}
	}

	// 获取总库存价值
	totalValue, err := s.inventoryRepo.GetTotalStockValue()
	if err != nil {
		return nil, fmt.Errorf("failed to get total stock value: %w", err)
	}

	return &InventoryStats{
		TotalProducts:      total,
		LowStockProducts:   lowStockCount,
		OutOfStockProducts: outOfStockCount,
		TotalStockValue:    totalValue,
		TotalStock:         totalStock,
		TotalReservedStock: totalReservedStock,
	}, nil
}

// CheckStockAvailability 检查库存可用性
func (s *inventoryService) CheckStockAvailability(productID int64, quantity int) (bool, error) {
	inventory, err := s.inventoryRepo.GetByProductID(productID)
	if err != nil {
		return false, fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return false, errors.New("inventory not found")
	}

	return inventory.CanReserve(quantity), nil
}
