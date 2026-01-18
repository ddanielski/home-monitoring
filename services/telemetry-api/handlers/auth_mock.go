package handlers

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MockAuthService is a mock implementation of AuthService for testing
type MockAuthService struct {
	mu             sync.Mutex
	tokens         map[string]mockToken // token -> device info
	CreateTokenErr error
	VerifyTokenErr error
}

type mockToken struct {
	deviceID  string
	claims    map[string]interface{}
	expiresAt time.Time
}

func NewMockAuthService() *MockAuthService {
	return &MockAuthService{
		tokens: make(map[string]mockToken),
	}
}

func (m *MockAuthService) CreateCustomToken(ctx context.Context, deviceID string, claims map[string]interface{}) (string, error) {
	if m.CreateTokenErr != nil {
		return "", m.CreateTokenErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate a simple token
	token := "mock-token-" + deviceID + "-" + time.Now().Format("20060102150405")
	m.tokens[token] = mockToken{
		deviceID:  deviceID,
		claims:    claims,
		expiresAt: time.Now().Add(1 * time.Hour),
	}

	return token, nil
}

func (m *MockAuthService) VerifyToken(ctx context.Context, token string) (string, map[string]interface{}, error) {
	if m.VerifyTokenErr != nil {
		return "", nil, m.VerifyTokenErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tokens[token]
	if !ok {
		return "", nil, errors.New("invalid token")
	}

	if time.Now().After(t.expiresAt) {
		return "", nil, errors.New("token expired")
	}

	return t.deviceID, t.claims, nil
}

// AddToken adds a token directly (for testing)
func (m *MockAuthService) AddToken(token, deviceID string, claims map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokens[token] = mockToken{
		deviceID:  deviceID,
		claims:    claims,
		expiresAt: time.Now().Add(1 * time.Hour),
	}
}
