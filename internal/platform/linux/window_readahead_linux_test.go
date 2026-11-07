package linux

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func queuedReadAheadState(events []unsafe.Pointer) *linuxWindowState {
	return &linuxWindowState{win: &window{id: 1, api: &xcbAPI{
		pollForEvent: func(uintptr) unsafe.Pointer {
			if len(events) == 0 {
				return nil
			}
			event := events[0]
			events = events[1:]
			return event
		},
		free: func(unsafe.Pointer) {},
	}}}
}

func TestConfigureReadAheadKeepsDragReleaseBeforeLaterKeys(t *testing.T) {
	configure := xcbConfigureEvent{ResponseType: xcbConfigureNotify, Window: 1, Width: 400, Height: 300}
	motions := []xcbInputEvent{
		{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask, EventX: 10},
		{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask, EventX: 20},
	}
	release := xcbInputEvent{ResponseType: xcbButtonRelease, Event: 1, State: xcbButton1Mask}
	key := xcbInputEvent{ResponseType: xcbKeyPress, Event: 1}
	state := queuedReadAheadState([]unsafe.Pointer{
		unsafe.Pointer(&configure), unsafe.Pointer(&motions[0]), unsafe.Pointer(&motions[1]),
		unsafe.Pointer(&release), unsafe.Pointer(&key),
	})
	if state.nextEvent() != unsafe.Pointer(&configure) {
		t.Fatal("configure was lost")
	}
	motion := (*xcbInputEvent)(state.nextEvent())
	if motion == nil || motion.ResponseType != xcbMotionNotify || motion.EventX != 20 {
		t.Fatalf("replayed drag = %+v, want its final position before the release", motion)
	}
	if state.nextEvent() != unsafe.Pointer(&release) {
		t.Fatal("drag replay lost or overtook the button release")
	}
	if state.nextEvent() != unsafe.Pointer(&key) || state.nextEvent() != nil {
		t.Fatal("drag replay lost or duplicated later keyboard input")
	}
}

func TestHeldDragDoesNotReadPastItsButtonRelease(t *testing.T) {
	motion := xcbInputEvent{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask, EventX: 10}
	release := xcbInputEvent{ResponseType: xcbButtonRelease, Event: 1, State: xcbButton1Mask}
	key := xcbInputEvent{ResponseType: xcbKeyPress, Event: 1}
	laterMotion := xcbInputEvent{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask, EventX: 30}
	state := queuedReadAheadState([]unsafe.Pointer{unsafe.Pointer(&laterMotion)})
	state.held = unsafe.Pointer(&motion)
	state.heldQueue = []unsafe.Pointer{unsafe.Pointer(&release), unsafe.Pointer(&key)}
	if got := (*xcbInputEvent)(state.nextEvent()); got != &motion || got.EventX != 10 {
		t.Fatalf("held drag = %+v, want its position before the held release", got)
	}
	if state.nextEvent() != unsafe.Pointer(&release) || state.nextEvent() != unsafe.Pointer(&key) {
		t.Fatal("held input boundaries were overwritten or reordered")
	}
	if state.nextEvent() != unsafe.Pointer(&laterMotion) || state.nextEvent() != nil {
		t.Fatal("motion after the input boundaries was lost")
	}
}

func TestHeldConfigureDoesNotReadPastInputBoundaries(t *testing.T) {
	configure := xcbConfigureEvent{ResponseType: xcbConfigureNotify, Window: 1, Width: 400, Height: 300}
	release := xcbInputEvent{ResponseType: xcbButtonRelease, Event: 1, State: xcbButton1Mask}
	key := xcbInputEvent{ResponseType: xcbKeyPress, Event: 1}
	laterConfigure := xcbConfigureEvent{ResponseType: xcbConfigureNotify, Window: 1, Width: 800, Height: 600}
	state := queuedReadAheadState([]unsafe.Pointer{unsafe.Pointer(&laterConfigure)})
	state.held = unsafe.Pointer(&configure)
	state.heldQueue = []unsafe.Pointer{unsafe.Pointer(&release), unsafe.Pointer(&key)}
	if got := (*xcbConfigureEvent)(state.nextEvent()); got != &configure || got.Width != 400 {
		t.Fatalf("held configure = %+v, want its size before the input boundary", got)
	}
	if state.nextEvent() != unsafe.Pointer(&release) || state.nextEvent() != unsafe.Pointer(&key) {
		t.Fatal("configure replay overtook held input")
	}
	if state.nextEvent() != unsafe.Pointer(&laterConfigure) || state.nextEvent() != nil {
		t.Fatal("configure after the input boundaries was lost")
	}
}

func TestKeyReleaseReadAheadChecksHeldAutorepeatFirst(t *testing.T) {
	release := xcbInputEvent{ResponseType: xcbKeyRelease, Detail: 24, Time: 100}
	repeat := xcbInputEvent{ResponseType: xcbKeyPress, Detail: 24, Time: 100}
	boundary := xcbInputEvent{ResponseType: xcbButtonRelease, Event: 1}
	laterKey := xcbInputEvent{ResponseType: xcbKeyPress, Detail: 25, Time: 101}
	state := queuedReadAheadState([]unsafe.Pointer{unsafe.Pointer(&laterKey)})
	state.router.text = &linuxTextInput{enabled: true, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	state.held = unsafe.Pointer(&repeat)
	state.heldQueue = []unsafe.Pointer{unsafe.Pointer(&boundary)}
	if state.keyReleased(&release) {
		t.Fatal("a held autorepeat press was mistaken for a real key release")
	}
	if state.nextEvent() != unsafe.Pointer(&repeat) || state.nextEvent() != unsafe.Pointer(&boundary) || state.nextEvent() != unsafe.Pointer(&laterKey) {
		t.Fatal("key release read-ahead lost or reordered pending input")
	}
}
