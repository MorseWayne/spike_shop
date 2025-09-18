// Package service 实现业务逻辑层，协调各种资源完成业务需求。
package service

import (
	"errors"
	"fmt"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/repo"
)

// ProductService 定义商品业务逻辑接口
type ProductService interface {
	// 商品管理
	CreateProduct(req *domain.CreateProductRequest) (*domain.Product, error)
	GetProduct(id int64) (*domain.Product, error)
	GetProductBySKU(sku string) (*domain.Product, error)
	UpdateProduct(id int64, req *domain.UpdateProductRequest) (*domain.Product, error)
	DeleteProduct(id int64) error

	// 商品查询
	ListProducts(req *domain.ProductListRequest) (*domain.ProductListResponse, error)
	GetProductsWithInventory(ids []int64) ([]*domain.ProductWithInventory, error)
	SearchProducts(keyword string, page, pageSize int) (*domain.ProductListResponse, error)

	// 商品统计
	GetProductStats() (*ProductStats, error)
}

// ProductStats 商品统计信息
type ProductStats struct {
	TotalProducts       int64   `json:"total_products"`
	ActiveProducts      int64   `json:"active_products"`
	InactiveProducts    int64   `json:"inactive_products"`
	AveragePrice        float64 `json:"average_price"`
	TotalInventoryValue float64 `json:"total_inventory_value"`
}

// productService 实现ProductService接口
type productService struct {
	productRepo   repo.ProductRepository
	inventoryRepo repo.InventoryRepository
}

// NewProductService 创建商品服务实例
func NewProductService(productRepo repo.ProductRepository, inventoryRepo repo.InventoryRepository) ProductService {
	return &productService{
		productRepo:   productRepo,
		inventoryRepo: inventoryRepo,
	}
}

// CreateProduct 创建商品
func (s *productService) CreateProduct(req *domain.CreateProductRequest) (*domain.Product, error) {
	// 验证SKU唯一性
	existing, err := s.productRepo.GetBySKU(req.SKU)
	if err != nil {
		return nil, fmt.Errorf("failed to check SKU uniqueness: %w", err)
	}
	if existing != nil {
		return nil, errors.New("SKU already exists")
	}

	// 创建商品实体
	product := &domain.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		CategoryID:  req.CategoryID,
		Brand:       req.Brand,
		SKU:         req.SKU,
		Status:      domain.ProductStatusActive,
		Weight:      req.Weight,
		ImageURL:    req.ImageURL,
	}

	// 保存商品
	err = s.productRepo.Create(product)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return product, nil
}

// GetProduct 获取商品详情
func (s *productService) GetProduct(id int64) (*domain.Product, error) {
	product, err := s.productRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	if product == nil {
		return nil, errors.New("product not found")
	}

	return product, nil
}

// GetProductBySKU 根据SKU获取商品
func (s *productService) GetProductBySKU(sku string) (*domain.Product, error) {
	product, err := s.productRepo.GetBySKU(sku)
	if err != nil {
		return nil, fmt.Errorf("failed to get product by SKU: %w", err)
	}
	if product == nil {
		return nil, errors.New("product not found")
	}

	return product, nil
}

// UpdateProduct 更新商品
func (s *productService) UpdateProduct(id int64, req *domain.UpdateProductRequest) (*domain.Product, error) {
	// 获取现有商品
	product, err := s.productRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	if product == nil {
		return nil, errors.New("product not found")
	}

	// 更新字段
	if req.Name != nil {
		product.Name = *req.Name
	}
	if req.Description != nil {
		product.Description = *req.Description
	}
	if req.Price != nil {
		if *req.Price <= 0 {
			return nil, errors.New("price must be greater than 0")
		}
		product.Price = *req.Price
	}
	if req.CategoryID != nil {
		product.CategoryID = req.CategoryID
	}
	if req.Brand != nil {
		product.Brand = *req.Brand
	}
	if req.Status != nil {
		product.Status = *req.Status
	}
	if req.Weight != nil {
		product.Weight = req.Weight
	}
	if req.ImageURL != nil {
		product.ImageURL = *req.ImageURL
	}

	// 保存更新
	err = s.productRepo.Update(product)
	if err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return product, nil
}

// DeleteProduct 删除商品
func (s *productService) DeleteProduct(id int64) error {
	// 检查商品是否存在
	product, err := s.productRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}
	if product == nil {
		return errors.New("product not found")
	}

	// 检查是否有库存
	inventory, err := s.inventoryRepo.GetByProductID(id)
	if err != nil {
		return fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory != nil && (inventory.Stock > 0 || inventory.ReservedStock > 0) {
		return errors.New("cannot delete product with existing stock")
	}

	// 软删除商品
	return s.productRepo.Delete(id)
}

// ListProducts 获取商品列表
func (s *productService) ListProducts(req *domain.ProductListRequest) (*domain.ProductListResponse, error) {
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

	// 查询商品列表
	products, total, err := s.productRepo.List(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	return &domain.ProductListResponse{
		Products: products,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetProductsWithInventory 获取带库存信息的商品列表
func (s *productService) GetProductsWithInventory(ids []int64) ([]*domain.ProductWithInventory, error) {
	// 获取商品信息
	products, err := s.productRepo.GetByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}

	// 获取库存信息
	inventories, err := s.inventoryRepo.GetByProductIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventories: %w", err)
	}

	// 构建库存映射
	inventoryMap := make(map[int64]*domain.Inventory)
	for _, inv := range inventories {
		inventoryMap[inv.ProductID] = inv
	}

	// 组合结果
	var result []*domain.ProductWithInventory
	for _, product := range products {
		item := &domain.ProductWithInventory{
			Product:   product,
			Inventory: inventoryMap[product.ID],
		}
		result = append(result, item)
	}

	return result, nil
}

// SearchProducts 搜索商品
func (s *productService) SearchProducts(keyword string, page, pageSize int) (*domain.ProductListResponse, error) {
	req := &domain.ProductListRequest{
		Page:     page,
		PageSize: pageSize,
		Keyword:  &keyword,
	}

	return s.ListProducts(req)
}

// GetProductStats 获取商品统计信息
func (s *productService) GetProductStats() (*ProductStats, error) {
	// 获取商品总数
	totalProducts, err := s.productRepo.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to get total products: %w", err)
	}

	// 获取不同状态的商品数量
	activeProducts, err := s.productRepo.CountByStatus(domain.ProductStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get active products count: %w", err)
	}

	inactiveProducts, err := s.productRepo.CountByStatus(domain.ProductStatusInactive)
	if err != nil {
		return nil, fmt.Errorf("failed to get inactive products count: %w", err)
	}

	// 获取总库存价值
	totalValue, err := s.inventoryRepo.GetTotalStockValue()
	if err != nil {
		return nil, fmt.Errorf("failed to get total stock value: %w", err)
	}

	// 计算平均价格（简单计算，实际应该根据具体需求）
	averagePrice := 0.0
	if totalProducts > 0 {
		// 这里简化处理，实际应该查询数据库计算
		averagePrice = totalValue / float64(totalProducts)
	}

	return &ProductStats{
		TotalProducts:       totalProducts,
		ActiveProducts:      activeProducts,
		InactiveProducts:    inactiveProducts,
		AveragePrice:        averagePrice,
		TotalInventoryValue: totalValue,
	}, nil
}
