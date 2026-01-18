package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := GetRequestID(r.Context())
		if reqID == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Test without X-Request-ID header (should generate one)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header in response")
	}

	// Test with X-Request-ID header (should use it)
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Request-ID", "custom-id-123")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Header().Get("X-Request-ID") != "custom-id-123" {
		t.Errorf("expected X-Request-ID 'custom-id-123', got %q", w2.Header().Get("X-Request-ID"))
	}
}

func TestGetRequestID_NoContext(t *testing.T) {
	ctx := context.Background()
	if id := GetRequestID(ctx); id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store, no-cache, must-revalidate, private",
	}

	for header, expected := range expectedHeaders {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("expected %s: %q, got %q", header, expected, got)
		}
	}
}

func TestCORS_DeviceEndpoint(t *testing.T) {
	handler := CORS(CORSConfig{
		AllowedAdminOrigins:  []string{"https://admin.example.com"},
		AllowAllForDeviceAPI: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Device endpoint should allow all origins
	req := httptest.NewRequest(http.MethodGet, "/telemetry", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", got)
	}
}

func TestCORS_AdminEndpoint(t *testing.T) {
	handler := CORS(CORSConfig{
		AllowedAdminOrigins:  []string{"https://admin.example.com"},
		AllowAllForDeviceAPI: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Admin endpoint with allowed origin
	req := httptest.NewRequest(http.MethodGet, "/admin/devices", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Errorf("expected Access-Control-Allow-Origin: https://admin.example.com, got %q", got)
	}

	// Admin endpoint with disallowed origin
	req2 := httptest.NewRequest(http.MethodGet, "/admin/devices", nil)
	req2.Header.Set("Origin", "https://evil.com")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin, got %q", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	handler := CORS(CORSConfig{
		AllowAllForDeviceAPI: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/telemetry", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d for OPTIONS, got %d", http.StatusNoContent, w.Code)
	}
}

func TestBodyLimit(t *testing.T) {
	handler := BodyLimit(BodyLimitConfig{
		DefaultLimit: 100,
		PathLimits: map[string]int64{
			"/large": 1000,
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to read body
		buf := make([]byte, 200)
		_, err := r.Body.Read(buf)
		if err != nil && err.Error() == "http: request body too large" {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Small body should succeed
	req := httptest.NewRequest(http.MethodPost, "/api", httptest.NewRequest(http.MethodPost, "/", nil).Body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Note: With empty body, this should succeed
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(100*time.Millisecond, 2)
	defer rl.Close()

	// First two requests should be allowed
	if !rl.Allow("test-key") {
		t.Error("expected first request to be allowed")
	}
	if !rl.Allow("test-key") {
		t.Error("expected second request to be allowed")
	}

	// Third request should be denied
	if rl.Allow("test-key") {
		t.Error("expected third request to be denied")
	}

	// Different key should be allowed
	if !rl.Allow("other-key") {
		t.Error("expected request with different key to be allowed")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow("test-key") {
		t.Error("expected request after window expiry to be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 1)
	defer rl.Close()

	handler := RateLimitMiddleware(rl, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Second request should be rate limited
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w2.Code)
	}

	if w2.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}
}

func TestRateLimitMiddleware_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 1)
	defer rl.Close()

	handler := RateLimitMiddleware(rl, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with X-Forwarded-For
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Same client IP should be rate limited
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w2.Code)
	}
}

func TestChain(t *testing.T) {
	var order []int

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 1)
			next.ServeHTTP(w, r)
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 2)
			next.ServeHTTP(w, r)
		})
	}

	handler := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, 3)
		w.WriteHeader(http.StatusOK)
	}), m1, m2)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// m1 should be outermost, then m2, then handler
	expected := []int{1, 2, 3}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d] = %d, got %d", i, v, order[i])
		}
	}
}
