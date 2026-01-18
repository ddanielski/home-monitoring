package cache

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestTTL_Get_CacheMiss(t *testing.T) {
	loadCount := 0
	loader := func(key string) (int, error) {
		loadCount++
		return len(key), nil
	}

	cache := NewTTL(loader, time.Hour)

	// First call should load
	val, err := cache.Get("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
	if loadCount != 1 {
		t.Errorf("expected 1 load, got %d", loadCount)
	}
}

func TestTTL_Get_CacheHit(t *testing.T) {
	loadCount := 0
	loader := func(key string) (int, error) {
		loadCount++
		return len(key), nil
	}

	cache := NewTTL(loader, time.Hour)

	// First call
	cache.Get("hello")
	// Second call should hit cache
	val, err := cache.Get("hello")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
	if loadCount != 1 {
		t.Errorf("expected 1 load (cache hit), got %d", loadCount)
	}
}

func TestTTL_Get_Expiration(t *testing.T) {
	loadCount := 0
	loader := func(key string) (int, error) {
		loadCount++
		return loadCount, nil // Return different values to detect reloads
	}

	// Very short TTL for testing
	cache := NewTTL(loader, 10*time.Millisecond)

	// First call
	val1, _ := cache.Get("key")
	if val1 != 1 {
		t.Errorf("expected 1, got %d", val1)
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should reload
	val2, _ := cache.Get("key")
	if val2 != 2 {
		t.Errorf("expected 2 (reloaded), got %d", val2)
	}
	if loadCount != 2 {
		t.Errorf("expected 2 loads, got %d", loadCount)
	}
}

func TestTTL_Get_LoaderError(t *testing.T) {
	expectedErr := errors.New("loader failed")
	loader := func(key string) (int, error) {
		return 0, expectedErr
	}

	cache := NewTTL(loader, time.Hour)

	val, err := cache.Get("key")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if val != 0 {
		t.Errorf("expected zero value, got %d", val)
	}
}

func TestTTL_Set(t *testing.T) {
	loadCount := 0
	loader := func(key string) (int, error) {
		loadCount++
		return 0, nil
	}

	cache := NewTTL(loader, time.Hour)

	// Set a value directly
	cache.Set("key", 42)

	// Get should return cached value without loading
	val, err := cache.Get("key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
	if loadCount != 0 {
		t.Errorf("expected 0 loads (value was set), got %d", loadCount)
	}
}

func TestTTL_Invalidate(t *testing.T) {
	loadCount := 0
	loader := func(key string) (int, error) {
		loadCount++
		return loadCount, nil
	}

	cache := NewTTL(loader, time.Hour)

	// Load value
	cache.Get("key")
	if loadCount != 1 {
		t.Errorf("expected 1 load, got %d", loadCount)
	}

	// Invalidate
	cache.Invalidate("key")

	// Should reload
	cache.Get("key")
	if loadCount != 2 {
		t.Errorf("expected 2 loads after invalidate, got %d", loadCount)
	}
}

func TestTTL_InvalidateAll(t *testing.T) {
	loadCount := 0
	loader := func(key string) (int, error) {
		loadCount++
		return loadCount, nil
	}

	cache := NewTTL(loader, time.Hour)

	// Load multiple values
	cache.Get("key1")
	cache.Get("key2")
	if loadCount != 2 {
		t.Errorf("expected 2 loads, got %d", loadCount)
	}

	// Invalidate all
	cache.InvalidateAll()

	// Both should reload
	cache.Get("key1")
	cache.Get("key2")
	if loadCount != 4 {
		t.Errorf("expected 4 loads after invalidate all, got %d", loadCount)
	}
}

func TestTTL_Len(t *testing.T) {
	loader := func(key string) (int, error) {
		return 1, nil
	}

	cache := NewTTL(loader, time.Hour)

	if cache.Len() != 0 {
		t.Errorf("expected 0, got %d", cache.Len())
	}

	cache.Get("key1")
	if cache.Len() != 1 {
		t.Errorf("expected 1, got %d", cache.Len())
	}

	cache.Get("key2")
	if cache.Len() != 2 {
		t.Errorf("expected 2, got %d", cache.Len())
	}

	cache.Invalidate("key1")
	if cache.Len() != 1 {
		t.Errorf("expected 1 after invalidate, got %d", cache.Len())
	}
}

func TestTTL_Concurrent(t *testing.T) {
	loadCount := 0
	var mu sync.Mutex
	loader := func(key string) (int, error) {
		mu.Lock()
		loadCount++
		mu.Unlock()
		return len(key), nil
	}

	cache := NewTTL(loader, time.Hour)

	// Run concurrent gets
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Get("key")
		}()
	}
	wg.Wait()

	// Should have loaded at least once but cached after that
	// Note: Due to race conditions, might load a few times
	if loadCount > 10 {
		t.Errorf("expected < 10 loads (caching), got %d", loadCount)
	}
}
