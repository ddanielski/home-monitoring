package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/middleware"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/validate"
)

// GetDevice handles GET /devices/{id}
// Device can only retrieve their own info
func (h *Handlers) GetDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	// Get authenticated device ID from context
	authDeviceID := GetDeviceIDFromContext(ctx)
	if authDeviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	requestedID := r.PathValue("id")
	if requestedID == "" {
		h.jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}

	// Validate ID format
	if err := validate.UUID(requestedID); err != nil {
		h.jsonError(w, "invalid device_id format", http.StatusBadRequest)
		return
	}

	// Security: device can only see their own info
	if requestedID != authDeviceID {
		h.logger.Warn("unauthorized device access attempt",
			"request_id", reqID,
			"requesting_device", authDeviceID,
			"requested_device", requestedID,
		)
		h.jsonError(w, "device not found", http.StatusNotFound) // Don't reveal it exists
		return
	}

	device, err := h.deviceStore.GetByID(ctx, requestedID)
	if err != nil {
		h.logger.Error("failed to get device",
			"request_id", reqID,
			"error", err,
			"device_id", requestedID,
		)
		h.jsonError(w, "device not found", http.StatusNotFound)
		return
	}

	// SecretHash is already omitted from JSON via json:"-" tag
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}
