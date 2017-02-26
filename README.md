# funcache [![GoDoc](https://godoc.org/github.com/aviddiviner/go-funcache?status.svg)](https://godoc.org/github.com/aviddiviner/go-funcache)
`funcache` provides an easy way to do fine-grained caching of function values in Go.

## Usage

The two main functions are: `Wrap(fn)` which wraps a function and caches its return value, and `Bust(fn)` which will bust any caching for function calls inside of it.

```go
cache := funcache.NewInMemCache()

func readConfig() Config {
    return cache.Wrap(func() interface{} {

        // Do some time consuming work, for example:
        data, _ := ioutil.ReadFile("config.yaml")
        var cfg Config
        yaml.Unmarshal(data, &cfg)
        return cfg

    }).(Config)
}

readConfig() // Does the work, and caches it
readConfig() // Reads from cache

cache.Bust(func() {
    readConfig()      // Does the work again, and caches it
    func() {          // A deeply nested function call
        readConfig()  // Does the work again, and caches it
    }
    anotherCachedFn() // Does the work again, and caches it
})
```

The simplicity of this API means you don't need to track if some deeply nested values have been cached. Also, you don't need to keep track of keys for cached objects, to invalidate them. Just wrap your call in a `cache.Bust()` and you can be sure no cached values will be used. This makes fragment caching (Russian doll caching) simple and painless.

You can also use other backing stores (other than the simple in-memory one), such as [LRU cache](https://github.com/hashicorp/golang-lru). As long as it implements the `Store` interface, it can be used.

```go
import "github.com/hashicorp/golang-lru"

store, _ := lru.New2Q(100)
cache := funcache.New(store)
```

For more info, be sure to check out [the docs](https://godoc.org/github.com/aviddiviner/go-funcache).
