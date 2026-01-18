package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

// IAMAuthConfig holds configuration for IAM-based authentication
type IAMAuthConfig struct {
	// Audience is the Cloud Run service URL (used to validate tokens)
	Audience string
	// ProvisionerEmails is a list of emails allowed to provision devices
	ProvisionerEmails []string
}

// IAMAuthMiddleware validates GCP identity tokens and checks authorization
type IAMAuthMiddleware struct {
	config   IAMAuthConfig
	validate func(ctx context.Context, token string, audience string) (*idtoken.Payload, error)
}

// NewIAMAuthMiddleware creates a new IAM auth middleware
func NewIAMAuthMiddleware(config IAMAuthConfig) *IAMAuthMiddleware {
	return &IAMAuthMiddleware{
		config:   config,
		validate: idtoken.Validate,
	}
}

// AuthenticatedUser represents a validated GCP identity
type AuthenticatedUser struct {
	Email  string
	UserID string // GCP subject ID
}

// Context key for authenticated user
type iamUserKey struct{}

// GetAuthenticatedUser extracts the authenticated user from context
func GetAuthenticatedUser(ctx context.Context) *AuthenticatedUser {
	if user, ok := ctx.Value(iamUserKey{}).(*AuthenticatedUser); ok {
		return user
	}
	return nil
}

// RequireIAMAuth middleware validates GCP identity tokens
// Use this for endpoints that any authenticated GCP user can access
func (m *IAMAuthMiddleware) RequireIAMAuth(h *Handlers, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := m.validateToken(r)
		if err != nil {
			h.logger.Warn("IAM auth failed", "error", err, "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), iamUserKey{}, user)
		next(w, r.WithContext(ctx))
	}
}

// RequireProvisioner middleware validates that the user is an authorized provisioner
// Use this for /admin/devices/provision endpoint only
func (m *IAMAuthMiddleware) RequireProvisioner(h *Handlers, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := m.validateToken(r)
		if err != nil {
			h.logger.Warn("IAM auth failed", "error", err, "path", r.URL.Path)
			h.jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is an authorized provisioner
		if !m.isProvisioner(user.Email) {
			h.logger.Warn("provisioner access denied",
				"email", user.Email,
				"path", r.URL.Path,
			)
			h.jsonError(w, "forbidden: not authorized to provision devices", http.StatusForbidden)
			return
		}

		h.logger.Info("provisioner authenticated",
			"email", user.Email,
			"path", r.URL.Path,
		)

		ctx := context.WithValue(r.Context(), iamUserKey{}, user)
		next(w, r.WithContext(ctx))
	}
}

func (m *IAMAuthMiddleware) validateToken(r *http.Request) (*AuthenticatedUser, error) {
	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, fmt.Errorf("invalid Authorization header format")
	}
	token := parts[1]

	// Determine audience for token validation
	// If SERVICE_URL is configured, use it; otherwise derive from request Host header
	// The Host header is secure on Cloud Run (validated at TLS termination)
	audience := m.config.Audience
	if audience == "" {
		audience = "https://" + r.Host
	}

	// Validate the identity token
	payload, err := m.validate(r.Context(), token, audience)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Extract email from token
	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return nil, fmt.Errorf("token missing email claim")
	}

	return &AuthenticatedUser{
		Email:  email,
		UserID: payload.Subject,
	}, nil
}

func (m *IAMAuthMiddleware) isProvisioner(email string) bool {
	email = strings.ToLower(email)
	for _, allowed := range m.config.ProvisionerEmails {
		if strings.ToLower(allowed) == email {
			return true
		}
	}
	return false
}

// ParseProvisionerEmails parses a comma-separated list of emails
func ParseProvisionerEmails(envValue string) []string {
	if envValue == "" {
		return nil
	}
	emails := strings.Split(envValue, ",")
	result := make([]string, 0, len(emails))
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e != "" {
			result = append(result, e)
		}
	}
	return result
}
