// Package repo 提供带缓存的商品仓储实现
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/MorseWayne/spike_shop/internal/cache"
	"github.com/MorseWayne/spike_shop/internal/domain"
)

// CachedProductRepository 带缓存的商品仓储
type CachedProductRepository struct {
	repo  ProductRepository
	cache cache.Cache
	ttl   time.Duration
}

// NewCachedProductRepository 创建带缓存的商品仓储
func NewCachedProductRepository(repo ProductRepository, cache cache.Cache, ttl time.Duration) ProductRepository {
	return &CachedProductRepository{
		repo:  repo,
		cache: cache,
		ttl:   ttl,
	}
}

// Create 创建商品（清除相关缓存）
func (r *CachedProductRepository) Create(product *domain.Product) error {
	err := r.repo.Create(product)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getProductCacheKey(product.ID))
	r.cache.Del(ctx, r.getProductSKUCacheKey(product.SKU))
	r.cache.Del(ctx, "products:list:*") // 简化处理，清除所有列表缓存

	return nil
}

// GetByID 根据ID获取商品（带缓存）
func (r *CachedProductRepository) GetByID(id int64) (*domain.Product, error) {
	ctx := context.Background()
	cacheKey := r.getProductCacheKey(id)

	// 尝试从缓存获取
	var product domain.Product
	err := r.cache.Get(ctx, cacheKey, &product)
	if err == nil {
		return &product, nil
	}

	// 缓存未命中，从数据库获取
	result, err := r.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	// 写入缓存
	r.cache.Set(ctx, cacheKey, result, r.ttl)

	return result, nil
}

// GetBySKU 根据SKU获取商品（带缓存）
func (r *CachedProductRepository) GetBySKU(sku string) (*domain.Product, error) {
	ctx := context.Background()
	cacheKey := r.getProductSKUCacheKey(sku)

	// 尝试从缓存获取
	var product domain.Product
	err := r.cache.Get(ctx, cacheKey, &product)
	if err == nil {
		return &product, nil
	}

	// 缓存未命中，从数据库获取
	result, err := r.repo.GetBySKU(sku)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	// 写入缓存
	r.cache.Set(ctx, cacheKey, result, r.ttl)
	// 同时缓存ID索引
	r.cache.Set(ctx, r.getProductCacheKey(result.ID), result, r.ttl)

	return result, nil
}

// Update 更新商品（清除相关缓存）
func (r *CachedProductRepository) Update(product *domain.Product) error {
	err := r.repo.Update(product)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getProductCacheKey(product.ID))
	r.cache.Del(ctx, r.getProductSKUCacheKey(product.SKU))

	return nil
}

// Delete 删除商品（清除相关缓存）
func (r *CachedProductRepository) Delete(id int64) error {
	// 先获取商品信息以便清除SKU缓存
	product, err := r.repo.GetByID(id)
	if err != nil {
		return err
	}

	err = r.repo.Delete(id)
	if err != nil {
		return err
	}

	// 清除相关缓存
	ctx := context.Background()
	r.cache.Del(ctx, r.getProductCacheKey(id))
	if product != nil {
		r.cache.Del(ctx, r.getProductSKUCacheKey(product.SKU))
	}

	return nil
}

// List 获取商品列表（不缓存，因为参数组合太多）
func (r *CachedProductRepository) List(req *domain.ProductListRequest) ([]*domain.Product, int64, error) {
	return r.repo.List(req)
}

// GetByIDs 批量获取商品（部分缓存）
func (r *CachedProductRepository) GetByIDs(ids []int64) ([]*domain.Product, error) {
	ctx := context.Background()
	var cachedProducts []*domain.Product
	var missingIDs []int64

	// 尝试从缓存获取
	for _, id := range ids {
		var product domain.Product
		err := r.cache.Get(ctx, r.getProductCacheKey(id), &product)
		if err == nil {
			cachedProducts = append(cachedProducts, &product)
		} else {
			missingIDs = append(missingIDs, id)
		}
	}

	// 如果所有数据都在缓存中，直接返回
	if len(missingIDs) == 0 {
		return cachedProducts, nil
	}

	// 从数据库获取未缓存的数据
	dbProducts, err := r.repo.GetByIDs(missingIDs)
	if err != nil {
		return nil, err
	}

	// 缓存从数据库获取的数据
	for _, product := range dbProducts {
		r.cache.Set(ctx, r.getProductCacheKey(product.ID), product, r.ttl)
	}

	// 合并结果
	allProducts := append(cachedProducts, dbProducts...)
	return allProducts, nil
}

// Count 获取商品总数（不缓存）
func (r *CachedProductRepository) Count() (int64, error) {
	return r.repo.Count()
}

// CountByStatus 根据状态统计商品数量（不缓存）
func (r *CachedProductRepository) CountByStatus(status domain.ProductStatus) (int64, error) {
	return r.repo.CountByStatus(status)
}

// 缓存键生成方法
func (r *CachedProductRepository) getProductCacheKey(id int64) string {
	return fmt.Sprintf("product:id:%d", id)
}

func (r *CachedProductRepository) getProductSKUCacheKey(sku string) string {
	return fmt.Sprintf("product:sku:%s", sku)
}
