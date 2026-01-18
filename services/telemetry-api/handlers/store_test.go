package handlers

import (
	"context"
	"testing"
	"time"
)

func TestMockTelemetryStore_SaveAndGet(t *testing.T) {
	store := NewMockTelemetryStore()
	ctx := context.Background()

	data := &TelemetryData{
		DeviceID:  "test-device",
		Type:      "temperature",
		Value:     25.5,
		Timestamp: time.Now(),
	}

	id, err := store.Save(ctx, data)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty ID")
	}

	results, err := store.GetByDeviceID(ctx, "test-device", 10)
	if err != nil {
		t.Fatalf("GetByDeviceID failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestMockTelemetryStore_Errors(t *testing.T) {
	store := NewMockTelemetryStore()
	ctx := context.Background()

	store.SaveErr = ErrNotFound
	_, err := store.Save(ctx, &TelemetryData{})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	store.GetErr = ErrNotFound
	_, err = store.GetByDeviceID(ctx, "test", 10)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockCommandStore_CRUD(t *testing.T) {
	store := NewMockCommandStore()
	ctx := context.Background()

	cmd := &Command{
		DeviceID: "test-device",
		Type:     "reboot",
		Status:   "pending",
	}

	// Save
	id, err := store.Save(ctx, cmd)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// GetByID
	retrieved, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if retrieved.DeviceID != "test-device" {
		t.Errorf("expected device_id 'test-device', got %q", retrieved.DeviceID)
	}

	// GetByDeviceID
	results, err := store.GetByDeviceID(ctx, "test-device", "pending")
	if err != nil {
		t.Fatalf("GetByDeviceID failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// Update
	err = store.Update(ctx, id, map[string]interface{}{"status": "acknowledged"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Delete
	err = store.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.GetByID(ctx, id)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMockCommandStore_Errors(t *testing.T) {
	store := NewMockCommandStore()
	ctx := context.Background()

	store.SaveErr = ErrNotFound
	_, err := store.Save(ctx, &Command{})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	store.SaveErr = nil

	store.GetErr = ErrNotFound
	_, err = store.GetByDeviceID(ctx, "test", "pending")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	_, err = store.GetByID(ctx, "test")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	store.GetErr = nil

	// Update non-existent
	err = store.Update(ctx, "nonexistent", map[string]interface{}{})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Delete non-existent
	err = store.Delete(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// UpdateErr
	store.data["test"] = Command{}
	store.UpdateErr = ErrNotFound
	err = store.Update(ctx, "test", map[string]interface{}{})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// DeleteErr
	store.DeleteErr = ErrNotFound
	err = store.Delete(ctx, "test")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockEventPublisher(t *testing.T) {
	pub := NewMockEventPublisher()
	ctx := context.Background()

	err := pub.Publish(ctx, "test-topic", []byte("test data"))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if len(pub.Published) != 1 {
		t.Errorf("expected 1 published event, got %d", len(pub.Published))
	}
	if pub.Published[0].Topic != "test-topic" {
		t.Errorf("expected topic 'test-topic', got %q", pub.Published[0].Topic)
	}

	pub.PublishErr = ErrNotFound
	err = pub.Publish(ctx, "test", []byte{})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTelemetryData_Fields(t *testing.T) {
	now := time.Now()
	data := TelemetryData{
		ID:        "test-id",
		DeviceID:  "device-1",
		Timestamp: now,
		Type:      "temperature",
		Value:     25.5,
		Unit:      "celsius",
		Metadata:  map[string]interface{}{"location": "room1"},
		CreatedAt: now,
	}

	if data.ID != "test-id" {
		t.Errorf("ID mismatch")
	}
	if data.DeviceID != "device-1" {
		t.Errorf("DeviceID mismatch")
	}
	if data.Value != 25.5 {
		t.Errorf("Value mismatch")
	}
}

func TestCommand_Fields(t *testing.T) {
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	cmd := Command{
		ID:        "cmd-1",
		DeviceID:  "device-1",
		Type:      "reboot",
		Payload:   map[string]interface{}{"reason": "test"},
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: &expires,
	}

	if cmd.ID != "cmd-1" {
		t.Errorf("ID mismatch")
	}
	if cmd.Status != "pending" {
		t.Errorf("Status mismatch")
	}
	if cmd.ExpiresAt == nil {
		t.Errorf("ExpiresAt should not be nil")
	}
}
