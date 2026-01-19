package handlers

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("not found")

// MockTelemetryStore is a mock implementation for testing
type MockTelemetryStore struct {
	mu      sync.RWMutex
	data    map[string]TelemetryData
	nextID  int
	SaveErr error
	GetErr  error
}

func NewMockTelemetryStore() *MockTelemetryStore {
	return &MockTelemetryStore{
		data: make(map[string]TelemetryData),
	}
}

func (m *MockTelemetryStore) Save(ctx context.Context, data *TelemetryData) (string, error) {
	if m.SaveErr != nil {
		return "", m.SaveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := "mock-" + string(rune('0'+m.nextID))
	m.data[id] = *data
	return id, nil
}

func (m *MockTelemetryStore) GetByDeviceID(ctx context.Context, deviceID string, limit int) ([]TelemetryData, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []TelemetryData
	for id, d := range m.data {
		if d.DeviceID == deviceID {
			d.ID = id
			results = append(results, d)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// MockCommandStore is a mock implementation for testing
type MockCommandStore struct {
	mu        sync.RWMutex
	data      map[string]Command
	nextID    int
	SaveErr   error
	GetErr    error
	UpdateErr error
	DeleteErr error
}

func NewMockCommandStore() *MockCommandStore {
	return &MockCommandStore{
		data: make(map[string]Command),
	}
}

func (m *MockCommandStore) Save(ctx context.Context, cmd *Command) (string, error) {
	if m.SaveErr != nil {
		return "", m.SaveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := "cmd-" + string(rune('0'+m.nextID))
	m.data[id] = *cmd
	return id, nil
}

func (m *MockCommandStore) GetByDeviceID(ctx context.Context, deviceID string, status string) ([]Command, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Command
	for id, c := range m.data {
		if c.DeviceID == deviceID && c.Status == status {
			c.ID = id
			results = append(results, c)
		}
	}
	return results, nil
}

func (m *MockCommandStore) GetByID(ctx context.Context, id string) (*Command, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd, ok := m.data[id]
	if !ok {
		return nil, ErrNotFound
	}
	cmd.ID = id
	return &cmd, nil
}

func (m *MockCommandStore) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd, ok := m.data[id]
	if !ok {
		return ErrNotFound
	}

	if status, ok := updates["status"].(string); ok {
		cmd.Status = status
	}
	m.data[id] = cmd
	return nil
}

func (m *MockCommandStore) Delete(ctx context.Context, id string) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.data[id]; !ok {
		return ErrNotFound
	}
	delete(m.data, id)
	return nil
}

// MockEventPublisher is a mock implementation for testing
type MockEventPublisher struct {
	mu         sync.Mutex
	Published  []PublishedEvent
	PublishErr error
}

type PublishedEvent struct {
	Topic string
	Data  []byte
}

func NewMockEventPublisher() *MockEventPublisher {
	return &MockEventPublisher{
		Published: make([]PublishedEvent, 0),
	}
}

func (m *MockEventPublisher) Publish(ctx context.Context, topic string, data []byte) error {
	if m.PublishErr != nil {
		return m.PublishErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Published = append(m.Published, PublishedEvent{Topic: topic, Data: data})
	return nil
}

// MockDeviceStore is a mock implementation for testing
type MockDeviceStore struct {
	mu          sync.RWMutex
	data        map[string]Device
	RegisterErr error
	GetErr      error
	UpdateErr   error
	RevokeErr   error
}

func NewMockDeviceStore() *MockDeviceStore {
	return &MockDeviceStore{
		data: make(map[string]Device),
	}
}

func (m *MockDeviceStore) Register(ctx context.Context, device *Device) error {
	if m.RegisterErr != nil {
		return m.RegisterErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[device.DeviceID] = *device
	return nil
}

func (m *MockDeviceStore) GetByID(ctx context.Context, deviceID string) (*Device, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.data[deviceID]
	if !ok {
		return nil, ErrNotFound
	}
	return &device, nil
}

func (m *MockDeviceStore) GetByMAC(ctx context.Context, macAddress string) (*Device, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, device := range m.data {
		if device.MACAddress == macAddress {
			return &device, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockDeviceStore) UpdateLastSeen(ctx context.Context, deviceID string) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.data[deviceID]
	if !ok {
		return ErrNotFound
	}
	device.LastSeen = device.LastSeen // Would be updated in real implementation
	m.data[deviceID] = device
	return nil
}

func (m *MockDeviceStore) UpdateAppInfo(ctx context.Context, deviceID, appName, appVersion string) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.data[deviceID]
	if !ok {
		return ErrNotFound
	}
	device.AppName = appName
	device.AppVersion = appVersion
	m.data[deviceID] = device
	return nil
}

func (m *MockDeviceStore) Revoke(ctx context.Context, deviceID string) error {
	if m.RevokeErr != nil {
		return m.RevokeErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.data[deviceID]
	if !ok {
		return ErrNotFound
	}
	device.Revoked = true
	m.data[deviceID] = device
	return nil
}

// MockSchemaStore is a mock implementation for testing
type MockSchemaStore struct {
	mu      sync.RWMutex
	data    map[string]MeasurementSchema
	SaveErr error
	GetErr  error
}

func NewMockSchemaStore() *MockSchemaStore {
	return &MockSchemaStore{
		data: make(map[string]MeasurementSchema),
	}
}

func (m *MockSchemaStore) Save(ctx context.Context, appName, version string, schema *MeasurementSchema) error {
	if m.SaveErr != nil {
		return m.SaveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := appName + ":" + version
	m.data[key] = *schema
	return nil
}

func (m *MockSchemaStore) Get(ctx context.Context, appName, version string) (*MeasurementSchema, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := appName + ":" + version
	schema, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return &schema, nil
}
