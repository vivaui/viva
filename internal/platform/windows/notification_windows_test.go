package windows

import (
	"github.com/vivaui/viva/internal/platform"
	"runtime"
	"syscall"
	"testing"
)

func TestNotificationActivationIdentityAndCancellation(t *testing.T) {
	previous := callNotifyIcon
	callNotifyIcon = func(uintptr, *notifyIconData) (uintptr, uintptr, error) { return 1, 0, nil }
	defer func() { callNotifyIcon = previous }()
	const host = uintptr(0x1234)
	id := notificationIDNamespace | 123
	calls := 0
	interactiveNotifications.Lock()
	if interactiveNotifications.entries == nil {
		interactiveNotifications.entries = make(map[uint32]interactiveNotification)
	}
	interactiveNotifications.entries[id] = interactiveNotification{host: host, activate: func() { calls++ }}
	interactiveNotifications.Unlock()
	activateSystemNotification(host+1, id, notificationBalloonClick)
	activateSystemNotification(host, id, 0)
	if calls != 0 {
		t.Fatal("wrong host or event activated a notification")
	}
	wakeProc(host, wmNotificationActivation, uintptr(id), notificationBalloonClick)
	wakeProc(host, wmNotificationActivation, uintptr(id), notificationBalloonClick)
	if calls != 1 {
		t.Fatalf("activation calls = %d", calls)
	}
	interactiveNotifications.Lock()
	interactiveNotifications.entries[id+1] = interactiveNotification{host: host, activate: func() { calls++ }}
	interactiveNotifications.Unlock()
	removeInteractiveNotifications(host)
	activateSystemNotification(host, id+1, notificationBalloonClick)
	if calls != 1 {
		t.Fatal("closed host retained activation")
	}
}

func TestNotificationTruncationPreservesUTF16(t *testing.T) {
	var short [3]uint16
	copyUTF16(short[:], "a😀b")
	if short[0] != 'a' || short[1] != 0 {
		t.Fatal("truncated title retained half a surrogate pair", short)
	}
	var fitting [4]uint16
	copyUTF16(fitting[:], "a😀b")
	if fitting[0] != 'a' || fitting[1] != 0xd83d || fitting[2] != 0xde00 || fitting[3] != 0 {
		t.Fatal("title lost a complete surrogate pair", fitting)
	}
	copyUTF16(nil, "ignored")
}

// The Shell call is mocked, so this exercises the real message-only carrier and
// queued lifecycle without displaying a notification or opening a visual window.
func TestNotificationCarrierAndQueuedCancellation(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	application := &windowsApplication{}
	if err := application.openWakeWindow(); err != nil {
		t.Fatal(err)
	}
	previous := callNotifyIcon
	defer func() { application.closeWakeWindow(); callNotifyIcon = previous }()
	var data notifyIconData
	adds, calls := 0, 0
	callNotifyIcon = func(operation uintptr, value *notifyIconData) (uintptr, uintptr, error) {
		if operation == notifyAdd {
			data = *value
			adds++
		}
		return 1, 0, nil
	}
	events := platform.NotificationEvents{Activated: func() { calls++ }, Failed: func(err error) { t.Error(err) }}
	cancel, err := ShowNotification("Title only", "", events)
	if err != nil {
		t.Fatal(err)
	}
	application.drainPosts()
	if adds != 1 || data.Window != application.wakeWindow.Load() || syscall.UTF16ToString(data.Info[:]) != "Title only" || data.State&notifyHidden == 0 || data.StateMask&notifyHidden == 0 || data.Flags&notifyState == 0 {
		t.Fatal("request lost its application carrier, title-only content or hidden state")
	}
	oldID := data.ID
	cancel()
	wakeProc(data.Window, wmNotificationActivation, uintptr(oldID), notificationBalloonClick)
	application.drainPosts()
	if calls != 0 {
		t.Fatal("late click escaped queued cancellation")
	}
	cancel, err = ShowNotification("Second", "body", events)
	if err != nil {
		t.Fatal(err)
	}
	application.drainPosts()
	if data.ID == oldID {
		t.Fatal("notification identity was reused")
	}
	wakeProc(data.Window, wmNotificationActivation, uintptr(oldID), notificationBalloonClick)
	wakeProc(data.Window, wmNotificationActivation, uintptr(data.ID), notificationBalloonClick)
	wakeProc(data.Window, wmNotificationActivation, uintptr(data.ID), notificationBalloonClick)
	if calls != 1 {
		t.Fatal("click identity did not select one live notification", calls)
	}
	cancel()
	application.drainPosts()
	cancel, err = ShowNotification("Cancelled before posting", "body", events)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	application.drainPosts()
	if adds != 2 {
		t.Fatal("cancelled queued request reached the Shell")
	}
}

func TestNotificationShellFailureReleasesIdentity(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	application := &windowsApplication{}
	if err := application.openWakeWindow(); err != nil {
		t.Fatal(err)
	}
	previous := callNotifyIcon
	defer func() { application.closeWakeWindow(); callNotifyIcon = previous }()
	var id uint32
	callNotifyIcon = func(operation uintptr, data *notifyIconData) (uintptr, uintptr, error) {
		id = data.ID
		if operation == notifyAdd {
			return 0, 0, syscall.EINVAL
		}
		return 1, 0, nil
	}
	errors := 0
	cancel, err := ShowNotification("title", "body", platform.NotificationEvents{Failed: func(err error) {
		if err == nil {
			t.Error("nil native failure")
		}
		errors++
	}})
	if err != nil {
		t.Fatal(err)
	}
	application.drainPosts()
	cancel()
	application.drainPosts()
	interactiveNotifications.Lock()
	_, retained := interactiveNotifications.entries[id]
	interactiveNotifications.Unlock()
	if errors != 1 || retained {
		t.Fatal("Shell failure retained a callback", errors, retained)
	}
}
