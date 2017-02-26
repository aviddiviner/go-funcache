package funcache

import (
	"testing"

	"github.com/hashicorp/golang-lru"
	"github.com/stretchr/testify/assert"
)

func TestCaller(t *testing.T) {
	assert.True(t, wasCalledBy("github.com/aviddiviner/funcache.TestCaller"))
	func() {
		assert.True(t, wasCalledBy("github.com/aviddiviner/funcache.TestCaller"))
	}()
}

func TestBasics(t *testing.T) {
	cache := NewInMemCache()

	var callCount int
	getFoo := func() string {
		return cache.Wrap("foo", func() interface{} {
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
	gotVal := cache.Wrap(key, func() interface{} {
		gotBust = true
		return val
	})
	assert.Equal(t, val, gotVal)
	assert.Equal(t, bust, gotBust)
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
