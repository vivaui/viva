//go:build darwin && !ios

package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

const wrongStartupThreadSubprocess = "VIVA_TEST_WRONG_STARTUP_THREAD"

func init() {
	if os.Getenv(wrongStartupThreadSubprocess) == "1" {
		// Keep the process main thread in TestMain without a Cocoa event
		// loop. The test goroutine must get an error without waiting for it.
		runtime.LockOSThread()
		if err := checkNativeApplicationThread(); err != nil {
			panic(err)
		}
	}
}

func TestNativeStartupRejectsNonMainThread(t *testing.T) {
	if os.Getenv(wrongStartupThreadSubprocess) != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeStartupRejectsNonMainThread$")
		cmd.Env = append(os.Environ(), wrongStartupThreadSubprocess+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("off-main startup subprocess: %v\n%s", err, output)
		}
		return
	}

	config := Config{Title: "Main-thread startup test", Width: 320, Height: 240}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"Run", func() error { return Run(config) }},
		{"RunCanvas", func() error { return RunCanvas(config, nil) }},
		{"RunApplication", func() error {
			return RunApplication(func(*Application) error {
				return errors.New("startup unexpectedly reached the application callback")
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil || !strings.Contains(err.Error(), "process main thread") ||
				!strings.Contains(err.Error(), "runtime.LockOSThread") {
				t.Fatalf("expected an actionable process main-thread error, got %v", err)
			}
		})
	}
}
