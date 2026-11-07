package linux

import (
	"testing"
	"unsafe"
)

func TestQueuedDragMotionKeepsLatestPositionBeforeRelease(t *testing.T) {
	events := make([]xcbInputEvent, 101)
	for index := range 100 {
		events[index] = xcbInputEvent{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask, EventX: int16(index)}
	}
	events[100] = xcbInputEvent{ResponseType: xcbButtonRelease, Event: 1, State: xcbButton1Mask}
	read := 0
	var freed []unsafe.Pointer
	state := linuxWindowState{win: &window{api: &xcbAPI{
		pollForEvent: func(uintptr) unsafe.Pointer {
			if read == len(events) {
				return nil
			}
			event := &events[read]
			read++
			return unsafe.Pointer(event)
		},
		free: func(event unsafe.Pointer) { freed = append(freed, event) },
	}}}
	motion := (*xcbInputEvent)(state.nextEvent())
	if motion == nil || motion.ResponseType != xcbMotionNotify || motion.EventX != 99 {
		t.Fatalf("drag delivered stale position: %+v", motion)
	}
	if release := state.nextEvent(); release != unsafe.Pointer(&events[100]) {
		t.Fatal("drag compression lost or reordered the button release")
	}
	if len(freed) != 99 || state.nextEvent() != nil {
		t.Fatalf("discarded motion count = %d, want 99, with an empty queue", len(freed))
	}
	for _, discarded := range freed {
		if discarded == unsafe.Pointer(motion) {
			t.Fatal("the delivered drag position was freed")
		}
	}
}

func TestQueuedDragMotionPreservesInputBoundaries(t *testing.T) {
	for _, boundary := range []xcbInputEvent{
		{ResponseType: xcbMotionNotify, Event: 2, State: xcbButton1Mask},
		{ResponseType: xcbMotionNotify, Event: 1, Root: 2, State: xcbButton1Mask},
		{ResponseType: xcbMotionNotify, Event: 1, Child: 2, State: xcbButton1Mask},
		{ResponseType: xcbMotionNotify, Event: 1, SameScreen: 1, State: xcbButton1Mask},
		{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask | xcbModShift},
		{ResponseType: xcbKeyPress},
		{ResponseType: xcbLeaveNotify},
	} {
		first := xcbInputEvent{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask}
		queue := []unsafe.Pointer{unsafe.Pointer(&first), unsafe.Pointer(&boundary)}
		state := linuxWindowState{win: &window{api: &xcbAPI{
			pollForEvent: func(uintptr) unsafe.Pointer {
				if len(queue) == 0 {
					return nil
				}
				event := queue[0]
				queue = queue[1:]
				return event
			},
			free: func(unsafe.Pointer) { t.Fatal("an input boundary was discarded") },
		}}}
		if state.nextEvent() != unsafe.Pointer(&first) || state.nextEvent() != unsafe.Pointer(&boundary) {
			t.Fatalf("input boundary reordered: %+v", boundary)
		}
	}
}

func TestQueuedHoverMotionIsNotCompressed(t *testing.T) {
	events := []xcbInputEvent{{ResponseType: xcbMotionNotify}, {ResponseType: xcbMotionNotify}}
	read, polls := 0, 0
	state := linuxWindowState{win: &window{api: &xcbAPI{
		pollForEvent: func(uintptr) unsafe.Pointer {
			polls++
			if read == len(events) {
				return nil
			}
			event := &events[read]
			read++
			return unsafe.Pointer(event)
		},
		free: func(unsafe.Pointer) { t.Fatal("hover motion was discarded") },
	}}}
	if state.nextEvent() != unsafe.Pointer(&events[0]) || polls != 1 || read != 1 {
		t.Fatal("hover motion was read past")
	}
}

func TestQueuedDragMotionBoundsTheBatch(t *testing.T) {
	events := make([]xcbInputEvent, 300)
	read := 0
	state := linuxWindowState{win: &window{api: &xcbAPI{
		pollForEvent: func(uintptr) unsafe.Pointer {
			if read == len(events) {
				return nil
			}
			event := &events[read]
			*event = xcbInputEvent{ResponseType: xcbMotionNotify, State: xcbButton1Mask, EventX: int16(read)}
			read++
			return unsafe.Pointer(event)
		},
		free: func(unsafe.Pointer) {},
	}}}
	if event := (*xcbInputEvent)(state.nextEvent()); event.EventX != maxQueuedDragMotions || read != maxQueuedDragMotions+1 {
		t.Fatalf("motion batch did not yield: position %d, read %d events", event.EventX, read)
	}
	if event := (*xcbInputEvent)(state.nextEvent()); event.EventX != 299 {
		t.Fatalf("remaining motion was lost: position %d", event.EventX)
	}
}

func BenchmarkQueuedDragMotion(benchmark *testing.B) {
	for _, compress := range []bool{false, true} {
		name := "uncompressed"
		if compress {
			name = "compressed"
		}
		benchmark.Run(name, func(benchmark *testing.B) {
			var events [100]xcbInputEvent
			for index := range events {
				events[index] = xcbInputEvent{ResponseType: xcbMotionNotify, Event: 1, State: xcbButton1Mask, EventX: int16(index)}
			}
			read := 0
			state := linuxWindowState{win: &window{api: &xcbAPI{
				pollForEvent: func(uintptr) unsafe.Pointer {
					if read == len(events) {
						return nil
					}
					event := &events[read]
					read++
					return unsafe.Pointer(event)
				},
				free: func(unsafe.Pointer) {},
			}}}
			delivered := 0
			benchmark.ReportAllocs()
			// The uncompressed path polls once per event without the loop's free,
			// which the compressed path pays inside nextEvent, so the difference
			// is an upper bound on what coalescing adds.
			for benchmark.Loop() {
				read, state.held = 0, nil
				for {
					var event unsafe.Pointer
					if compress {
						event = state.nextEvent()
					} else {
						event = state.win.api.pollForEvent(0)
					}
					if event == nil {
						break
					}
					delivered++
				}
			}
			benchmark.ReportMetric(float64(delivered)/float64(benchmark.N), "deliveries/burst")
		})
	}
}

func TestConfigureCoalescingKeepsPointerCrossingsInOrder(t *testing.T) {
	configure := func(width uint16) *xcbConfigureEvent {
		return &xcbConfigureEvent{ResponseType: xcbConfigureNotify, Window: 1, Width: width, Height: 100}
	}
	pointer := func(kind uint8, x int16) *xcbInputEvent {
		return &xcbInputEvent{ResponseType: kind, Event: 1, EventX: x}
	}
	leave, enter := pointer(xcbLeaveNotify, 0), pointer(xcbEnterNotify, 0)
	queue := []unsafe.Pointer{
		unsafe.Pointer(configure(200)),
		unsafe.Pointer(pointer(xcbMotionNotify, 1)),
		unsafe.Pointer(leave),
		unsafe.Pointer(pointer(xcbMotionNotify, 2)),
		unsafe.Pointer(enter),
		unsafe.Pointer(configure(210)),
	}
	freed := 0
	state := &linuxWindowState{win: &window{id: 1, api: &xcbAPI{
		pollForEvent: func(uintptr) unsafe.Pointer {
			if len(queue) == 0 {
				return nil
			}
			event := queue[0]
			queue = queue[1:]
			return event
		},
		free: func(unsafe.Pointer) { freed++ },
	}}}
	if merged := (*xcbConfigureEvent)(state.nextEvent()); merged.Width != 210 {
		t.Fatalf("merged configure width = %d, want 210", merged.Width)
	}
	if first := state.nextEvent(); first != unsafe.Pointer(leave) {
		t.Fatal("a Leave between configures was dropped or reordered")
	}
	if second := state.nextEvent(); second != unsafe.Pointer(enter) {
		t.Fatal("an Enter between configures was dropped or reordered")
	}
	if state.nextEvent() != nil {
		t.Fatal("stale motion was replayed")
	}
	// The two stale motions are discarded; the superseded configure is freed.
	if freed != 3 {
		t.Fatalf("freed %d events, want 3", freed)
	}
}
