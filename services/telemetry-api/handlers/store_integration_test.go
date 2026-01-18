package handlers

import (
	"context"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
)

func skipIfNoEmulator(t *testing.T) {
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("Skipping integration test: FIRESTORE_EMULATOR_HOST not set")
	}
}

func skipIfNoPubSubEmulator(t *testing.T) {
	if os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		t.Skip("Skipping integration test: PUBSUB_EMULATOR_HOST not set")
	}
}

func setupFirestoreClient(t *testing.T) *firestore.Client {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to create Firestore client: %v", err)
	}
	return client
}

func setupPubSubClient(t *testing.T) *pubsub.Client {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to create Pub/Sub client: %v", err)
	}
	return client
}

// =============================================================================
// Firestore Telemetry Store Integration Tests
// =============================================================================

func TestFirestoreTelemetryStore_SaveAndGet(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreTelemetryStore(client)
	ctx := context.Background()

	// Save telemetry
	data := &TelemetryData{
		DeviceID:  "integration-test-device",
		Type:      "temperature",
		Value:     25.5,
		Unit:      "celsius",
		Timestamp: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}

	id, err := store.Save(ctx, data)
	if err != nil {
		t.Fatalf("Failed to save telemetry: %v", err)
	}
	if id == "" {
		t.Error("Expected non-empty ID")
	}

	// Retrieve telemetry
	results, err := store.GetByDeviceID(ctx, "integration-test-device", 10)
	if err != nil {
		t.Fatalf("Failed to get telemetry: %v", err)
	}
	if len(results) == 0 {
		t.Error("Expected at least one result")
	}

	// Verify data
	found := false
	for _, r := range results {
		if r.ID == id {
			found = true
			if r.DeviceID != data.DeviceID {
				t.Errorf("DeviceID mismatch: got %q, want %q", r.DeviceID, data.DeviceID)
			}
			if r.Type != data.Type {
				t.Errorf("Type mismatch: got %q, want %q", r.Type, data.Type)
			}
		}
	}
	if !found {
		t.Error("Saved document not found in results")
	}
}

func TestFirestoreTelemetryStore_GetByDeviceID_Empty(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreTelemetryStore(client)
	ctx := context.Background()

	results, err := store.GetByDeviceID(ctx, "nonexistent-device-12345", 10)
	if err != nil {
		t.Fatalf("Failed to get telemetry: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

// =============================================================================
// Firestore Command Store Integration Tests
// =============================================================================

func TestFirestoreCommandStore_CRUD(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreCommandStore(client)
	ctx := context.Background()

	// Create command
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)
	cmd := &Command{
		DeviceID:  "integration-test-device",
		Type:      "reboot",
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: &expires,
	}

	id, err := store.Save(ctx, cmd)
	if err != nil {
		t.Fatalf("Failed to save command: %v", err)
	}
	if id == "" {
		t.Error("Expected non-empty ID")
	}

	// Get by ID
	retrieved, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("Failed to get command by ID: %v", err)
	}
	if retrieved.DeviceID != cmd.DeviceID {
		t.Errorf("DeviceID mismatch: got %q, want %q", retrieved.DeviceID, cmd.DeviceID)
	}
	if retrieved.Status != "pending" {
		t.Errorf("Status mismatch: got %q, want %q", retrieved.Status, "pending")
	}

	// Get by device ID
	results, err := store.GetByDeviceID(ctx, "integration-test-device", "pending")
	if err != nil {
		t.Fatalf("Failed to get commands by device ID: %v", err)
	}
	if len(results) == 0 {
		t.Error("Expected at least one result")
	}

	// Update
	err = store.Update(ctx, id, map[string]interface{}{
		"status":     "acknowledged",
		"updated_at": time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Failed to update command: %v", err)
	}

	// Verify update
	updated, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("Failed to get updated command: %v", err)
	}
	if updated.Status != "acknowledged" {
		t.Errorf("Status not updated: got %q, want %q", updated.Status, "acknowledged")
	}

	// Delete
	err = store.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Failed to delete command: %v", err)
	}

	// Verify deletion
	_, err = store.GetByID(ctx, id)
	if err == nil {
		t.Error("Expected error when getting deleted command")
	}
}

func TestFirestoreCommandStore_GetByDeviceID_Empty(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreCommandStore(client)
	ctx := context.Background()

	results, err := store.GetByDeviceID(ctx, "nonexistent-device-12345", "pending")
	if err != nil {
		t.Fatalf("Failed to get commands: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

// =============================================================================
// Pub/Sub Publisher Integration Tests
// =============================================================================

func TestPubSubPublisher_Publish(t *testing.T) {
	skipIfNoPubSubEmulator(t)
	client := setupPubSubClient(t)
	defer client.Close()

	publisher := NewPubSubPublisher(client)
	ctx := context.Background()

	// Publish a message
	err := publisher.Publish(ctx, "test-topic", []byte(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}
}

func TestPubSubPublisher_CreateTopicIfNotExists(t *testing.T) {
	skipIfNoPubSubEmulator(t)
	client := setupPubSubClient(t)
	defer client.Close()

	publisher := NewPubSubPublisher(client)
	ctx := context.Background()

	// Publish to a new topic (should create it)
	uniqueTopic := "test-topic-" + time.Now().Format("20060102150405")
	err := publisher.Publish(ctx, uniqueTopic, []byte(`{"test": "new topic"}`))
	if err != nil {
		t.Fatalf("Failed to publish to new topic: %v", err)
	}

	// Verify topic exists
	topic := client.Topic(uniqueTopic)
	exists, err := topic.Exists(ctx)
	if err != nil {
		t.Fatalf("Failed to check topic existence: %v", err)
	}
	if !exists {
		t.Error("Expected topic to exist after publishing")
	}
}

// =============================================================================
// Firestore Device Store Integration Tests
// =============================================================================

func TestFirestoreDeviceStore_RegisterAndGet(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreDeviceStore(client)
	ctx := context.Background()

	// Generate unique MAC for this test
	uniqueMAC := "aa" + time.Now().Format("150405") + "0001"

	// Register device
	device := &Device{
		DeviceID:     "test-device-" + time.Now().Format("20060102150405"),
		MACAddress:   uniqueMAC,
		AppName:      "test-app",
		AppVersion:   "1.0.0",
		SecretHash:   "hash123",
		Revoked:      false,
		RegisteredAt: time.Now().UTC(),
		LastSeen:     time.Now().UTC(),
	}

	err := store.Register(ctx, device)
	if err != nil {
		t.Fatalf("Failed to register device: %v", err)
	}

	// Get by ID
	retrieved, err := store.GetByID(ctx, device.DeviceID)
	if err != nil {
		t.Fatalf("Failed to get device by ID: %v", err)
	}
	if retrieved.DeviceID != device.DeviceID {
		t.Errorf("DeviceID mismatch: got %q, want %q", retrieved.DeviceID, device.DeviceID)
	}
	if retrieved.MACAddress != device.MACAddress {
		t.Errorf("MACAddress mismatch: got %q, want %q", retrieved.MACAddress, device.MACAddress)
	}
	if retrieved.AppName != device.AppName {
		t.Errorf("AppName mismatch: got %q, want %q", retrieved.AppName, device.AppName)
	}

	// Get by MAC
	byMAC, err := store.GetByMAC(ctx, uniqueMAC)
	if err != nil {
		t.Fatalf("Failed to get device by MAC: %v", err)
	}
	if byMAC.DeviceID != device.DeviceID {
		t.Errorf("DeviceID mismatch when getting by MAC: got %q, want %q", byMAC.DeviceID, device.DeviceID)
	}
}

func TestFirestoreDeviceStore_GetByID_NotFound(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreDeviceStore(client)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent-device-12345")
	if err == nil {
		t.Error("Expected error when getting nonexistent device")
	}
}

func TestFirestoreDeviceStore_GetByMAC_NotFound(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreDeviceStore(client)
	ctx := context.Background()

	_, err := store.GetByMAC(ctx, "nonexistent123456")
	if err == nil {
		t.Error("Expected error when getting nonexistent MAC")
	}
}

func TestFirestoreDeviceStore_UpdateLastSeen(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreDeviceStore(client)
	ctx := context.Background()

	// Generate unique MAC for this test
	uniqueMAC := "bb" + time.Now().Format("150405") + "0002"

	// First register a device
	device := &Device{
		DeviceID:     "test-device-lastseen-" + time.Now().Format("20060102150405"),
		MACAddress:   uniqueMAC,
		AppName:      "test-app",
		AppVersion:   "1.0.0",
		SecretHash:   "hash123",
		Revoked:      false,
		RegisteredAt: time.Now().UTC(),
		LastSeen:     time.Now().Add(-24 * time.Hour).UTC(), // Old timestamp
	}
	err := store.Register(ctx, device)
	if err != nil {
		t.Fatalf("Failed to register device: %v", err)
	}

	// Update last seen
	err = store.UpdateLastSeen(ctx, device.DeviceID)
	if err != nil {
		t.Fatalf("Failed to update last seen: %v", err)
	}

	// Verify update
	updated, err := store.GetByID(ctx, device.DeviceID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	if updated.LastSeen.Before(device.LastSeen) {
		t.Error("LastSeen was not updated")
	}
}

func TestFirestoreDeviceStore_Revoke(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreDeviceStore(client)
	ctx := context.Background()

	// Generate unique MAC for this test
	uniqueMAC := "cc" + time.Now().Format("150405") + "0003"

	// First register a device
	device := &Device{
		DeviceID:     "test-device-revoke-" + time.Now().Format("20060102150405"),
		MACAddress:   uniqueMAC,
		AppName:      "test-app",
		AppVersion:   "1.0.0",
		SecretHash:   "hash123",
		Revoked:      false,
		RegisteredAt: time.Now().UTC(),
		LastSeen:     time.Now().UTC(),
	}
	err := store.Register(ctx, device)
	if err != nil {
		t.Fatalf("Failed to register device: %v", err)
	}

	// Revoke
	err = store.Revoke(ctx, device.DeviceID)
	if err != nil {
		t.Fatalf("Failed to revoke device: %v", err)
	}

	// Verify revocation
	revoked, err := store.GetByID(ctx, device.DeviceID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	if !revoked.Revoked {
		t.Error("Device was not revoked")
	}
}

// =============================================================================
// Firestore Schema Store Integration Tests
// =============================================================================

func TestFirestoreSchemaStore_SaveAndGet(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreSchemaStore(client)
	ctx := context.Background()

	// Create schema
	schema := &MeasurementSchema{
		AppName: "test-app-" + time.Now().Format("20060102150405"),
		Version: "1.0.0",
		Measurements: map[string]MeasurementMeta{
			"temperature": {ID: 1, Name: "temperature", Type: "float", Unit: "celsius"},
			"humidity":    {ID: 2, Name: "humidity", Type: "float", Unit: "percent"},
		},
		CreatedAt: time.Now().UTC(),
	}

	// Save
	err := store.Save(ctx, schema.AppName, schema.Version, schema)
	if err != nil {
		t.Fatalf("Failed to save schema: %v", err)
	}

	// Get
	retrieved, err := store.Get(ctx, schema.AppName, schema.Version)
	if err != nil {
		t.Fatalf("Failed to get schema: %v", err)
	}
	if retrieved.AppName != schema.AppName {
		t.Errorf("AppName mismatch: got %q, want %q", retrieved.AppName, schema.AppName)
	}
	if retrieved.Version != schema.Version {
		t.Errorf("Version mismatch: got %q, want %q", retrieved.Version, schema.Version)
	}
	if len(retrieved.Measurements) != 2 {
		t.Errorf("Measurements count mismatch: got %d, want %d", len(retrieved.Measurements), 2)
	}
}

func TestFirestoreSchemaStore_Get_NotFound(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreSchemaStore(client)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent-app", "1.0.0")
	if err == nil {
		t.Error("Expected error when getting nonexistent schema")
	}
}

func TestFirestoreSchemaStore_Update(t *testing.T) {
	skipIfNoEmulator(t)
	client := setupFirestoreClient(t)
	defer client.Close()

	store := NewFirestoreSchemaStore(client)
	ctx := context.Background()

	appName := "test-app-update-" + time.Now().Format("20060102150405")
	version := "1.0.0"

	// Create initial schema
	schema1 := &MeasurementSchema{
		AppName: appName,
		Version: version,
		Measurements: map[string]MeasurementMeta{
			"temperature": {ID: 1, Name: "temperature", Type: "float", Unit: "celsius"},
		},
		CreatedAt: time.Now().UTC(),
	}
	err := store.Save(ctx, appName, version, schema1)
	if err != nil {
		t.Fatalf("Failed to save initial schema: %v", err)
	}

	// Update with more measurements
	schema2 := &MeasurementSchema{
		AppName: appName,
		Version: version,
		Measurements: map[string]MeasurementMeta{
			"temperature": {ID: 1, Name: "temperature", Type: "float", Unit: "celsius"},
			"humidity":    {ID: 2, Name: "humidity", Type: "float", Unit: "percent"},
			"pressure":    {ID: 3, Name: "pressure", Type: "float", Unit: "hpa"},
		},
		CreatedAt: time.Now().UTC(),
	}
	err = store.Save(ctx, appName, version, schema2)
	if err != nil {
		t.Fatalf("Failed to update schema: %v", err)
	}

	// Verify update
	retrieved, err := store.Get(ctx, appName, version)
	if err != nil {
		t.Fatalf("Failed to get updated schema: %v", err)
	}
	if len(retrieved.Measurements) != 3 {
		t.Errorf("Measurements count after update mismatch: got %d, want %d", len(retrieved.Measurements), 3)
	}
}

// =============================================================================
// Handlers New() Integration Test
// =============================================================================

func TestHandlers_New(t *testing.T) {
	skipIfNoEmulator(t)
	skipIfNoPubSubEmulator(t)

	ctx := context.Background()
	cfg := Config{ProjectID: "test-project"}

	h, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}
	defer h.Close()

	if h.telemetryStore == nil {
		t.Error("Expected telemetryStore to be set")
	}
	if h.commandStore == nil {
		t.Error("Expected commandStore to be set")
	}
	if h.publisher == nil {
		t.Error("Expected publisher to be set")
	}
}
