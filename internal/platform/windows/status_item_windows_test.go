package windows

import (
	"syscall"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// Version 4 icons have no standard tooltip unless the shell is asked for one:
// Windows suppresses it in that mode, and the application is expected to draw
// its own. The icon must name itself, or hovering it shows nothing at all.
func TestStatusItemAsksTheShellForTheStandardTooltip(t *testing.T) {
	application := &windowsApplication{status: &statusState{window: 1}}
	item := &statusItem{application: application, tooltip: "Motty"}
	data := item.data()
	if data.Flags&notifyShowTip == 0 {
		t.Fatal("the status item does not ask for the standard tooltip that version 4 suppresses")
	}
	if got := syscall.UTF16ToString(data.Tip[:]); got != "Motty" {
		t.Fatalf("tip = %q, want the tooltip text", got)
	}
}

// Exercise the notification dispatcher, not only its button decoder. A menu
// must not swallow primary activation when the application also has a callback.
func TestStatusNotificationRoutesActivationAndContextMenu(t *testing.T) {
	for _, test := range []struct {
		name                    string
		callback, menu, removed bool
		events                  []uint32
		wantClicks, wantMenus   int
		wantButton              gfx.PointerButton
	}{
		{"left with callback and menu", true, true, false, []uint32{0x0201, 0x0202, ninSelect}, 1, 0, gfx.PointerButtonPrimary},
		{"keyboard activation", true, true, false, []uint32{ninKeySelect}, 1, 0, gfx.PointerButtonPrimary},
		{"right with callback and menu", true, true, false, []uint32{0x0204, 0x0205, wmContextMenu}, 0, 1, gfx.PointerButtonNone},
		{"left menu only", false, true, false, []uint32{ninSelect}, 0, 1, gfx.PointerButtonNone},
		{"right menu only", false, true, false, []uint32{wmContextMenu}, 0, 1, gfx.PointerButtonNone},
		{"left callback only", true, false, false, []uint32{ninSelect}, 1, 0, gfx.PointerButtonPrimary},
		{"right callback only", true, false, false, []uint32{wmContextMenu}, 1, 0, gfx.PointerButtonSecondary},
		{"no handlers", false, false, false, []uint32{ninSelect, wmContextMenu}, 0, 0, gfx.PointerButtonNone},
		{"removed", true, true, true, []uint32{ninSelect, wmContextMenu}, 0, 0, gfx.PointerButtonNone},
		{"hover", true, true, false, []uint32{0x0200}, 0, 0, gfx.PointerButtonNone},
	} {
		t.Run(test.name, func(t *testing.T) {
			clicks, menus := 0, 0
			button := gfx.PointerButtonNone
			item := &statusItem{id: 2}
			item.removed.Store(test.removed)
			if test.callback {
				item.onClick = func(click platform.StatusClick) { clicks++; button = click.Button }
			}
			if test.menu {
				item.menu = func() []platform.MenuItem { menus++; return nil }
			}
			status := &statusState{items: map[uint32]*statusItem{2: item}}
			for _, event := range test.events {
				status.notified(0, 0, uintptr(2<<16|event))
			}
			if clicks != test.wantClicks || menus != test.wantMenus || button != test.wantButton {
				t.Fatalf("clicks=%d menus=%d button=%v, want %d/%d/%v", clicks, menus, button, test.wantClicks, test.wantMenus, test.wantButton)
			}
		})
	}
}

func BenchmarkStatusPrimaryNotification(b *testing.B) {
	item := &statusItem{id: 2, onClick: func(platform.StatusClick) {}, menu: func() []platform.MenuItem { return nil }}
	status := &statusState{items: map[uint32]*statusItem{2: item}}
	b.ReportAllocs()
	for b.Loop() {
		status.notified(0, 0, 2<<16|ninSelect)
	}
}

func TestStatusHoverDeclinesOverflowAndLegacyShells(t *testing.T) {
	for _, class := range []string{"NotifyIconOverflowWindow", "TopLevelWindowForOverflowXamlIsland", ""} {
		if taskbarSupportsHover(class) {
			t.Fatalf("accepted %q", class)
		}
	}
	if !taskbarSupportsHover("Shell_TrayWnd") || !taskbarSupportsHover("Shell_SecondaryTrayWnd") {
		t.Fatal("visible taskbar does not support hover")
	}
	item := &statusItem{}
	if item.SetMenuOnHover(true) || item.SetHoverPanel(func(platform.ScreenRect) platform.Window { return nil }) {
		t.Fatal("legacy shell accepted hover")
	}
}

func TestStatusLegacyActivationUsesWParamOnce(t *testing.T) {
	clicks := 0
	item := &statusItem{id: 7, onClick: func(platform.StatusClick) { clicks++ }}
	status := &statusState{items: map[uint32]*statusItem{7: item}}
	status.notified(0, 7, wmLeftDown)
	status.notified(0, 7, wmLeftUp)
	status.notified(0, 7, ninSelect)
	status.notified(0, 1, wmLeftUp)
	if clicks != 1 {
		t.Fatalf("legacy activation fired %d times", clicks)
	}
}

func TestStatusHoverWaitsForShellCloseBeforeReopening(t *testing.T) {
	const id = 2
	opens := 0
	item := &statusItem{id: id, hoverPanel: func(platform.ScreenRect) platform.Window { opens++; return nil }}
	status := &statusState{items: map[uint32]*statusItem{id: item}}
	bounds := platform.ScreenRect{Width: 20, Height: 20}
	status.notified(0, 0, id<<16|wmMouseMove)
	if opens != 0 {
		t.Fatal("motion opened a panel without a shell popup")
	}
	status.openHover(item, bounds)
	status.notified(0, 0, 3<<16|ninPopupClose)
	status.openHover(item, bounds)
	if opens != 1 {
		t.Fatal("reopened without leaving the item")
	}
	status.notified(0, 0, id<<16|ninPopupClose)
	status.openHover(item, bounds)
	if opens != 2 {
		t.Fatal("failed to reopen after a new entry")
	}
}

func TestStatusHoverGapIsOneShot(t *testing.T) {
	status := &statusState{}
	panel := &nativeWindow{state: &windowState{hoverStatus: status}}
	status.hover.panel = panel
	panel.closed.Store(true)
	status.startHoverGap()
	status.finishHoverGap()
	if status.hover.panel != panel {
		t.Fatal("a queued timer closed a new hover session before its grace period")
	}
	status.hover.gapUntil = monotonicNow() - 1
	status.finishHoverGap()
	if status.hover.panel != nil || status.hover.gapUntil != 0 || panel.state.hoverStatus != nil {
		t.Fatal("a closed panel or its leave timer remained attached")
	}
}

// A single click arrives as several messages; exactly one of them may count, or
// a panel that a click opens is closed again by the same click.
func TestStatusMessagesOfOneClickCountOnce(t *testing.T) {
	const (
		leftUp   = 0x0202
		rightUp  = 0x0205
		moveOver = 0x0200
	)
	leftClick := []uint32{leftUp, ninSelect}
	rightClick := []uint32{rightUp, wmContextMenu}
	count := func(messages []uint32) (primary, secondary int) {
		for _, message := range messages {
			if button, ok := statusButton(message); ok {
				if button == gfx.PointerButtonPrimary {
					primary++
				} else {
					secondary++
				}
			}
		}
		return primary, secondary
	}
	if primary, secondary := count(leftClick); primary != 1 || secondary != 0 {
		t.Fatalf("a left click counted %d primary and %d secondary", primary, secondary)
	}
	if primary, secondary := count(rightClick); primary != 0 || secondary != 1 {
		t.Fatalf("a right click counted %d primary and %d secondary", primary, secondary)
	}
	// The keyboard sends only the select and context-menu messages.
	if primary, _ := count([]uint32{ninKeySelect}); primary != 1 {
		t.Fatal("selecting the icon from the keyboard did nothing")
	}
	if primary, secondary := count([]uint32{moveOver, 0x0201, 0x0203}); primary+secondary != 0 {
		t.Fatal("a message that is not a click counted as one")
	}
}

func TestTaskbarCreatedInvalidatesTheNotificationCarrier(t *testing.T) {
	const (
		statusWindow = uintptr(0x101)
		hostWindow   = uintptr(0x202)
		taskbarEvent = uint32(0x8000c123)
	)
	status := &statusState{
		window:         statusWindow,
		taskbarCreated: taskbarEvent,
		items:          make(map[uint32]*statusItem),
	}
	statusStates.Store(statusWindow, status)
	defer statusStates.Delete(statusWindow)

	previousCall := callNotifyIcon
	operation := ^uintptr(0)
	callNotifyIcon = func(operationHint uintptr, data *notifyIconData) (uintptr, uintptr, error) {
		operation = operationHint
		return 1, 0, nil
	}
	defer func() { callNotifyIcon = previousCall }()

	notificationMu.Lock()
	previousAdded, previousWindow := notificationAdded, notificationWindow
	notificationAdded, notificationWindow = true, hostWindow
	notificationMu.Unlock()
	defer func() {
		notificationMu.Lock()
		notificationAdded, notificationWindow = previousAdded, previousWindow
		notificationMu.Unlock()
	}()

	if result := statusProc(statusWindow, uintptr(taskbarEvent), 0, 0); result != 0 {
		t.Fatalf("TaskbarCreated returned %#x", result)
	}
	notificationMu.Lock()
	defer notificationMu.Unlock()
	if notificationAdded || notificationWindow != 0 {
		t.Fatalf("notification carrier survived TaskbarCreated: added=%v window=%#x", notificationAdded, notificationWindow)
	}
	if operation != notifyDelete {
		t.Fatalf("TaskbarCreated operation = %d, want NIM_DELETE", operation)
	}
}

func TestNotificationModifyFailureRetriesAdd(t *testing.T) {
	const host = uintptr(0x303)
	previousCall := callNotifyIcon
	operations := make([]uintptr, 0, 2)
	callNotifyIcon = func(operation uintptr, data *notifyIconData) (uintptr, uintptr, error) {
		operations = append(operations, operation)
		if operation == notifyModify {
			return 0, 0, syscall.EINVAL
		}
		return 1, 0, nil
	}
	defer func() { callNotifyIcon = previousCall }()

	notificationMu.Lock()
	previousAdded, previousWindow := notificationAdded, notificationWindow
	notificationAdded, notificationWindow = true, host
	notificationMu.Unlock()
	defer func() {
		notificationMu.Lock()
		notificationAdded, notificationWindow = previousAdded, previousWindow
		notificationMu.Unlock()
	}()

	data := notificationIconData(host, 7, "Motty", "Motty is still running")
	if err := sendNotificationIcon(&data); err != nil {
		t.Fatal(err)
	}
	if len(operations) != 3 || operations[0] != notifyModify || operations[1] != notifyDelete || operations[2] != notifyAdd {
		t.Fatalf("notification operations = %v, want [NIM_MODIFY NIM_DELETE NIM_ADD]", operations)
	}
	notificationMu.Lock()
	defer notificationMu.Unlock()
	if !notificationAdded || notificationWindow != host {
		t.Fatalf("recreated carrier was not recorded: added=%v window=%#x", notificationAdded, notificationWindow)
	}
}

// A modify that fails while the shell still holds the carrier, followed by an
// add that fails over it, must not wedge the carrier: the next notification
// deletes the leftover instead of adding over it again, which would fail
// forever, and the shell restart would be the only way out.
func TestNotificationCarrierRecoversAfterAModifyAndAddFailure(t *testing.T) {
	const host = uintptr(0x404)
	previousCall := callNotifyIcon
	var operations []uintptr
	failAdds := true
	callNotifyIcon = func(operation uintptr, data *notifyIconData) (uintptr, uintptr, error) {
		operations = append(operations, operation)
		switch {
		case operation == notifyModify, operation == notifyAdd && failAdds:
			return 0, 0, syscall.EINVAL
		}
		return 1, 0, nil
	}
	defer func() { callNotifyIcon = previousCall }()

	notificationMu.Lock()
	previousAdded, previousWindow := notificationAdded, notificationWindow
	notificationAdded, notificationWindow = true, host
	notificationMu.Unlock()
	defer func() {
		notificationMu.Lock()
		notificationAdded, notificationWindow = previousAdded, previousWindow
		notificationMu.Unlock()
	}()

	data := notificationIconData(host, 7, "Motty", "Motty is still running")
	if err := sendNotificationIcon(&data); err == nil {
		t.Fatal("a failed add reported success")
	}
	notificationMu.Lock()
	if notificationAdded || notificationWindow != host {
		notificationMu.Unlock()
		t.Fatalf("a failed add forgot the window it may still be under: added=%v window=%#x", notificationAdded, notificationWindow)
	}
	notificationMu.Unlock()

	failAdds = false
	if err := sendNotificationIcon(&data); err != nil {
		t.Fatal(err)
	}
	want := []uintptr{notifyModify, notifyDelete, notifyAdd, notifyDelete, notifyAdd}
	if len(operations) != len(want) {
		t.Fatalf("notification operations = %v, want %v", operations, want)
	}
	for i := range want {
		if operations[i] != want[i] {
			t.Fatalf("notification operations = %v, want %v", operations, want)
		}
	}
	notificationMu.Lock()
	defer notificationMu.Unlock()
	if !notificationAdded || notificationWindow != host {
		t.Fatalf("the carrier did not recover: added=%v window=%#x", notificationAdded, notificationWindow)
	}
}

func TestHoverPanelUsesShellPopupNotifications(t *testing.T) {
	item := &statusItem{application: &windowsApplication{status: &statusState{}}}
	if item.data().Flags&notifyShowTip == 0 {
		t.Fatal("regular tooltip disabled")
	}
	item.hoverPanel = func(platform.ScreenRect) platform.Window { return nil }
	if item.data().Flags&notifyShowTip != 0 {
		t.Fatal("shell tooltip suppresses rich hover")
	}
}
