// Package middleware provides HTTP middleware for security, observability, and request handling.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Request ID Middleware
// =============================================================================

type requestIDKey struct{}

// RequestID adds a unique request ID to each request for tracing.
// If X-Request-ID header is present, it uses that; otherwise generates a new UUID.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey{}, reqID)
		w.Header().Set("X-Request-ID", reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// =============================================================================
// Security Headers Middleware
// =============================================================================

// SecurityHeaders adds security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// XSS protection (legacy browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Content Security Policy - strict for API
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// Referrer policy
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Cache control for API responses (no caching of sensitive data)
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Pragma", "no-cache")

		next.ServeHTTP(w, r)
	})
}

// =============================================================================
// CORS Middleware with Admin Restrictions
// =============================================================================

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	// AllowedOrigins for admin endpoints (empty means no CORS for admin)
	AllowedAdminOrigins []string
	// AllowAllForDeviceAPI allows * origin for device endpoints
	AllowAllForDeviceAPI bool
}

// CORS handles Cross-Origin Resource Sharing with different rules for admin vs device endpoints.
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	adminOrigins := make(map[string]bool)
	for _, origin := range config.AllowedAdminOrigins {
		adminOrigins[strings.ToLower(origin)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			isAdminEndpoint := strings.HasPrefix(r.URL.Path, "/admin")

			// Determine allowed origin based on endpoint type
			var allowedOrigin string
			if isAdminEndpoint {
				// Admin endpoints: only allow specific origins
				if origin != "" && adminOrigins[strings.ToLower(origin)] {
					allowedOrigin = origin
				}
				// If no match, don't set CORS headers (browser will block)
			} else {
				// Device/public endpoints: allow all origins (devices don't use browsers)
				if config.AllowAllForDeviceAPI {
					allowedOrigin = "*"
				} else if origin != "" {
					allowedOrigin = origin
				}
			}

			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
			}

			// Handle preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// =============================================================================
// Body Size Limit Middleware
// =============================================================================

// BodyLimit limits the size of request bodies.
// Different limits can be applied to different paths.
type BodyLimitConfig struct {
	DefaultLimit int64            // Default limit for all paths
	PathLimits   map[string]int64 // Path prefix -> limit overrides
}

// BodyLimit returns middleware that limits request body size.
func BodyLimit(config BodyLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Determine limit for this path
			limit := config.DefaultLimit
			for prefix, pathLimit := range config.PathLimits {
				if strings.HasPrefix(r.URL.Path, prefix) {
					limit = pathLimit
					break
				}
			}

			// Apply limit
			if limit > 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// =============================================================================
// Rate Limiter
// =============================================================================

// RateLimiter implements a sliding window rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	window   time.Duration
	limit    int
	cleanupC chan struct{}
}

// NewRateLimiter creates a new rate limiter.
// window is the time window for counting requests.
// limit is the maximum number of requests allowed in the window.
func NewRateLimiter(window time.Duration, limit int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		window:   window,
		limit:    limit,
		cleanupC: make(chan struct{}),
	}

	// Background cleanup of old entries
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given key should be allowed.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get existing requests for this key
	existing := rl.requests[key]

	// Filter to only requests within the window
	var valid []time.Time
	for _, t := range existing {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Check if limit exceeded
	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		return false
	}

	// Add new request
	valid = append(valid, now)
	rl.requests[key] = valid

	return true
}

// cleanup periodically removes old entries from the rate limiter.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for key, times := range rl.requests {
				var valid []time.Time
				for _, t := range times {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.requests, key)
				} else {
					rl.requests[key] = valid
				}
			}
			rl.mu.Unlock()
		case <-rl.cleanupC:
			return
		}
	}
}

// Close stops the rate limiter's background cleanup.
func (rl *RateLimiter) Close() {
	close(rl.cleanupC)
}

// RateLimitMiddleware creates middleware that rate limits requests by IP.
// keyFunc extracts the rate limit key from the request (default: RemoteAddr).
func RateLimitMiddleware(rl *RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	if keyFunc == nil {
		keyFunc = func(r *http.Request) string {
			// Use X-Forwarded-For if behind a proxy, otherwise RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				// Take the first IP (client IP)
				if idx := strings.Index(xff, ","); idx != -1 {
					return strings.TrimSpace(xff[:idx])
				}
				return strings.TrimSpace(xff)
			}
			// Remove port from RemoteAddr
			addr := r.RemoteAddr
			if idx := strings.LastIndex(addr, ":"); idx != -1 {
				return addr[:idx]
			}
			return addr
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)

			if !rl.Allow(key) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// =============================================================================
// Logging Middleware
// =============================================================================

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Logging logs HTTP requests with structured logging.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			// Get request ID from context
			reqID := GetRequestID(r.Context())

			logger.Info("request",
				"request_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", wrapped.written,
				"user_agent", r.UserAgent(),
			)
		})
	}
}

// Recovery recovers from panics and logs them.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					reqID := GetRequestID(r.Context())
					logger.Error("panic recovered",
						"request_id", reqID,
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
					)
					http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// =============================================================================
// Middleware Chain Helper
// =============================================================================

// Chain applies middlewares in order (first middleware is outermost).
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply in reverse order so first middleware is outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
