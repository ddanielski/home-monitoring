package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test UUIDs for commands tests
const (
	cmdTestDeviceUUID  = "550e8400-e29b-41d4-a716-446655440010"
	cmdTestDeviceUUID2 = "550e8400-e29b-41d4-a716-446655440011"
	cmdTestCommandUUID = "550e8400-e29b-41d4-a716-446655440020"
)

// withDeviceCtx adds a device ID to the request context for testing
func withDeviceCtx(r *http.Request, deviceID string) *http.Request {
	ctx := context.WithValue(r.Context(), deviceIDKey, deviceID)
	return r.WithContext(ctx)
}

func TestCreateCommand_Success(t *testing.T) {
	mockStore := NewMockCommandStore()
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	body := `{"device_id": "` + cmdTestDeviceUUID + `", "type": "reboot"}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response Command
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.ID == "" {
		t.Error("expected ID to be set")
	}
	if response.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", response.Status)
	}
	if response.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestCreateCommand_MissingDeviceID(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	body := `{"type": "reboot"}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateCommand_InvalidDeviceIDFormat(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	body := `{"device_id": "not-a-uuid", "type": "reboot"}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateCommand_MissingType(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	body := `{"device_id": "` + cmdTestDeviceUUID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateCommand_InvalidType(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	// Type starting with number is invalid
	body := `{"device_id": "` + cmdTestDeviceUUID + `", "type": "123invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateCommand_InvalidJSON(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	body := `{invalid}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateCommand_StoreError(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.SaveErr = errors.New("database error")
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	body := `{"device_id": "` + cmdTestDeviceUUID + `", "type": "reboot"}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetCommands_Success(t *testing.T) {
	mockStore := NewMockCommandStore()
	// Add test command
	mockStore.Save(context.Background(), &Command{DeviceID: cmdTestDeviceUUID, Type: "reboot", Status: "pending"})

	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req = withDeviceCtx(req, cmdTestDeviceUUID) // Device auth context
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response struct {
		Data  []Command `json:"data"`
		Count int       `json:"count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.Count != 1 {
		t.Errorf("expected count 1, got %d", response.Count)
	}
}

func TestGetCommands_Unauthorized(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	// No device context = unauthorized
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestGetCommands_InvalidStatus(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands?status=invalid", nil)
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetCommands_FilterExpired(t *testing.T) {
	mockStore := NewMockCommandStore()
	expired := time.Now().Add(-1 * time.Hour)
	valid := time.Now().Add(1 * time.Hour)

	// Add one expired and one valid command
	mockStore.data["cmd-expired"] = Command{
		DeviceID:  cmdTestDeviceUUID,
		Type:      "reboot",
		Status:    "pending",
		ExpiresAt: &expired,
	}
	mockStore.data["cmd-valid"] = Command{
		DeviceID:  cmdTestDeviceUUID,
		Type:      "config",
		Status:    "pending",
		ExpiresAt: &valid,
	}

	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Data  []Command `json:"data"`
		Count int       `json:"count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	// Only the non-expired command should be returned
	if response.Count != 1 {
		t.Errorf("expected count 1 (expired filtered), got %d", response.Count)
	}
}

func TestAckCommand_Success(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.data[cmdTestCommandUUID] = Command{
		DeviceID: cmdTestDeviceUUID,
		Type:     "reboot",
		Status:   "pending",
	}

	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/commands/"+cmdTestCommandUUID+"/ack", nil)
	req.SetPathValue("id", cmdTestCommandUUID)
	req = withDeviceCtx(req, cmdTestDeviceUUID) // Device auth context
	w := httptest.NewRecorder()

	h.AckCommand(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response Command
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.Status != "acknowledged" {
		t.Errorf("expected status 'acknowledged', got %q", response.Status)
	}
}

func TestAckCommand_InvalidIDFormat(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/commands/not-a-uuid/ack", nil)
	req.SetPathValue("id", "not-a-uuid")
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.AckCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAckCommand_WrongDevice(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.data[cmdTestCommandUUID] = Command{
		DeviceID: cmdTestDeviceUUID,
		Type:     "reboot",
		Status:   "pending",
	}

	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/commands/"+cmdTestCommandUUID+"/ack", nil)
	req.SetPathValue("id", cmdTestCommandUUID)
	req = withDeviceCtx(req, cmdTestDeviceUUID2) // Different device
	w := httptest.NewRecorder()

	h.AckCommand(w, req)

	// Should return 404 (don't reveal command exists)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteCommand_Delete(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.data[cmdTestCommandUUID] = Command{
		DeviceID: cmdTestDeviceUUID,
		Type:     "reboot",
		Status:   "pending",
	}

	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/commands/"+cmdTestCommandUUID, nil)
	req.SetPathValue("id", cmdTestCommandUUID)
	w := httptest.NewRecorder()

	h.DeleteCommand(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify command was deleted
	if _, exists := mockStore.data[cmdTestCommandUUID]; exists {
		t.Error("expected command to be deleted")
	}
}

func TestDeleteCommand_InvalidIDFormat(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/commands/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.DeleteCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestDeleteCommand_NotFound(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/commands/"+cmdTestCommandUUID, nil)
	req.SetPathValue("id", cmdTestCommandUUID)
	w := httptest.NewRecorder()

	h.DeleteCommand(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteCommand_MissingID(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/commands/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	h.DeleteCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetCommands_StoreError(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.GetErr = errors.New("database error")
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetCommands_CustomStatus(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.data["cmd-1"] = Command{DeviceID: cmdTestDeviceUUID, Type: "reboot", Status: "acknowledged"}
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands?status=acknowledged", nil)
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCreateCommand_WithPayload(t *testing.T) {
	mockStore := NewMockCommandStore()
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	body := `{"device_id": "` + cmdTestDeviceUUID + `", "type": "config", "payload": {"key": "value"}}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestCreateCommand_WithCustomExpiry(t *testing.T) {
	mockStore := NewMockCommandStore()
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	body := `{"device_id": "` + cmdTestDeviceUUID + `", "type": "reboot", "expires_at": "2030-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestAckCommand_UpdateError(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.data[cmdTestCommandUUID] = Command{DeviceID: cmdTestDeviceUUID, Type: "reboot", Status: "pending"}
	mockStore.UpdateErr = errors.New("update failed")
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/commands/"+cmdTestCommandUUID+"/ack", nil)
	req.SetPathValue("id", cmdTestCommandUUID)
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.AckCommand(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestDeleteCommand_DeleteError(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.data[cmdTestCommandUUID] = Command{DeviceID: cmdTestDeviceUUID, Type: "reboot", Status: "pending"}
	mockStore.DeleteErr = errors.New("delete failed")
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/commands/"+cmdTestCommandUUID, nil)
	req.SetPathValue("id", cmdTestCommandUUID)
	w := httptest.NewRecorder()

	h.DeleteCommand(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetCommands_NoExpiredFilter(t *testing.T) {
	mockStore := NewMockCommandStore()
	// Command with no expiry
	mockStore.data["cmd-1"] = Command{DeviceID: cmdTestDeviceUUID, Type: "reboot", Status: "pending", ExpiresAt: nil}
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Count int `json:"count"`
	}
	json.NewDecoder(w.Body).Decode(&response)

	if response.Count != 1 {
		t.Errorf("expected count 1, got %d", response.Count)
	}
}

func TestCreateCommand_EmptyBody(t *testing.T) {
	h := NewWithStores(nil, NewMockCommandStore(), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetCommands_EmptyResult(t *testing.T) {
	mockStore := NewMockCommandStore()
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req = withDeviceCtx(req, "550e8400-e29b-41d4-a716-446655440099") // nonexistent device
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Data  []Command `json:"data"`
		Count int       `json:"count"`
	}
	json.NewDecoder(w.Body).Decode(&response)

	if response.Count != 0 {
		t.Errorf("expected count 0, got %d", response.Count)
	}
}

func TestGetCommands_MultipleCommands(t *testing.T) {
	mockStore := NewMockCommandStore()
	future := time.Now().Add(24 * time.Hour)
	mockStore.data["cmd-1"] = Command{DeviceID: cmdTestDeviceUUID, Type: "reboot", Status: "pending", ExpiresAt: &future}
	mockStore.data["cmd-2"] = Command{DeviceID: cmdTestDeviceUUID, Type: "config", Status: "pending", ExpiresAt: &future}
	mockStore.data["cmd-3"] = Command{DeviceID: cmdTestDeviceUUID2, Type: "reboot", Status: "pending", ExpiresAt: &future}
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req = withDeviceCtx(req, cmdTestDeviceUUID)
	w := httptest.NewRecorder()

	h.GetCommands(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Count int `json:"count"`
	}
	json.NewDecoder(w.Body).Decode(&response)

	if response.Count != 2 {
		t.Errorf("expected count 2, got %d", response.Count)
	}
}

func TestCreateCommand_AllFieldsSet(t *testing.T) {
	mockStore := NewMockCommandStore()
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	body := `{
		"device_id": "` + cmdTestDeviceUUID + `",
		"type": "config",
		"payload": {"setting": "value", "enabled": true},
		"expires_at": "2030-12-31T23:59:59Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCommand(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response Command
	json.NewDecoder(w.Body).Decode(&response)

	if response.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", response.Status)
	}
	if response.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if response.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestDeleteCommand_AcknowledgeFalse(t *testing.T) {
	mockStore := NewMockCommandStore()
	mockStore.data[cmdTestCommandUUID] = Command{DeviceID: cmdTestDeviceUUID, Type: "reboot", Status: "pending"}
	h := NewWithStores(nil, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/commands/"+cmdTestCommandUUID+"?acknowledge=false", nil)
	req.SetPathValue("id", cmdTestCommandUUID)
	w := httptest.NewRecorder()

	h.DeleteCommand(w, req)

	// acknowledge=false should delete the command
	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}
