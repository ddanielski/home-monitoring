package handlers

import (
	"context"
	"time"
)

// TelemetryStore defines the interface for telemetry data storage
type TelemetryStore interface {
	Save(ctx context.Context, data *TelemetryData) (string, error)
	GetByDeviceID(ctx context.Context, deviceID string, limit int) ([]TelemetryData, error)
}

// CommandStore defines the interface for command storage
type CommandStore interface {
	Save(ctx context.Context, cmd *Command) (string, error)
	GetByDeviceID(ctx context.Context, deviceID string, status string) ([]Command, error)
	GetByID(ctx context.Context, id string) (*Command, error)
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	Delete(ctx context.Context, id string) error
}

// EventPublisher defines the interface for publishing events
type EventPublisher interface {
	Publish(ctx context.Context, topic string, data []byte) error
}

// TelemetryData represents a telemetry reading
type TelemetryData struct {
	ID        string                 `json:"id,omitempty" firestore:"-"`
	DeviceID  string                 `json:"device_id" firestore:"device_id"`
	Timestamp time.Time              `json:"timestamp" firestore:"timestamp"`
	Type      string                 `json:"type" firestore:"type"`
	Value     float64                `json:"value" firestore:"value"`
	Unit      string                 `json:"unit,omitempty" firestore:"unit,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" firestore:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at" firestore:"created_at"`
}

// Command represents a command to be sent to a device
type Command struct {
	ID        string                 `json:"id,omitempty" firestore:"-"`
	DeviceID  string                 `json:"device_id" firestore:"device_id"`
	Type      string                 `json:"type" firestore:"type"`
	Payload   map[string]interface{} `json:"payload,omitempty" firestore:"payload,omitempty"`
	Status    string                 `json:"status" firestore:"status"`
	CreatedAt time.Time              `json:"created_at" firestore:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" firestore:"updated_at"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty" firestore:"expires_at,omitempty"`
}

// DeviceStore defines the interface for device storage
type DeviceStore interface {
	Register(ctx context.Context, device *Device) error
	GetByID(ctx context.Context, deviceID string) (*Device, error)
	GetByMAC(ctx context.Context, macAddress string) (*Device, error) // Check if MAC already registered
	UpdateLastSeen(ctx context.Context, deviceID string) error
	Revoke(ctx context.Context, deviceID string) error
}

// SchemaStore defines the interface for measurement schema storage
type SchemaStore interface {
	Save(ctx context.Context, appName, version string, schema *MeasurementSchema) error
	Get(ctx context.Context, appName, version string) (*MeasurementSchema, error)
}

// Device represents a registered IoT device
type Device struct {
	DeviceID     string    `json:"device_id" firestore:"device_id"`     // UUID - logical identity
	MACAddress   string    `json:"mac_address" firestore:"mac_address"` // Normalized MAC (lowercase, no colons)
	AppName      string    `json:"app_name" firestore:"app_name"`
	AppVersion   string    `json:"app_version" firestore:"app_version"`
	SecretHash   string    `json:"-" firestore:"secret_hash"`   // bcrypt hash, never exposed in JSON
	Revoked      bool      `json:"revoked" firestore:"revoked"` // true if device access is revoked
	RegisteredAt time.Time `json:"registered_at" firestore:"registered_at"`
	LastSeen     time.Time `json:"last_seen" firestore:"last_seen"`
}

// MeasurementSchema defines the schema for measurement interpretation
type MeasurementSchema struct {
	AppName      string                     `json:"app_name" firestore:"app_name"`
	Version      string                     `json:"version" firestore:"version"`
	Measurements map[string]MeasurementMeta `json:"measurements" firestore:"measurements"`
	CreatedAt    time.Time                  `json:"created_at" firestore:"created_at"`
}

// MeasurementMeta defines metadata for a measurement type
type MeasurementMeta struct {
	ID   uint32 `json:"id" firestore:"id"`
	Name string `json:"name" firestore:"name"`
	Type string `json:"type" firestore:"type"`
	Unit string `json:"unit" firestore:"unit"`
}
