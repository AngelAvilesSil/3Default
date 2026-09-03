package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/AngelAvilesSil/3Default/internal/api"
)

type fakeDatabase struct {
	err error
}

func (f fakeDatabase) Ping(context.Context) error {
	return f.err
}

func TestGetHealth(t *testing.T) {
	handler := NewHandler(NewServer(fakeDatabase{}))

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

func TestGetReadyWhenDatabaseIsAvailable(t *testing.T) {
	handler := NewHandler(NewServer(fakeDatabase{}))

	request := httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body api.ReadyResponse

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != api.Ready {
		t.Fatalf("expected status %q, got %q", api.Ready, body.Status)
	}
}

func TestGetReadyWhenDatabaseIsUnavailable(t *testing.T) {
	handler := NewHandler(NewServer(fakeDatabase{
		err: errors.New("database unavailable"),
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			response.Code,
		)
	}

	var body api.ReadyResponse

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != api.Unavailable {
		t.Fatalf(
			"expected status %q, got %q",
			api.Unavailable,
			body.Status,
		)
	}
}
