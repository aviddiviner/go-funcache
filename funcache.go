package funcache

import (
	"fmt"
	"sync"
)

type Store interface {
	Add(key, value interface{})
	Get(key interface{}) (value interface{}, ok bool)

	// Contains(key interface{}) bool
	// Peek(key interface{}) (interface{}, bool)
	// Purge()
	// Remove(key interface{})
}

// -----------------------------------------------------------------------------

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

// func (sm *syncMap) Purge() {
// 	sm.Lock()
// 	defer sm.Unlock()
// 	sm.m = make(map[string]interface{})
// }

// func (sm *syncMap) Remove(key interface{}) {
// 	sm.Lock()
// 	defer sm.Unlock()
// 	delete(sm.m, keyFromInterface(key))
// }

// -----------------------------------------------------------------------------

const cacheBustingFn = "github.com/aviddiviner/funcache.(*Cache).Bust"

type Cache struct{ store Store }

func New(store Store) *Cache { return &Cache{store} }

// NewInMemCache returns a Cache backed by a simple in-memory map, safe for
// concurrent access.
func NewInMemCache() *Cache { return New(newSyncMap()) }

func (cache *Cache) Bust(fn func()) { fn() }

func (cache *Cache) Wrap(key interface{}, fn func() interface{}) interface{} {
	if !wasCalledBy(cacheBustingFn) {
		if data, ok := cache.store.Get(key); ok {
			return data
		}
	}
	data := fn()
	cache.store.Add(key, data)
	return data
}
