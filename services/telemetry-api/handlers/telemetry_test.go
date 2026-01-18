package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test UUIDs for telemetry tests
const (
	telTestDeviceUUID = "550e8400-e29b-41d4-a716-446655440030"
)

// withDeviceContext adds a device ID to the request context for testing
func withDeviceContext(r *http.Request, deviceID string) *http.Request {
	ctx := context.WithValue(r.Context(), deviceIDKey, deviceID)
	return r.WithContext(ctx)
}

func TestHandleTelemetry_Success(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockPublisher := NewMockEventPublisher()
	h := NewWithStores(mockStore, nil, nil, nil, nil, mockPublisher)

	body := `{"type": "temperature", "value": 23.5}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID) // Auth context
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response TelemetryData
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.ID == "" {
		t.Error("expected ID to be set")
	}
	if response.DeviceID != telTestDeviceUUID {
		t.Errorf("expected device_id %q, got %q", telTestDeviceUUID, response.DeviceID)
	}

	// Verify event was published
	if len(mockPublisher.Published) != 1 {
		t.Errorf("expected 1 published event, got %d", len(mockPublisher.Published))
	}
}

func TestHandleTelemetry_Unauthorized(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	body := `{"type": "temperature", "value": 23.5}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No device context = unauthorized
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleTelemetry_MissingType(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	body := `{"value": 23.5}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTelemetry_InvalidType(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	// Type starting with number is invalid
	body := `{"type": "123invalid", "value": 23.5}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTelemetry_InvalidJSON(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTelemetry_StoreError(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockStore.SaveErr = errors.New("database error")
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	body := `{"type": "temperature", "value": 23.5}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetTelemetry_Success(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	// Add some test data
	mockStore.Save(context.Background(), &TelemetryData{DeviceID: telTestDeviceUUID, Type: "temperature", Value: 23.5})
	mockStore.Save(context.Background(), &TelemetryData{DeviceID: telTestDeviceUUID, Type: "temperature", Value: 24.0})

	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/telemetry?limit=10", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response struct {
		Data  []TelemetryData `json:"data"`
		Count int             `json:"count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.Count != 2 {
		t.Errorf("expected count 2, got %d", response.Count)
	}
}

func TestGetTelemetry_Unauthorized(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/telemetry", nil)
	// No device context = unauthorized
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestGetTelemetry_StoreError(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockStore.GetErr = errors.New("database error")
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/telemetry", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetTelemetry_WithLimit(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	for i := 0; i < 5; i++ {
		mockStore.Save(context.Background(), &TelemetryData{DeviceID: telTestDeviceUUID, Type: "temp", Value: float64(i)})
	}
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/telemetry?limit=3", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetTelemetry_InvalidLimit(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/telemetry?limit=invalid", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	// Should return bad request for invalid limit
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTelemetry_PublishError(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockPublisher := NewMockEventPublisher()
	mockPublisher.PublishErr = errors.New("publish error")
	h := NewWithStores(mockStore, nil, nil, nil, nil, mockPublisher)

	body := `{"type": "temperature", "value": 23.5}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	// Should still succeed - publish errors are logged but don't fail request
	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestHandleTelemetry_WithTimestamp(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	body := `{"type": "temperature", "value": 23.5, "timestamp": "2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
}

func TestHandleTelemetry_WithMetadata(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	body := `{"type": "temperature", "value": 23.5, "metadata": {"location": "room1"}}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestHandleTelemetry_NilPublisher(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	// Explicitly set publisher to nil to test the nil check
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	body := `{"type": "temperature", "value": 23.5}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	// Should succeed even without publisher
	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestGetTelemetry_LimitAboveMax(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	// Request with limit > 1000 should return error
	req := httptest.NewRequest(http.MethodGet, "/telemetry?limit=5000", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetTelemetry_NegativeLimit(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	// Request with negative limit should return error
	req := httptest.NewRequest(http.MethodGet, "/telemetry?limit=-5", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetTelemetry_ZeroLimit(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	// Request with limit=0 should return error
	req := httptest.NewRequest(http.MethodGet, "/telemetry?limit=0", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetTelemetry_TypeFilter(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockStore.Save(context.Background(), &TelemetryData{DeviceID: telTestDeviceUUID, Type: "temperature", Value: 23.5})
	mockStore.Save(context.Background(), &TelemetryData{DeviceID: telTestDeviceUUID, Type: "humidity", Value: 65.0})
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/telemetry?type=temperature", nil)
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetTelemetry(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleTelemetry_EmptyBody(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTelemetry_AllFieldsPresent(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockPublisher := NewMockEventPublisher()
	h := NewWithStores(mockStore, nil, nil, nil, nil, mockPublisher)

	body := `{
		"type": "temperature",
		"value": 23.5,
		"unit": "celsius",
		"timestamp": "2026-01-01T12:00:00Z",
		"metadata": {"location": "room1", "floor": 1}
	}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetry(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response TelemetryData
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.Unit != "celsius" {
		t.Errorf("expected unit 'celsius', got %q", response.Unit)
	}
}

// ============ Batch Telemetry Tests ============

func TestHandleTelemetryBatch_Success(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockPublisher := NewMockEventPublisher()
	h := NewWithStores(mockStore, nil, nil, nil, nil, mockPublisher)

	body := `[
		{"type": "temperature", "value": 23.5},
		{"type": "humidity", "value": 65.0},
		{"type": "pressure", "value": 1013.25}
	]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response["saved"].(float64) != 3 {
		t.Errorf("expected 3 saved, got %v", response["saved"])
	}
	if response["received"].(float64) != 3 {
		t.Errorf("expected 3 received, got %v", response["received"])
	}
}

func TestHandleTelemetryBatch_Unauthorized(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	body := `[{"type": "temperature", "value": 23.5}]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No device context = unauthorized
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleTelemetryBatch_InvalidJSON(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTelemetryBatch_EmptyBatch(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	body := `[]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandleTelemetryBatch_ExceedsMaxSize(t *testing.T) {
	h := NewWithStores(NewMockTelemetryStore(), nil, nil, nil, nil, nil)

	// Create batch with 101 items (exceeds max of 100)
	var items []string
	for i := 0; i < 101; i++ {
		items = append(items, `{"type": "temperature", "value": 23.5}`)
	}
	body := "[" + items[0]
	for _, item := range items[1:] {
		body += "," + item
	}
	body += "]"

	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTelemetryBatch_SkipsMissingType(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	// Second item missing type - should be skipped
	body := `[
		{"type": "temperature", "value": 23.5},
		{"value": 65.0},
		{"type": "pressure", "value": 1013.25}
	]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	// Should save 2 out of 3 (one skipped due to missing type)
	if response["saved"].(float64) != 2 {
		t.Errorf("expected 2 saved, got %v", response["saved"])
	}
	if response["received"].(float64) != 3 {
		t.Errorf("expected 3 received, got %v", response["received"])
	}
}

func TestHandleTelemetryBatch_SkipsInvalidType(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	// Second item has invalid type (starts with number)
	body := `[
		{"type": "temperature", "value": 23.5},
		{"type": "123invalid", "value": 65.0},
		{"type": "pressure", "value": 1013.25}
	]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	// Should save 2 out of 3 (one skipped due to invalid type)
	if response["saved"].(float64) != 2 {
		t.Errorf("expected 2 saved, got %v", response["saved"])
	}
}

func TestHandleTelemetryBatch_WithTimestamps(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	body := `[
		{"type": "temperature", "value": 23.5, "timestamp": "2026-01-15T10:00:00Z"},
		{"type": "temperature", "value": 23.7, "timestamp": "2026-01-15T10:05:00Z"},
		{"type": "temperature", "value": 23.9, "timestamp": "2026-01-15T10:10:00Z"}
	]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["saved"].(float64) != 3 {
		t.Errorf("expected 3 saved, got %v", response["saved"])
	}
}

func TestHandleTelemetryBatch_StoreError(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockStore.SaveErr = errors.New("database error")
	h := NewWithStores(mockStore, nil, nil, nil, nil, nil)

	body := `[{"type": "temperature", "value": 23.5}]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	// Should still return 201 but with 0 saved (errors are logged, not fatal)
	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["saved"].(float64) != 0 {
		t.Errorf("expected 0 saved (due to errors), got %v", response["saved"])
	}
}

func TestHandleTelemetryBatch_WithPublisher(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockPublisher := NewMockEventPublisher()
	h := NewWithStores(mockStore, nil, nil, nil, nil, mockPublisher)

	body := `[
		{"type": "temperature", "value": 23.5},
		{"type": "humidity", "value": 65.0}
	]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	// Should publish an event for each telemetry item
	if len(mockPublisher.Published) != 2 {
		t.Errorf("expected 2 published events, got %d", len(mockPublisher.Published))
	}
}

func TestHandleTelemetryBatch_PublishError(t *testing.T) {
	mockStore := NewMockTelemetryStore()
	mockPublisher := NewMockEventPublisher()
	mockPublisher.PublishErr = errors.New("publish error")
	h := NewWithStores(mockStore, nil, nil, nil, nil, mockPublisher)

	body := `[{"type": "temperature", "value": 23.5}]`
	req := httptest.NewRequest(http.MethodPost, "/telemetry/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, telTestDeviceUUID)
	w := httptest.NewRecorder()

	h.HandleTelemetryBatch(w, req)

	// Should still succeed - publish errors are logged but don't fail request
	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}
