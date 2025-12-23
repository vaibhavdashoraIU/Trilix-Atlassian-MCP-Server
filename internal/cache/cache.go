package cache

import (
	"sync"
	"time"
)

// CacheEntry holds a cached value with expiration
type CacheEntry struct {
	Value      interface{}
	Expiration time.Time
}

// SimpleCache is a thread-safe in-memory cache with TTL
type SimpleCache struct {
	mu    sync.RWMutex
	items map[string]CacheEntry
}

// NewSimpleCache creates a new cache instance
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		items: make(map[string]CacheEntry),
	}
}

// Get retrieves a value from cache if it exists and hasn't expired
func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.Expiration) {
		return nil, false
	}

	return entry.Value, true
}

// Set stores a value in cache with the given TTL
func (c *SimpleCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = CacheEntry{
		Value:      value,
		Expiration: time.Now().Add(ttl),
	}
}

// Delete removes a key from cache
func (c *SimpleCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear removes all entries from cache
func (c *SimpleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]CacheEntry)
}
