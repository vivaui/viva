//go:build darwin && !ios

package darwin

import (
	"errors"
	"github.com/vivaui/viva/internal/platform"
	"testing"
)

func TestNotificationNativeActivationConsumesCallback(t *testing.T) {
	calls := 0
	id := notificationCallbacks.Add(platform.NotificationEvents{Activated: func() { calls++ }})
	activateNotification(id)
	activateNotification(id)
	if calls != 1 || notificationCallbacks.Has(id) {
		t.Fatal("native activation was not consumed exactly once")
	}
	id = notificationCallbacks.Add(platform.NotificationEvents{Activated: func() { calls++ }})
	notificationCallbacks.Remove(id)
	activateNotification(id)
	if calls != 1 {
		t.Fatal("cancelled native identity was reactivated")
	}
}
func TestNotificationNativeRejectsNULBeforeDelivery(t *testing.T) {
	if close, err := ShowNotification("bad\x00title", "body", platform.NotificationEvents{}); err == nil || close != nil {
		t.Fatal("invalid native string reached notification center")
	}
	if close, err := ShowNotification("title", "bad\x00body", platform.NotificationEvents{}); err == nil || close != nil {
		t.Fatal("invalid native body reached notification center")
	}
}

func TestNotificationNativeFailureAndLateCancellation(t *testing.T) {
	calls := 0
	var got error
	id := notificationCallbacks.Add(platform.NotificationEvents{Failed: func(err error) { calls++; got = err }})
	failNotification(id, "denied", true)
	failNotification(id, "duplicate", false)
	if calls != 1 || !errors.Is(got, ErrPermissionDenied) || notificationCallbacks.Has(id) {
		t.Fatal("permission failure did not consume native callback", calls, got)
	}
	id = notificationCallbacks.Add(platform.NotificationEvents{Failed: func(error) { t.Error("cancelled failure delivered") }})
	notificationCallbacks.Remove(id)
	failNotification(id, "late failure", false)
}
