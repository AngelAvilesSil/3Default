package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/AngelAvilesSil/3Default/internal/api"
)

func TestGetHealth(t *testing.T) {
	handler := NewHandler(NewServer())

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body api.HealthResponse

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != api.Ok {
		t.Fatalf("expected status %q, got %q", api.Ok, body.Status)
	}
}
