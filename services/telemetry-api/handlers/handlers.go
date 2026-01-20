package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/cache"
)

// Cache TTLs
const (
	// Schema cache TTL - schemas only change on firmware updates (rare)
	SchemaCacheTTL = 24 * time.Hour
	// Device status cache TTL - controls max revocation delay
	// Shorter = faster revocation, more Firestore reads
	DeviceStatusCacheTTL = 5 * time.Minute
)

// Config holds handler configuration
type Config struct {
	ProjectID string
}

// Handlers contains all HTTP handlers and their dependencies
type Handlers struct {
	config         Config
	telemetryStore TelemetryStore
	commandStore   CommandStore
	deviceStore    DeviceStore
	schemaStore    SchemaStore
	authService    AuthService
	publisher      EventPublisher
	logger         *slog.Logger

	// Cache for device revocation status (reduces Firestore reads)
	deviceStatusCache *cache.TTL[string, bool]

	// Keep references for cleanup
	firestoreClient *firestore.Client
	pubsubClient    *pubsub.Client
}

// New creates a new Handlers instance with initialized clients
func New(ctx context.Context, cfg Config) (*Handlers, error) {
	fsClient, err := firestore.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, err
	}

	psClient, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		fsClient.Close()
		return nil, err
	}

	// Initialize JWT Auth service (self-signed tokens, no Firebase exchange needed)
	authService, err := NewJWTAuthService(ctx, cfg.ProjectID)
	if err != nil {
		fsClient.Close()
		psClient.Close()
		return nil, err
	}

	// Wrap schema store with in-memory cache
	schemaStore := NewSchemaCache(NewFirestoreSchemaStore(fsClient), SchemaCacheTTL)
	deviceStore := NewFirestoreDeviceStore(fsClient)

	// Create device status cache for revocation checks
	// The loader fetches the revoked status from the device store
	deviceStatusCache := cache.NewTTL(func(deviceID string) (bool, error) {
		device, err := deviceStore.GetByID(context.Background(), deviceID)
		if err != nil {
			// If device not found, treat as revoked (fail closed)
			return true, nil
		}
		return device.Revoked, nil
	}, DeviceStatusCacheTTL)

	return &Handlers{
		config:            cfg,
		telemetryStore:    NewFirestoreTelemetryStore(fsClient),
		commandStore:      NewFirestoreCommandStore(fsClient),
		deviceStore:       deviceStore,
		schemaStore:       schemaStore,
		authService:       authService,
		publisher:         NewPubSubPublisher(psClient),
		logger:            slog.Default(),
		deviceStatusCache: deviceStatusCache,
		firestoreClient:   fsClient,
		pubsubClient:      psClient,
	}, nil
}

// NewWithStores creates handlers with custom stores (for testing)
func NewWithStores(telemetry TelemetryStore, commands CommandStore, devices DeviceStore, schemas SchemaStore, auth AuthService, publisher EventPublisher) *Handlers {
	// For testing, create a cache that always returns false (not revoked)
	// Tests that need revocation behavior should set deviceStatusCache explicitly
	var deviceStatusCache *cache.TTL[string, bool]
	if devices != nil {
		deviceStatusCache = cache.NewTTL(func(deviceID string) (bool, error) {
			device, err := devices.GetByID(context.Background(), deviceID)
			if err != nil {
				return true, nil // Fail closed
			}
			return device.Revoked, nil
		}, DeviceStatusCacheTTL)
	}

	return &Handlers{
		telemetryStore:    telemetry,
		commandStore:      commands,
		deviceStore:       devices,
		schemaStore:       schemas,
		authService:       auth,
		publisher:         publisher,
		logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		deviceStatusCache: deviceStatusCache,
	}
}

// Close cleans up handler resources
func (h *Handlers) Close() error {
	if h.firestoreClient != nil {
		h.firestoreClient.Close()
	}
	if h.pubsubClient != nil {
		h.pubsubClient.Close()
	}
	return nil
}

// Collections and topics
const (
	TelemetryCollection = "telemetry"
	CommandsCollection  = "commands"
	DevicesCollection   = "devices"
	SchemasCollection   = "schemas"
	TelemetryTopic      = "telemetry-events"
)

// Health returns service health status
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}
