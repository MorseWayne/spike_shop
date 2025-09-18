// Package repo 实现数据访问层，负责与数据库的交互。
package repo

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

// ProductRepository 定义商品数据访问接口
type ProductRepository interface {
	// 基本CRUD操作
	Create(product *domain.Product) error
	GetByID(id int64) (*domain.Product, error)
	GetBySKU(sku string) (*domain.Product, error)
	Update(product *domain.Product) error
	Delete(id int64) error

	// 查询操作
	List(req *domain.ProductListRequest) ([]*domain.Product, int64, error)
	GetByIDs(ids []int64) ([]*domain.Product, error)

	// 统计操作
	Count() (int64, error)
	CountByStatus(status domain.ProductStatus) (int64, error)
}

// productRepo 实现ProductRepository接口
type productRepo struct {
	db *sql.DB
}

// NewProductRepository 创建商品仓储实例
func NewProductRepository(db *sql.DB) ProductRepository {
	return &productRepo{db: db}
}

// Create 创建商品
func (r *productRepo) Create(product *domain.Product) error {
	query := `
		INSERT INTO products (name, description, price, category_id, brand, sku, status, weight, image_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		product.Name,
		product.Description,
		product.Price,
		product.CategoryID,
		product.Brand,
		product.SKU,
		product.Status,
		product.Weight,
		product.ImageURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create product: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	product.ID = id
	return nil
}

// GetByID 根据ID获取商品
func (r *productRepo) GetByID(id int64) (*domain.Product, error) {
	query := `
		SELECT id, name, description, price, category_id, brand, sku, status, weight, image_url, created_at, updated_at
		FROM products 
		WHERE id = ? AND status != 'deleted'
	`

	product := &domain.Product{}
	err := r.db.QueryRow(query, id).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.CategoryID,
		&product.Brand,
		&product.SKU,
		&product.Status,
		&product.Weight,
		&product.ImageURL,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product by id: %w", err)
	}

	return product, nil
}

// GetBySKU 根据SKU获取商品
func (r *productRepo) GetBySKU(sku string) (*domain.Product, error) {
	query := `
		SELECT id, name, description, price, category_id, brand, sku, status, weight, image_url, created_at, updated_at
		FROM products 
		WHERE sku = ? AND status != 'deleted'
	`

	product := &domain.Product{}
	err := r.db.QueryRow(query, sku).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.CategoryID,
		&product.Brand,
		&product.SKU,
		&product.Status,
		&product.Weight,
		&product.ImageURL,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product by sku: %w", err)
	}

	return product, nil
}

// Update 更新商品
func (r *productRepo) Update(product *domain.Product) error {
	query := `
		UPDATE products 
		SET name = ?, description = ?, price = ?, category_id = ?, brand = ?, status = ?, weight = ?, image_url = ?
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		product.Name,
		product.Description,
		product.Price,
		product.CategoryID,
		product.Brand,
		product.Status,
		product.Weight,
		product.ImageURL,
		product.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	return nil
}

// Delete 软删除商品
func (r *productRepo) Delete(id int64) error {
	query := `UPDATE products SET status = 'deleted' WHERE id = ?`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	return nil
}

// List 获取商品列表
func (r *productRepo) List(req *domain.ProductListRequest) ([]*domain.Product, int64, error) {
	// 构建查询条件
	where, args := r.buildListWhereClause(req)

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM products %s", where)
	var total int64
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	// 构建排序和分页
	orderBy := r.buildOrderClause(req)
	limit := req.PageSize
	offset := (req.Page - 1) * req.PageSize

	// 查询数据
	query := fmt.Sprintf(`
		SELECT id, name, description, price, category_id, brand, sku, status, weight, image_url, created_at, updated_at
		FROM products %s %s LIMIT ? OFFSET ?
	`, where, orderBy)

	args = append(args, limit, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		product := &domain.Product{}
		err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.Price,
			&product.CategoryID,
			&product.Brand,
			&product.SKU,
			&product.Status,
			&product.Weight,
			&product.ImageURL,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	return products, total, nil
}

// GetByIDs 根据ID列表批量获取商品
func (r *productRepo) GetByIDs(ids []int64) ([]*domain.Product, error) {
	if len(ids) == 0 {
		return []*domain.Product{}, nil
	}

	// 构建IN子句
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	query := fmt.Sprintf(`
		SELECT id, name, description, price, category_id, brand, sku, status, weight, image_url, created_at, updated_at
		FROM products 
		WHERE id IN (%s) AND status != 'deleted'
		ORDER BY id
	`, placeholders)

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query products by ids: %w", err)
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		product := &domain.Product{}
		err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.Price,
			&product.CategoryID,
			&product.Brand,
			&product.SKU,
			&product.Status,
			&product.Weight,
			&product.ImageURL,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	return products, nil
}

// Count 获取商品总数
func (r *productRepo) Count() (int64, error) {
	query := "SELECT COUNT(*) FROM products WHERE status != 'deleted'"

	var count int64
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count products: %w", err)
	}

	return count, nil
}

// CountByStatus 根据状态统计商品数量
func (r *productRepo) CountByStatus(status domain.ProductStatus) (int64, error) {
	query := "SELECT COUNT(*) FROM products WHERE status = ?"

	var count int64
	err := r.db.QueryRow(query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count products by status: %w", err)
	}

	return count, nil
}

// buildListWhereClause 构建查询条件子句
func (r *productRepo) buildListWhereClause(req *domain.ProductListRequest) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// 默认排除已删除的商品
	conditions = append(conditions, "status != 'deleted'")

	// 状态过滤
	if req.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *req.Status)
	}

	// 分类过滤
	if req.CategoryID != nil {
		conditions = append(conditions, "category_id = ?")
		args = append(args, *req.CategoryID)
	}

	// 品牌过滤
	if req.Brand != nil && *req.Brand != "" {
		conditions = append(conditions, "brand = ?")
		args = append(args, *req.Brand)
	}

	// 关键词搜索
	if req.Keyword != nil && *req.Keyword != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ? OR sku LIKE ?)")
		keyword := "%" + *req.Keyword + "%"
		args = append(args, keyword, keyword, keyword)
	}

	if len(conditions) > 0 {
		return "WHERE " + strings.Join(conditions, " AND "), args
	}

	return "", args
}

// buildOrderClause 构建排序子句
func (r *productRepo) buildOrderClause(req *domain.ProductListRequest) string {
	sortBy := "created_at"
	sortOrder := "DESC"

	if req.SortBy != nil {
		switch *req.SortBy {
		case "price", "created_at", "name", "updated_at":
			sortBy = *req.SortBy
		}
	}

	if req.SortOrder != nil {
		if strings.ToUpper(*req.SortOrder) == "ASC" {
			sortOrder = "ASC"
		}
	}

	return fmt.Sprintf("ORDER BY %s %s", sortBy, sortOrder)
}
