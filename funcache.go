// Package funcache provides simple, fine-grained caching of function values.
package funcache

import (
	"fmt"
	"sync"
)

// Store is any backing store used by the cache. Note that the cache doesn't do
// any eviction of keys. That's up to your particular store to manage, however
// it sees fit.
type Store interface {
	Add(key, value interface{})
	Get(key interface{}) (value interface{}, ok bool)

	// Contains(key interface{}) bool
	// Peek(key interface{}) (interface{}, bool)
	// Purge()
	// Remove(key interface{})
}

// -----------------------------------------------------------------------------
// Dummy store, used for testing and init().

type nilStore struct{}

func (*nilStore) Add(key, value interface{})                       { return }
func (*nilStore) Get(key interface{}) (value interface{}, ok bool) { return }

func nilCache() *Cache { return New(&nilStore{}) }

// -----------------------------------------------------------------------------
// Simple in-memory map, safe for concurrent access.

type syncMap struct {
	sync.RWMutex
	m map[string]interface{}
}

func keyFromInterface(key interface{}) string {
	return fmt.Sprint(key)
}

func newSyncMap() *syncMap {
	return &syncMap{m: make(map[string]interface{})}
}

func (sm *syncMap) Add(key, value interface{}) {
	sm.Lock()
	defer sm.Unlock()
	sm.m[keyFromInterface(key)] = value
}

func (sm *syncMap) Get(key interface{}) (value interface{}, ok bool) {
	sm.RLock()
	defer sm.RUnlock()
	value, ok = sm.m[keyFromInterface(key)]
	return
}

// -----------------------------------------------------------------------------

type Cache struct{ store Store }

// New returns a Cache backed by the store you provide.
func New(store Store) *Cache { return &Cache{store} }

// NewInMemCache returns a Cache backed by a simple in-memory map, safe for
// concurrent access.
func NewInMemCache() *Cache { return New(newSyncMap()) }

// Bust calls the given function, invalidating any cached values in nested
// function calls.
func (cache *Cache) Bust(fn func()) { fn() }

// Cache takes a function and caches its return value. It saves it in the store
// under the given key. Subsequent calls to Cache, with the same key, will return
// the cached value (if it still exists in the store), otherwise the function
// will be called again.
func (cache *Cache) Cache(key interface{}, fn func() interface{}) interface{} {
	if !wasCalledByCacheBustingFn() {
		if data, ok := cache.store.Get(key); ok {
			return data
		}
	}
	data := fn()
	cache.store.Add(key, data)
	return data
}

// Wrap caches the return value of the given function. It is the same as Cache,
// except that it auto-assigns a cache key, which is just the function name.
func (cache *Cache) Wrap(fn func() interface{}) interface{} {
	return cache.Cache(getFnName(fn), fn)
}
