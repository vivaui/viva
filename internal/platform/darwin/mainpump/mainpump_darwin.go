//go:build darwin && !ios

// Package mainpump lets a test binary serve the main dispatch queue.
//
// The platform package hops to the main thread with dispatch_sync, which a Go test
// binary never serves: the main thread sits in the testing package, so a test that
// reaches such a call passes or hangs by luck of which thread its goroutine runs on.
// Only test binaries import this package; nothing in an application links it.
package mainpump

/*
#cgo darwin LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

static void viva_pump_main_once(void) {
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.01, true);
}
*/
import "C"

import (
	"runtime"
	"testing"
)

func init() {
	// The main goroutine starts on the main thread; keep it there.
	runtime.LockOSThread()
}

// Run runs the tests on another goroutine while this one, which is on the main
// thread, serves the main dispatch queue until they finish, and exits with their
// result. Call it from TestMain.
func Run(m *testing.M) int {
	result := make(chan int, 1)
	go func() { result <- m.Run() }()
	for {
		select {
		case code := <-result:
			return code
		default:
			C.viva_pump_main_once()
		}
	}
}
