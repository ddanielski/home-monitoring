package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/middleware"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/validate"
)

// UpdateDeviceInfoRequest is the request body for updating device info
type UpdateDeviceInfoRequest struct {
	AppName    string `json:"app_name"`
	AppVersion string `json:"app_version"`
}

// UpdateDeviceInfoResponse is the response body after updating device info
type UpdateDeviceInfoResponse struct {
	DeviceID   string `json:"device_id"`
	AppName    string `json:"app_name"`
	AppVersion string `json:"app_version"`
	Message    string `json:"message"`
}

// UpdateDeviceInfo handles PUT /devices/info
// Device updates its own firmware info (app_name, app_version)
func (h *Handlers) UpdateDeviceInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	// Get authenticated device ID from context
	deviceID := GetDeviceIDFromContext(ctx)
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req UpdateDeviceInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate app_name
	if req.AppName == "" {
		h.jsonError(w, "app_name is required", http.StatusBadRequest)
		return
	}
	if err := validate.Identifier(req.AppName); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid app_name: %v", err), http.StatusBadRequest)
		return
	}

	// Validate app_version
	if req.AppVersion == "" {
		h.jsonError(w, "app_version is required", http.StatusBadRequest)
		return
	}
	if err := validate.Version(req.AppVersion); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid app_version: %v", err), http.StatusBadRequest)
		return
	}

	// Update device info
	if err := h.deviceStore.UpdateAppInfo(ctx, deviceID, req.AppName, req.AppVersion); err != nil {
		h.logger.Error("failed to update device info",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
		)
		h.jsonError(w, "failed to update device info", http.StatusInternalServerError)
		return
	}

	h.logger.Info("device info updated",
		"request_id", reqID,
		"device_id", deviceID,
		"app_name", req.AppName,
		"app_version", req.AppVersion,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UpdateDeviceInfoResponse{
		DeviceID:   deviceID,
		AppName:    req.AppName,
		AppVersion: req.AppVersion,
		Message:    "device info updated",
	})
}

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
