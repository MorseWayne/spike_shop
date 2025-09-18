// Package repo 实现库存数据访问层，负责与数据库的交互。
package repo

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

// InventoryRepository 定义库存数据访问接口
type InventoryRepository interface {
	// 基本CRUD操作
	Create(inventory *domain.Inventory) error
	GetByID(id int64) (*domain.Inventory, error)
	GetByProductID(productID int64) (*domain.Inventory, error)
	Update(inventory *domain.Inventory) error
	UpdateWithVersion(inventory *domain.Inventory) error // 乐观锁更新
	Delete(id int64) error

	// 批量操作
	GetByProductIDs(productIDs []int64) ([]*domain.Inventory, error)
	BatchUpdateStock(updates []StockUpdate) error

	// 查询操作
	List(req *domain.InventoryListRequest) ([]*domain.Inventory, int64, error)
	GetLowStockProducts() ([]*domain.Inventory, error)

	// 库存操作
	ReserveStock(productID int64, quantity int) error
	ReleaseStock(productID int64, quantity int) error
	ConsumeStock(productID int64, quantity int) error
	AdjustStock(productID int64, quantity int, reason string) error

	// 统计操作
	Count() (int64, error)
	GetTotalStockValue() (float64, error)
}

// StockUpdate 表示批量库存更新项
type StockUpdate struct {
	ProductID int64
	Quantity  int
	Type      string // "reserve", "release", "consume", "adjust"
}

// inventoryRepo 实现InventoryRepository接口
type inventoryRepo struct {
	db *sql.DB
}

// NewInventoryRepository 创建库存仓储实例
func NewInventoryRepository(db *sql.DB) InventoryRepository {
	return &inventoryRepo{db: db}
}

// Create 创建库存记录
func (r *inventoryRepo) Create(inventory *domain.Inventory) error {
	query := `
		INSERT INTO inventory (product_id, stock, reserved_stock, sold_stock, reorder_point, max_stock)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		inventory.ProductID,
		inventory.Stock,
		inventory.ReservedStock,
		inventory.SoldStock,
		inventory.ReorderPoint,
		inventory.MaxStock,
	)
	if err != nil {
		return fmt.Errorf("failed to create inventory: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	inventory.ID = id
	return nil
}

// GetByID 根据ID获取库存
func (r *inventoryRepo) GetByID(id int64) (*domain.Inventory, error) {
	query := `
		SELECT id, product_id, stock, reserved_stock, sold_stock, reorder_point, max_stock, version, created_at, updated_at
		FROM inventory 
		WHERE id = ?
	`

	inventory := &domain.Inventory{}
	err := r.db.QueryRow(query, id).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Stock,
		&inventory.ReservedStock,
		&inventory.SoldStock,
		&inventory.ReorderPoint,
		&inventory.MaxStock,
		&inventory.Version,
		&inventory.CreatedAt,
		&inventory.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory by id: %w", err)
	}

	return inventory, nil
}

// GetByProductID 根据商品ID获取库存
func (r *inventoryRepo) GetByProductID(productID int64) (*domain.Inventory, error) {
	query := `
		SELECT id, product_id, stock, reserved_stock, sold_stock, reorder_point, max_stock, version, created_at, updated_at
		FROM inventory 
		WHERE product_id = ?
	`

	inventory := &domain.Inventory{}
	err := r.db.QueryRow(query, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Stock,
		&inventory.ReservedStock,
		&inventory.SoldStock,
		&inventory.ReorderPoint,
		&inventory.MaxStock,
		&inventory.Version,
		&inventory.CreatedAt,
		&inventory.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory by product id: %w", err)
	}

	return inventory, nil
}

// Update 更新库存
func (r *inventoryRepo) Update(inventory *domain.Inventory) error {
	query := `
		UPDATE inventory 
		SET stock = ?, reserved_stock = ?, sold_stock = ?, reorder_point = ?, max_stock = ?, version = version + 1
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		inventory.Stock,
		inventory.ReservedStock,
		inventory.SoldStock,
		inventory.ReorderPoint,
		inventory.MaxStock,
		inventory.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update inventory: %w", err)
	}

	return nil
}

// UpdateWithVersion 使用乐观锁更新库存
func (r *inventoryRepo) UpdateWithVersion(inventory *domain.Inventory) error {
	query := `
		UPDATE inventory 
		SET stock = ?, reserved_stock = ?, sold_stock = ?, reorder_point = ?, max_stock = ?, version = version + 1
		WHERE id = ? AND version = ?
	`

	result, err := r.db.Exec(query,
		inventory.Stock,
		inventory.ReservedStock,
		inventory.SoldStock,
		inventory.ReorderPoint,
		inventory.MaxStock,
		inventory.ID,
		inventory.Version,
	)

	if err != nil {
		return fmt.Errorf("failed to update inventory with version: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("inventory version conflict or record not found")
	}

	inventory.Version++
	return nil
}

// Delete 删除库存记录
func (r *inventoryRepo) Delete(id int64) error {
	query := `DELETE FROM inventory WHERE id = ?`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete inventory: %w", err)
	}

	return nil
}

// GetByProductIDs 根据商品ID列表批量获取库存
func (r *inventoryRepo) GetByProductIDs(productIDs []int64) ([]*domain.Inventory, error) {
	if len(productIDs) == 0 {
		return []*domain.Inventory{}, nil
	}

	// 构建IN子句
	placeholders := strings.Repeat("?,", len(productIDs)-1) + "?"
	query := fmt.Sprintf(`
		SELECT id, product_id, stock, reserved_stock, sold_stock, reorder_point, max_stock, version, created_at, updated_at
		FROM inventory 
		WHERE product_id IN (%s)
		ORDER BY product_id
	`, placeholders)

	args := make([]interface{}, len(productIDs))
	for i, id := range productIDs {
		args[i] = id
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventories by product ids: %w", err)
	}
	defer rows.Close()

	var inventories []*domain.Inventory
	for rows.Next() {
		inventory := &domain.Inventory{}
		err := rows.Scan(
			&inventory.ID,
			&inventory.ProductID,
			&inventory.Stock,
			&inventory.ReservedStock,
			&inventory.SoldStock,
			&inventory.ReorderPoint,
			&inventory.MaxStock,
			&inventory.Version,
			&inventory.CreatedAt,
			&inventory.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory: %w", err)
		}
		inventories = append(inventories, inventory)
	}

	return inventories, nil
}

// BatchUpdateStock 批量更新库存
func (r *inventoryRepo) BatchUpdateStock(updates []StockUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, update := range updates {
		switch update.Type {
		case "reserve":
			err = r.reserveStockInTx(tx, update.ProductID, update.Quantity)
		case "release":
			err = r.releaseStockInTx(tx, update.ProductID, update.Quantity)
		case "consume":
			err = r.consumeStockInTx(tx, update.ProductID, update.Quantity)
		default:
			err = fmt.Errorf("unknown stock update type: %s", update.Type)
		}

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// List 获取库存列表
func (r *inventoryRepo) List(req *domain.InventoryListRequest) ([]*domain.Inventory, int64, error) {
	// 构建查询条件
	where, args := r.buildListWhereClause(req)

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM inventory %s", where)
	var total int64
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count inventories: %w", err)
	}

	// 构建排序和分页
	orderBy := r.buildOrderClause(req)
	limit := req.PageSize
	offset := (req.Page - 1) * req.PageSize

	// 查询数据
	query := fmt.Sprintf(`
		SELECT id, product_id, stock, reserved_stock, sold_stock, reorder_point, max_stock, version, created_at, updated_at
		FROM inventory %s %s LIMIT ? OFFSET ?
	`, where, orderBy)

	args = append(args, limit, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query inventories: %w", err)
	}
	defer rows.Close()

	var inventories []*domain.Inventory
	for rows.Next() {
		inventory := &domain.Inventory{}
		err := rows.Scan(
			&inventory.ID,
			&inventory.ProductID,
			&inventory.Stock,
			&inventory.ReservedStock,
			&inventory.SoldStock,
			&inventory.ReorderPoint,
			&inventory.MaxStock,
			&inventory.Version,
			&inventory.CreatedAt,
			&inventory.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan inventory: %w", err)
		}
		inventories = append(inventories, inventory)
	}

	return inventories, total, nil
}

// GetLowStockProducts 获取低库存商品
func (r *inventoryRepo) GetLowStockProducts() ([]*domain.Inventory, error) {
	query := `
		SELECT id, product_id, stock, reserved_stock, sold_stock, reorder_point, max_stock, version, created_at, updated_at
		FROM inventory 
		WHERE stock <= reorder_point
		ORDER BY stock ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query low stock products: %w", err)
	}
	defer rows.Close()

	var inventories []*domain.Inventory
	for rows.Next() {
		inventory := &domain.Inventory{}
		err := rows.Scan(
			&inventory.ID,
			&inventory.ProductID,
			&inventory.Stock,
			&inventory.ReservedStock,
			&inventory.SoldStock,
			&inventory.ReorderPoint,
			&inventory.MaxStock,
			&inventory.Version,
			&inventory.CreatedAt,
			&inventory.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory: %w", err)
		}
		inventories = append(inventories, inventory)
	}

	return inventories, nil
}

// ReserveStock 预留库存
func (r *inventoryRepo) ReserveStock(productID int64, quantity int) error {
	query := `
		UPDATE inventory 
		SET reserved_stock = reserved_stock + ?, version = version + 1
		WHERE product_id = ? AND (stock - reserved_stock) >= ?
	`

	result, err := r.db.Exec(query, quantity, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to reserve stock: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("insufficient stock to reserve")
	}

	return nil
}

// ReleaseStock 释放预留库存
func (r *inventoryRepo) ReleaseStock(productID int64, quantity int) error {
	query := `
		UPDATE inventory 
		SET reserved_stock = reserved_stock - ?, version = version + 1
		WHERE product_id = ? AND reserved_stock >= ?
	`

	result, err := r.db.Exec(query, quantity, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to release stock: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("insufficient reserved stock to release")
	}

	return nil
}

// ConsumeStock 消费库存
func (r *inventoryRepo) ConsumeStock(productID int64, quantity int) error {
	query := `
		UPDATE inventory 
		SET stock = stock - ?, reserved_stock = reserved_stock - ?, sold_stock = sold_stock + ?, version = version + 1
		WHERE product_id = ? AND reserved_stock >= ?
	`

	result, err := r.db.Exec(query, quantity, quantity, quantity, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to consume stock: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("insufficient reserved stock to consume")
	}

	return nil
}

// AdjustStock 调整库存
func (r *inventoryRepo) AdjustStock(productID int64, quantity int, reason string) error {
	query := `
		UPDATE inventory 
		SET stock = stock + ?, version = version + 1
		WHERE product_id = ? AND stock + ? >= 0
	`

	result, err := r.db.Exec(query, quantity, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to adjust stock: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("stock adjustment would result in negative stock")
	}

	return nil
}

// Count 获取库存记录总数
func (r *inventoryRepo) Count() (int64, error) {
	query := "SELECT COUNT(*) FROM inventory"

	var count int64
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count inventories: %w", err)
	}

	return count, nil
}

// GetTotalStockValue 获取总库存价值
func (r *inventoryRepo) GetTotalStockValue() (float64, error) {
	query := `
		SELECT COALESCE(SUM(i.stock * p.price), 0)
		FROM inventory i
		JOIN products p ON i.product_id = p.id
		WHERE p.status = 'active'
	`

	var value float64
	err := r.db.QueryRow(query).Scan(&value)
	if err != nil {
		return 0, fmt.Errorf("failed to get total stock value: %w", err)
	}

	return value, nil
}

// 事务内的库存操作方法
func (r *inventoryRepo) reserveStockInTx(tx *sql.Tx, productID int64, quantity int) error {
	query := `
		UPDATE inventory 
		SET reserved_stock = reserved_stock + ?, version = version + 1
		WHERE product_id = ? AND (stock - reserved_stock) >= ?
	`

	result, err := tx.Exec(query, quantity, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to reserve stock in tx: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("insufficient stock to reserve")
	}

	return nil
}

func (r *inventoryRepo) releaseStockInTx(tx *sql.Tx, productID int64, quantity int) error {
	query := `
		UPDATE inventory 
		SET reserved_stock = reserved_stock - ?, version = version + 1
		WHERE product_id = ? AND reserved_stock >= ?
	`

	result, err := tx.Exec(query, quantity, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to release stock in tx: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("insufficient reserved stock to release")
	}

	return nil
}

func (r *inventoryRepo) consumeStockInTx(tx *sql.Tx, productID int64, quantity int) error {
	query := `
		UPDATE inventory 
		SET stock = stock - ?, reserved_stock = reserved_stock - ?, sold_stock = sold_stock + ?, version = version + 1
		WHERE product_id = ? AND reserved_stock >= ?
	`

	result, err := tx.Exec(query, quantity, quantity, quantity, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to consume stock in tx: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("insufficient reserved stock to consume")
	}

	return nil
}

// buildListWhereClause 构建查询条件子句
func (r *inventoryRepo) buildListWhereClause(req *domain.InventoryListRequest) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// 商品ID过滤
	if req.ProductID != nil {
		conditions = append(conditions, "product_id = ?")
		args = append(args, *req.ProductID)
	}

	// 低库存过滤
	if req.LowStock != nil && *req.LowStock {
		conditions = append(conditions, "stock <= reorder_point")
	}

	// 最小库存过滤
	if req.MinStock != nil {
		conditions = append(conditions, "stock >= ?")
		args = append(args, *req.MinStock)
	}

	// 最大库存过滤
	if req.MaxStock != nil {
		conditions = append(conditions, "stock <= ?")
		args = append(args, *req.MaxStock)
	}

	if len(conditions) > 0 {
		return "WHERE " + strings.Join(conditions, " AND "), args
	}

	return "", args
}

// buildOrderClause 构建排序子句
func (r *inventoryRepo) buildOrderClause(req *domain.InventoryListRequest) string {
	sortBy := "updated_at"
	sortOrder := "DESC"

	if req.SortBy != nil {
		switch *req.SortBy {
		case "stock", "updated_at", "created_at":
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
