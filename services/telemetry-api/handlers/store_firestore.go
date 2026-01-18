package handlers

import (
	"context"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// FirestoreTelemetryStore implements TelemetryStore using Firestore
type FirestoreTelemetryStore struct {
	client     *firestore.Client
	collection string
}

// NewFirestoreTelemetryStore creates a new Firestore-backed telemetry store
func NewFirestoreTelemetryStore(client *firestore.Client) *FirestoreTelemetryStore {
	return &FirestoreTelemetryStore{
		client:     client,
		collection: TelemetryCollection,
	}
}

func (s *FirestoreTelemetryStore) Save(ctx context.Context, data *TelemetryData) (string, error) {
	docRef, _, err := s.client.Collection(s.collection).Add(ctx, data)
	if err != nil {
		return "", err
	}
	return docRef.ID, nil
}

func (s *FirestoreTelemetryStore) GetByDeviceID(ctx context.Context, deviceID string, limit int) ([]TelemetryData, error) {
	query := s.client.Collection(s.collection).
		Where("device_id", "==", deviceID).
		OrderBy("created_at", firestore.Desc).
		Limit(limit)

	iter := query.Documents(ctx)
	defer iter.Stop()

	var results []TelemetryData
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data TelemetryData
		if err := doc.DataTo(&data); err != nil {
			return nil, err
		}
		data.ID = doc.Ref.ID
		results = append(results, data)
	}
	return results, nil
}

// FirestoreCommandStore implements CommandStore using Firestore
type FirestoreCommandStore struct {
	client     *firestore.Client
	collection string
}

// NewFirestoreCommandStore creates a new Firestore-backed command store
func NewFirestoreCommandStore(client *firestore.Client) *FirestoreCommandStore {
	return &FirestoreCommandStore{
		client:     client,
		collection: CommandsCollection,
	}
}

func (s *FirestoreCommandStore) Save(ctx context.Context, cmd *Command) (string, error) {
	docRef, _, err := s.client.Collection(s.collection).Add(ctx, cmd)
	if err != nil {
		return "", err
	}
	return docRef.ID, nil
}

func (s *FirestoreCommandStore) GetByDeviceID(ctx context.Context, deviceID string, status string) ([]Command, error) {
	query := s.client.Collection(s.collection).
		Where("device_id", "==", deviceID).
		Where("status", "==", status).
		OrderBy("created_at", firestore.Desc)

	iter := query.Documents(ctx)
	defer iter.Stop()

	var results []Command
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var cmd Command
		if err := doc.DataTo(&cmd); err != nil {
			return nil, err
		}
		cmd.ID = doc.Ref.ID
		results = append(results, cmd)
	}
	return results, nil
}

func (s *FirestoreCommandStore) GetByID(ctx context.Context, id string) (*Command, error) {
	doc, err := s.client.Collection(s.collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}

	var cmd Command
	if err := doc.DataTo(&cmd); err != nil {
		return nil, err
	}
	cmd.ID = doc.Ref.ID
	return &cmd, nil
}

func (s *FirestoreCommandStore) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	var firestoreUpdates []firestore.Update
	for k, v := range updates {
		firestoreUpdates = append(firestoreUpdates, firestore.Update{Path: k, Value: v})
	}
	_, err := s.client.Collection(s.collection).Doc(id).Update(ctx, firestoreUpdates)
	return err
}

func (s *FirestoreCommandStore) Delete(ctx context.Context, id string) error {
	_, err := s.client.Collection(s.collection).Doc(id).Delete(ctx)
	return err
}

// FirestoreDeviceStore implements DeviceStore using Firestore
type FirestoreDeviceStore struct {
	client     *firestore.Client
	collection string
}

// NewFirestoreDeviceStore creates a new Firestore-backed device store
func NewFirestoreDeviceStore(client *firestore.Client) *FirestoreDeviceStore {
	return &FirestoreDeviceStore{
		client:     client,
		collection: DevicesCollection,
	}
}

func (s *FirestoreDeviceStore) Register(ctx context.Context, device *Device) error {
	_, err := s.client.Collection(s.collection).Doc(device.DeviceID).Set(ctx, device)
	return err
}

func (s *FirestoreDeviceStore) GetByID(ctx context.Context, deviceID string) (*Device, error) {
	doc, err := s.client.Collection(s.collection).Doc(deviceID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var device Device
	if err := doc.DataTo(&device); err != nil {
		return nil, err
	}
	return &device, nil
}

func (s *FirestoreDeviceStore) GetByMAC(ctx context.Context, macAddress string) (*Device, error) {
	// Query by MAC address (normalized, lowercase, no colons)
	iter := s.client.Collection(s.collection).Where("mac_address", "==", macAddress).Limit(1).Documents(ctx)
	doc, err := iter.Next()
	if err != nil {
		return nil, err // includes iterator.Done if no results
	}

	var device Device
	if err := doc.DataTo(&device); err != nil {
		return nil, err
	}
	device.DeviceID = doc.Ref.ID
	return &device, nil
}

func (s *FirestoreDeviceStore) UpdateLastSeen(ctx context.Context, deviceID string) error {
	_, err := s.client.Collection(s.collection).Doc(deviceID).Update(ctx, []firestore.Update{
		{Path: "last_seen", Value: firestore.ServerTimestamp},
	})
	return err
}

func (s *FirestoreDeviceStore) Revoke(ctx context.Context, deviceID string) error {
	_, err := s.client.Collection(s.collection).Doc(deviceID).Update(ctx, []firestore.Update{
		{Path: "revoked", Value: true},
	})
	return err
}

// FirestoreSchemaStore implements SchemaStore using Firestore
type FirestoreSchemaStore struct {
	client     *firestore.Client
	collection string
}

// NewFirestoreSchemaStore creates a new Firestore-backed schema store
func NewFirestoreSchemaStore(client *firestore.Client) *FirestoreSchemaStore {
	return &FirestoreSchemaStore{
		client:     client,
		collection: SchemasCollection,
	}
}

func (s *FirestoreSchemaStore) Save(ctx context.Context, appName, version string, schema *MeasurementSchema) error {
	docID := appName + ":" + version
	_, err := s.client.Collection(s.collection).Doc(docID).Set(ctx, schema)
	return err
}

func (s *FirestoreSchemaStore) Get(ctx context.Context, appName, version string) (*MeasurementSchema, error) {
	docID := appName + ":" + version
	doc, err := s.client.Collection(s.collection).Doc(docID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var schema MeasurementSchema
	if err := doc.DataTo(&schema); err != nil {
		return nil, err
	}
	return &schema, nil
}
