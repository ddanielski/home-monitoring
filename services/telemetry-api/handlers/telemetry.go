package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/middleware"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/validate"
)

// HandleTelemetry handles POST /telemetry - ingests telemetry data (JSON)
func (h *Handlers) HandleTelemetry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	// Get device ID from auth context (device can only submit their own data)
	deviceID := GetDeviceIDFromContext(ctx)
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var data TelemetryData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Override device_id from auth context (prevent spoofing)
	data.DeviceID = deviceID

	if data.Type == "" {
		h.jsonError(w, "type is required", http.StatusBadRequest)
		return
	}

	// Validate telemetry type
	if err := validate.TelemetryType(data.Type); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid telemetry type: %v", err), http.StatusBadRequest)
		return
	}

	// Set server-side timestamp if not provided
	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now().UTC()
	}
	data.CreatedAt = time.Now().UTC()

	// Store telemetry data
	id, err := h.telemetryStore.Save(ctx, &data)
	if err != nil {
		h.logger.Error("failed to store telemetry",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
			"type", data.Type,
		)
		h.jsonError(w, "failed to store telemetry", http.StatusInternalServerError)
		return
	}
	data.ID = id

	// Publish to event stream for async processing
	if err := h.publishTelemetry(ctx, &data); err != nil {
		// Log but don't fail - data is already stored
		h.logger.Error("failed to publish telemetry event",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
		)
	}

	h.jsonResponse(w, data, http.StatusCreated)
}

// HandleTelemetryBatch handles POST /telemetry/batch - ingests multiple telemetry readings (JSON)
func (h *Handlers) HandleTelemetryBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	deviceID := GetDeviceIDFromContext(ctx)
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var readings []TelemetryData
	if err := json.NewDecoder(r.Body).Decode(&readings); err != nil {
		h.jsonError(w, "invalid request body: expected array", http.StatusBadRequest)
		return
	}

	if len(readings) == 0 {
		h.jsonError(w, "empty batch", http.StatusBadRequest)
		return
	}

	if len(readings) > 100 {
		h.jsonError(w, "batch too large: maximum 100 readings", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	saved := 0

	for i := range readings {
		data := &readings[i]

		// Override device_id from auth context
		data.DeviceID = deviceID

		if data.Type == "" {
			continue // Skip invalid readings
		}

		if err := validate.TelemetryType(data.Type); err != nil {
			continue // Skip invalid types
		}

		if data.Timestamp.IsZero() {
			data.Timestamp = now
		}
		data.CreatedAt = now

		id, err := h.telemetryStore.Save(ctx, data)
		if err != nil {
			h.logger.Error("failed to store telemetry in batch",
				"request_id", reqID,
				"error", err,
				"device_id", deviceID,
				"index", i,
			)
			continue
		}
		data.ID = id
		saved++

		// Publish each reading
		if err := h.publishTelemetry(ctx, data); err != nil {
			h.logger.Error("failed to publish telemetry event",
				"request_id", reqID,
				"error", err,
			)
		}
	}

	h.jsonResponse(w, map[string]interface{}{
		"saved":    saved,
		"received": len(readings),
	}, http.StatusCreated)
}

// GetTelemetry handles GET /telemetry - retrieves telemetry for authenticated device
func (h *Handlers) GetTelemetry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	// Get device ID from auth context (device can only see their own data)
	deviceID := GetDeviceIDFromContext(ctx)
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse and validate limit parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 {
			h.jsonError(w, "invalid limit: must be a positive integer", http.StatusBadRequest)
			return
		}
		if l > 1000 {
			h.jsonError(w, "invalid limit: maximum is 1000", http.StatusBadRequest)
			return
		}
		limit = l
	}

	results, err := h.telemetryStore.GetByDeviceID(ctx, deviceID, limit)
	if err != nil {
		h.logger.Error("failed to retrieve telemetry",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
		)
		h.jsonError(w, "failed to retrieve telemetry", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"data":  results,
		"count": len(results),
	}, http.StatusOK)
}

// publishTelemetry publishes telemetry data to event stream
func (h *Handlers) publishTelemetry(ctx context.Context, data *TelemetryData) error {
	if h.publisher == nil {
		return nil
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry: %w", err)
	}

	if err := h.publisher.Publish(ctx, TelemetryTopic, payload); err != nil {
		return fmt.Errorf("failed to publish to %s: %w", TelemetryTopic, err)
	}

	return nil
}
