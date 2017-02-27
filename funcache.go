// Package funcache provides simple, fine-grained caching of function values.
package funcache

import (
	"sync"
	"sync/atomic"
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
	m map[interface{}]interface{}
}

func newSyncMap() *syncMap {
	return &syncMap{m: make(map[interface{}]interface{})}
}

func (sm *syncMap) Add(key, value interface{}) {
	sm.Lock()
	defer sm.Unlock()
	sm.m[key] = value
}

func (sm *syncMap) Get(key interface{}) (value interface{}, ok bool) {
	sm.RLock()
	defer sm.RUnlock()
	value, ok = sm.m[key]
	return
}

// -----------------------------------------------------------------------------
// Copy-on-write in-memory map, safe for concurrent access.

type cowMap struct {
	sync.Mutex // Used only when writing
	m          atomic.Value
}

func newCopyOnWriteMap() *cowMap {
	cm := &cowMap{}
	cm.m.Store(make(map[interface{}]interface{}))
	return cm
}

func (cm *cowMap) Add(key, value interface{}) {
	cm.Lock()
	defer cm.Unlock()
	m1 := cm.m.Load().(map[interface{}]interface{})
	m2 := make(map[interface{}]interface{})
	for k, v := range m1 {
		m2[k] = v
	}
	m2[key] = value
	cm.m.Store(m2)
}

func (cm *cowMap) Get(key interface{}) (value interface{}, ok bool) {
	m := cm.m.Load().(map[interface{}]interface{})
	value, ok = m[key]
	return
}

// -----------------------------------------------------------------------------

type Cache struct {
	store Store
	// Small optimization: maintain a counter of actively cache busting callers.
	// If no one is cache busting, then don't go through the extra effort of
	// checking the caller stack.
	busting uint32
}

// New returns a Cache backed by the store you provide.
func New(store Store) *Cache { return &Cache{store: store} }

// NewInMemCache returns a Cache backed by a simple in-memory map, safe for
// concurrent access.
func NewInMemCache() *Cache { return New(newSyncMap()) }

// Bust calls the given function, invalidating any cached values in nested
// function calls.
func (cache *Cache) Bust(fn func()) {
	atomic.AddUint32(&cache.busting, 1)                // Increment
	defer atomic.AddUint32(&cache.busting, ^uint32(0)) // Decrement
	fn()
}

// Cache takes a function and caches its return value. It saves it in the store
// under the given key. Subsequent calls to Cache, with the same key, will return
// the cached value (if it still exists in the store), otherwise the function
// will be called again.
func (cache *Cache) Cache(key interface{}, fn func() interface{}) interface{} {
	if atomic.LoadUint32(&cache.busting) == 0 || !wasCalledByCacheBustingFn() {
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
