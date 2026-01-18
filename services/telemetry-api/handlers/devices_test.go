package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test UUIDs for device tests
const (
	devTestDeviceUUID  = "550e8400-e29b-41d4-a716-446655440040"
	devTestDeviceUUID2 = "550e8400-e29b-41d4-a716-446655440041"
)

func TestGetDevice_Success(t *testing.T) {
	mockStore := NewMockDeviceStore()
	mockStore.data[devTestDeviceUUID] = Device{
		DeviceID:     devTestDeviceUUID,
		MACAddress:   "aabbccddeeff",
		AppName:      "testapp",
		AppVersion:   "1.0.0",
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}

	h := NewWithStores(nil, nil, mockStore, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/devices/"+devTestDeviceUUID, nil)
	req.SetPathValue("id", devTestDeviceUUID)
	req = withDeviceContext(req, devTestDeviceUUID) // Same device requesting own info
	w := httptest.NewRecorder()

	h.GetDevice(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var device Device
	if err := json.NewDecoder(w.Body).Decode(&device); err != nil {
		t.Fatal(err)
	}
	if device.DeviceID != devTestDeviceUUID {
		t.Errorf("expected device_id %q, got %q", devTestDeviceUUID, device.DeviceID)
	}
	if device.AppName != "testapp" {
		t.Errorf("expected app_name 'testapp', got %q", device.AppName)
	}
}

func TestGetDevice_Unauthorized(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/devices/"+devTestDeviceUUID, nil)
	req.SetPathValue("id", devTestDeviceUUID)
	// No device context
	w := httptest.NewRecorder()

	h.GetDevice(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestGetDevice_WrongDevice(t *testing.T) {
	mockStore := NewMockDeviceStore()
	mockStore.data[devTestDeviceUUID] = Device{DeviceID: devTestDeviceUUID}

	h := NewWithStores(nil, nil, mockStore, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/devices/"+devTestDeviceUUID, nil)
	req.SetPathValue("id", devTestDeviceUUID)
	req = withDeviceContext(req, devTestDeviceUUID2) // Different device
	w := httptest.NewRecorder()

	h.GetDevice(w, req)

	// Should return 404 (not reveal that device exists)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetDevice_MissingID(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/devices/", nil)
	req.SetPathValue("id", "")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetDevice_InvalidIDFormat(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/devices/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetDevice_NotFound(t *testing.T) {
	mockStore := NewMockDeviceStore()
	// Don't add any device
	h := NewWithStores(nil, nil, mockStore, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/devices/"+devTestDeviceUUID, nil)
	req.SetPathValue("id", devTestDeviceUUID)
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetDevice_StoreError(t *testing.T) {
	mockStore := NewMockDeviceStore()
	mockStore.GetErr = ErrNotFound
	h := NewWithStores(nil, nil, mockStore, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/devices/"+devTestDeviceUUID, nil)
	req.SetPathValue("id", devTestDeviceUUID)
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Note: withDeviceContext is defined in telemetry_test.go
