package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iamcredentials/v1"
)

// JWTAuthService implements AuthService using self-signed JWTs
// Tokens are signed by the GCP service account and can be verified
// without any client-side exchange (unlike Firebase custom tokens)
type JWTAuthService struct {
	projectID          string
	serviceAccountEmail string
	serviceURL         string // Cloud Run service URL for JWT audience
	iamService         *iamcredentials.Service
	
	// Cache for service account public keys
	publicKeys   map[string]*rsa.PublicKey
	keysMu       sync.RWMutex
	keysExpiry   time.Time
}

// JWTClaims represents the claims in our device tokens
type JWTClaims struct {
	jwt.RegisteredClaims
	DeviceID   string                 `json:"device_id"`
	AppName    string                 `json:"app_name,omitempty"`
	AppVersion string                 `json:"app_version,omitempty"`
	Claims     map[string]interface{} `json:"claims,omitempty"`
}

// NewJWTAuthService creates a new JWT auth service that uses GCP service account signing
// serviceURL is the Cloud Run service URL to use as JWT audience (if empty, constructs from projectID)
func NewJWTAuthService(ctx context.Context, projectID string, serviceURL string) (*JWTAuthService, error) {
	// Get default credentials to find service account email
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}

	// Parse service account email from credentials
	var credInfo struct {
		ClientEmail string `json:"client_email"`
	}
	
	// Try to parse credential info - may be empty for compute engine default SA
	serviceAccountEmail := ""
	if creds.JSON != nil {
		if err := json.Unmarshal(creds.JSON, &credInfo); err == nil && credInfo.ClientEmail != "" {
			serviceAccountEmail = credInfo.ClientEmail
		}
	}
	
	// Fallback to constructing from project ID (for Cloud Run)
	if serviceAccountEmail == "" {
		serviceAccountEmail = fmt.Sprintf("telemetry-api-sa@%s.iam.gserviceaccount.com", projectID)
	}

	// Create IAM credentials service for signing
	iamService, err := iamcredentials.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM credentials service: %w", err)
	}

	// If serviceURL not provided, construct default (backward compatibility)
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://telemetry-api.%s.run.app", projectID)
	}

	return &JWTAuthService{
		projectID:          projectID,
		serviceAccountEmail: serviceAccountEmail,
		serviceURL:         serviceURL,
		iamService:         iamService,
		publicKeys:         make(map[string]*rsa.PublicKey),
	}, nil
}

// CreateCustomToken creates a JWT signed by the service account
// This token can be used directly by devices without any exchange step
func (s *JWTAuthService) CreateCustomToken(ctx context.Context, deviceID string, claims map[string]interface{}) (string, error) {
	now := time.Now()
	
	// Extract known claims
	appName := ""
	appVersion := ""
	if v, ok := claims["app_name"].(string); ok {
		appName = v
	}
	if v, ok := claims["app_version"].(string); ok {
		appVersion = v
	}

	// Create JWT claims
	jwtClaims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.serviceAccountEmail,
			Subject:   deviceID,
			Audience:  jwt.ClaimStrings{s.serviceURL},
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("%s-%d", deviceID, now.UnixNano()),
		},
		DeviceID:   deviceID,
		AppName:    appName,
		AppVersion: appVersion,
		Claims:     claims,
	}

	// Create unsigned token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
	
	// Get the signing string (header.payload)
	signingString, err := token.SigningString()
	if err != nil {
		return "", fmt.Errorf("failed to create signing string: %w", err)
	}

	// Sign using IAM credentials API
	signReq := &iamcredentials.SignBlobRequest{
		Payload: base64.StdEncoding.EncodeToString([]byte(signingString)),
	}
	
	name := fmt.Sprintf("projects/-/serviceAccounts/%s", s.serviceAccountEmail)
	signResp, err := s.iamService.Projects.ServiceAccounts.SignBlob(name, signReq).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Decode signature and create final token
	signature, err := base64.StdEncoding.DecodeString(signResp.SignedBlob)
	if err != nil {
		return "", fmt.Errorf("failed to decode signature: %w", err)
	}

	// Build final JWT: header.payload.signature
	signedToken := signingString + "." + base64.RawURLEncoding.EncodeToString(signature)
	
	return signedToken, nil
}

// VerifyToken verifies a JWT and returns the device ID and claims
func (s *JWTAuthService) VerifyToken(ctx context.Context, tokenString string) (string, map[string]interface{}, error) {
	// Parse and verify the token
	// Configure parser to validate audience against actual service URL
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from header
		kid, _ := token.Header["kid"].(string)
		
		// Get public key for verification
		return s.getPublicKey(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithAudience(s.serviceURL))

	if err != nil {
		return "", nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return "", nil, fmt.Errorf("invalid token claims")
	}

	// Verify issuer
	if claims.Issuer != s.serviceAccountEmail {
		return "", nil, fmt.Errorf("invalid token issuer")
	}

	// Build claims map for compatibility
	claimsMap := map[string]interface{}{
		"app_name":    claims.AppName,
		"app_version": claims.AppVersion,
		"device_id":   claims.DeviceID,
	}
	for k, v := range claims.Claims {
		claimsMap[k] = v
	}

	return claims.DeviceID, claimsMap, nil
}

// getPublicKey fetches the public key for the service account
func (s *JWTAuthService) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	s.keysMu.RLock()
	if time.Now().Before(s.keysExpiry) && len(s.publicKeys) > 0 {
		// Try to find key by kid, or return first key if no kid
		if kid != "" {
			if key, ok := s.publicKeys[kid]; ok {
				s.keysMu.RUnlock()
				return key, nil
			}
		} else {
			// Return any key if no kid specified
			for _, key := range s.publicKeys {
				s.keysMu.RUnlock()
				return key, nil
			}
		}
	}
	s.keysMu.RUnlock()

	// Fetch fresh keys
	return s.fetchPublicKeys(ctx, kid)
}

// fetchPublicKeys fetches the service account's public keys from Google
func (s *JWTAuthService) fetchPublicKeys(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	s.keysMu.Lock()
	defer s.keysMu.Unlock()

	// Fetch from Google's public key endpoint for service accounts
	url := fmt.Sprintf("https://www.googleapis.com/service_accounts/v1/metadata/x509/%s", s.serviceAccountEmail)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch public keys: %s - %s", resp.Status, string(body))
	}

	// Parse response - map of key ID to X.509 certificate (PEM)
	var certs map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&certs); err != nil {
		return nil, fmt.Errorf("failed to decode public keys: %w", err)
	}

	// Parse certificates and extract public keys
	s.publicKeys = make(map[string]*rsa.PublicKey)
	for keyID, certPEM := range certs {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(certPEM))
		if err != nil {
			continue // Skip invalid certs
		}
		s.publicKeys[keyID] = key
	}

	// Cache for 1 hour
	s.keysExpiry = time.Now().Add(1 * time.Hour)

	// Return requested key
	if kid != "" {
		if key, ok := s.publicKeys[kid]; ok {
			return key, nil
		}
	}
	
	// Return first available key
	for _, key := range s.publicKeys {
		return key, nil
	}

	return nil, fmt.Errorf("no valid public keys found")
}

// Close is a no-op
func (s *JWTAuthService) Close() error {
	return nil
}
