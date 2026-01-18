// Package cache provides generic in-memory caching utilities.
package cache

import (
	"sync"
	"time"
)

// entry holds a cached value with its expiration time.
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTL is a thread-safe in-memory cache with time-based expiration.
// It wraps a Loader function that fetches values on cache misses.
type TTL[K comparable, V any] struct {
	loader Loader[K, V]
	cache  map[K]*entry[V]
	mu     sync.RWMutex
	ttl    time.Duration
}

// Loader is a function that loads a value for a given key.
// It's called on cache misses.
type Loader[K comparable, V any] func(key K) (V, error)

// NewTTL creates a new TTL cache with the given loader and TTL duration.
func NewTTL[K comparable, V any](loader Loader[K, V], ttl time.Duration) *TTL[K, V] {
	return &TTL[K, V]{
		loader: loader,
		cache:  make(map[K]*entry[V]),
		ttl:    ttl,
	}
}

// Get retrieves a value from the cache, loading it if necessary.
// Returns the cached value if not expired, otherwise calls the loader.
func (c *TTL[K, V]) Get(key K) (V, error) {
	// Check cache first (read lock)
	c.mu.RLock()
	if e, ok := c.cache[key]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.value, nil
	}
	c.mu.RUnlock()

	// Cache miss - load value
	value, err := c.loader(key)
	if err != nil {
		var zero V
		return zero, err
	}

	// Update cache (write lock)
	c.mu.Lock()
	c.cache[key] = &entry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return value, nil
}

// Set stores a value in the cache, bypassing the loader.
// Useful when you've just created/updated a value and want to cache it immediately.
func (c *TTL[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.cache[key] = &entry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// Invalidate removes a key from the cache.
func (c *TTL[K, V]) Invalidate(key K) {
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *TTL[K, V]) InvalidateAll() {
	c.mu.Lock()
	c.cache = make(map[K]*entry[V])
	c.mu.Unlock()
}

// Len returns the number of items in the cache (including expired ones).
func (c *TTL[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}
