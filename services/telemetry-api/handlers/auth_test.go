package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// Test UUIDs for consistent testing
const (
	testDeviceUUID    = "550e8400-e29b-41d4-a716-446655440000"
	testDeviceUUID2   = "550e8400-e29b-41d4-a716-446655440001"
	nonExistentUUID   = "550e8400-e29b-41d4-a716-446655440099"
	testValidSecret   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 64 hex chars
	testInvalidSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdee" // Different
)

// --- AuthDevice Tests ---

func TestAuthDevice_Success(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockAuthService := NewMockAuthService()

	// Create a test device with hashed secret
	hash, _ := HashDeviceSecret(testValidSecret)
	mockDeviceStore.data[testDeviceUUID] = Device{
		DeviceID:   testDeviceUUID,
		MACAddress: "aabbccddeeff",
		AppName:    "testapp",
		AppVersion: "1.0.0",
		SecretHash: hash,
		Revoked:    false,
	}

	h := NewWithStores(nil, nil, mockDeviceStore, nil, mockAuthService, nil)

	body := `{"device_id": "` + testDeviceUUID + `", "secret": "` + testValidSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response DeviceAuthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Token == "" {
		t.Error("expected non-empty token")
	}
	if response.ExpiresIn != 3600 {
		t.Errorf("expected expires_in 3600, got %d", response.ExpiresIn)
	}
}

func TestAuthDevice_InvalidJSON(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, NewMockAuthService(), nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(`{invalid}`))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAuthDevice_MissingDeviceID(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, NewMockAuthService(), nil)

	body := `{"secret": "` + testValidSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAuthDevice_InvalidDeviceIDFormat(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, NewMockAuthService(), nil)

	body := `{"device_id": "not-a-uuid", "secret": "` + testValidSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAuthDevice_MissingSecret(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, NewMockAuthService(), nil)

	body := `{"device_id": "` + testDeviceUUID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAuthDevice_InvalidSecretLength(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, NewMockAuthService(), nil)

	body := `{"device_id": "` + testDeviceUUID + `", "secret": "too-short"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthDevice_DeviceNotFound(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	h := NewWithStores(nil, nil, mockDeviceStore, nil, NewMockAuthService(), nil)

	body := `{"device_id": "` + nonExistentUUID + `", "secret": "` + testValidSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthDevice_DeviceRevoked(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	hash, _ := HashDeviceSecret(testValidSecret)
	mockDeviceStore.data[testDeviceUUID] = Device{
		DeviceID:   testDeviceUUID,
		SecretHash: hash,
		Revoked:    true,
	}
	h := NewWithStores(nil, nil, mockDeviceStore, nil, NewMockAuthService(), nil)

	body := `{"device_id": "` + testDeviceUUID + `", "secret": "` + testValidSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestAuthDevice_WrongSecret(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	hash, _ := HashDeviceSecret(testValidSecret)
	mockDeviceStore.data[testDeviceUUID] = Device{
		DeviceID:   testDeviceUUID,
		SecretHash: hash,
		Revoked:    false,
	}
	h := NewWithStores(nil, nil, mockDeviceStore, nil, NewMockAuthService(), nil)

	body := `{"device_id": "` + testDeviceUUID + `", "secret": "` + testInvalidSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthDevice_TokenCreationFailure(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockAuthService := NewMockAuthService()

	hash, _ := HashDeviceSecret(testValidSecret)
	mockDeviceStore.data[testDeviceUUID] = Device{
		DeviceID:   testDeviceUUID,
		SecretHash: hash,
		Revoked:    false,
	}
	mockAuthService.CreateTokenErr = errors.New("firebase error")

	h := NewWithStores(nil, nil, mockDeviceStore, nil, mockAuthService, nil)

	body := `{"device_id": "` + testDeviceUUID + `", "secret": "` + testValidSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.AuthDevice(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- RefreshToken Tests ---

func TestRefreshToken_Success(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockAuthService := NewMockAuthService()
	mockDeviceStore.data[testDeviceUUID] = Device{
		DeviceID:   testDeviceUUID,
		AppName:    "testapp",
		AppVersion: "1.0.0",
		Revoked:    false,
	}

	h := NewWithStores(nil, nil, mockDeviceStore, nil, mockAuthService, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req = withDeviceContext(req, testDeviceUUID)
	w := httptest.NewRecorder()

	h.RefreshToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response DeviceAuthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestRefreshToken_Unauthorized(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, NewMockAuthService(), nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	// No device context
	w := httptest.NewRecorder()

	h.RefreshToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRefreshToken_DeviceNotFound(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	h := NewWithStores(nil, nil, mockDeviceStore, nil, NewMockAuthService(), nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req = withDeviceContext(req, nonExistentUUID)
	w := httptest.NewRecorder()

	h.RefreshToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRefreshToken_DeviceRevoked(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockDeviceStore.data[testDeviceUUID] = Device{
		DeviceID: testDeviceUUID,
		Revoked:  true,
	}
	h := NewWithStores(nil, nil, mockDeviceStore, nil, NewMockAuthService(), nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req = withDeviceContext(req, testDeviceUUID)
	w := httptest.NewRecorder()

	h.RefreshToken(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestRefreshToken_TokenCreationFailure(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockAuthService := NewMockAuthService()
	mockDeviceStore.data[testDeviceUUID] = Device{
		DeviceID: testDeviceUUID,
		Revoked:  false,
	}
	mockAuthService.CreateTokenErr = errors.New("firebase error")

	h := NewWithStores(nil, nil, mockDeviceStore, nil, mockAuthService, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req = withDeviceContext(req, testDeviceUUID)
	w := httptest.NewRecorder()

	h.RefreshToken(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- RevokeDevice Tests ---

func TestRevokeDevice_Success(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockDeviceStore.data[testDeviceUUID] = Device{DeviceID: testDeviceUUID}
	h := NewWithStores(nil, nil, mockDeviceStore, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/devices/"+testDeviceUUID+"/revoke", nil)
	req.SetPathValue("id", testDeviceUUID)
	w := httptest.NewRecorder()

	h.RevokeDevice(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify device was revoked
	if !mockDeviceStore.data[testDeviceUUID].Revoked {
		t.Error("expected device to be revoked")
	}
}

func TestRevokeDevice_MissingID(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/devices//revoke", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	h.RevokeDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRevokeDevice_InvalidIDFormat(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/devices/not-a-uuid/revoke", nil)
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.RevokeDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRevokeDevice_StoreError(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockDeviceStore.RevokeErr = errors.New("database error")
	h := NewWithStores(nil, nil, mockDeviceStore, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/devices/"+testDeviceUUID+"/revoke", nil)
	req.SetPathValue("id", testDeviceUUID)
	w := httptest.NewRecorder()

	h.RevokeDevice(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- ProvisionDevice Tests ---

func TestProvisionDevice_Success(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	h := NewWithStores(nil, nil, mockDeviceStore, nil, nil, nil)

	body := `{"mac_address": "AA:BB:CC:DD:EE:FF"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/devices/provision", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ProvisionDevice(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response ProvisionDeviceResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.DeviceID == "" {
		t.Error("expected non-empty device_id")
	}
	if response.Secret == "" {
		t.Error("expected non-empty secret")
	}
	if len(response.Secret) != 64 {
		t.Errorf("expected secret length 64, got %d", len(response.Secret))
	}
	if response.MACAddress != "aabbccddeeff" { // Should be normalized
		t.Errorf("expected normalized mac 'aabbccddeeff', got %q", response.MACAddress)
	}
}

func TestProvisionDevice_InvalidJSON(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/devices/provision", bytes.NewBufferString(`{invalid}`))
	w := httptest.NewRecorder()

	h.ProvisionDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestProvisionDevice_MissingMAC(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/admin/devices/provision", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ProvisionDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestProvisionDevice_InvalidMAC(t *testing.T) {
	h := NewWithStores(nil, nil, NewMockDeviceStore(), nil, nil, nil)

	body := `{"mac_address": "invalid-mac"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/devices/provision", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ProvisionDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestProvisionDevice_DuplicateMAC(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockDeviceStore.data["existing"] = Device{
		DeviceID:   "existing",
		MACAddress: "aabbccddeeff",
	}
	h := NewWithStores(nil, nil, mockDeviceStore, nil, nil, nil)

	body := `{"mac_address": "AA:BB:CC:DD:EE:FF"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/devices/provision", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ProvisionDevice(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestProvisionDevice_StoreError(t *testing.T) {
	mockDeviceStore := NewMockDeviceStore()
	mockDeviceStore.RegisterErr = errors.New("database error")
	h := NewWithStores(nil, nil, mockDeviceStore, nil, nil, nil)

	body := `{"mac_address": "AA:BB:CC:DD:EE:FF"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/devices/provision", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ProvisionDevice(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestProvisionDevice_MACFormats(t *testing.T) {
	testCases := []struct {
		name          string
		inputMAC      string
		normalizedMAC string
	}{
		{"colon format uppercase", "AA:BB:CC:DD:EE:FF", "aabbccddeeff"},
		{"colon format lowercase", "aa:bb:cc:dd:ee:ff", "aabbccddeeff"},
		{"hyphen format", "AA-BB-CC-DD-EE-FF", "aabbccddeeff"},
		{"dot format", "AABB.CCDD.EEFF", "aabbccddeeff"},
		{"no separator", "AABBCCDDEEFF", "aabbccddeeff"},
		{"mixed case", "aA:Bb:cC:Dd:Ee:fF", "aabbccddeeff"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDeviceStore := NewMockDeviceStore()
			h := NewWithStores(nil, nil, mockDeviceStore, nil, nil, nil)

			body := `{"mac_address": "` + tc.inputMAC + `"}`
			req := httptest.NewRequest(http.MethodPost, "/admin/devices/provision", bytes.NewBufferString(body))
			w := httptest.NewRecorder()

			h.ProvisionDevice(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
				return
			}

			var response ProvisionDeviceResponse
			json.NewDecoder(w.Body).Decode(&response)
			if response.MACAddress != tc.normalizedMAC {
				t.Errorf("expected normalized mac %q, got %q", tc.normalizedMAC, response.MACAddress)
			}
		})
	}
}

// --- AuthMiddleware Tests ---

func TestAuthMiddleware_Success(t *testing.T) {
	mockAuthService := NewMockAuthService()
	// Pre-add a valid token for testing (must be long enough to pass length check)
	longToken := strings.Repeat("a", 200)
	mockAuthService.AddToken(longToken, testDeviceUUID, map[string]interface{}{"app_name": "test"})
	h := NewWithStores(nil, nil, nil, nil, mockAuthService, nil)

	called := false
	handler := h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		deviceID := GetDeviceIDFromContext(r.Context())
		if deviceID != testDeviceUUID {
			t.Errorf("expected device_id %q, got %q", testDeviceUUID, deviceID)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+longToken)
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	h := NewWithStores(nil, nil, nil, nil, NewMockAuthService(), nil)

	handler := h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	h := NewWithStores(nil, nil, nil, nil, NewMockAuthService(), nil)

	handler := h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic invalid")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_TokenTooShort(t *testing.T) {
	h := NewWithStores(nil, nil, nil, nil, NewMockAuthService(), nil)

	handler := h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer short-token")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	mockAuthService := NewMockAuthService()
	mockAuthService.VerifyTokenErr = errors.New("invalid token")
	h := NewWithStores(nil, nil, nil, nil, mockAuthService, nil)

	handler := h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 200))
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// --- Helper function tests ---

func TestGenerateDeviceSecret(t *testing.T) {
	secret1, err := GenerateDeviceSecret()
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}

	if len(secret1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected secret length 64, got %d", len(secret1))
	}

	// Generate another and ensure they're different
	secret2, _ := GenerateDeviceSecret()
	if secret1 == secret2 {
		t.Error("secrets should be unique")
	}
}

func TestHashDeviceSecret(t *testing.T) {
	secret := testValidSecret
	hash, err := HashDeviceSecret(secret)
	if err != nil {
		t.Fatalf("failed to hash secret: %v", err)
	}

	if hash == secret {
		t.Error("hash should not equal original secret")
	}

	// Verify the hash works
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)); err != nil {
		t.Errorf("hash verification failed: %v", err)
	}
}

func TestGetDeviceIDFromContext(t *testing.T) {
	// Test with device ID
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = withDeviceContext(req, testDeviceUUID)
	if got := GetDeviceIDFromContext(req.Context()); got != testDeviceUUID {
		t.Errorf("expected %q, got %q", testDeviceUUID, got)
	}

	// Test without device ID
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := GetDeviceIDFromContext(req2.Context()); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestGetDeviceClaimsFromContext(t *testing.T) {
	// Test without claims
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	claims := GetDeviceClaimsFromContext(req.Context())
	if claims != nil {
		t.Errorf("expected nil claims, got %v", claims)
	}
}

// --- NormalizeMAC and ValidateMAC (deprecated wrappers) ---

func TestNormalizeMAC(t *testing.T) {
	if got := NormalizeMAC("AA:BB:CC:DD:EE:FF"); got != "aabbccddeeff" {
		t.Errorf("expected 'aabbccddeeff', got %q", got)
	}
}

func TestValidateMAC(t *testing.T) {
	if !ValidateMAC("AA:BB:CC:DD:EE:FF") {
		t.Error("expected valid MAC")
	}
	if ValidateMAC("invalid") {
		t.Error("expected invalid MAC")
	}
}
