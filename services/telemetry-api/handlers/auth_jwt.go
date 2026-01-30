package handlers

import (
	"context"
	"crypto/rsa"
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
	projectID           string
	serviceAccountEmail string
	serviceURL          string // Cloud Run service URL for JWT audience
	iamService          *iamcredentials.Service

	// Cache for service account public keys
	publicKeys map[string]*rsa.PublicKey
	keysMu     sync.RWMutex
	keysExpiry time.Time
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
func NewJWTAuthService(ctx context.Context, projectID string, serviceURL string) (*JWTAuthService, error) {
	// Get default credentials to find service account email
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}

	var credInfo struct {
		ClientEmail string `json:"client_email"`
	}

	serviceAccountEmail := ""
	if creds.JSON != nil {
		if err := json.Unmarshal(creds.JSON, &credInfo); err == nil && credInfo.ClientEmail != "" {
			serviceAccountEmail = credInfo.ClientEmail
		}
	}

	if serviceAccountEmail == "" {
		serviceAccountEmail = fmt.Sprintf("telemetry-api-sa@%s.iam.gserviceaccount.com", projectID)
	}

	iamService, err := iamcredentials.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM credentials service: %w", err)
	}

	return &JWTAuthService{
		projectID:           projectID,
		serviceAccountEmail: serviceAccountEmail,
		serviceURL:          serviceURL,
		iamService:          iamService,
		publicKeys:          make(map[string]*rsa.PublicKey),
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

	// Build JWT payload as JSON (SignJwt handles the header and signature)
	payload := map[string]interface{}{
		"iss":         s.serviceAccountEmail,
		"sub":         deviceID,
		"aud":         s.serviceURL,
		"exp":         now.Add(1 * time.Hour).Unix(),
		"iat":         now.Unix(),
		"nbf":         now.Unix(),
		"jti":         fmt.Sprintf("%s-%d", deviceID, now.UnixNano()),
		"device_id":   deviceID,
		"app_name":    appName,
		"app_version": appVersion,
		"claims":      claims,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}

	// Sign using IAM SignJwt API (includes kid header automatically)
	signReq := &iamcredentials.SignJwtRequest{
		Payload: string(payloadJSON),
	}

	name := fmt.Sprintf("projects/-/serviceAccounts/%s", s.serviceAccountEmail)
	signResp, err := s.iamService.Projects.ServiceAccounts.SignJwt(name, signReq).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signResp.SignedJwt, nil
}

// VerifyToken verifies a JWT and returns the device ID and claims
func (s *JWTAuthService) VerifyToken(ctx context.Context, tokenString string) (string, map[string]interface{}, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		// SignJwt includes kid header, use it to get the correct key
		kid, _ := t.Header["kid"].(string)
		return s.getPublicKey(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}))

	if err != nil {
		return "", nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return "", nil, fmt.Errorf("invalid token claims")
	}

	if claims.Issuer != s.serviceAccountEmail {
		return "", nil, fmt.Errorf("invalid token issuer")
	}

	audienceValid := false
	for _, aud := range claims.Audience {
		if aud == s.serviceURL {
			audienceValid = true
			break
		}
	}

	if !audienceValid && len(claims.Audience) > 0 {
		return "", nil, fmt.Errorf("invalid token audience: expected %s, got %v", s.serviceURL, claims.Audience)
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

// getPublicKey returns the public key for the given kid, refreshing cache if needed
func (s *JWTAuthService) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	s.keysMu.RLock()
	if time.Now().Before(s.keysExpiry) && len(s.publicKeys) > 0 {
		if key, ok := s.publicKeys[kid]; ok {
			s.keysMu.RUnlock()
			return key, nil
		}
	}
	s.keysMu.RUnlock()

	// Fetch fresh keys (might be a newly rotated key)
	if err := s.refreshPublicKeys(ctx); err != nil {
		return nil, err
	}

	s.keysMu.RLock()
	defer s.keysMu.RUnlock()
	if key, ok := s.publicKeys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("public key not found for kid: %s", kid)
}

// refreshPublicKeys fetches the service account's public keys from Google
func (s *JWTAuthService) refreshPublicKeys(ctx context.Context) error {
	s.keysMu.Lock()
	defer s.keysMu.Unlock()

	// Fetch from Google's public key endpoint for service accounts
	url := fmt.Sprintf("https://www.googleapis.com/service_accounts/v1/metadata/x509/%s", s.serviceAccountEmail)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch public keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch public keys: %s - %s", resp.Status, string(body))
	}

	// Parse response - map of key ID to X.509 certificate (PEM)
	var certs map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&certs); err != nil {
		return fmt.Errorf("failed to decode public keys: %w", err)
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

	if len(s.publicKeys) == 0 {
		return fmt.Errorf("no valid public keys found")
	}

	// Cache for 1 hour
	s.keysExpiry = time.Now().Add(1 * time.Hour)
	return nil
}

// Close is a no-op
func (s *JWTAuthService) Close() error {
	return nil
}
