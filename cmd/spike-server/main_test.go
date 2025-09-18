package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorseWayne/spike_shop/internal/resp"
)

func TestHealthz_OK(t *testing.T) {
	// Build a minimal mux identical to main's handler for /healthz
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		data := map[string]any{
			"status":  "ok",
			"version": "test",
		}
		resp.OK(w, &data, "test-req", "")
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rw := httptest.NewRecorder()
	mux.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rw.Code)
	}
	var body struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Code != 0 || body.Data["status"] != "ok" {
		t.Fatalf("unexpected body: %+v", body)
	}
}
