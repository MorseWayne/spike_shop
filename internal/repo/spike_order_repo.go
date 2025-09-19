// Package repo 实现秒杀订单数据访问层，负责与数据库的交互。
package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

// SpikeOrderRepository 定义秒杀订单数据访问接口
type SpikeOrderRepository interface {
	// 基本CRUD操作
	Create(order *domain.SpikeOrder) error
	GetByID(id int64) (*domain.SpikeOrder, error)
	Update(order *domain.SpikeOrder) error
	Delete(id int64) error

	// 查询操作
	List(req *domain.SpikeOrderListRequest) ([]*domain.SpikeOrder, int64, error)
	GetByUserID(userID int64) ([]*domain.SpikeOrder, error)
	GetBySpikeEventID(spikeEventID int64) ([]*domain.SpikeOrder, error)
	GetByIdempotencyKey(key string) (*domain.SpikeOrder, error)

	// 业务特定操作
	GetByUserAndEvent(userID, spikeEventID int64) (*domain.SpikeOrder, error)
	UpdateStatus(id int64, status domain.SpikeOrderStatus) error
	UpdateOrderID(id int64, orderID int64) error
	UpdatePaymentInfo(id int64, paidAt time.Time) error
	GetExpiredOrders(before time.Time) ([]*domain.SpikeOrder, error)

	// 统计操作
	Count() (int64, error)
	CountByStatus(status domain.SpikeOrderStatus) (int64, error)
	CountByUserAndEvent(userID, spikeEventID int64) (int64, error)
}

// spikeOrderRepo 实现SpikeOrderRepository接口
type spikeOrderRepo struct {
	db *sql.DB
}

// NewSpikeOrderRepository 创建秒杀订单仓储实例
func NewSpikeOrderRepository(db *sql.DB) SpikeOrderRepository {
	return &spikeOrderRepo{db: db}
}

// Create 创建秒杀订单
func (r *spikeOrderRepo) Create(order *domain.SpikeOrder) error {
	query := `
		INSERT INTO spike_orders (spike_event_id, user_id, order_id, quantity, spike_price, 
			total_amount, status, idempotency_key, expire_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		order.SpikeEventID,
		order.UserID,
		order.OrderID,
		order.Quantity,
		order.SpikePrice,
		order.TotalAmount,
		order.Status,
		order.IdempotencyKey,
		order.ExpireAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create spike order: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	order.ID = id
	return nil
}

// GetByID 根据ID获取秒杀订单
func (r *spikeOrderRepo) GetByID(id int64) (*domain.SpikeOrder, error) {
	query := `
		SELECT id, spike_event_id, user_id, order_id, quantity, spike_price, total_amount,
			status, idempotency_key, expire_at, paid_at, cancelled_at, created_at, updated_at
		FROM spike_orders
		WHERE id = ?
	`

	order := &domain.SpikeOrder{}
	err := r.db.QueryRow(query, id).Scan(
		&order.ID,
		&order.SpikeEventID,
		&order.UserID,
		&order.OrderID,
		&order.Quantity,
		&order.SpikePrice,
		&order.TotalAmount,
		&order.Status,
		&order.IdempotencyKey,
		&order.ExpireAt,
		&order.PaidAt,
		&order.CancelledAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("spike order with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to get spike order by id: %w", err)
	}

	return order, nil
}

// Update 更新秒杀订单
func (r *spikeOrderRepo) Update(order *domain.SpikeOrder) error {
	query := `
		UPDATE spike_orders 
		SET spike_event_id = ?, user_id = ?, order_id = ?, quantity = ?, spike_price = ?,
			total_amount = ?, status = ?, idempotency_key = ?, expire_at = ?, paid_at = ?, cancelled_at = ?
		WHERE id = ?
	`

	result, err := r.db.Exec(query,
		order.SpikeEventID,
		order.UserID,
		order.OrderID,
		order.Quantity,
		order.SpikePrice,
		order.TotalAmount,
		order.Status,
		order.IdempotencyKey,
		order.ExpireAt,
		order.PaidAt,
		order.CancelledAt,
		order.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update spike order: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike order with id %d not found", order.ID)
	}

	return nil
}

// Delete 删除秒杀订单
func (r *spikeOrderRepo) Delete(id int64) error {
	query := `DELETE FROM spike_orders WHERE id = ?`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete spike order: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike order with id %d not found", id)
	}

	return nil
}

// List 分页查询秒杀订单列表
func (r *spikeOrderRepo) List(req *domain.SpikeOrderListRequest) ([]*domain.SpikeOrder, int64, error) {
	// 构建WHERE条件
	var conditions []string
	var args []interface{}

	if req.UserID != nil {
		conditions = append(conditions, "user_id = ?")
		args = append(args, *req.UserID)
	}

	if req.SpikeEventID != nil {
		conditions = append(conditions, "spike_event_id = ?")
		args = append(args, *req.SpikeEventID)
	}

	if req.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *req.Status)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 构建排序
	sortBy := "created_at"
	if req.SortBy != nil {
		switch *req.SortBy {
		case "created_at", "total_amount":
			sortBy = *req.SortBy
		}
	}

	sortOrder := "DESC"
	if req.SortOrder != nil && strings.ToUpper(*req.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM spike_orders %s", whereClause)
	var total int64
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count spike orders: %w", err)
	}

	// 分页参数
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize

	// 查询数据
	query := fmt.Sprintf(`
		SELECT id, spike_event_id, user_id, order_id, quantity, spike_price, total_amount,
			status, idempotency_key, expire_at, paid_at, cancelled_at, created_at, updated_at
		FROM spike_orders %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, whereClause, sortBy, sortOrder)

	args = append(args, req.PageSize, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query spike orders: %w", err)
	}
	defer rows.Close()

	var orders []*domain.SpikeOrder
	for rows.Next() {
		order := &domain.SpikeOrder{}
		err := rows.Scan(
			&order.ID,
			&order.SpikeEventID,
			&order.UserID,
			&order.OrderID,
			&order.Quantity,
			&order.SpikePrice,
			&order.TotalAmount,
			&order.Status,
			&order.IdempotencyKey,
			&order.ExpireAt,
			&order.PaidAt,
			&order.CancelledAt,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan spike order: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return orders, total, nil
}

// GetByUserID 根据用户ID获取秒杀订单列表
func (r *spikeOrderRepo) GetByUserID(userID int64) ([]*domain.SpikeOrder, error) {
	query := `
		SELECT id, spike_event_id, user_id, order_id, quantity, spike_price, total_amount,
			status, idempotency_key, expire_at, paid_at, cancelled_at, created_at, updated_at
		FROM spike_orders
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query spike orders by user id: %w", err)
	}
	defer rows.Close()

	var orders []*domain.SpikeOrder
	for rows.Next() {
		order := &domain.SpikeOrder{}
		err := rows.Scan(
			&order.ID,
			&order.SpikeEventID,
			&order.UserID,
			&order.OrderID,
			&order.Quantity,
			&order.SpikePrice,
			&order.TotalAmount,
			&order.Status,
			&order.IdempotencyKey,
			&order.ExpireAt,
			&order.PaidAt,
			&order.CancelledAt,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan spike order: %w", err)
		}
		orders = append(orders, order)
	}

	return orders, rows.Err()
}

// GetBySpikeEventID 根据秒杀活动ID获取订单列表
func (r *spikeOrderRepo) GetBySpikeEventID(spikeEventID int64) ([]*domain.SpikeOrder, error) {
	query := `
		SELECT id, spike_event_id, user_id, order_id, quantity, spike_price, total_amount,
			status, idempotency_key, expire_at, paid_at, cancelled_at, created_at, updated_at
		FROM spike_orders
		WHERE spike_event_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, spikeEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query spike orders by event id: %w", err)
	}
	defer rows.Close()

	var orders []*domain.SpikeOrder
	for rows.Next() {
		order := &domain.SpikeOrder{}
		err := rows.Scan(
			&order.ID,
			&order.SpikeEventID,
			&order.UserID,
			&order.OrderID,
			&order.Quantity,
			&order.SpikePrice,
			&order.TotalAmount,
			&order.Status,
			&order.IdempotencyKey,
			&order.ExpireAt,
			&order.PaidAt,
			&order.CancelledAt,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan spike order: %w", err)
		}
		orders = append(orders, order)
	}

	return orders, rows.Err()
}

// GetByIdempotencyKey 根据幂等键获取秒杀订单
func (r *spikeOrderRepo) GetByIdempotencyKey(key string) (*domain.SpikeOrder, error) {
	query := `
		SELECT id, spike_event_id, user_id, order_id, quantity, spike_price, total_amount,
			status, idempotency_key, expire_at, paid_at, cancelled_at, created_at, updated_at
		FROM spike_orders
		WHERE idempotency_key = ?
	`

	order := &domain.SpikeOrder{}
	err := r.db.QueryRow(query, key).Scan(
		&order.ID,
		&order.SpikeEventID,
		&order.UserID,
		&order.OrderID,
		&order.Quantity,
		&order.SpikePrice,
		&order.TotalAmount,
		&order.Status,
		&order.IdempotencyKey,
		&order.ExpireAt,
		&order.PaidAt,
		&order.CancelledAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 幂等键不存在
		}
		return nil, fmt.Errorf("failed to get spike order by idempotency key: %w", err)
	}

	return order, nil
}

// GetByUserAndEvent 根据用户ID和活动ID获取秒杀订单
func (r *spikeOrderRepo) GetByUserAndEvent(userID, spikeEventID int64) (*domain.SpikeOrder, error) {
	query := `
		SELECT id, spike_event_id, user_id, order_id, quantity, spike_price, total_amount,
			status, idempotency_key, expire_at, paid_at, cancelled_at, created_at, updated_at
		FROM spike_orders
		WHERE user_id = ? AND spike_event_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	order := &domain.SpikeOrder{}
	err := r.db.QueryRow(query, userID, spikeEventID).Scan(
		&order.ID,
		&order.SpikeEventID,
		&order.UserID,
		&order.OrderID,
		&order.Quantity,
		&order.SpikePrice,
		&order.TotalAmount,
		&order.Status,
		&order.IdempotencyKey,
		&order.ExpireAt,
		&order.PaidAt,
		&order.CancelledAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 订单不存在
		}
		return nil, fmt.Errorf("failed to get spike order by user and event: %w", err)
	}

	return order, nil
}

// UpdateStatus 更新订单状态
func (r *spikeOrderRepo) UpdateStatus(id int64, status domain.SpikeOrderStatus) error {
	var query string
	var args []interface{}

	switch status {
	case domain.SpikeOrderStatusCancelled:
		query = `UPDATE spike_orders SET status = ?, cancelled_at = ? WHERE id = ?`
		args = []interface{}{status, time.Now(), id}
	default:
		query = `UPDATE spike_orders SET status = ? WHERE id = ?`
		args = []interface{}{status, id}
	}

	result, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike order with id %d not found", id)
	}

	return nil
}

// UpdateOrderID 更新关联的普通订单ID
func (r *spikeOrderRepo) UpdateOrderID(id int64, orderID int64) error {
	query := `UPDATE spike_orders SET order_id = ? WHERE id = ?`

	result, err := r.db.Exec(query, orderID, id)
	if err != nil {
		return fmt.Errorf("failed to update order id: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike order with id %d not found", id)
	}

	return nil
}

// UpdatePaymentInfo 更新支付信息
func (r *spikeOrderRepo) UpdatePaymentInfo(id int64, paidAt time.Time) error {
	query := `UPDATE spike_orders SET status = ?, paid_at = ? WHERE id = ?`

	result, err := r.db.Exec(query, domain.SpikeOrderStatusPaid, paidAt, id)
	if err != nil {
		return fmt.Errorf("failed to update payment info: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike order with id %d not found", id)
	}

	return nil
}

// GetExpiredOrders 获取过期的订单
func (r *spikeOrderRepo) GetExpiredOrders(before time.Time) ([]*domain.SpikeOrder, error) {
	query := `
		SELECT id, spike_event_id, user_id, order_id, quantity, spike_price, total_amount,
			status, idempotency_key, expire_at, paid_at, cancelled_at, created_at, updated_at
		FROM spike_orders
		WHERE status = ? AND expire_at IS NOT NULL AND expire_at < ?
		ORDER BY expire_at ASC
	`

	rows, err := r.db.Query(query, domain.SpikeOrderStatusPending, before)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired orders: %w", err)
	}
	defer rows.Close()

	var orders []*domain.SpikeOrder
	for rows.Next() {
		order := &domain.SpikeOrder{}
		err := rows.Scan(
			&order.ID,
			&order.SpikeEventID,
			&order.UserID,
			&order.OrderID,
			&order.Quantity,
			&order.SpikePrice,
			&order.TotalAmount,
			&order.Status,
			&order.IdempotencyKey,
			&order.ExpireAt,
			&order.PaidAt,
			&order.CancelledAt,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan expired order: %w", err)
		}
		orders = append(orders, order)
	}

	return orders, rows.Err()
}

// Count 统计秒杀订单总数
func (r *spikeOrderRepo) Count() (int64, error) {
	query := `SELECT COUNT(*) FROM spike_orders`

	var count int64
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count spike orders: %w", err)
	}

	return count, nil
}

// CountByStatus 根据状态统计秒杀订单数量
func (r *spikeOrderRepo) CountByStatus(status domain.SpikeOrderStatus) (int64, error) {
	query := `SELECT COUNT(*) FROM spike_orders WHERE status = ?`

	var count int64
	err := r.db.QueryRow(query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count spike orders by status: %w", err)
	}

	return count, nil
}

// CountByUserAndEvent 统计用户在特定活动的订单数量
func (r *spikeOrderRepo) CountByUserAndEvent(userID, spikeEventID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM spike_orders WHERE user_id = ? AND spike_event_id = ?`

	var count int64
	err := r.db.QueryRow(query, userID, spikeEventID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count spike orders by user and event: %w", err)
	}

	return count, nil
}
