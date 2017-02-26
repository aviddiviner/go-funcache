package funcache

import (
	"runtime"
	"testing"

	"github.com/hashicorp/golang-lru"
	"github.com/stretchr/testify/assert"
)

func testGetCallingFuncs() (funcNames []string) {
	// Skip the first 3 callers:
	// 1. runtime.Callers
	// 2. github.com/aviddiviner/funcache.getAllCallers
	// 3. testGetCallingFuncs (this)
	pcs := getAllCallers(3)
	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()
		funcNames = append(funcNames, frame.Function)
		if !more {
			break
		}
	}
	return
}

func TestCaller(t *testing.T) {
	fns := testGetCallingFuncs()
	var found bool
	for _, fn := range fns {
		if fn == "github.com/aviddiviner/funcache.TestCaller" {
			found = true
		}
	}
	assert.True(t, found)

	cache := nilCache()
	cache.Bust(func() {
		assert.True(t, wasCalledByCacheBustingFn())
	})
}

func TestWrapIsDistinct(t *testing.T) {
	cache := nilCache()

	getValueA := func() (value, caller string) {
		value = cache.Wrap(func() interface{} {
			caller = testGetCallingFuncs()[0]
			return "A"
		}).(string)
		return
	}
	valueA, callerA := getValueA()
	assert.Equal(t, "A", valueA)

	valueA, callerA1 := getValueA() // Inner func called again, because nilCache
	assert.Equal(t, "A", valueA)

	assert.Equal(t, callerA, callerA1)

	var callerB string
	valueB := cache.Wrap(func() interface{} {
		callerB = testGetCallingFuncs()[0]
		return "B"
	})
	assert.Equal(t, "B", valueB)

	assert.NotEqual(t, callerA, callerB)
}

func TestBasics(t *testing.T) {
	cache := NewInMemCache()

	var callCount int
	getFoo := func() string {
		return cache.Wrap(func() interface{} {
			callCount += 1
			return "Foo!"
		}).(string)
	}

	assert.Equal(t, "Foo!", getFoo())
	assert.Equal(t, 1, callCount)

	assert.Equal(t, "Foo!", getFoo())
	assert.Equal(t, 1, callCount)

	cache.Bust(func() {
		assert.Equal(t, "Foo!", getFoo())
		assert.Equal(t, 2, callCount)

		assert.Equal(t, "Foo!", getFoo())
		assert.Equal(t, 3, callCount)
	})
}

func testCacheUse(t *testing.T, cache *Cache, key, val interface{}, bust bool) {
	var gotBust bool
	gotVal := cache.Cache(key, func() interface{} {
		gotBust = true
		return val
	})
	assert.Equal(t, val, gotVal)
	assert.Equal(t, bust, gotBust)
}

func TestDeeplyNestedCacheBusting(t *testing.T) {
	cache := NewInMemCache()

	testCacheUse(t, cache, "foo", "Foo!", true)
	testCacheUse(t, cache, "foo", "Foo!", false)

	cache.Bust(func() {
		testCacheUse(t, cache, "foo", "Foo!", true)
		func() {
			testCacheUse(t, cache, "foo", "Foo!", true)
		}()
	})
}

func TestBackedByAnotherStore(t *testing.T) {
	store, err := lru.New2Q(10)
	assert.NoError(t, err)
	cache := New(store)

	testCacheUse(t, cache, "foo", "Foo!", true)
	testCacheUse(t, cache, "foo", "Foo!", false)
	testCacheUse(t, cache, "bar", "Bar!", true)
	testCacheUse(t, cache, "foo", "Foo!", false)

	cache.Bust(func() {
		testCacheUse(t, cache, "bar", "Bar!", true)
		testCacheUse(t, cache, "foo", "Foo!", true)
	})

	testCacheUse(t, cache, "foo", "Foo!", false)
	testCacheUse(t, cache, "bar", "Bar!", false)
}

func TestCacheNil(t *testing.T) {
	cache := NewInMemCache()

	testCacheUse(t, cache, nil, "Foo!", true)
	testCacheUse(t, cache, nil, "Foo!", false)
}
