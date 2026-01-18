package handlers

import (
	"context"
	"time"

	"github.com/ddanielski/home-monitoring/services/telemetry-api/pkg/cache"
)

// schemaKey is the cache key for schemas (app:version)
type schemaKey struct {
	appName string
	version string
}

// SchemaCache provides cached access to measurement schemas.
// It wraps a SchemaStore and caches results for the configured TTL.
type SchemaCache struct {
	store SchemaStore
	cache *cache.TTL[schemaKey, *MeasurementSchema]
}

// NewSchemaCache creates a cache wrapping a SchemaStore.
func NewSchemaCache(store SchemaStore, ttl time.Duration) *SchemaCache {
	sc := &SchemaCache{store: store}

	// Create the generic cache with a loader that calls the store
	// Note: loader doesn't have context, so we use a background context
	// This is a tradeoff for the generic cache design
	sc.cache = cache.NewTTL(func(key schemaKey) (*MeasurementSchema, error) {
		return store.Get(context.Background(), key.appName, key.version)
	}, ttl)

	return sc
}

// Get retrieves a schema, using cache if available.
func (c *SchemaCache) Get(ctx context.Context, appName, version string) (*MeasurementSchema, error) {
	// For now, we ignore ctx since the generic cache doesn't support it
	// In a production system, you might want a context-aware cache
	return c.cache.Get(schemaKey{appName, version})
}

// Save saves to store and updates cache.
func (c *SchemaCache) Save(ctx context.Context, appName, version string, schema *MeasurementSchema) error {
	if err := c.store.Save(ctx, appName, version, schema); err != nil {
		return err
	}

	// Update cache immediately
	c.cache.Set(schemaKey{appName, version}, schema)
	return nil
}

// Invalidate removes a schema from cache.
func (c *SchemaCache) Invalidate(appName, version string) {
	c.cache.Invalidate(schemaKey{appName, version})
}

// InvalidateAll clears the entire cache.
func (c *SchemaCache) InvalidateAll() {
	c.cache.InvalidateAll()
}
