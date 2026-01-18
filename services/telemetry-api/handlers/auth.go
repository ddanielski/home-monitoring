package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/mac"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/middleware"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/validate"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles device authentication via Firebase
type AuthService interface {
	// CreateCustomToken creates a Firebase custom token for a device
	CreateCustomToken(ctx context.Context, deviceID string, claims map[string]interface{}) (string, error)
	// VerifyToken verifies a Firebase ID token and returns the device ID
	VerifyToken(ctx context.Context, token string) (deviceID string, claims map[string]interface{}, err error)
}

// DeviceAuthRequest is the request body for device authentication
type DeviceAuthRequest struct {
	DeviceID string `json:"device_id"`
	Secret   string `json:"secret"`
}

// DeviceAuthResponse is the response body for device authentication
type DeviceAuthResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"` // seconds
}

// AuthDevice handles POST /auth/device
// Device sends device_id + secret, receives Firebase custom token
func (h *Handlers) AuthDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	var req DeviceAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.DeviceID == "" {
		h.jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}

	// Validate device_id format
	if err := validate.UUID(req.DeviceID); err != nil {
		h.jsonError(w, "invalid device_id format", http.StatusBadRequest)
		return
	}

	if req.Secret == "" {
		h.jsonError(w, "secret is required", http.StatusBadRequest)
		return
	}

	// Validate secret length (64 hex chars = 32 bytes)
	if len(req.Secret) != 64 {
		h.jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Lookup device
	device, err := h.deviceStore.GetByID(ctx, req.DeviceID)
	if err != nil {
		// Don't reveal if device exists or not
		h.logger.Debug("device lookup failed",
			"request_id", reqID,
			"device_id", req.DeviceID,
			"error", err,
		)
		h.jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check if device is revoked
	if device.Revoked {
		h.logger.Info("auth attempt on revoked device",
			"request_id", reqID,
			"device_id", req.DeviceID,
		)
		h.jsonError(w, "device revoked", http.StatusForbidden)
		return
	}

	// Verify secret (constant-time comparison via bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(device.SecretHash), []byte(req.Secret)); err != nil {
		h.logger.Warn("invalid credentials attempt",
			"request_id", reqID,
			"device_id", req.DeviceID,
		)
		h.jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create Firebase custom token with device claims
	claims := map[string]interface{}{
		"app_name":    device.AppName,
		"app_version": device.AppVersion,
	}

	token, err := h.authService.CreateCustomToken(ctx, req.DeviceID, claims)
	if err != nil {
		h.logger.Error("failed to create custom token",
			"request_id", reqID,
			"error", err,
			"device_id", req.DeviceID,
		)
		h.jsonError(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Update last seen
	if err := h.deviceStore.UpdateLastSeen(ctx, req.DeviceID); err != nil {
		h.logger.Warn("failed to update last_seen",
			"request_id", reqID,
			"error", err,
			"device_id", req.DeviceID,
		)
	}

	h.logger.Info("device authenticated",
		"request_id", reqID,
		"device_id", req.DeviceID,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DeviceAuthResponse{
		Token:     token,
		ExpiresIn: 3600, // Firebase custom tokens are valid for 1 hour
	})
}

// AuthMiddleware verifies Firebase tokens on protected endpoints
func (h *Handlers) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		reqID := middleware.GetRequestID(ctx)

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.jsonError(w, "authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			h.jsonError(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}
		token := parts[1]

		// Basic token format validation (Firebase tokens are JWTs)
		if len(token) < 100 || len(token) > 2000 {
			h.jsonError(w, "invalid token format", http.StatusUnauthorized)
			return
		}

		// Verify token
		deviceID, claims, err := h.authService.VerifyToken(ctx, token)
		if err != nil {
			h.logger.Warn("token verification failed",
				"request_id", reqID,
				"error", err,
			)
			h.jsonError(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// Add device info to request context
		ctx = context.WithValue(ctx, deviceIDKey, deviceID)
		ctx = context.WithValue(ctx, deviceClaimsKey, claims)

		next(w, r.WithContext(ctx))
	}
}

// Context keys for device info
type contextKey string

const (
	deviceIDKey     contextKey = "device_id"
	deviceClaimsKey contextKey = "device_claims"
)

// GetDeviceIDFromContext extracts device ID from request context
func GetDeviceIDFromContext(ctx context.Context) string {
	if v := ctx.Value(deviceIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetDeviceClaimsFromContext extracts device claims from request context
func GetDeviceClaimsFromContext(ctx context.Context) map[string]interface{} {
	if v := ctx.Value(deviceClaimsKey); v != nil {
		return v.(map[string]interface{})
	}
	return nil
}

// GenerateDeviceSecret generates a cryptographically secure device secret
func GenerateDeviceSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// HashDeviceSecret hashes a device secret for storage
func HashDeviceSecret(secret string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash secret: %w", err)
	}
	return string(hash), nil
}

// RefreshToken handles POST /auth/refresh
// Allows device to get a new token before current one expires
func (h *Handlers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	// Get device ID from current valid token
	deviceID := GetDeviceIDFromContext(ctx)
	if deviceID == "" {
		h.jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Lookup device to get current info
	device, err := h.deviceStore.GetByID(ctx, deviceID)
	if err != nil {
		h.logger.Error("device not found during refresh",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
		)
		h.jsonError(w, "device not found", http.StatusUnauthorized)
		return
	}

	if device.Revoked {
		h.logger.Info("refresh attempt on revoked device",
			"request_id", reqID,
			"device_id", deviceID,
		)
		h.jsonError(w, "device revoked", http.StatusForbidden)
		return
	}

	// Create new token
	claims := map[string]interface{}{
		"app_name":    device.AppName,
		"app_version": device.AppVersion,
	}

	token, err := h.authService.CreateCustomToken(ctx, deviceID, claims)
	if err != nil {
		h.logger.Error("failed to create refresh token",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
		)
		h.jsonError(w, "token refresh failed", http.StatusInternalServerError)
		return
	}

	// Update last seen
	_ = h.deviceStore.UpdateLastSeen(ctx, deviceID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DeviceAuthResponse{
		Token:     token,
		ExpiresIn: 3600,
	})
}

// RevokeDevice handles POST /admin/devices/{id}/revoke
// Admin endpoint to revoke a device's access
func (h *Handlers) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	deviceID := r.PathValue("id")
	if deviceID == "" {
		h.jsonError(w, "device_id required", http.StatusBadRequest)
		return
	}

	// Validate device_id format
	if err := validate.UUID(deviceID); err != nil {
		h.jsonError(w, "invalid device_id format", http.StatusBadRequest)
		return
	}

	if err := h.deviceStore.Revoke(ctx, deviceID); err != nil {
		h.logger.Error("failed to revoke device",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
		)
		h.jsonError(w, "failed to revoke device", http.StatusInternalServerError)
		return
	}

	h.logger.Info("device revoked",
		"request_id", reqID,
		"device_id", deviceID,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "device revoked",
		"device_id": deviceID,
	})
}

// NormalizeMAC normalizes a MAC address to lowercase with no separators.
// Deprecated: Use pkg/mac.Normalize instead.
func NormalizeMAC(addr string) string {
	return mac.Normalize(addr)
}

// ValidateMAC checks if a string is a valid MAC address.
// Deprecated: Use pkg/mac.IsValid instead.
func ValidateMAC(addr string) bool {
	return mac.IsValid(addr)
}

// ProvisionDevice handles POST /admin/devices/provision
// Admin endpoint to provision a new device with credentials
type ProvisionDeviceRequest struct {
	MACAddress string `json:"mac_address"` // Device MAC address (any format)
	AppName    string `json:"app_name"`
	AppVersion string `json:"app_version"`
}

type ProvisionDeviceResponse struct {
	DeviceID   string `json:"device_id"`   // UUID to store on device
	MACAddress string `json:"mac_address"` // Normalized MAC (for confirmation)
	Secret     string `json:"secret"`      // Only returned once during provisioning
	Message    string `json:"message"`
}

func (h *Handlers) ProvisionDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := middleware.GetRequestID(ctx)

	var req ProvisionDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.MACAddress == "" {
		h.jsonError(w, "mac_address is required", http.StatusBadRequest)
		return
	}
	if !ValidateMAC(req.MACAddress) {
		h.jsonError(w, "invalid mac_address format", http.StatusBadRequest)
		return
	}
	if req.AppName == "" {
		h.jsonError(w, "app_name is required", http.StatusBadRequest)
		return
	}

	// Validate app_name format
	if err := validate.Identifier(req.AppName); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid app_name: %v", err), http.StatusBadRequest)
		return
	}

	if req.AppVersion == "" {
		h.jsonError(w, "app_version is required", http.StatusBadRequest)
		return
	}

	// Validate app_version format
	if err := validate.Version(req.AppVersion); err != nil {
		h.jsonError(w, fmt.Sprintf("invalid app_version: %v", err), http.StatusBadRequest)
		return
	}

	// Normalize MAC address
	normalizedMAC := NormalizeMAC(req.MACAddress)

	// Check if MAC is already registered
	existingDevice, err := h.deviceStore.GetByMAC(ctx, normalizedMAC)
	if err == nil && existingDevice != nil {
		h.logger.Warn("duplicate MAC registration attempt",
			"request_id", reqID,
			"mac_address", normalizedMAC,
		)
		h.jsonError(w, "mac_address already registered", http.StatusConflict)
		return
	}

	// Generate UUID for device identity
	deviceID := uuid.New().String()

	// Generate device secret
	secret, err := GenerateDeviceSecret()
	if err != nil {
		h.logger.Error("failed to generate secret",
			"request_id", reqID,
			"error", err,
		)
		h.jsonError(w, "failed to provision device", http.StatusInternalServerError)
		return
	}

	// Hash secret for storage
	secretHash, err := HashDeviceSecret(secret)
	if err != nil {
		h.logger.Error("failed to hash secret",
			"request_id", reqID,
			"error", err,
		)
		h.jsonError(w, "failed to provision device", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	device := &Device{
		DeviceID:     deviceID,
		MACAddress:   normalizedMAC,
		AppName:      req.AppName,
		AppVersion:   req.AppVersion,
		SecretHash:   secretHash,
		Revoked:      false,
		RegisteredAt: now,
		LastSeen:     now,
	}

	if err := h.deviceStore.Register(ctx, device); err != nil {
		h.logger.Error("failed to register device",
			"request_id", reqID,
			"error", err,
			"device_id", deviceID,
			"mac", normalizedMAC,
		)
		h.jsonError(w, "failed to provision device", http.StatusInternalServerError)
		return
	}

	h.logger.Info("device provisioned",
		"request_id", reqID,
		"device_id", deviceID,
		"mac_address", normalizedMAC,
		"app_name", req.AppName,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ProvisionDeviceResponse{
		DeviceID:   deviceID,
		MACAddress: normalizedMAC,
		Secret:     secret, // Only returned once!
		Message:    "device provisioned - save device_id and secret, they won't be shown again",
	})
}
