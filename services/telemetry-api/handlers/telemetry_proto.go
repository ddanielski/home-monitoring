package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/proto"
)

// Measurement represents a decoded measurement from the device.
// ID is the semantic identifier defined by firmware (0 to UINT32_MAX).
// Value type is auto-detected from the protobuf wire format.
type Measurement struct {
	ID    uint32
	Value interface{}
}

// Measurement proto field numbers (defined by the proto schema)
const (
	measurementFieldID     uint32 = 1
	measurementFieldFloat  uint32 = 2
	measurementFieldDouble uint32 = 3
	measurementFieldInt32  uint32 = 4
	measurementFieldInt64  uint32 = 5
	measurementFieldUint32 uint32 = 6
	measurementFieldUint64 uint32 = 7
	measurementFieldBool   uint32 = 8
)

// decodeMeasurementBatch decodes a MeasurementBatch protobuf into Measurement structs.
// Uses the generic protobuf decoder and interprets fields based on the measurement schema.
func decodeMeasurementBatch(data []byte) ([]Measurement, error) {
	// Decode the batch - field 1 contains repeated Measurement messages
	messages, err := proto.DecodeRepeated(data, 1)
	if err != nil {
		return nil, err
	}

	measurements := make([]Measurement, 0, len(messages))
	for _, msg := range messages {
		m := Measurement{}

		// Get measurement ID (field 1)
		if idField := msg.GetField(measurementFieldID); idField != nil {
			m.ID = idField.Value.AsUint32()
		}

		// Auto-detect value from which oneof field is present
		for _, field := range msg {
			switch field.Num {
			case measurementFieldFloat:
				m.Value = field.Value.AsFloat32()
			case measurementFieldDouble:
				m.Value = field.Value.AsFloat64()
			case measurementFieldInt32:
				m.Value = field.Value.AsInt32()
			case measurementFieldInt64:
				m.Value = field.Value.AsInt64()
			case measurementFieldUint32:
				m.Value = field.Value.AsUint32()
			case measurementFieldUint64:
				m.Value = field.Value.AsUint64()
			case measurementFieldBool:
				m.Value = field.Value.AsBool()
			default:
				continue
			}
			break // Found the value field
		}

		measurements = append(measurements, m)
	}

	return measurements, nil
}

// HandleTelemetryProto handles POST /telemetry/proto
// Accepts protobuf-encoded MeasurementBatch and decodes it using the device's schema
// Must be called with AuthMiddleware - device ID comes from verified token
func (h *Handlers) HandleTelemetryProto(w http.ResponseWriter, r *http.Request) {
	// Get device ID from authenticated context (set by AuthMiddleware)
	deviceID := GetDeviceIDFromContext(r.Context())
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Lookup device for app/version info
	device, err := h.deviceStore.GetByID(r.Context(), deviceID)
	if err != nil {
		h.jsonError(w, "device not found", http.StatusNotFound)
		return
	}

	// Get schema for this device's app/version (cached)
	schema, err := h.schemaStore.Get(r.Context(), device.AppName, device.AppVersion)
	if err != nil {
		h.jsonError(w, "schema not found for device app/version", http.StatusNotFound)
		return
	}

	// Read protobuf body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.jsonError(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Decode protobuf using the measurement-specific decoder
	measurements, err := decodeMeasurementBatch(body)
	if err != nil {
		h.jsonError(w, fmt.Sprintf("failed to decode protobuf: %v", err), http.StatusBadRequest)
		return
	}

	// Build ID lookup map from schema (id -> metadata)
	idToMeta := make(map[uint32]MeasurementMeta)
	for _, meta := range schema.Measurements {
		idToMeta[meta.ID] = meta
	}

	// Process measurements
	now := time.Now().UTC()
	var timestamp time.Time
	var savedCount int

	// First pass: find timestamp
	for _, m := range measurements {
		meta, ok := idToMeta[m.ID]
		if ok && meta.Name == "timestamp" {
			// Timestamp is in milliseconds
			switch ts := m.Value.(type) {
			case uint64:
				timestamp = time.UnixMilli(int64(ts))
			case int64:
				timestamp = time.UnixMilli(ts)
			case uint32:
				timestamp = time.UnixMilli(int64(ts))
			case int32:
				timestamp = time.UnixMilli(int64(ts))
			}
			break
		}
	}

	// If no timestamp in payload, use server time
	if timestamp.IsZero() {
		timestamp = now
	}

	// Second pass: save measurements
	for _, m := range measurements {
		meta, ok := idToMeta[m.ID]
		if !ok {
			h.logger.Warn("unknown measurement ID", "id", m.ID, "device_id", deviceID)
			continue
		}

		// Skip timestamp measurement (already extracted)
		if meta.Name == "timestamp" {
			continue
		}

		if m.Value == nil {
			continue
		}

		// Convert to float64 for storage
		floatVal, err := toFloat64(m.Value)
		if err != nil {
			h.logger.Warn("failed to convert value", "error", err, "measurement", meta.Name)
			continue
		}

		data := &TelemetryData{
			DeviceID:  deviceID,
			Timestamp: timestamp,
			Type:      meta.Name,
			Value:     floatVal,
			Unit:      meta.Unit,
			CreatedAt: now,
		}

		if _, err := h.telemetryStore.Save(r.Context(), data); err != nil {
			h.logger.Error("failed to save telemetry", "error", err, "measurement", meta.Name)
			continue
		}
		savedCount++
	}

	// Update device last seen
	if err := h.deviceStore.UpdateLastSeen(r.Context(), deviceID); err != nil {
		h.logger.Warn("failed to update device last_seen", "error", err, "device_id", deviceID)
	}

	// Publish event
	if h.publisher != nil {
		eventData := map[string]interface{}{
			"device_id":   deviceID,
			"app_name":    device.AppName,
			"app_version": device.AppVersion,
			"count":       savedCount,
			"timestamp":   timestamp,
		}
		eventJSON, _ := json.Marshal(eventData)
		if err := h.publisher.Publish(r.Context(), TelemetryTopic, eventJSON); err != nil {
			h.logger.Warn("failed to publish telemetry event", "error", err)
		}
	}

	h.logger.Info("telemetry batch processed",
		"device_id", deviceID,
		"measurements", savedCount,
		"timestamp", timestamp,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "telemetry received",
		"measurements": savedCount,
	})
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}
