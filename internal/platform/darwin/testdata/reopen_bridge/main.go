//go:build darwin

// This subprocess exercises the real Cocoa/Go bridge and exits without input.
package main

/*
#cgo LDFLAGS: -framework Cocoa -framework Metal
int reopen_bridge_available(void);
void reopen_bridge_start(void);
*/
import "C"

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/vivaui/viva/app"
)

func init() { runtime.LockOSThread() }

var state struct {
	application    *app.Application
	window         *app.Window
	closeRequests  int
	waitingForShow bool
	deadline       time.Time
	failure        error
	completed      bool
}

//export reopenBridgeStep
func reopenBridgeStep(visible C.int) C.int {
	fail := func(message string) C.int {
		state.failure = fmt.Errorf("%s: visible=%t hidden=%t closes=%d", message, visible != 0, state.window.Hidden(), state.closeRequests)
		state.application.Quit()
		return 3
	}
	if time.Now().After(state.deadline) {
		return fail("visibility transition timed out")
	}
	if state.window.Closed() {
		return fail("a close-to-hide request closed the window")
	}
	if state.waitingForShow {
		if visible == 0 {
			return 0
		}
		if state.window.Hidden() {
			return fail("Dock showed the native window without clearing Hidden")
		}
		state.waitingForShow = false
		return 1
	}
	if visible != 0 {
		return 0
	}
	if !state.window.Hidden() {
		return fail("the native hide lost its cached state")
	}
	if state.closeRequests == 3 {
		state.completed = true
		state.application.Quit()
		return 3
	}
	// A Dock reopen must follow these already-queued requests, not bypass them.
	state.window.Show()
	state.window.Hide()
	state.waitingForShow = true
	return 2
}

func main() {
	if C.reopen_bridge_available() == 0 {
		fmt.Fprintln(os.Stderr, "no Metal device")
		os.Exit(77)
	}
	err := app.RunApplication(func(application *app.Application) error {
		state.application = application
		var err error
		state.window, err = application.OpenWindow(app.Config{
			Title: "Viva reopen bridge regression", Width: 240, Height: 120,
			OnCloseRequest: func(allow func()) {
				state.closeRequests++
				state.window.Hide()
			},
		}, nil)
		if err != nil {
			return err
		}
		state.deadline = time.Now().Add(5 * time.Second)
		state.window.Hide()
		C.reopen_bridge_start()
		return nil
	})
	if state.failure != nil {
		err = state.failure
	} else if err == nil && !state.completed {
		err = fmt.Errorf("native bridge exited before completing the close/reopen sequence")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("real Cocoa/Go bridge: 3 red-close cycles preserve queued Show/Hide order")
}
