package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/MorseWayne/spike_shop/internal/domain"
	"github.com/MorseWayne/spike_shop/internal/service"
)

// MockSpikeService for testing
type MockSpikeService struct {
	participateFunc     func(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64) (*domain.SpikeParticipationResponse, error)
	getEventDetailFunc  func(ctx context.Context, eventID int64) (*domain.SpikeEventWithProduct, error)
	getActiveEventsFunc func(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error)
	getUserOrdersFunc   func(ctx context.Context, userID int64, req *domain.SpikeOrderListRequest) (*domain.SpikeOrderListResponse, error)
	getOrderDetailFunc  func(ctx context.Context, orderID, userID int64) (*domain.SpikeOrderWithDetails, error)
	cancelOrderFunc     func(ctx context.Context, orderID, userID int64, req *domain.CancelSpikeOrderRequest) error
	getSpikeStatsFunc   func(ctx context.Context, eventID int64) (*service.SpikeStats, error)
	warmupStockFunc     func(ctx context.Context, eventID int64) error
}

func (m *MockSpikeService) ParticipateSpike(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64) (*domain.SpikeParticipationResponse, error) {
	if m.participateFunc != nil {
		return m.participateFunc(ctx, req, userID)
	}
	return &domain.SpikeParticipationResponse{Success: true, Message: "success"}, nil
}

func (m *MockSpikeService) GetSpikeEventDetail(ctx context.Context, eventID int64) (*domain.SpikeEventWithProduct, error) {
	if m.getEventDetailFunc != nil {
		return m.getEventDetailFunc(ctx, eventID)
	}
	return &domain.SpikeEventWithProduct{
		SpikeEvent: &domain.SpikeEvent{ID: eventID, Name: "Test Event"},
		Product:    &domain.Product{ID: 1, Name: "Test Product"},
	}, nil
}

func (m *MockSpikeService) GetActiveEvents(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error) {
	if m.getActiveEventsFunc != nil {
		return m.getActiveEventsFunc(ctx, req)
	}
	return &domain.SpikeEventListResponse{
		Events: []*domain.SpikeEvent{
			{ID: 1, Name: "Test Event 1"},
			{ID: 2, Name: "Test Event 2"},
		},
		Total:    2,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (m *MockSpikeService) GetUserSpikeOrders(ctx context.Context, userID int64, req *domain.SpikeOrderListRequest) (*domain.SpikeOrderListResponse, error) {
	if m.getUserOrdersFunc != nil {
		return m.getUserOrdersFunc(ctx, userID, req)
	}
	return &domain.SpikeOrderListResponse{
		Orders: []*domain.SpikeOrder{
			{ID: 1, UserID: userID, Status: domain.SpikeOrderStatusPending},
		},
		Total:    1,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (m *MockSpikeService) GetSpikeOrderDetail(ctx context.Context, orderID, userID int64) (*domain.SpikeOrderWithDetails, error) {
	if m.getOrderDetailFunc != nil {
		return m.getOrderDetailFunc(ctx, orderID, userID)
	}
	return &domain.SpikeOrderWithDetails{
		SpikeOrder: &domain.SpikeOrder{ID: orderID, UserID: userID},
		SpikeEvent: &domain.SpikeEvent{ID: 1, Name: "Test Event"},
		User:       &domain.User{ID: userID, Username: "testuser"},
	}, nil
}

func (m *MockSpikeService) CancelSpikeOrder(ctx context.Context, orderID, userID int64, req *domain.CancelSpikeOrderRequest) error {
	if m.cancelOrderFunc != nil {
		return m.cancelOrderFunc(ctx, orderID, userID, req)
	}
	return nil
}

func (m *MockSpikeService) GetSpikeStats(ctx context.Context, eventID int64) (*service.SpikeStats, error) {
	if m.getSpikeStatsFunc != nil {
		return m.getSpikeStatsFunc(ctx, eventID)
	}
	return &service.SpikeStats{
		EventID:        eventID,
		TotalStock:     100,
		SoldCount:      20,
		RemainingStock: 80,
		SoldOut:        false,
		IsActive:       true,
		StartAt:        time.Now().Add(-time.Hour),
		EndAt:          time.Now().Add(time.Hour),
	}, nil
}

func (m *MockSpikeService) WarmupStock(ctx context.Context, eventID int64) error {
	if m.warmupStockFunc != nil {
		return m.warmupStockFunc(ctx, eventID)
	}
	return nil
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func TestSpikeHandler_HealthCheck(t *testing.T) {
	mockService := &MockSpikeService{}
	handler := NewSpikeHandler(mockService, zap.NewNop())

	router := setupTestRouter()
	router.GET("/health", handler.HealthCheck)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HealthCheck() status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("HealthCheck() failed to parse response: %v", err)
	}

	if data, ok := response["data"].(map[string]interface{}); ok {
		if service, ok := data["service"].(string); !ok || service != "spike-service" {
			t.Errorf("HealthCheck() service = %v, want spike-service", service)
		}
		if status, ok := data["status"].(string); !ok || status != "healthy" {
			t.Errorf("HealthCheck() status = %v, want healthy", status)
		}
	} else {
		t.Errorf("HealthCheck() invalid response data")
	}
}

func TestSpikeHandler_ParticipateSpike(t *testing.T) {
	tests := []struct {
		name        string
		userID      int64
		requestBody interface{}
		mockFunc    func(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64) (*domain.SpikeParticipationResponse, error)
		wantStatus  int
		wantSuccess bool
	}{
		{
			name:   "successful participation",
			userID: 123,
			requestBody: map[string]interface{}{
				"spike_event_id":  1,
				"quantity":        1,
				"idempotency_key": "test_key_1",
			},
			mockFunc: func(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64) (*domain.SpikeParticipationResponse, error) {
				return &domain.SpikeParticipationResponse{
					Success: true,
					Message: "秒杀成功",
				}, nil
			},
			wantStatus:  http.StatusOK,
			wantSuccess: true,
		},
		{
			name:   "sold out",
			userID: 123,
			requestBody: map[string]interface{}{
				"spike_event_id":  1,
				"quantity":        1,
				"idempotency_key": "test_key_2",
			},
			mockFunc: func(ctx context.Context, req *domain.SpikeParticipationRequest, userID int64) (*domain.SpikeParticipationResponse, error) {
				return &domain.SpikeParticipationResponse{
					Success: false,
					Message: "商品已售罄",
				}, nil
			},
			wantStatus:  http.StatusOK,
			wantSuccess: false,
		},
		{
			name:   "invalid request body",
			userID: 123,
			requestBody: map[string]interface{}{
				"invalid_field": "invalid_value",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "unauthorized user",
			userID: 0,
			requestBody: map[string]interface{}{
				"spike_event_id":  1,
				"quantity":        1,
				"idempotency_key": "test_key_3",
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockSpikeService{
				participateFunc: tt.mockFunc,
			}
			handler := NewSpikeHandler(mockService, zap.NewNop())

			router := setupTestRouter()
			router.POST("/participate", func(c *gin.Context) {
				// 模拟用户认证中间件
				if tt.userID > 0 {
					c.Set("user_id", tt.userID)
				}
				handler.ParticipateSpike(c)
			})

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/participate", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ParticipateSpike() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("ParticipateSpike() failed to parse response: %v", err)
				}

				if data, ok := response["data"].(map[string]interface{}); ok {
					if success, ok := data["success"].(bool); ok && success != tt.wantSuccess {
						t.Errorf("ParticipateSpike() success = %v, want %v", success, tt.wantSuccess)
					}
				}
			}
		})
	}
}

func TestSpikeHandler_GetSpikeEventDetail(t *testing.T) {
	tests := []struct {
		name       string
		eventID    string
		mockFunc   func(ctx context.Context, eventID int64) (*domain.SpikeEventWithProduct, error)
		wantStatus int
	}{
		{
			name:    "valid event ID",
			eventID: "1",
			mockFunc: func(ctx context.Context, eventID int64) (*domain.SpikeEventWithProduct, error) {
				return &domain.SpikeEventWithProduct{
					SpikeEvent: &domain.SpikeEvent{ID: eventID, Name: "Test Event"},
					Product:    &domain.Product{ID: 1, Name: "Test Product"},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid event ID",
			eventID:    "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:    "event not found",
			eventID: "999",
			mockFunc: func(ctx context.Context, eventID int64) (*domain.SpikeEventWithProduct, error) {
				return nil, domain.ErrSpikeEventNotFound
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockSpikeService{
				getEventDetailFunc: tt.mockFunc,
			}
			handler := NewSpikeHandler(mockService, zap.NewNop())

			router := setupTestRouter()
			router.GET("/events/:id", handler.GetSpikeEventDetail)

			req := httptest.NewRequest("GET", "/events/"+tt.eventID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GetSpikeEventDetail() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("GetSpikeEventDetail() failed to parse response: %v", err)
				}

				if data, ok := response["data"].(map[string]interface{}); !ok {
					t.Errorf("GetSpikeEventDetail() missing data field")
				} else {
					if _, ok := data["spike_event"]; !ok {
						t.Errorf("GetSpikeEventDetail() missing spike_event field")
					}
					if _, ok := data["product"]; !ok {
						t.Errorf("GetSpikeEventDetail() missing product field")
					}
				}
			}
		})
	}
}

func TestSpikeHandler_GetActiveEvents(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mockFunc   func(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error)
		wantStatus int
		wantCount  int
	}{
		{
			name:  "default parameters",
			query: "",
			mockFunc: func(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error) {
				return &domain.SpikeEventListResponse{
					Events: []*domain.SpikeEvent{
						{ID: 1, Name: "Event 1"},
						{ID: 2, Name: "Event 2"},
					},
					Total:    2,
					Page:     req.Page,
					PageSize: req.PageSize,
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:  "with pagination",
			query: "?page=2&page_size=5",
			mockFunc: func(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error) {
				if req.Page != 2 || req.PageSize != 5 {
					t.Errorf("GetActiveEvents() page=%d pageSize=%d, want page=2 pageSize=5", req.Page, req.PageSize)
				}
				return &domain.SpikeEventListResponse{
					Events:   []*domain.SpikeEvent{},
					Total:    0,
					Page:     req.Page,
					PageSize: req.PageSize,
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:  "with sorting",
			query: "?sort_by=start_at&sort_order=asc",
			mockFunc: func(ctx context.Context, req *domain.SpikeEventListRequest) (*domain.SpikeEventListResponse, error) {
				if req.SortBy == nil || *req.SortBy != "start_at" {
					t.Errorf("GetActiveEvents() sortBy want start_at")
				}
				if req.SortOrder == nil || *req.SortOrder != "asc" {
					t.Errorf("GetActiveEvents() sortOrder want asc")
				}
				return &domain.SpikeEventListResponse{
					Events:   []*domain.SpikeEvent{{ID: 1}},
					Total:    1,
					Page:     req.Page,
					PageSize: req.PageSize,
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockSpikeService{
				getActiveEventsFunc: tt.mockFunc,
			}
			handler := NewSpikeHandler(mockService, zap.NewNop())

			router := setupTestRouter()
			router.GET("/events", handler.GetActiveEvents)

			req := httptest.NewRequest("GET", "/events"+tt.query, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GetActiveEvents() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("GetActiveEvents() failed to parse response: %v", err)
				}

				if data, ok := response["data"].(map[string]interface{}); ok {
					if events, ok := data["events"].([]interface{}); ok {
						if len(events) != tt.wantCount {
							t.Errorf("GetActiveEvents() event count = %d, want %d", len(events), tt.wantCount)
						}
					}
				}
			}
		})
	}
}

func TestSpikeHandler_GetUserSpikeOrders(t *testing.T) {
	tests := []struct {
		name       string
		userID     int64
		query      string
		mockFunc   func(ctx context.Context, userID int64, req *domain.SpikeOrderListRequest) (*domain.SpikeOrderListResponse, error)
		wantStatus int
	}{
		{
			name:   "authenticated user",
			userID: 123,
			query:  "",
			mockFunc: func(ctx context.Context, userID int64, req *domain.SpikeOrderListRequest) (*domain.SpikeOrderListResponse, error) {
				return &domain.SpikeOrderListResponse{
					Orders: []*domain.SpikeOrder{
						{ID: 1, UserID: userID},
					},
					Total:    1,
					Page:     req.Page,
					PageSize: req.PageSize,
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "unauthenticated user",
			userID:     0,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:   "with status filter",
			userID: 123,
			query:  "?status=pending",
			mockFunc: func(ctx context.Context, userID int64, req *domain.SpikeOrderListRequest) (*domain.SpikeOrderListResponse, error) {
				if req.Status == nil || *req.Status != domain.SpikeOrderStatusPending {
					t.Errorf("GetUserSpikeOrders() status filter not applied correctly")
				}
				return &domain.SpikeOrderListResponse{
					Orders:   []*domain.SpikeOrder{},
					Total:    0,
					Page:     req.Page,
					PageSize: req.PageSize,
				}, nil
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockSpikeService{
				getUserOrdersFunc: tt.mockFunc,
			}
			handler := NewSpikeHandler(mockService, zap.NewNop())

			router := setupTestRouter()
			router.GET("/orders", func(c *gin.Context) {
				// 模拟用户认证中间件
				if tt.userID > 0 {
					c.Set("user_id", tt.userID)
				}
				handler.GetUserSpikeOrders(c)
			})

			req := httptest.NewRequest("GET", "/orders"+tt.query, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GetUserSpikeOrders() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSpikeHandler_CancelSpikeOrder(t *testing.T) {
	tests := []struct {
		name        string
		userID      int64
		orderID     string
		requestBody interface{}
		mockFunc    func(ctx context.Context, orderID, userID int64, req *domain.CancelSpikeOrderRequest) error
		wantStatus  int
	}{
		{
			name:    "successful cancellation",
			userID:  123,
			orderID: "1",
			requestBody: map[string]interface{}{
				"reason": "不想要了",
			},
			mockFunc: func(ctx context.Context, orderID, userID int64, req *domain.CancelSpikeOrderRequest) error {
				if req.Reason != "不想要了" {
					t.Errorf("CancelSpikeOrder() reason = %v, want 不想要了", req.Reason)
				}
				return nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "invalid order ID",
			userID:  123,
			orderID: "invalid",
			requestBody: map[string]interface{}{
				"reason": "test",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:    "order not found",
			userID:  123,
			orderID: "999",
			requestBody: map[string]interface{}{
				"reason": "test",
			},
			mockFunc: func(ctx context.Context, orderID, userID int64, req *domain.CancelSpikeOrderRequest) error {
				return domain.ErrSpikeOrderNotFound
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:    "unauthorized user",
			userID:  0,
			orderID: "1",
			requestBody: map[string]interface{}{
				"reason": "test",
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockSpikeService{
				cancelOrderFunc: tt.mockFunc,
			}
			handler := NewSpikeHandler(mockService, zap.NewNop())

			router := setupTestRouter()
			router.POST("/orders/:id/cancel", func(c *gin.Context) {
				// 模拟用户认证中间件
				if tt.userID > 0 {
					c.Set("user_id", tt.userID)
				}
				handler.CancelSpikeOrder(c)
			})

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/orders/"+tt.orderID+"/cancel", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("CancelSpikeOrder() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSpikeHandler_WarmupStock(t *testing.T) {
	tests := []struct {
		name       string
		userRole   string
		eventID    string
		mockFunc   func(ctx context.Context, eventID int64) error
		wantStatus int
	}{
		{
			name:     "admin user",
			userRole: "admin",
			eventID:  "1",
			mockFunc: func(ctx context.Context, eventID int64) error {
				return nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-admin user",
			userRole:   "customer",
			eventID:    "1",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "invalid event ID",
			userRole:   "admin",
			eventID:    "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:     "warmup failed",
			userRole: "admin",
			eventID:  "1",
			mockFunc: func(ctx context.Context, eventID int64) error {
				return domain.ErrSpikeEventNotFound
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockSpikeService{
				warmupStockFunc: tt.mockFunc,
			}
			handler := NewSpikeHandler(mockService, zap.NewNop())

			router := setupTestRouter()
			router.POST("/admin/events/:id/warmup", func(c *gin.Context) {
				// 模拟管理员权限中间件
				if tt.userRole == "admin" {
					c.Set("user_role", "admin")
				} else {
					c.Set("user_role", "customer")
				}
				handler.WarmupStock(c)
			})

			req := httptest.NewRequest("POST", "/admin/events/"+tt.eventID+"/warmup", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("WarmupStock() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// 测试辅助方法
func TestSpikeHandler_HelperMethods(t *testing.T) {
	handler := NewSpikeHandler(nil, zap.NewNop())

	// 测试 getCurrentUserID
	router := setupTestRouter()
	router.GET("/test-user-id", func(c *gin.Context) {
		c.Set("user_id", int64(123))
		userID := handler.getCurrentUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	req := httptest.NewRequest("GET", "/test-user-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if userID, ok := response["user_id"].(float64); !ok || int64(userID) != 123 {
		t.Errorf("getCurrentUserID() = %v, want 123", userID)
	}

	// 测试 isAdmin
	router = setupTestRouter()
	router.GET("/test-admin", func(c *gin.Context) {
		c.Set("user_role", "admin")
		isAdmin := handler.isAdmin(c)
		c.JSON(http.StatusOK, gin.H{"is_admin": isAdmin})
	})

	req = httptest.NewRequest("GET", "/test-admin", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &response)
	if isAdmin, ok := response["is_admin"].(bool); !ok || !isAdmin {
		t.Errorf("isAdmin() = %v, want true", isAdmin)
	}
}
