package funcache

import (
	"reflect"
	"runtime"
)

const cacheBustingFn = "github.com/aviddiviner/funcache.(*Cache).Bust"

var cacheBustingFnPc uintptr

// Return the program counters of function invocations all the way up the stack.
func getAllCallers(skip int) (pcs []uintptr) {
	// Arbitrarily do this in batches of 64
	var batch = make([]uintptr, 64)
	var n int
	for {
		n = runtime.Callers(skip, batch)
		pcs = append(pcs, batch[:n]...)
		if n < len(batch) {
			break
		}
		skip += n
	}
	return
}

// Check if any of the callers were our cache busting function.
func wasCalledByCacheBustingFn() bool {
	// Skip the first 3 callers:
	// 1. runtime.Callers
	// 2. github.com/aviddiviner/funcache.getAllCallers
	// 3. github.com/aviddiviner/funcache.wasCalledByCacheBustingFn
	//
	// From there on it should be:
	// 4. github.com/aviddiviner/funcache.(*Cache).Wrap
	// ...
	pcs := getAllCallers(3)
	for _, pc := range pcs {
		if pc == cacheBustingFnPc {
			return true
		}
	}
	return false
}

func getFnName(fn func() interface{}) string {
	ptr := reflect.ValueOf(fn).Pointer()
	return runtime.FuncForPC(ptr).Name()
}

func init() {
	nilCache().Bust(func() {
		cacheBustingFnPc, _, _, _ = runtime.Caller(1)
	})
	// Sanity check that we have the right cache busting function
	fn := runtime.FuncForPC(cacheBustingFnPc)
	if fn.Name() != cacheBustingFn {
		panic("funcache: init: unable to identify cache busting func")
	}
}
