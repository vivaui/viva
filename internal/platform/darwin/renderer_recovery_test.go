//go:build darwin

package darwin

import (
	"strings"
	"testing"
	"time"
)

// A failed instance buffer or texture is the renderer losing its resources: the
// window makes them again and the frame is sent once more. A range the scene got
// wrong is not: making the device again would send the same rejected frame.
func TestOnlyResourceFailuresRecoverTheDevice(t *testing.T) {
	if err := rectangleUpdateError(2); err == nil || isResourceError(err) {
		t.Fatalf("a rejected update range was taken for a lost device: %v", err)
	}
	if err := rectangleUpdateError(3); err == nil || !isResourceError(err) {
		t.Fatalf("a failed buffer build was not taken for a lost device: %v", err)
	}
	if err := rectangleUpdateError(0); err != nil {
		t.Fatalf("a successful update returned an error: %v", err)
	}
}

// Losses that keep arriving close together stop the window instead of making
// the device again for good.
func TestRepeatedResourceLossesStopTheWindow(t *testing.T) {
	state := &runState{}
	now := time.Now()
	for range 3 {
		state.devices.Next(now)
	}
	err := state.recoverRenderer(nil, Frame{}, false, &resourceError{"buffer"})
	if err == nil || !strings.Contains(err.Error(), "again and again") {
		t.Fatalf("the fourth loss in a row did not stop the window: %v", err)
	}
}
