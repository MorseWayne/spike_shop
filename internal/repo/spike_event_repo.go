// Package repo 实现秒杀活动数据访问层，负责与数据库的交互。
package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

// SpikeEventRepository 定义秒杀活动数据访问接口
type SpikeEventRepository interface {
	// 基本CRUD操作
	Create(event *domain.SpikeEvent) error
	GetByID(id int64) (*domain.SpikeEvent, error)
	Update(event *domain.SpikeEvent) error
	Delete(id int64) error

	// 查询操作
	List(req *domain.SpikeEventListRequest) ([]*domain.SpikeEvent, int64, error)
	GetByProductID(productID int64) ([]*domain.SpikeEvent, error)
	GetActiveEvents() ([]*domain.SpikeEvent, error)
	GetEventsByTimeRange(start, end time.Time) ([]*domain.SpikeEvent, error)

	// 业务特定操作
	UpdateSoldCount(id int64, count int64) error
	UpdateStatus(id int64, status domain.SpikeEventStatus) error
	GetCurrentActiveEventByProductID(productID int64) (*domain.SpikeEvent, error)

	// 统计操作
	Count() (int64, error)
	CountByStatus(status domain.SpikeEventStatus) (int64, error)
}

// spikeEventRepo 实现SpikeEventRepository接口
type spikeEventRepo struct {
	db *sql.DB
}

// NewSpikeEventRepository 创建秒杀活动仓储实例
func NewSpikeEventRepository(db *sql.DB) SpikeEventRepository {
	return &spikeEventRepo{db: db}
}

// Create 创建秒杀活动
func (r *spikeEventRepo) Create(event *domain.SpikeEvent) error {
	query := `
		INSERT INTO spike_events (product_id, name, description, spike_price, original_price, 
			spike_stock, sold_count, start_at, end_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		event.ProductID,
		event.Name,
		event.Description,
		event.SpikePrice,
		event.OriginalPrice,
		event.SpikeStock,
		event.SoldCount,
		event.StartAt,
		event.EndAt,
		event.Status,
	)

	if err != nil {
		return fmt.Errorf("failed to create spike event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	event.ID = id
	return nil
}

// GetByID 根据ID获取秒杀活动
func (r *spikeEventRepo) GetByID(id int64) (*domain.SpikeEvent, error) {
	query := `
		SELECT id, product_id, name, description, spike_price, original_price,
			spike_stock, sold_count, start_at, end_at, status, created_at, updated_at
		FROM spike_events
		WHERE id = ?
	`

	event := &domain.SpikeEvent{}
	err := r.db.QueryRow(query, id).Scan(
		&event.ID,
		&event.ProductID,
		&event.Name,
		&event.Description,
		&event.SpikePrice,
		&event.OriginalPrice,
		&event.SpikeStock,
		&event.SoldCount,
		&event.StartAt,
		&event.EndAt,
		&event.Status,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("spike event with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to get spike event by id: %w", err)
	}

	return event, nil
}

// Update 更新秒杀活动
func (r *spikeEventRepo) Update(event *domain.SpikeEvent) error {
	query := `
		UPDATE spike_events 
		SET product_id = ?, name = ?, description = ?, spike_price = ?, original_price = ?,
			spike_stock = ?, sold_count = ?, start_at = ?, end_at = ?, status = ?
		WHERE id = ?
	`

	result, err := r.db.Exec(query,
		event.ProductID,
		event.Name,
		event.Description,
		event.SpikePrice,
		event.OriginalPrice,
		event.SpikeStock,
		event.SoldCount,
		event.StartAt,
		event.EndAt,
		event.Status,
		event.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update spike event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike event with id %d not found", event.ID)
	}

	return nil
}

// Delete 删除秒杀活动
func (r *spikeEventRepo) Delete(id int64) error {
	query := `DELETE FROM spike_events WHERE id = ?`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete spike event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike event with id %d not found", id)
	}

	return nil
}

// List 分页查询秒杀活动列表
func (r *spikeEventRepo) List(req *domain.SpikeEventListRequest) ([]*domain.SpikeEvent, int64, error) {
	// 构建WHERE条件
	var conditions []string
	var args []interface{}

	if req.ProductID != nil {
		conditions = append(conditions, "product_id = ?")
		args = append(args, *req.ProductID)
	}

	if req.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *req.Status)
	}

	if req.Active != nil && *req.Active {
		now := time.Now()
		conditions = append(conditions, "status = ? AND start_at <= ? AND end_at > ?")
		args = append(args, domain.SpikeEventStatusActive, now, now)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 构建排序
	sortBy := "created_at"
	if req.SortBy != nil {
		switch *req.SortBy {
		case "start_at", "spike_price", "created_at":
			sortBy = *req.SortBy
		}
	}

	sortOrder := "DESC"
	if req.SortOrder != nil && strings.ToUpper(*req.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM spike_events %s", whereClause)
	var total int64
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count spike events: %w", err)
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
		SELECT id, product_id, name, description, spike_price, original_price,
			spike_stock, sold_count, start_at, end_at, status, created_at, updated_at
		FROM spike_events %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, whereClause, sortBy, sortOrder)

	args = append(args, req.PageSize, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query spike events: %w", err)
	}
	defer rows.Close()

	var events []*domain.SpikeEvent
	for rows.Next() {
		event := &domain.SpikeEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ProductID,
			&event.Name,
			&event.Description,
			&event.SpikePrice,
			&event.OriginalPrice,
			&event.SpikeStock,
			&event.SoldCount,
			&event.StartAt,
			&event.EndAt,
			&event.Status,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan spike event: %w", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return events, total, nil
}

// GetByProductID 根据商品ID获取秒杀活动列表
func (r *spikeEventRepo) GetByProductID(productID int64) ([]*domain.SpikeEvent, error) {
	query := `
		SELECT id, product_id, name, description, spike_price, original_price,
			spike_stock, sold_count, start_at, end_at, status, created_at, updated_at
		FROM spike_events
		WHERE product_id = ?
		ORDER BY start_at DESC
	`

	rows, err := r.db.Query(query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to query spike events by product id: %w", err)
	}
	defer rows.Close()

	var events []*domain.SpikeEvent
	for rows.Next() {
		event := &domain.SpikeEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ProductID,
			&event.Name,
			&event.Description,
			&event.SpikePrice,
			&event.OriginalPrice,
			&event.SpikeStock,
			&event.SoldCount,
			&event.StartAt,
			&event.EndAt,
			&event.Status,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan spike event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// GetActiveEvents 获取当前活跃的秒杀活动
func (r *spikeEventRepo) GetActiveEvents() ([]*domain.SpikeEvent, error) {
	now := time.Now()
	query := `
		SELECT id, product_id, name, description, spike_price, original_price,
			spike_stock, sold_count, start_at, end_at, status, created_at, updated_at
		FROM spike_events
		WHERE status = ? AND start_at <= ? AND end_at > ?
		ORDER BY start_at ASC
	`

	rows, err := r.db.Query(query, domain.SpikeEventStatusActive, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to query active spike events: %w", err)
	}
	defer rows.Close()

	var events []*domain.SpikeEvent
	for rows.Next() {
		event := &domain.SpikeEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ProductID,
			&event.Name,
			&event.Description,
			&event.SpikePrice,
			&event.OriginalPrice,
			&event.SpikeStock,
			&event.SoldCount,
			&event.StartAt,
			&event.EndAt,
			&event.Status,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan spike event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// GetEventsByTimeRange 根据时间范围获取秒杀活动
func (r *spikeEventRepo) GetEventsByTimeRange(start, end time.Time) ([]*domain.SpikeEvent, error) {
	query := `
		SELECT id, product_id, name, description, spike_price, original_price,
			spike_stock, sold_count, start_at, end_at, status, created_at, updated_at
		FROM spike_events
		WHERE start_at < ? AND end_at > ?
		ORDER BY start_at ASC
	`

	rows, err := r.db.Query(query, end, start)
	if err != nil {
		return nil, fmt.Errorf("failed to query spike events by time range: %w", err)
	}
	defer rows.Close()

	var events []*domain.SpikeEvent
	for rows.Next() {
		event := &domain.SpikeEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ProductID,
			&event.Name,
			&event.Description,
			&event.SpikePrice,
			&event.OriginalPrice,
			&event.SpikeStock,
			&event.SoldCount,
			&event.StartAt,
			&event.EndAt,
			&event.Status,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan spike event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// UpdateSoldCount 更新已售数量
func (r *spikeEventRepo) UpdateSoldCount(id int64, count int64) error {
	query := `UPDATE spike_events SET sold_count = ? WHERE id = ?`

	result, err := r.db.Exec(query, count, id)
	if err != nil {
		return fmt.Errorf("failed to update sold count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike event with id %d not found", id)
	}

	return nil
}

// UpdateStatus 更新活动状态
func (r *spikeEventRepo) UpdateStatus(id int64, status domain.SpikeEventStatus) error {
	query := `UPDATE spike_events SET status = ? WHERE id = ?`

	result, err := r.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("spike event with id %d not found", id)
	}

	return nil
}

// GetCurrentActiveEventByProductID 获取商品当前活跃的秒杀活动
func (r *spikeEventRepo) GetCurrentActiveEventByProductID(productID int64) (*domain.SpikeEvent, error) {
	now := time.Now()
	query := `
		SELECT id, product_id, name, description, spike_price, original_price,
			spike_stock, sold_count, start_at, end_at, status, created_at, updated_at
		FROM spike_events
		WHERE product_id = ? AND status = ? AND start_at <= ? AND end_at > ?
		ORDER BY start_at DESC
		LIMIT 1
	`

	event := &domain.SpikeEvent{}
	err := r.db.QueryRow(query, productID, domain.SpikeEventStatusActive, now, now).Scan(
		&event.ID,
		&event.ProductID,
		&event.Name,
		&event.Description,
		&event.SpikePrice,
		&event.OriginalPrice,
		&event.SpikeStock,
		&event.SoldCount,
		&event.StartAt,
		&event.EndAt,
		&event.Status,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 没有活跃的秒杀活动
		}
		return nil, fmt.Errorf("failed to get current active event: %w", err)
	}

	return event, nil
}

// Count 统计秒杀活动总数
func (r *spikeEventRepo) Count() (int64, error) {
	query := `SELECT COUNT(*) FROM spike_events`

	var count int64
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count spike events: %w", err)
	}

	return count, nil
}

// CountByStatus 根据状态统计秒杀活动数量
func (r *spikeEventRepo) CountByStatus(status domain.SpikeEventStatus) (int64, error) {
	query := `SELECT COUNT(*) FROM spike_events WHERE status = ?`

	var count int64
	err := r.db.QueryRow(query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count spike events by status: %w", err)
	}

	return count, nil
}
