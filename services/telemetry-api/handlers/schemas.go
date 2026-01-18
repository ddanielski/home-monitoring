package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/middleware"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/validate"
)

// SchemaUploadRequest is the request body for schema upload
type SchemaUploadRequest struct {
	Measurements map[string]MeasurementMeta `json:"measurements"`
}

// UploadSchema handles POST /schemas/{app}/{version}
func (h *Handlers) UploadSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	appName := r.PathValue("app")
	version := r.PathValue("version")

	// Validate path parameters (prevent injection)
	if err := validate.Identifier(appName); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid app name: %v", err), http.StatusBadRequest)
		return
	}
	if err := validate.Version(version); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid version: %v", err), http.StatusBadRequest)
		return
	}

	var req SchemaUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Measurements) == 0 {
		h.jsonError(w, "measurements are required", http.StatusBadRequest)
		return
	}

	// Validate measurement names
	for name := range req.Measurements {
		if err := validate.Identifier(name); err != nil {
			h.jsonError(w, fmt.Sprintf("invalid measurement name %q: %v", name, err), http.StatusBadRequest)
			return
		}
	}

	schema := &MeasurementSchema{
		AppName:      appName,
		Version:      version,
		Measurements: req.Measurements,
		CreatedAt:    time.Now().UTC(),
	}

	if err := h.schemaStore.Save(ctx, appName, version, schema); err != nil {
		h.logger.Error("failed to save schema",
			"request_id", reqID,
			"error", err,
			"app", appName,
			"version", version,
		)
		h.jsonError(w, "failed to save schema", http.StatusInternalServerError)
		return
	}

	h.logger.Info("schema uploaded",
		"request_id", reqID,
		"app", appName,
		"version", version,
		"measurements", len(req.Measurements),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "schema uploaded successfully",
		"app":     appName,
		"version": version,
	})
}

// GetSchema handles GET /schemas/{app}/{version}
func (h *Handlers) GetSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	appName := r.PathValue("app")
	version := r.PathValue("version")

	// Validate path parameters (prevent injection)
	if err := validate.Identifier(appName); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid app name: %v", err), http.StatusBadRequest)
		return
	}
	if err := validate.Version(version); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid version: %v", err), http.StatusBadRequest)
		return
	}

	schema, err := h.schemaStore.Get(ctx, appName, version)
	if err != nil {
		h.logger.Debug("schema not found",
			"request_id", reqID,
			"app", appName,
			"version", version,
			"error", err,
		)
		h.jsonError(w, "schema not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
}
