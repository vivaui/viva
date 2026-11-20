package linux

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// Run under a dedicated X11 display with an EWMH window manager and xdotool.
func TestUserCanRaiseSecondaryWindow(t *testing.T) {
	if os.Getenv("VIVA_LINUX_WM_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_WM_SMOKE=1 under an isolated window manager")
	}
	t.Setenv("VIVA_SOFTWARE_RENDERING", "1")
	application := &linuxApplication{}
	t.Cleanup(application.closeAll)
	open := func(title string) *nativeWindow {
		control, err := application.OpenWindow(platform.Config{
			Title: title, Class: title, Width: 200, Height: 140, WindowMode: platform.WindowCustomTitlebar,
			OnPointer:  func(platform.PointerEvent) (platform.Frame, error) { return platform.Frame{}, nil },
			OnMinimize: func(bool) {},
		})
		if err != nil {
			t.Fatal(err)
		}
		return control.(*nativeWindow)
	}
	wait := func(description string, condition func() bool) {
		t.Helper()
		for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); {
			for _, state := range application.windows {
				for event := state.nextEvent(); event != nil; event = state.nextEvent() {
					closed, err := state.handleEvent(event)
					state.win.api.free(event)
					if closed || err != nil {
						t.Fatalf("%s: closed=%v, err=%v", description, closed, err)
					}
				}
			}
			if condition() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", description)
	}
	main := open("VivaMain")
	win := main.state.win
	root := &window{api: win.api, connection: win.connection, id: win.screen.Root}
	front := func(target *nativeWindow) bool {
		active, _ := windowProperty(t, root, "_NET_ACTIVE_WINDOW", 0)
		stack, _ := windowProperty(t, root, "_NET_CLIENT_LIST_STACKING", 0)
		id := target.state.win.id
		return len(active) == 4 && binary.LittleEndian.Uint32(active) == id &&
			len(stack) >= 4 && binary.LittleEndian.Uint32(stack[len(stack)-4:]) == id
	}
	xdotool := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("xdotool", args...).CombinedOutput(); err != nil {
			t.Fatalf("xdotool %v: %v: %s", args, err, output)
		}
	}
	wait("main window mapped", func() bool { return main.state.exposed })
	clickMain := func() {
		before := application.lastUserTime
		xdotool("mousemove", "--window", fmt.Sprint(win.id), "80", "80", "click", "1")
		wait("user action in main window", func() bool { return application.lastUserTime != before })
	}
	clickMain()
	wait("clicked main window active and in front", func() bool { return front(main) })
	child := open("VivaHistory")
	child.Show()
	wait("new history window active and in front", func() bool { return front(child) })
	property, _ := windowProperty(t, child.state.win, "_NET_WM_USER_TIME", xcbAtomCardinal)
	if len(property) != 4 || binary.LittleEndian.Uint32(property) != application.lastUserTime {
		t.Fatalf("secondary window user time = %v, want %d", property, application.lastUserTime)
	}

	for _, state := range []string{"covered", "minimized", "hidden"} {
		switch state {
		case "minimized":
			xdotool("windowminimize", fmt.Sprint(child.state.win.id))
			wait("iconic history window", func() bool { return child.state.win.minimized })
		case "hidden":
			child.Hide()
			wait("withdrawn history window", func() bool {
				data, _ := windowProperty(t, root, "_NET_CLIENT_LIST", 0)
				for len(data) >= 4 {
					if binary.LittleEndian.Uint32(data) == child.state.win.id {
						return false
					}
					data = data[4:]
				}
				return child.state.hidden
			})
		}
		main.Show()
		wait("main window above "+state+" history", func() bool { return front(main) })
		clickMain()
		child.Show()
		wait(state+" history window active and in front", func() bool { return front(child) && child.state.exposed })
	}
}
