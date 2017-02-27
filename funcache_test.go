package funcache

import (
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/stretchr/testify/assert"
)

type noisyTestStore struct {
	t *testing.T
	m map[interface{}]interface{}
}

func noisyTestCache(t *testing.T) *Cache {
	return New(&noisyTestStore{t: t, m: make(map[interface{}]interface{})})
}

func (ts *noisyTestStore) Add(key, value interface{}) {
	ts.m[key] = value
	ts.t.Logf("Add(%v, %v)", key, value)
}

func (ts *noisyTestStore) Get(key interface{}) (value interface{}, ok bool) {
	value, ok = ts.m[key]
	ts.t.Logf("Get(%v) -> (%v, %v)", key, value, ok)
	return
}

// -----------------------------------------------------------------------------

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
		if fn == "github.com/aviddiviner/go-funcache.TestCaller" {
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
	cache := noisyTestCache(t)

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

func withTestTimeout(t *testing.T, millis int, fn func()) {
	done := make(chan bool)
	go func() {
		fn()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(time.Duration(millis) * time.Millisecond):
		t.Fatal("timed out")
	}
}

func TestFibonacci(t *testing.T) {
	cache := NewInMemCache()
	var fib func(k int) int
	fib = func(k int) int {
		if k < 2 {
			return k
		}
		a := cache.Cache(k-1, func() interface{} { return fib(k - 1) })
		b := cache.Cache(k-2, func() interface{} { return fib(k - 2) })
		return a.(int) + b.(int)
	}
	withTestTimeout(t, 500, func() { fib(36) }) // Retardedly slow without caching
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

func TestNestedCachingAndBusting(t *testing.T) {
	cache := NewInMemCache()

	var callCount int
	getFoo := func() interface{} {
		return cache.Wrap(func() interface{} {
			callCount += 1
			return "Foo!"
		})
	}
	getBar := func() interface{} {
		return cache.Wrap(func() interface{} {
			getFoo()
			getFoo()
			callCount += 1
			return "Bar!"
		})
	}

	assert.Equal(t, "Foo!", getFoo())
	assert.Equal(t, 1, callCount)

	assert.Equal(t, "Bar!", getBar())
	assert.Equal(t, 2, callCount)

	cache.Bust(func() {
		assert.Equal(t, "Foo!", getFoo())
		assert.Equal(t, 3, callCount)

		assert.Equal(t, "Bar!", getBar())
		assert.Equal(t, 6, callCount)

		func() {
			assert.Equal(t, "Bar!", getBar())
			assert.Equal(t, 9, callCount)
		}()

		cache.Bust(func() {
			assert.Equal(t, "Bar!", getBar())
			assert.Equal(t, 12, callCount)
		})
	})

	assert.Equal(t, "Bar!", getBar())
	assert.Equal(t, 12, callCount)
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

func TestCacheMixedKeys(t *testing.T) {
	cache := NewInMemCache()

	testCacheUse(t, cache, "abc", "Foo!", true)
	testCacheUse(t, cache, 123, "Foo!", true)
}

func TestDeferredFuncs(t *testing.T) {
	cache := NewInMemCache()

	testCacheUse(t, cache, "foo", "Foo!", true)
	defer testCacheUse(t, cache, "foo", "Foo!", false)
	defer cache.Bust(func() {
		testCacheUse(t, cache, "foo", "Foo!", true)
	})
}

// -----------------------------------------------------------------------------

func BenchmarkUncached(b *testing.B) {
	for n := 0; n < b.N; n++ {
		func() interface{} {
			return "xyz"
		}()
	}
}

func BenchmarkCacheHitsMem(b *testing.B) {
	cache := NewInMemCache()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Cache("xyz", func() interface{} {
			return "xyz"
		})
	}
}
func BenchmarkCacheHitsCow(b *testing.B) {
	cache := New(newCopyOnWriteMap())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Cache("xyz", func() interface{} {
			return "xyz"
		})
	}
}
func BenchmarkCacheMisses(b *testing.B) {
	cache := nilCache()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Cache("xyz", func() interface{} {
			return "xyz"
		})
	}
}
func BenchmarkCacheBusted(b *testing.B) {
	cache := nilCache()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Bust(func() {
			cache.Cache("xyz", func() interface{} {
				return "xyz"
			})
		})
	}
}
func BenchmarkCacheBustedMem(b *testing.B) {
	cache := NewInMemCache()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Bust(func() {
			cache.Cache("xyz", func() interface{} {
				return "xyz"
			})
		})
	}
}
func BenchmarkCacheBustedCow(b *testing.B) {
	cache := New(newCopyOnWriteMap())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Bust(func() {
			cache.Cache("xyz", func() interface{} {
				return "xyz"
			})
		})
	}
}

func BenchmarkWrapHitsMem(b *testing.B) {
	cache := NewInMemCache()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Wrap(func() interface{} {
			return "xyz"
		})
	}
}
func BenchmarkWrapHitsCow(b *testing.B) {
	cache := New(newCopyOnWriteMap())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Wrap(func() interface{} {
			return "xyz"
		})
	}
}
func BenchmarkWrapMisses(b *testing.B) {
	cache := nilCache()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Wrap(func() interface{} {
			return "xyz"
		})
	}
}
func BenchmarkWrapBusted(b *testing.B) {
	cache := nilCache()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.Bust(func() {
			cache.Wrap(func() interface{} {
				return "xyz"
			})
		})
	}
}
