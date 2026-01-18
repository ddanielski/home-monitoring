package handlers

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// FirebaseAuthService implements AuthService using Firebase Admin SDK
type FirebaseAuthService struct {
	client *auth.Client
}

// NewFirebaseAuthService creates a new Firebase auth service
func NewFirebaseAuthService(ctx context.Context, projectID string) (*FirebaseAuthService, error) {
	// Initialize Firebase app
	// Uses GOOGLE_APPLICATION_CREDENTIALS or default service account in GCP
	config := &firebase.Config{
		ProjectID: projectID,
	}

	app, err := firebase.NewApp(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize firebase app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get firebase auth client: %w", err)
	}

	return &FirebaseAuthService{client: client}, nil
}

// CreateCustomToken creates a Firebase custom token for a device
func (s *FirebaseAuthService) CreateCustomToken(ctx context.Context, deviceID string, claims map[string]interface{}) (string, error) {
	// Create custom token with device ID as the UID
	// Claims are attached to the token and available after verification
	token, err := s.client.CustomTokenWithClaims(ctx, deviceID, claims)
	if err != nil {
		return "", fmt.Errorf("failed to create custom token: %w", err)
	}
	return token, nil
}

// VerifyToken verifies a Firebase ID token and returns the device ID
func (s *FirebaseAuthService) VerifyToken(ctx context.Context, token string) (string, map[string]interface{}, error) {
	// Verify the ID token
	decodedToken, err := s.client.VerifyIDToken(ctx, token)
	if err != nil {
		return "", nil, fmt.Errorf("failed to verify token: %w", err)
	}

	return decodedToken.UID, decodedToken.Claims, nil
}

// Close is a no-op for Firebase (no cleanup needed)
func (s *FirebaseAuthService) Close() error {
	return nil
}
