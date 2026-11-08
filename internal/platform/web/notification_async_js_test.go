//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"
	"time"
)

func installNotification(t *testing.T, script string) {
	t.Helper()
	notification := js.Global().Get("Function").New(script).Invoke()
	js.Global().Get("Object").Call("defineProperty", js.Global(), "Notification",
		map[string]any{"value": notification, "configurable": true, "writable": true})
}

func requestNotificationPermission(t *testing.T) (bool, error) {
	t.Helper()
	type result struct {
		granted bool
		err     error
	}
	done := make(chan result, 1)
	if err := RequestNotificationPermission(func(granted bool, err error) { done <- result{granted, err} }); err != nil {
		return false, err
	}
	select {
	case got := <-done:
		return got.granted, got.err
	case <-time.After(5 * time.Second):
		t.Fatal("the permission request never completed")
		return false, nil
	}
}

func TestRequestNotificationPermission(t *testing.T) {
	installNotification(t, `const Notification = function() {};
		Notification.permission = "default";
		Notification.requestPermission = () => Promise.resolve("granted");
		return Notification;`)
	granted, err := requestNotificationPermission(t)
	if err != nil || !granted {
		t.Fatalf("granted = %v, %v", granted, err)
	}

	installNotification(t, `const Notification = function() {};
		Notification.permission = "default";
		Notification.requestPermission = callback => { callback("denied") };
		return Notification;`)
	granted, err = requestNotificationPermission(t)
	if err != nil || granted {
		t.Fatalf("denied = %v, %v", granted, err)
	}
}

func TestRequestNotificationPermissionRequiresTheBrowserAPI(t *testing.T) {
	js.Global().Get("Object").Call("defineProperty", js.Global(), "Notification",
		map[string]any{"value": js.Undefined(), "configurable": true, "writable": true})
	if _, err := requestNotificationPermission(t); err == nil {
		t.Fatal("a missing Notification API was accepted")
	}
}
