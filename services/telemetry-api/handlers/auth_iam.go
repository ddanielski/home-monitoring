package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminAuthConfig holds configuration for admin API key authentication
type AdminAuthConfig struct {
	// APIKey is the secret key for admin endpoints (loaded from Secret Manager)
	APIKey string
}

// AdminAuthMiddleware validates admin API keys
type AdminAuthMiddleware struct {
	config AdminAuthConfig
}

// NewAdminAuthMiddleware creates a new admin auth middleware
func NewAdminAuthMiddleware(config AdminAuthConfig) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{
		config: config,
	}
}

// RequireAdminKey middleware validates the admin API key
// Provisioners get the key via: gcloud secrets versions access latest --secret=admin-api-key
func (m *AdminAuthMiddleware) RequireAdminKey(h *Handlers, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.logger.Warn("admin auth failed: missing Authorization header", "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			h.logger.Warn("admin auth failed: invalid Authorization format", "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		providedKey := parts[1]

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(m.config.APIKey)) != 1 {
			h.logger.Warn("admin auth failed: invalid API key", "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		h.logger.Info("admin authenticated", "path", r.URL.Path)
		next(w, r)
	}
}
