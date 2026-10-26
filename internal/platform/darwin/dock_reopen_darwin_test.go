package darwin

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeDockReopenUsesWindowHandler(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "dock-reopen")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/dock_reopen.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native Dock reopen test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native Dock reopen: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}

func TestNativeDockReopenBridgePreservesQueuedVisibility(t *testing.T) {
	// A real main-thread process crosses the cgo bridge instead of callback stubs.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	executable := filepath.Join(t.TempDir(), "reopen-bridge-test")
	command := exec.CommandContext(ctx, "go", "build", "-o", executable, "./testdata/reopen_bridge")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native reopen bridge test: %v\n%s", err, output)
	}
	ctx, cancelRun := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRun()
	output, err := exec.CommandContext(ctx, executable).CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 77 {
		t.Skipf("native reopen bridge unavailable: %s", output)
	}
	if err != nil {
		t.Fatalf("native reopen bridge: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
