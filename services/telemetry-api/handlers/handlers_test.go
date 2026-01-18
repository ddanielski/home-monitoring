package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth_Success(t *testing.T) {
	h := NewWithStores(nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %q", response["status"])
	}
}

func TestHealth_ContentType(t *testing.T) {
	h := NewWithStores(nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}
}

func TestNewWithStores(t *testing.T) {
	telemetry := NewMockTelemetryStore()
	commands := NewMockCommandStore()
	devices := NewMockDeviceStore()
	schemas := NewMockSchemaStore()
	auth := NewMockAuthService()
	publisher := NewMockEventPublisher()

	h := NewWithStores(telemetry, commands, devices, schemas, auth, publisher)

	if h == nil {
		t.Fatal("expected handlers to be created")
	}
	if h.telemetryStore != telemetry {
		t.Error("telemetry store not set correctly")
	}
	if h.commandStore != commands {
		t.Error("command store not set correctly")
	}
	if h.deviceStore != devices {
		t.Error("device store not set correctly")
	}
	if h.schemaStore != schemas {
		t.Error("schema store not set correctly")
	}
	if h.authService != auth {
		t.Error("auth service not set correctly")
	}
	if h.publisher != publisher {
		t.Error("publisher not set correctly")
	}
}

func TestHandlers_Close(t *testing.T) {
	// Test Close with nil clients (shouldn't panic)
	h := NewWithStores(nil, nil, nil, nil, nil, nil)
	err := h.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestHandlers_CloseWithNilClients(t *testing.T) {
	h := &Handlers{
		firestoreClient: nil,
		pubsubClient:    nil,
	}
	err := h.Close()
	if err != nil {
		t.Errorf("expected no error closing nil clients, got %v", err)
	}
}

func TestConstants(t *testing.T) {
	// Verify constants are defined correctly
	if TelemetryCollection == "" {
		t.Error("TelemetryCollection should not be empty")
	}
	if CommandsCollection == "" {
		t.Error("CommandsCollection should not be empty")
	}
	if TelemetryTopic == "" {
		t.Error("TelemetryTopic should not be empty")
	}
}

func TestConfig(t *testing.T) {
	cfg := Config{ProjectID: "test-project"}
	if cfg.ProjectID != "test-project" {
		t.Errorf("expected project ID 'test-project', got %q", cfg.ProjectID)
	}
}
