package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/handlers"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/middleware"
)

// Body size limits
const (
	DefaultBodyLimit   = 64 * 1024
	TelemetryBodyLimit = 1 * 1024 * 1024
	SchemaBodyLimit    = 256 * 1024
)

// Rate limiting configuration
const (
	AuthRateLimitWindow = 1 * time.Minute
	AuthRateLimitMax    = 10
)

func main() {
	// Structured logging (industry standard)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Configuration from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		slog.Error("GCP_PROJECT environment variable is required")
		os.Exit(1)
	}

	// Service URL for IAM token validation
	serviceURL := os.Getenv("SERVICE_URL")

	// CORS origins for admin endpoints (comma-separated)
	adminOrigins := parseCommaSeparated(os.Getenv("ADMIN_CORS_ORIGINS"))

	// Provisioner emails (comma-separated list of emails allowed to provision devices)
	provisionerEmails := handlers.ParseProvisionerEmails(os.Getenv("PROVISIONER_EMAILS"))

	// Initialize handlers with dependencies
	ctx := context.Background()
	h, err := handlers.New(ctx, handlers.Config{
		ProjectID:         projectID,
		ServiceURL:        serviceURL,
		ProvisionerEmails: provisionerEmails,
	})
	if err != nil {
		slog.Error("failed to initialize handlers", "error", err)
		os.Exit(1)
	}
	defer h.Close()

	// IAM middleware for admin endpoints
	iamAuth := handlers.NewIAMAuthMiddleware(handlers.IAMAuthConfig{
		Audience:          serviceURL,
		ProvisionerEmails: provisionerEmails,
	})

	// Rate limiter for auth endpoint (protect against brute force)
	authRateLimiter := middleware.NewRateLimiter(AuthRateLimitWindow, AuthRateLimitMax)
	defer authRateLimiter.Close()

	// Setup routes
	mux := http.NewServeMux()

	// =========================================================================
	// Public endpoints (no auth required)
	// =========================================================================
	mux.HandleFunc("GET /health", h.Health) // Cloud Run health checks

	// Auth endpoint with rate limiting
	mux.Handle("POST /auth/device",
		middleware.RateLimitMiddleware(authRateLimiter, nil)(
			http.HandlerFunc(h.AuthDevice),
		),
	)

	// =========================================================================
	// Device endpoints (require Firebase device token)
	// =========================================================================
	mux.HandleFunc("POST /auth/refresh", h.AuthMiddleware(h.RefreshToken))
	mux.HandleFunc("POST /telemetry", h.AuthMiddleware(h.HandleTelemetry))
	mux.HandleFunc("POST /telemetry/batch", h.AuthMiddleware(h.HandleTelemetryBatch))
	mux.HandleFunc("POST /telemetry/proto", h.AuthMiddleware(h.HandleTelemetryProto))
	mux.HandleFunc("GET /telemetry", h.AuthMiddleware(h.GetTelemetry))
	mux.HandleFunc("GET /commands", h.AuthMiddleware(h.GetCommands))
	mux.HandleFunc("POST /commands/{id}/ack", h.AuthMiddleware(h.AckCommand)) // Device acknowledges command
	mux.HandleFunc("GET /devices/{id}", h.AuthMiddleware(h.GetDevice))

	// =========================================================================
	// Admin endpoints (require GCP IAM identity token + provisioner email)
	// =========================================================================
	mux.HandleFunc("POST /admin/devices/provision", iamAuth.RequireProvisioner(h, h.ProvisionDevice))
	mux.HandleFunc("POST /admin/devices/{id}/revoke", iamAuth.RequireProvisioner(h, h.RevokeDevice))
	mux.HandleFunc("POST /admin/commands", iamAuth.RequireProvisioner(h, h.CreateCommand))
	mux.HandleFunc("DELETE /admin/commands/{id}", iamAuth.RequireProvisioner(h, h.DeleteCommand))
	mux.HandleFunc("POST /admin/schemas/{app}/{version}", iamAuth.RequireProvisioner(h, h.UploadSchema))
	mux.HandleFunc("GET /admin/schemas/{app}/{version}", iamAuth.RequireProvisioner(h, h.GetSchema))

	// Apply middleware chain (order matters: first is outermost)
	handler := middleware.Chain(mux,
		// 1. Request ID first (available to all other middleware)
		middleware.RequestID,
		// 2. Security headers
		middleware.SecurityHeaders,
		// 3. Panic recovery
		middleware.Recovery(logger),
		// 4. CORS handling
		middleware.CORS(middleware.CORSConfig{
			AllowedAdminOrigins:  adminOrigins,
			AllowAllForDeviceAPI: true,
		}),
		// 5. Body size limits
		middleware.BodyLimit(middleware.BodyLimitConfig{
			DefaultLimit: DefaultBodyLimit,
			PathLimits: map[string]int64{
				"/telemetry":     TelemetryBodyLimit,
				"/admin/schemas": SchemaBodyLimit,
			},
		}),
		// 6. Request logging (last to capture accurate timing)
		middleware.Logging(logger),
	)

	// Server configuration
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second, // Protect against Slowloris attacks
		MaxHeaderBytes:    1 << 16,         // 64KB max headers
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	slog.Info("starting server",
		"port", port,
		"auth_rate_limit", AuthRateLimitMax,
		"auth_rate_window", AuthRateLimitWindow.String(),
	)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

// parseCommaSeparated splits a comma-separated string into a slice of trimmed strings.
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
