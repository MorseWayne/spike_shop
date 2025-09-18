package service

import (
	"testing"

	"github.com/MorseWayne/spike_shop/internal/domain"
)

// Test cases for ProductService
func TestProductService_CreateProduct(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewProductService(productRepo, inventoryRepo)

	tests := []struct {
		name    string
		req     *domain.CreateProductRequest
		wantErr bool
	}{
		{
			name: "valid product",
			req: &domain.CreateProductRequest{
				Name:        "Test Product",
				Description: "Test Description",
				Price:       99.99,
				SKU:         "TEST-001",
				Brand:       "Test Brand",
			},
			wantErr: false,
		},
		{
			name: "duplicate SKU",
			req: &domain.CreateProductRequest{
				Name:        "Test Product 2",
				Description: "Test Description 2",
				Price:       199.99,
				SKU:         "TEST-001", // Same SKU as above
				Brand:       "Test Brand",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product, err := service.CreateProduct(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProduct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if product == nil {
					t.Errorf("CreateProduct() returned nil product")
					return
				}
				if product.Name != tt.req.Name {
					t.Errorf("CreateProduct() name = %v, want %v", product.Name, tt.req.Name)
				}
				if product.SKU != tt.req.SKU {
					t.Errorf("CreateProduct() SKU = %v, want %v", product.SKU, tt.req.SKU)
				}
			}
		})
	}
}

func TestProductService_GetProduct(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewProductService(productRepo, inventoryRepo)

	// Create a test product
	req := &domain.CreateProductRequest{
		Name:        "Test Product",
		Description: "Test Description",
		Price:       99.99,
		SKU:         "TEST-001",
		Brand:       "Test Brand",
	}
	createdProduct, err := service.CreateProduct(req)
	if err != nil {
		t.Fatalf("Failed to create test product: %v", err)
	}

	tests := []struct {
		name    string
		id      int64
		wantErr bool
	}{
		{
			name:    "existing product",
			id:      createdProduct.ID,
			wantErr: false,
		},
		{
			name:    "non-existing product",
			id:      999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product, err := service.GetProduct(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProduct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if product == nil {
					t.Errorf("GetProduct() returned nil product")
					return
				}
				if product.ID != tt.id {
					t.Errorf("GetProduct() ID = %v, want %v", product.ID, tt.id)
				}
			}
		})
	}
}

func TestProductService_UpdateProduct(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewProductService(productRepo, inventoryRepo)

	// Create a test product
	req := &domain.CreateProductRequest{
		Name:        "Test Product",
		Description: "Test Description",
		Price:       99.99,
		SKU:         "TEST-001",
		Brand:       "Test Brand",
	}
	createdProduct, err := service.CreateProduct(req)
	if err != nil {
		t.Fatalf("Failed to create test product: %v", err)
	}

	newName := "Updated Product"
	newPrice := 199.99
	updateReq := &domain.UpdateProductRequest{
		Name:  &newName,
		Price: &newPrice,
	}

	updatedProduct, err := service.UpdateProduct(createdProduct.ID, updateReq)
	if err != nil {
		t.Errorf("UpdateProduct() error = %v", err)
		return
	}

	if updatedProduct.Name != newName {
		t.Errorf("UpdateProduct() name = %v, want %v", updatedProduct.Name, newName)
	}
	if updatedProduct.Price != newPrice {
		t.Errorf("UpdateProduct() price = %v, want %v", updatedProduct.Price, newPrice)
	}
}

func TestProductService_DeleteProduct(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewProductService(productRepo, inventoryRepo)

	// Create a test product
	req := &domain.CreateProductRequest{
		Name:        "Test Product",
		Description: "Test Description",
		Price:       99.99,
		SKU:         "TEST-001",
		Brand:       "Test Brand",
	}
	createdProduct, err := service.CreateProduct(req)
	if err != nil {
		t.Fatalf("Failed to create test product: %v", err)
	}

	// Test deleting the product
	err = service.DeleteProduct(createdProduct.ID)
	if err != nil {
		t.Errorf("DeleteProduct() error = %v", err)
		return
	}

	// Verify product is deleted
	_, err = service.GetProduct(createdProduct.ID)
	if err == nil {
		t.Errorf("Expected error when getting deleted product, but got none")
	}
}

func TestProductService_ListProducts(t *testing.T) {
	productRepo := newMockProductRepository()
	inventoryRepo := newMockInventoryRepository()
	service := NewProductService(productRepo, inventoryRepo)

	// Create test products
	for i := 1; i <= 3; i++ {
		req := &domain.CreateProductRequest{
			Name:        "Test Product " + string(rune(i+'0')),
			Description: "Test Description",
			Price:       float64(i * 100),
			SKU:         "TEST-00" + string(rune(i+'0')),
			Brand:       "Test Brand",
		}
		_, err := service.CreateProduct(req)
		if err != nil {
			t.Fatalf("Failed to create test product %d: %v", i, err)
		}
	}

	listReq := &domain.ProductListRequest{
		Page:     1,
		PageSize: 10,
	}

	result, err := service.ListProducts(listReq)
	if err != nil {
		t.Errorf("ListProducts() error = %v", err)
		return
	}

	if len(result.Products) != 3 {
		t.Errorf("ListProducts() returned %d products, want 3", len(result.Products))
	}
	if result.Total != 3 {
		t.Errorf("ListProducts() total = %d, want 3", result.Total)
	}
}
