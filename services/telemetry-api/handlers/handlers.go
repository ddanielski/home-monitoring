package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
)

// Schema cache TTL - schemas only change on firmware updates (rare)
const SchemaCacheTTL = 24 * time.Hour

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

	// Initialize Firebase Auth
	authService, err := NewFirebaseAuthService(ctx, cfg.ProjectID)
	if err != nil {
		fsClient.Close()
		psClient.Close()
		return nil, err
	}

	// Wrap schema store with in-memory cache
	schemaStore := NewSchemaCache(NewFirestoreSchemaStore(fsClient), SchemaCacheTTL)

	return &Handlers{
		config:          cfg,
		telemetryStore:  NewFirestoreTelemetryStore(fsClient),
		commandStore:    NewFirestoreCommandStore(fsClient),
		deviceStore:     NewFirestoreDeviceStore(fsClient),
		schemaStore:     schemaStore,
		authService:     authService,
		publisher:       NewPubSubPublisher(psClient),
		logger:          slog.Default(),
		firestoreClient: fsClient,
		pubsubClient:    psClient,
	}, nil
}

// NewWithStores creates handlers with custom stores (for testing)
func NewWithStores(telemetry TelemetryStore, commands CommandStore, devices DeviceStore, schemas SchemaStore, auth AuthService, publisher EventPublisher) *Handlers {
	return &Handlers{
		telemetryStore: telemetry,
		commandStore:   commands,
		deviceStore:    devices,
		schemaStore:    schemas,
		authService:    auth,
		publisher:      publisher,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
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
