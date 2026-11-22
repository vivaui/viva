package linux

import "testing"

// Windows draw with one Vulkan device: the second window makes no instance or
// device of its own, closing one window leaves the others drawing, and the
// device goes when the last window does.
func TestWindowsShareOneVulkanDevice(t *testing.T) {
	_, first := newWindowRenderer(t, nil)
	_, second := newWindowRenderer(t, nil)
	if first.shared == nil || first.shared != second.shared || first.device != second.device || first.instance != second.instance {
		t.Fatalf("windows draw with devices %p and %p", first.shared, second.shared)
	}
	shared := first.shared
	if err := first.present(); err != nil {
		t.Fatal(err)
	}
	if err := second.present(); err != nil {
		t.Fatal(err)
	}
	second.close()
	if shared.users != 1 || shared.handle == 0 {
		t.Fatalf("closing one window left %d users, device %d", shared.users, shared.handle)
	}
	if err := first.present(); err != nil {
		t.Fatalf("the remaining window cannot draw: %v", err)
	}
	first.close()
	if shared.handle != 0 || shared.instance != 0 || windowDevice.current != nil {
		t.Fatal("the device outlived its last window")
	}
}

// A lost device is lost for every window. The window that finds out first makes
// itself again on a new device; another keeps the old one until it finds out
// too, and then joins the new one.
func TestALostDeviceIsReplacedForEveryWindow(t *testing.T) {
	_, first := newWindowRenderer(t, nil)
	_, second := newWindowRenderer(t, nil)
	old := first.shared
	loseOnce := func(renderer *vulkanRenderer) {
		present := renderer.api.queuePresent
		lost := true
		renderer.api.queuePresent = func(queue uintptr, info *vkPresentInfo) int32 {
			if lost {
				lost = false
				return vkErrorDeviceLost
			}
			return present(queue, info)
		}
	}
	loseOnce(first)
	if err := first.present(); err != nil {
		t.Fatalf("present after a lost device: %v", err)
	}
	if first.shared == old || !old.lost || second.shared != old {
		t.Fatalf("after the loss the windows draw with %p and %p (old %p, lost %v)", first.shared, second.shared, old, old.lost)
	}
	loseOnce(second)
	if err := second.present(); err != nil {
		t.Fatalf("present after the other window replaced the device: %v", err)
	}
	if second.shared != first.shared || old.handle != 0 {
		t.Fatalf("the second window draws with %p, not the new %p; old device %d", second.shared, first.shared, old.handle)
	}
	if err := first.present(); err != nil {
		t.Fatal(err)
	}
}
