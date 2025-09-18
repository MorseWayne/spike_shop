package service

import (
	"errors"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/repo"
)

// Mock ProductRepository for testing
type mockProductRepository struct {
	products map[int64]*domain.Product
	skuMap   map[string]*domain.Product
	nextID   int64
}

func newMockProductRepository() *mockProductRepository {
	return &mockProductRepository{
		products: make(map[int64]*domain.Product),
		skuMap:   make(map[string]*domain.Product),
		nextID:   1,
	}
}

func (m *mockProductRepository) Create(product *domain.Product) error {
	if _, exists := m.skuMap[product.SKU]; exists {
		return errors.New("SKU already exists")
	}

	product.ID = m.nextID
	m.nextID++

	m.products[product.ID] = product
	m.skuMap[product.SKU] = product

	return nil
}

func (m *mockProductRepository) GetByID(id int64) (*domain.Product, error) {
	product, exists := m.products[id]
	if !exists {
		return nil, nil
	}
	return product, nil
}

func (m *mockProductRepository) GetBySKU(sku string) (*domain.Product, error) {
	product, exists := m.skuMap[sku]
	if !exists {
		return nil, nil
	}
	return product, nil
}

func (m *mockProductRepository) Update(product *domain.Product) error {
	if _, exists := m.products[product.ID]; !exists {
		return errors.New("product not found")
	}
	m.products[product.ID] = product
	return nil
}

func (m *mockProductRepository) Delete(id int64) error {
	product, exists := m.products[id]
	if !exists {
		return errors.New("product not found")
	}
	delete(m.products, id)
	delete(m.skuMap, product.SKU)
	return nil
}

func (m *mockProductRepository) List(req *domain.ProductListRequest) ([]*domain.Product, int64, error) {
	var result []*domain.Product
	for _, product := range m.products {
		result = append(result, product)
	}
	return result, int64(len(result)), nil
}

func (m *mockProductRepository) GetByIDs(ids []int64) ([]*domain.Product, error) {
	var result []*domain.Product
	for _, id := range ids {
		if product, exists := m.products[id]; exists {
			result = append(result, product)
		}
	}
	return result, nil
}

func (m *mockProductRepository) Count() (int64, error) {
	return int64(len(m.products)), nil
}

func (m *mockProductRepository) CountByStatus(status domain.ProductStatus) (int64, error) {
	count := int64(0)
	for _, product := range m.products {
		if product.Status == status {
			count++
		}
	}
	return count, nil
}

// Mock InventoryRepository for testing
type mockInventoryRepository struct {
	inventories map[int64]*domain.Inventory
	productMap  map[int64]*domain.Inventory
	nextID      int64
}

func newMockInventoryRepository() *mockInventoryRepository {
	return &mockInventoryRepository{
		inventories: make(map[int64]*domain.Inventory),
		productMap:  make(map[int64]*domain.Inventory),
		nextID:      1,
	}
}

func (m *mockInventoryRepository) Create(inventory *domain.Inventory) error {
	inventory.ID = m.nextID
	m.nextID++

	m.inventories[inventory.ID] = inventory
	m.productMap[inventory.ProductID] = inventory

	return nil
}

func (m *mockInventoryRepository) GetByID(id int64) (*domain.Inventory, error) {
	inventory, exists := m.inventories[id]
	if !exists {
		return nil, nil
	}
	return inventory, nil
}

func (m *mockInventoryRepository) GetByProductID(productID int64) (*domain.Inventory, error) {
	inventory, exists := m.productMap[productID]
	if !exists {
		return nil, nil
	}
	return inventory, nil
}

func (m *mockInventoryRepository) Update(inventory *domain.Inventory) error {
	if _, exists := m.inventories[inventory.ID]; !exists {
		return errors.New("inventory not found")
	}
	m.inventories[inventory.ID] = inventory
	m.productMap[inventory.ProductID] = inventory
	return nil
}

func (m *mockInventoryRepository) UpdateWithVersion(inventory *domain.Inventory) error {
	return m.Update(inventory)
}

func (m *mockInventoryRepository) Delete(id int64) error {
	inventory, exists := m.inventories[id]
	if !exists {
		return errors.New("inventory not found")
	}
	delete(m.inventories, id)
	delete(m.productMap, inventory.ProductID)
	return nil
}

func (m *mockInventoryRepository) GetByProductIDs(productIDs []int64) ([]*domain.Inventory, error) {
	var result []*domain.Inventory
	for _, productID := range productIDs {
		if inventory, exists := m.productMap[productID]; exists {
			result = append(result, inventory)
		}
	}
	return result, nil
}

func (m *mockInventoryRepository) BatchUpdateStock(updates []repo.StockUpdate) error {
	return nil
}

func (m *mockInventoryRepository) List(req *domain.InventoryListRequest) ([]*domain.Inventory, int64, error) {
	var result []*domain.Inventory
	for _, inventory := range m.inventories {
		result = append(result, inventory)
	}
	return result, int64(len(result)), nil
}

func (m *mockInventoryRepository) GetLowStockProducts() ([]*domain.Inventory, error) {
	var result []*domain.Inventory
	for _, inventory := range m.inventories {
		if inventory.IsLowStock() {
			result = append(result, inventory)
		}
	}
	return result, nil
}

func (m *mockInventoryRepository) ReserveStock(productID int64, quantity int) error {
	inventory, exists := m.productMap[productID]
	if !exists {
		return errors.New("inventory not found")
	}
	if !inventory.CanReserve(quantity) {
		return errors.New("insufficient stock")
	}
	inventory.ReservedStock += quantity
	return nil
}

func (m *mockInventoryRepository) ReleaseStock(productID int64, quantity int) error {
	inventory, exists := m.productMap[productID]
	if !exists {
		return errors.New("inventory not found")
	}
	if inventory.ReservedStock < quantity {
		return errors.New("insufficient reserved stock")
	}
	inventory.ReservedStock -= quantity
	return nil
}

func (m *mockInventoryRepository) ConsumeStock(productID int64, quantity int) error {
	inventory, exists := m.productMap[productID]
	if !exists {
		return errors.New("inventory not found")
	}
	if inventory.ReservedStock < quantity {
		return errors.New("insufficient reserved stock")
	}
	inventory.ReservedStock -= quantity
	inventory.Stock -= quantity
	inventory.SoldStock += quantity
	return nil
}

func (m *mockInventoryRepository) AdjustStock(productID int64, quantity int, reason string) error {
	inventory, exists := m.productMap[productID]
	if !exists {
		return errors.New("inventory not found")
	}
	if inventory.Stock+quantity < 0 {
		return errors.New("adjustment would result in negative stock")
	}
	inventory.Stock += quantity
	return nil
}

func (m *mockInventoryRepository) Count() (int64, error) {
	return int64(len(m.inventories)), nil
}

func (m *mockInventoryRepository) GetTotalStockValue() (float64, error) {
	return 0, nil
}
