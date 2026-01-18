package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/middleware"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/validate"
)

// GetCommands handles GET /commands - retrieves pending commands for authenticated device
func (h *Handlers) GetCommands(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	// Get device ID from auth context (device can only see their own commands)
	deviceID := GetDeviceIDFromContext(ctx)
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}

	// Validate status to prevent injection
	validStatuses := map[string]bool{
		"pending":      true,
		"acknowledged": true,
		"completed":    true,
		"failed":       true,
	}
	if !validStatuses[status] {
		h.jsonError(w, "invalid status: must be one of pending, acknowledged, completed, failed", http.StatusBadRequest)
		return
	}

	commands, err := h.commandStore.GetByDeviceID(ctx, deviceID, status)
	if err != nil {
		h.logger.Error("failed to retrieve commands",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
		)
		h.jsonError(w, "failed to retrieve commands", http.StatusInternalServerError)
		return
	}

	// Filter expired commands
	var validCommands []Command
	now := time.Now()
	for _, cmd := range commands {
		if cmd.ExpiresAt == nil || cmd.ExpiresAt.After(now) {
			validCommands = append(validCommands, cmd)
		}
	}

	h.jsonResponse(w, map[string]interface{}{
		"data":  validCommands,
		"count": len(validCommands),
	}, http.StatusOK)
}

// CreateCommand handles POST /commands - creates a new command (admin only)
func (h *Handlers) CreateCommand(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		h.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if cmd.DeviceID == "" {
		h.jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}

	// Validate device_id format (should be a UUID)
	if err := validate.UUID(cmd.DeviceID); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid device_id: %v", err), http.StatusBadRequest)
		return
	}

	if cmd.Type == "" {
		h.jsonError(w, "type is required", http.StatusBadRequest)
		return
	}

	// Validate command type
	if err := validate.CommandType(cmd.Type); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid command type: %v", err), http.StatusBadRequest)
		return
	}

	// Validate payload size if present (prevent oversized payloads)
	if cmd.Payload != nil {
		payloadBytes, err := json.Marshal(cmd.Payload)
		if err != nil {
			h.jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		const maxPayloadSize = 32 * 1024 // 32KB max payload
		if len(payloadBytes) > maxPayloadSize {
			h.jsonError(w, "payload too large (max 32KB)", http.StatusBadRequest)
			return
		}
	}

	// Set defaults
	now := time.Now().UTC()
	cmd.Status = "pending"
	cmd.CreatedAt = now
	cmd.UpdatedAt = now

	// Default expiration: 24 hours
	if cmd.ExpiresAt == nil {
		expires := now.Add(24 * time.Hour)
		cmd.ExpiresAt = &expires
	}

	id, err := h.commandStore.Save(ctx, &cmd)
	if err != nil {
		h.logger.Error("failed to store command",
			"request_id", reqID,
			"error", err,
			"device_id", cmd.DeviceID,
			"type", cmd.Type,
		)
		h.jsonError(w, "failed to create command", http.StatusInternalServerError)
		return
	}
	cmd.ID = id

	h.logger.Info("command created",
		"request_id", reqID,
		"command_id", id,
		"device_id", cmd.DeviceID,
		"type", cmd.Type,
	)

	h.jsonResponse(w, cmd, http.StatusCreated)
}

// AckCommand handles POST /commands/{id}/ack - device acknowledges a command
func (h *Handlers) AckCommand(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	// Get device ID from auth context
	deviceID := GetDeviceIDFromContext(ctx)
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	commandID := r.PathValue("id")
	if commandID == "" {
		h.jsonError(w, "command id is required", http.StatusBadRequest)
		return
	}

	// Validate command ID format
	if err := validate.UUID(commandID); err != nil {
		h.jsonError(w, "invalid command id", http.StatusBadRequest)
		return
	}

	// Check if command exists and belongs to this device
	cmd, err := h.commandStore.GetByID(ctx, commandID)
	if err != nil {
		h.logger.Debug("command not found for ack",
			"request_id", reqID,
			"command_id", commandID,
			"device_id", deviceID,
		)
		h.jsonError(w, "command not found", http.StatusNotFound)
		return
	}

	// Security: device can only acknowledge their own commands
	if cmd.DeviceID != deviceID {
		h.logger.Warn("unauthorized command ack attempt",
			"request_id", reqID,
			"command_id", commandID,
			"requesting_device", deviceID,
			"owning_device", cmd.DeviceID,
		)
		h.jsonError(w, "command not found", http.StatusNotFound) // Don't reveal it exists
		return
	}

	// Mark as acknowledged
	err = h.commandStore.Update(ctx, commandID, map[string]interface{}{
		"status":     "acknowledged",
		"updated_at": time.Now().UTC(),
	})
	if err != nil {
		h.logger.Error("failed to acknowledge command",
			"request_id", reqID,
			"error", err,
			"command_id", commandID,
		)
		h.jsonError(w, "failed to acknowledge command", http.StatusInternalServerError)
		return
	}

	cmd.Status = "acknowledged"
	h.jsonResponse(w, cmd, http.StatusOK)
}

// DeleteCommand handles DELETE /admin/commands/{id} - admin deletes a command
func (h *Handlers) DeleteCommand(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	commandID := r.PathValue("id")
	if commandID == "" {
		h.jsonError(w, "command id is required", http.StatusBadRequest)
		return
	}

	// Validate command ID format
	if err := validate.UUID(commandID); err != nil {
		h.jsonError(w, "invalid command id", http.StatusBadRequest)
		return
	}

	// Check if command exists
	_, err := h.commandStore.GetByID(ctx, commandID)
	if err != nil {
		h.jsonError(w, "command not found", http.StatusNotFound)
		return
	}

	// Delete the command
	if err := h.commandStore.Delete(ctx, commandID); err != nil {
		h.logger.Error("failed to delete command",
			"request_id", reqID,
			"error", err,
			"command_id", commandID,
		)
		h.jsonError(w, "failed to delete command", http.StatusInternalServerError)
		return
	}

	h.logger.Info("command deleted",
		"request_id", reqID,
		"command_id", commandID,
	)

	w.WriteHeader(http.StatusNoContent)
}

// Helper functions

func (h *Handlers) jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handlers) jsonError(w http.ResponseWriter, message string, status int) {
	h.jsonResponse(w, map[string]string{"error": message}, status)
}
