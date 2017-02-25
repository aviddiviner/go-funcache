package funcache

import (
	"runtime"
)

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

// Check if any of the calling stack frames were our
func wasCalledBy(funcName string) bool {
	// Skip the first 3 callers:
	// 1. runtime.Callers
	// 2. main.getAllCallers
	// 3. main.wasCalledBy
	pcs := getAllCallers(3)
	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()
		// println(frame.Function)
		if frame.Function == funcName {
			return true
		}
		if !more {
			break
		}
	}
	return false
}
