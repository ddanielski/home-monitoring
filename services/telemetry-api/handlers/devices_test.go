package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// containsSubstring is a helper for checking error messages
func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

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

// =============================================================================
// UpdateDeviceInfo Tests
// =============================================================================

func TestUpdateDeviceInfo_Success(t *testing.T) {
	mockStore := NewMockDeviceStore()
	mockStore.data[devTestDeviceUUID] = Device{
		DeviceID:   devTestDeviceUUID,
		MACAddress: "aabbccddeeff",
		AppName:    "oldapp",
		AppVersion: "0.9.0",
	}

	h := NewWithStores(nil, nil, mockStore, nil, nil, nil)

	body := `{"app_name": "testapp", "app_version": "1.0.0"}`
	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp UpdateDeviceInfoResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.DeviceID != devTestDeviceUUID {
		t.Errorf("expected device_id %q, got %q", devTestDeviceUUID, resp.DeviceID)
	}
	if resp.AppName != "testapp" {
		t.Errorf("expected app_name 'testapp', got %q", resp.AppName)
	}
	if resp.AppVersion != "1.0.0" {
		t.Errorf("expected app_version '1.0.0', got %q", resp.AppVersion)
	}

	// Verify stored
	device := mockStore.data[devTestDeviceUUID]
	if device.AppName != "testapp" {
		t.Errorf("expected stored app_name 'testapp', got %q", device.AppName)
	}
	if device.AppVersion != "1.0.0" {
		t.Errorf("expected stored app_version '1.0.0', got %q", device.AppVersion)
	}
}

func TestUpdateDeviceInfo_Unauthorized(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	body := `{"app_name": "testapp", "app_version": "1.0.0"}`
	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No device context
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUpdateDeviceInfo_InvalidJSON(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUpdateDeviceInfo_MissingAppName(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	body := `{"app_version": "1.0.0"}`
	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if !containsSubstring(w.Body.String(), "app_name") {
		t.Error("expected error to mention app_name")
	}
}

func TestUpdateDeviceInfo_InvalidAppName(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	body := `{"app_name": "invalid app!", "app_version": "1.0.0"}`
	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if !containsSubstring(w.Body.String(), "invalid app_name") {
		t.Error("expected error to mention invalid app_name")
	}
}

func TestUpdateDeviceInfo_MissingAppVersion(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	body := `{"app_name": "testapp"}`
	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if !containsSubstring(w.Body.String(), "app_version") {
		t.Error("expected error to mention app_version")
	}
}

func TestUpdateDeviceInfo_InvalidAppVersion(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	// Version with special characters not allowed by validator
	body := `{"app_name": "testapp", "app_version": "1.0.0+invalid!"}`
	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if !containsSubstring(w.Body.String(), "invalid app_version") {
		t.Error("expected error to mention invalid app_version")
	}
}

func TestUpdateDeviceInfo_StoreError(t *testing.T) {
	mockStore := NewMockDeviceStore()
	mockStore.data[devTestDeviceUUID] = Device{DeviceID: devTestDeviceUUID}
	mockStore.UpdateErr = ErrNotFound // Simulate store error

	h := NewWithStores(nil, nil, mockStore, nil, nil, nil)

	body := `{"app_name": "testapp", "app_version": "1.0.0"}`
	req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDeviceContext(req, devTestDeviceUUID)
	w := httptest.NewRecorder()

	h.UpdateDeviceInfo(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestUpdateDeviceInfo_ValidVersionFormats(t *testing.T) {
	// Version regex: ^[a-zA-Z0-9][a-zA-Z0-9._-]{0,31}$
	testCases := []struct {
		version string
		wantOK  bool
	}{
		{"1.0.0", true},
		{"0.1.0", true},
		{"10.20.30", true},
		{"1.0.0-alpha", true},
		{"1.0.0-beta.1", true},
		{"v1.0.0", true},              // Valid: starts with letter
		{"1.0", true},                 // Valid: simplified version
		{"abc", true},                 // Valid: letters only
		{"1.0.0+build", false},        // Invalid: + not allowed
		{"", false},                   // Invalid: empty
		{"1.0.0 space", false},        // Invalid: spaces not allowed
		{"-1.0.0", false},             // Invalid: starts with -
		{"super-long-version-that-exceeds-the-32-char-limit", false}, // Too long
	}

	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			mockStore := NewMockDeviceStore()
			mockStore.data[devTestDeviceUUID] = Device{DeviceID: devTestDeviceUUID}
			h := NewWithStores(nil, nil, mockStore, nil, nil, nil)

			body := `{"app_name": "testapp", "app_version": "` + tc.version + `"}`
			req := httptest.NewRequest(http.MethodPut, "/devices/info", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req = withDeviceContext(req, devTestDeviceUUID)
			w := httptest.NewRecorder()

			h.UpdateDeviceInfo(w, req)

			if tc.wantOK {
				if w.Code != http.StatusOK {
					t.Errorf("expected OK for %q, got %d: %s", tc.version, w.Code, w.Body.String())
				}
			} else {
				if w.Code != http.StatusBadRequest {
					t.Errorf("expected BadRequest for %q, got %d", tc.version, w.Code)
				}
			}
		})
	}
}

// Note: withDeviceContext is defined in telemetry_test.go
