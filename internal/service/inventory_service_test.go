package service

import (
	"testing"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

func TestInventoryService_CreateInventory(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewInventoryService(inventoryRepo, productRepo)

	// Create a test product first
	testProduct := &domain.Product{
		Name:        "Test Product",
		Description: "Test Description",
		Price:       99.99,
		SKU:         "TEST-001",
		Brand:       "Test Brand",
		Status:      domain.ProductStatusActive,
	}
	err := productRepo.Create(testProduct)
	if err != nil {
		t.Fatalf("Failed to create test product: %v", err)
	}

	tests := []struct {
		name    string
		req     *domain.CreateInventoryRequest
		wantErr bool
	}{
		{
			name: "valid inventory",
			req: &domain.CreateInventoryRequest{
				ProductID:    1, // Assuming first product gets ID 1
				Stock:        100,
				ReorderPoint: 10,
				MaxStock:     1000,
			},
			wantErr: false,
		},
		{
			name: "invalid product ID",
			req: &domain.CreateInventoryRequest{
				ProductID:    999,
				Stock:        100,
				ReorderPoint: 10,
				MaxStock:     1000,
			},
			wantErr: true,
		},
		{
			name: "negative stock",
			req: &domain.CreateInventoryRequest{
				ProductID:    1,
				Stock:        -10,
				ReorderPoint: 10,
				MaxStock:     1000,
			},
			wantErr: true,
		},
		{
			name: "stock exceeds max stock",
			req: &domain.CreateInventoryRequest{
				ProductID:    1,
				Stock:        1100,
				ReorderPoint: 10,
				MaxStock:     1000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inventory, err := service.CreateInventory(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateInventory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if inventory == nil {
					t.Errorf("CreateInventory() returned nil inventory")
					return
				}
				if inventory.ProductID != tt.req.ProductID {
					t.Errorf("CreateInventory() ProductID = %v, want %v", inventory.ProductID, tt.req.ProductID)
				}
				if inventory.Stock != tt.req.Stock {
					t.Errorf("CreateInventory() Stock = %v, want %v", inventory.Stock, tt.req.Stock)
				}
			}
		})
	}
}

func TestInventoryService_ReserveStock(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewInventoryService(inventoryRepo, productRepo)

	// Create a test product
	product := &domain.Product{
		ID:     1,
		Name:   "Test Product",
		SKU:    "TEST-001",
		Price:  99.99,
		Status: domain.ProductStatusActive,
	}
	productRepo.products[1] = product

	// Create inventory
	inventory := &domain.Inventory{
		ID:            1,
		ProductID:     1,
		Stock:         100,
		ReservedStock: 0,
		SoldStock:     0,
		ReorderPoint:  10,
		MaxStock:      1000,
	}
	inventoryRepo.inventories[1] = inventory
	inventoryRepo.productMap[1] = inventory

	tests := []struct {
		name    string
		req     *domain.ReserveStockRequest
		wantErr bool
	}{
		{
			name: "valid reservation",
			req: &domain.ReserveStockRequest{
				ProductID: 1,
				Quantity:  10,
			},
			wantErr: false,
		},
		{
			name: "insufficient stock",
			req: &domain.ReserveStockRequest{
				ProductID: 1,
				Quantity:  200, // More than available stock
			},
			wantErr: true,
		},
		{
			name: "invalid product",
			req: &domain.ReserveStockRequest{
				ProductID: 999,
				Quantity:  10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ReserveStock(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReserveStock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInventoryService_ReleaseStock(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewInventoryService(inventoryRepo, productRepo)

	// Create inventory with reserved stock
	inventory := &domain.Inventory{
		ID:            1,
		ProductID:     1,
		Stock:         100,
		ReservedStock: 20,
		SoldStock:     0,
		ReorderPoint:  10,
		MaxStock:      1000,
	}
	inventoryRepo.inventories[1] = inventory
	inventoryRepo.productMap[1] = inventory

	tests := []struct {
		name    string
		req     *domain.ReleaseStockRequest
		wantErr bool
	}{
		{
			name: "valid release",
			req: &domain.ReleaseStockRequest{
				ProductID: 1,
				Quantity:  10,
			},
			wantErr: false,
		},
		{
			name: "insufficient reserved stock",
			req: &domain.ReleaseStockRequest{
				ProductID: 1,
				Quantity:  50, // More than reserved
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ReleaseStock(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReleaseStock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInventoryService_ConsumeStock(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewInventoryService(inventoryRepo, productRepo)

	// Create inventory with reserved stock
	inventory := &domain.Inventory{
		ID:            1,
		ProductID:     1,
		Stock:         100,
		ReservedStock: 20,
		SoldStock:     0,
		ReorderPoint:  10,
		MaxStock:      1000,
	}
	inventoryRepo.inventories[1] = inventory
	inventoryRepo.productMap[1] = inventory

	tests := []struct {
		name    string
		req     *domain.ConsumeStockRequest
		wantErr bool
	}{
		{
			name: "valid consumption",
			req: &domain.ConsumeStockRequest{
				ProductID: 1,
				Quantity:  10,
			},
			wantErr: false,
		},
		{
			name: "insufficient reserved stock",
			req: &domain.ConsumeStockRequest{
				ProductID: 1,
				Quantity:  50, // More than reserved
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ConsumeStock(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConsumeStock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInventoryService_AdjustStock(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewInventoryService(inventoryRepo, productRepo)

	// Create a test product
	product := &domain.Product{
		ID:     1,
		Name:   "Test Product",
		SKU:    "TEST-001",
		Price:  99.99,
		Status: domain.ProductStatusActive,
	}
	productRepo.products[1] = product

	// Create inventory
	inventory := &domain.Inventory{
		ID:            1,
		ProductID:     1,
		Stock:         100,
		ReservedStock: 0,
		SoldStock:     0,
		ReorderPoint:  10,
		MaxStock:      1000,
	}
	inventoryRepo.inventories[1] = inventory
	inventoryRepo.productMap[1] = inventory

	tests := []struct {
		name    string
		req     *domain.StockAdjustmentRequest
		wantErr bool
	}{
		{
			name: "valid increase",
			req: &domain.StockAdjustmentRequest{
				Quantity: 50,
				Reason:   "Restock",
				Type:     "in",
			},
			wantErr: false,
		},
		{
			name: "valid decrease",
			req: &domain.StockAdjustmentRequest{
				Quantity: 10,
				Reason:   "Damaged goods",
				Type:     "out",
			},
			wantErr: false,
		},
		{
			name: "excessive decrease",
			req: &domain.StockAdjustmentRequest{
				Quantity: 200,
				Reason:   "Loss",
				Type:     "out",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.AdjustStock(1, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("AdjustStock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInventoryService_GetLowStockAlerts(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewInventoryService(inventoryRepo, productRepo)

	// Create test products
	product1 := &domain.Product{
		ID:    1,
		Name:  "Low Stock Product",
		SKU:   "LOW-001",
		Price: 99.99,
	}
	product2 := &domain.Product{
		ID:    2,
		Name:  "Normal Stock Product",
		SKU:   "NORMAL-001",
		Price: 199.99,
	}
	productRepo.products[1] = product1
	productRepo.products[2] = product2

	// Create inventories
	lowStockInventory := &domain.Inventory{
		ID:           1,
		ProductID:    1,
		Stock:        5, // Below reorder point
		ReorderPoint: 10,
	}
	normalStockInventory := &domain.Inventory{
		ID:           2,
		ProductID:    2,
		Stock:        50, // Above reorder point
		ReorderPoint: 10,
	}

	inventoryRepo.inventories[1] = lowStockInventory
	inventoryRepo.inventories[2] = normalStockInventory
	inventoryRepo.productMap[1] = lowStockInventory
	inventoryRepo.productMap[2] = normalStockInventory

	alerts, err := service.GetLowStockAlerts()
	if err != nil {
		t.Errorf("GetLowStockAlerts() error = %v", err)
		return
	}

	if len(alerts) != 1 {
		t.Errorf("GetLowStockAlerts() returned %d alerts, want 1", len(alerts))
		return
	}

	alert := alerts[0]
	if alert.ProductID != 1 {
		t.Errorf("GetLowStockAlerts() ProductID = %v, want 1", alert.ProductID)
	}
	if alert.CurrentStock != 5 {
		t.Errorf("GetLowStockAlerts() CurrentStock = %v, want 5", alert.CurrentStock)
	}
	if alert.StockShortage != 5 { // 10 - 5
		t.Errorf("GetLowStockAlerts() StockShortage = %v, want 5", alert.StockShortage)
	}
}

func TestInventoryService_CheckStockAvailability(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewInventoryService(inventoryRepo, productRepo)

	// Create inventory
	inventory := &domain.Inventory{
		ID:            1,
		ProductID:     1,
		Stock:         100,
		ReservedStock: 20,
		SoldStock:     0,
		ReorderPoint:  10,
		MaxStock:      1000,
	}
	inventoryRepo.inventories[1] = inventory
	inventoryRepo.productMap[1] = inventory

	tests := []struct {
		name      string
		productID int64
		quantity  int
		want      bool
		wantErr   bool
	}{
		{
			name:      "available stock",
			productID: 1,
			quantity:  50, // Less than available (100-20=80)
			want:      true,
			wantErr:   false,
		},
		{
			name:      "insufficient stock",
			productID: 1,
			quantity:  90, // More than available (100-20=80)
			want:      false,
			wantErr:   false,
		},
		{
			name:      "non-existent inventory",
			productID: 999,
			quantity:  10,
			want:      false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			available, err := service.CheckStockAvailability(tt.productID, tt.quantity)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckStockAvailability() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if available != tt.want {
				t.Errorf("CheckStockAvailability() = %v, want %v", available, tt.want)
			}
		})
	}
}
