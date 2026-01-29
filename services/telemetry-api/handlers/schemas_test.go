package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadSchema_Success(t *testing.T) {
	mockStore := NewMockSchemaStore()
	h := NewWithStores(nil, nil, nil, mockStore, nil, nil)

	body := `{
		"measurements": {
			"temperature": {"id": 1, "name": "temperature", "type": "float", "unit": "celsius"},
			"humidity": {"id": 2, "name": "humidity", "type": "float", "unit": "percent"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/schemas/test-app/1.0.0", bytes.NewBufferString(body))
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "1.0.0")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UploadSchema(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["app"] != "test-app" {
		t.Errorf("expected app 'test-app', got %q", response["app"])
	}
	if response["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", response["version"])
	}
}

func TestUploadSchema_MissingApp(t *testing.T) {
	h := NewWithStores(nil, nil, nil, NewMockSchemaStore(), nil, nil)

	body := `{"measurements": {"temp": {"id": 1}}}`
	req := httptest.NewRequest(http.MethodPost, "/schemas//1.0.0", bytes.NewBufferString(body))
	req.SetPathValue("app", "")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.UploadSchema(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUploadSchema_MissingVersion(t *testing.T) {
	h := NewWithStores(nil, nil, nil, NewMockSchemaStore(), nil, nil)

	body := `{"measurements": {"temp": {"id": 1}}}`
	req := httptest.NewRequest(http.MethodPost, "/schemas/test-app/", bytes.NewBufferString(body))
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "")
	w := httptest.NewRecorder()

	h.UploadSchema(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUploadSchema_InvalidJSON(t *testing.T) {
	h := NewWithStores(nil, nil, nil, NewMockSchemaStore(), nil, nil)

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/schemas/test-app/1.0.0", bytes.NewBufferString(body))
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.UploadSchema(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUploadSchema_EmptyMeasurements(t *testing.T) {
	h := NewWithStores(nil, nil, nil, NewMockSchemaStore(), nil, nil)

	body := `{"measurements": {}}`
	req := httptest.NewRequest(http.MethodPost, "/schemas/test-app/1.0.0", bytes.NewBufferString(body))
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.UploadSchema(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUploadSchema_StoreError(t *testing.T) {
	mockStore := NewMockSchemaStore()
	mockStore.SaveErr = errors.New("database error")
	h := NewWithStores(nil, nil, nil, mockStore, nil, nil)

	body := `{"measurements": {"temp": {"id": 1, "name": "temp", "type": "float", "unit": "c"}}}`
	req := httptest.NewRequest(http.MethodPost, "/schemas/test-app/1.0.0", bytes.NewBufferString(body))
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.UploadSchema(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetSchema_Success(t *testing.T) {
	mockStore := NewMockSchemaStore()
	mockStore.data["test-app:1.0.0"] = MeasurementSchema{
		AppName: "test-app",
		Version: "1.0.0",
		Measurements: map[string]MeasurementMeta{
			"temperature": {ID: 1, Name: "temperature", Type: "float", Unit: "celsius"},
		},
	}
	h := NewWithStores(nil, nil, nil, mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/schemas/test-app/1.0.0", nil)
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.GetSchema(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var schema MeasurementSchema
	if err := json.NewDecoder(w.Body).Decode(&schema); err != nil {
		t.Fatal(err)
	}
	if schema.AppName != "test-app" {
		t.Errorf("expected app_name 'test-app', got %q", schema.AppName)
	}
	if len(schema.Measurements) != 1 {
		t.Errorf("expected 1 measurement, got %d", len(schema.Measurements))
	}
}

func TestGetSchema_MissingApp(t *testing.T) {
	h := NewWithStores(nil, nil, nil, NewMockSchemaStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/schemas//1.0.0", nil)
	req.SetPathValue("app", "")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.GetSchema(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetSchema_MissingVersion(t *testing.T) {
	h := NewWithStores(nil, nil, nil, NewMockSchemaStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/schemas/test-app/", nil)
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "")
	w := httptest.NewRecorder()

	h.GetSchema(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetSchema_NotFound(t *testing.T) {
	mockStore := NewMockSchemaStore()
	// Don't add any schema
	h := NewWithStores(nil, nil, nil, mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/schemas/nonexistent/1.0.0", nil)
	req.SetPathValue("app", "nonexistent")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.GetSchema(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetSchema_StoreError(t *testing.T) {
	mockStore := NewMockSchemaStore()
	mockStore.GetErr = errors.New("database error")
	h := NewWithStores(nil, nil, nil, mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/schemas/test-app/1.0.0", nil)
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.GetSchema(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUploadSchema_FieldIDZero(t *testing.T) {
	h := NewWithStores(nil, nil, nil, NewMockSchemaStore(), nil, nil)

	body := `{"measurements": {"temp": {"id": 0, "name": "temp", "type": "float", "unit": "c"}}}`
	req := httptest.NewRequest(http.MethodPost, "/schemas/test-app/1.0.0", bytes.NewBufferString(body))
	req.SetPathValue("app", "test-app")
	req.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.UploadSchema(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if errorMsg, ok := response["error"].(string); !ok || !contains(errorMsg, "field number 0 is reserved") {
		t.Errorf("expected error about field 0 being reserved, got %v", response["error"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
