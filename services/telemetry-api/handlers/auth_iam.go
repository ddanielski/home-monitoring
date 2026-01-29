package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminAuthConfig holds configuration for admin API key authentication
type AdminAuthConfig struct {
	APIKey              string
	GithubActionsAPIKey string
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

func (m *AdminAuthMiddleware) validateAPIKey(providedKey string, allowedKeys []string) bool {
	for _, allowedKey := range allowedKeys {
		if allowedKey != "" && subtle.ConstantTimeCompare([]byte(providedKey), []byte(allowedKey)) == 1 {
			return true
		}
	}
	return false
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

		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(m.config.APIKey)) != 1 {
			h.logger.Warn("admin auth failed: invalid API key", "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		h.logger.Info("admin authenticated", "path", r.URL.Path)
		next(w, r)
	}
}

// RequireGithubActionsKey middleware validates either GitHub Actions API key OR admin key
// GitHub Actions CI/CD uses: gcloud secrets versions access latest --secret=github-actions-api-key
// Admin key is also accepted for manual operations (backward compatibility)
func (m *AdminAuthMiddleware) RequireGithubActionsKey(h *Handlers, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.logger.Warn("github actions auth failed: missing Authorization header", "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			h.logger.Warn("github actions auth failed: invalid Authorization format", "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		providedKey := parts[1]

		// Allow either GitHub Actions key (for CI/CD) or admin key
		allowedKeys := []string{m.config.GithubActionsAPIKey, m.config.APIKey}
		if !m.validateAPIKey(providedKey, allowedKeys) {
			h.logger.Warn("github actions auth failed: invalid API key", "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		h.logger.Info("github actions authenticated", "path", r.URL.Path)
		next(w, r)
	}
}
