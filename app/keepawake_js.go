//go:build js && wasm

package app

// nativePreventSleep has no page API to ask: a page cannot keep the machine
// awake.
func nativePreventSleep(string) (func(), error) { return nil, ErrSleepPreventionUnavailable }
