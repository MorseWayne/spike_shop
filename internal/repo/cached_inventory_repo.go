// Package repo 提供带缓存的库存仓储实现
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/MorseWayne/spike_shop/internal/cache"
	"github.com/MorseWayne/spike_shop/internal/domain"
)

// CachedInventoryRepository 带缓存的库存仓储
type CachedInventoryRepository struct {
	repo  InventoryRepository
	cache cache.Cache
	ttl   time.Duration
}

// NewCachedInventoryRepository 创建带缓存的库存仓储
func NewCachedInventoryRepository(repo InventoryRepository, cache cache.Cache, ttl time.Duration) InventoryRepository {
	return &CachedInventoryRepository{
		repo:  repo,
		cache: cache,
		ttl:   ttl,
	}
}

// Create 创建库存记录（清除相关缓存）
func (r *CachedInventoryRepository) Create(inventory *domain.Inventory) error {
	err := r.repo.Create(inventory)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryCacheKey(inventory.ID))
	r.cache.Del(ctx, r.getInventoryProductCacheKey(inventory.ProductID))

	return nil
}

// GetByID 根据ID获取库存（带缓存）
func (r *CachedInventoryRepository) GetByID(id int64) (*domain.Inventory, error) {
	ctx := context.Background()
	cacheKey := r.getInventoryCacheKey(id)

	// 尝试从缓存获取
	var inventory domain.Inventory
	err := r.cache.Get(ctx, cacheKey, &inventory)
	if err == nil {
		return &inventory, nil
	}

	// 缓存未命中，从数据库获取
	result, err := r.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	// 写入缓存（库存数据TTL设置较短，因为变化频繁）
	r.cache.Set(ctx, cacheKey, result, r.ttl/2)

	return result, nil
}

// GetByProductID 根据商品ID获取库存（带缓存）
func (r *CachedInventoryRepository) GetByProductID(productID int64) (*domain.Inventory, error) {
	ctx := context.Background()
	cacheKey := r.getInventoryProductCacheKey(productID)

	// 尝试从缓存获取
	var inventory domain.Inventory
	err := r.cache.Get(ctx, cacheKey, &inventory)
	if err == nil {
		return &inventory, nil
	}

	// 缓存未命中，从数据库获取
	result, err := r.repo.GetByProductID(productID)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	// 写入缓存
	r.cache.Set(ctx, cacheKey, result, r.ttl/2)
	// 同时缓存ID索引
	r.cache.Set(ctx, r.getInventoryCacheKey(result.ID), result, r.ttl/2)

	return result, nil
}

// Update 更新库存（清除相关缓存）
func (r *CachedInventoryRepository) Update(inventory *domain.Inventory) error {
	err := r.repo.Update(inventory)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryCacheKey(inventory.ID))
	r.cache.Del(ctx, r.getInventoryProductCacheKey(inventory.ProductID))

	return nil
}

// UpdateWithVersion 使用乐观锁更新库存（清除相关缓存）
func (r *CachedInventoryRepository) UpdateWithVersion(inventory *domain.Inventory) error {
	err := r.repo.UpdateWithVersion(inventory)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryCacheKey(inventory.ID))
	r.cache.Del(ctx, r.getInventoryProductCacheKey(inventory.ProductID))

	return nil
}

// Delete 删除库存记录（清除相关缓存）
func (r *CachedInventoryRepository) Delete(id int64) error {
	// 先获取库存信息以便清除商品缓存
	inventory, err := r.repo.GetByID(id)
	if err != nil {
		return err
	}

	err = r.repo.Delete(id)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryCacheKey(id))
	if inventory != nil {
		r.cache.Del(ctx, r.getInventoryProductCacheKey(inventory.ProductID))
	}

	return nil
}

// GetByProductIDs 批量获取库存（部分缓存）
func (r *CachedInventoryRepository) GetByProductIDs(productIDs []int64) ([]*domain.Inventory, error) {
	ctx := context.Background()
	var cachedInventories []*domain.Inventory
	var missingProductIDs []int64

	// 尝试从缓存获取
	for _, productID := range productIDs {
		var inventory domain.Inventory
		err := r.cache.Get(ctx, r.getInventoryProductCacheKey(productID), &inventory)
		if err == nil {
			cachedInventories = append(cachedInventories, &inventory)
		} else {
			missingProductIDs = append(missingProductIDs, productID)
		}
	}

	// 如果所有数据都在缓存中，直接返回
	if len(missingProductIDs) == 0 {
		return cachedInventories, nil
	}

	// 从数据库获取未缓存的数据
	dbInventories, err := r.repo.GetByProductIDs(missingProductIDs)
	if err != nil {
		return nil, err
	}

	// 缓存从数据库获取的数据
	for _, inventory := range dbInventories {
		r.cache.Set(ctx, r.getInventoryProductCacheKey(inventory.ProductID), inventory, r.ttl/2)
		r.cache.Set(ctx, r.getInventoryCacheKey(inventory.ID), inventory, r.ttl/2)
	}

	// 合并结果
	allInventories := append(cachedInventories, dbInventories...)
	return allInventories, nil
}

// BatchUpdateStock 批量更新库存（清除相关缓存）
func (r *CachedInventoryRepository) BatchUpdateStock(updates []StockUpdate) error {
	err := r.repo.BatchUpdateStock(updates)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	for _, update := range updates {
		r.cache.Del(ctx, r.getInventoryProductCacheKey(update.ProductID))
	}

	return nil
}

// List 获取库存列表（不缓存，因为参数组合太多）
func (r *CachedInventoryRepository) List(req *domain.InventoryListRequest) ([]*domain.Inventory, int64, error) {
	return r.repo.List(req)
}

// GetLowStockProducts 获取低库存商品（不缓存）
func (r *CachedInventoryRepository) GetLowStockProducts() ([]*domain.Inventory, error) {
	return r.repo.GetLowStockProducts()
}

// 库存操作方法（清除相关缓存）

// ReserveStock 预留库存
func (r *CachedInventoryRepository) ReserveStock(productID int64, quantity int) error {
	err := r.repo.ReserveStock(productID, quantity)
	if err != nil {
		return err
	}

	// 清除缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryProductCacheKey(productID))

	return nil
}

// ReleaseStock 释放预留库存
func (r *CachedInventoryRepository) ReleaseStock(productID int64, quantity int) error {
	err := r.repo.ReleaseStock(productID, quantity)
	if err != nil {
		return err
	}

	// 清除缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryProductCacheKey(productID))

	return nil
}

// ConsumeStock 消费库存
func (r *CachedInventoryRepository) ConsumeStock(productID int64, quantity int) error {
	err := r.repo.ConsumeStock(productID, quantity)
	if err != nil {
		return err
	}

	// 清除缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryProductCacheKey(productID))

	return nil
}

// AdjustStock 调整库存
func (r *CachedInventoryRepository) AdjustStock(productID int64, quantity int, reason string) error {
	err := r.repo.AdjustStock(productID, quantity, reason)
	if err != nil {
		return err
	}

	// 清除缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getInventoryProductCacheKey(productID))

	return nil
}

// Count 获取库存记录总数（不缓存）
func (r *CachedInventoryRepository) Count() (int64, error) {
	return r.repo.Count()
}

// GetTotalStockValue 获取总库存价值（不缓存）
func (r *CachedInventoryRepository) GetTotalStockValue() (float64, error) {
	return r.repo.GetTotalStockValue()
}

// 缓存键生成方法
func (r *CachedInventoryRepository) getInventoryCacheKey(id int64) string {
	return fmt.Sprintf("inventory:id:%d", id)
}

func (r *CachedInventoryRepository) getInventoryProductCacheKey(productID int64) string {
	return fmt.Sprintf("inventory:product:%d", productID)
}
