package diag

import (
	"sync"
	"testing"
)

func resetStream(t *testing.T) {
	t.Helper()
	handlerPointer.Store(nil)
	buffer.Store(nil)
	minimum.Store(0)
	contextFn.Store(nil)
	t.Cleanup(func() {
		handlerPointer.Store(nil)
		buffer.Store(nil)
		minimum.Store(0)
		contextFn.Store(nil)
	})
}

func TestReportReachesTheHandler(t *testing.T) {
	resetStream(t)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	Report(Info, "render", "first-frame", "first frame presented", Attr{Key: "milliseconds", Value: 12.5})
	if len(events) != 1 {
		t.Fatalf("handler saw %d events", len(events))
	}
	event := events[0]
	if event.Level != Info || event.Subsystem != "render" || event.Code != "first-frame" {
		t.Errorf("event = %+v", event)
	}
	if event.Message != "first frame presented" {
		t.Errorf("message = %q", event.Message)
	}
	if len(event.Attrs) != 1 || event.Attrs[0].Key != "milliseconds" || event.Attrs[0].Value != 12.5 {
		t.Errorf("attrs = %v", event.Attrs)
	}
	if event.Time.IsZero() {
		t.Error("event has no time")
	}
}

func TestLevelFiltersDebugByDefault(t *testing.T) {
	resetStream(t)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	Report(Debug, "render", "detail", "not delivered")
	if len(events) != 0 {
		t.Fatalf("Debug reached the handler: %v", events)
	}
	SetLevel(Debug)
	Report(Debug, "render", "detail", "delivered")
	if len(events) != 1 || events[0].Level != Debug {
		t.Fatalf("events = %v", events)
	}
}

func TestNothingIsEnabledWithoutAListener(t *testing.T) {
	resetStream(t)
	if Enabled(Debug) || Enabled(Info) || Enabled(Error) {
		t.Error("a level is enabled with no handler and no buffer")
	}
	if allocations := testing.AllocsPerRun(100, func() {
		Report(Info, "app", "test", "ignored")
	}); allocations != 0 {
		t.Errorf("disabled Report allocated %v times per run", allocations)
	}
}

func TestGuardedAttributesDoNotAllocateWhileDisabled(t *testing.T) {
	resetStream(t)
	if allocations := testing.AllocsPerRun(100, func() {
		if Enabled(Info) {
			Report(Info, "app", "test", "ignored", Attr{Key: "key", Value: "value"})
		}
	}); allocations != 0 {
		t.Errorf("a guarded report allocated %v times per run", allocations)
	}
}

func TestCaptureKeepsTheNewestEvents(t *testing.T) {
	resetStream(t)
	Capture(2)
	for _, message := range []string{"one", "two", "three"} {
		Report(Info, "app", "test", message)
	}
	events := Recent(10)
	if len(events) != 2 {
		t.Fatalf("Recent returned %d events, want 2", len(events))
	}
	if events[0].Message != "two" || events[1].Message != "three" {
		t.Errorf("events = %q, %q", events[0].Message, events[1].Message)
	}
	if limited := Recent(1); len(limited) != 1 || limited[0].Message != "three" {
		t.Errorf("Recent(1) = %v", limited)
	}
	Capture(0)
	if Recent(10) != nil {
		t.Error("Recent still returns events after Capture(0)")
	}
}

func TestCaptureCopiesAttributes(t *testing.T) {
	resetStream(t)
	Capture(1)
	attrs := []Attr{{Key: "value", Value: "original"}}
	Report(Info, "app", "test", "message", attrs...)
	attrs[0].Value = "changed"
	events := Recent(1)
	if len(events) != 1 || events[0].Attrs[0].Value != "original" {
		t.Errorf("captured attrs = %v", events)
	}
}

func TestCaptureIsBounded(t *testing.T) {
	resetStream(t)
	Capture(maxCapture * 2)
	capture := buffer.Load()
	if capture == nil {
		t.Fatal("Capture installed no buffer")
	}
	if got := cap(capture.events); got != maxCapture {
		t.Errorf("buffer capacity = %d, want %d", got, maxCapture)
	}
}

func TestFaultCarriesWhereStackAndContext(t *testing.T) {
	resetStream(t)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	SetContext(func() []Attr {
		return []Attr{{Key: "version", Value: "1.4.2"}, {Key: "document", Value: "notes.md"}}
	})
	ReportFault("app", "window procedure", "boom", []byte("goroutine 1\n..."))
	if len(events) != 1 {
		t.Fatalf("handler saw %d events", len(events))
	}
	event := events[0]
	if event.Level != Error || event.Code != "fault" || event.Message != "boom" {
		t.Errorf("event = %+v", event)
	}
	values := make(map[string]any, len(event.Attrs))
	for _, attr := range event.Attrs {
		values[attr.Key] = attr.Value
	}
	for key, want := range map[string]any{
		"where":    "window procedure",
		"stack":    "goroutine 1\n...",
		"version":  "1.4.2",
		"document": "notes.md",
	} {
		if values[key] != want {
			t.Errorf("attr %q = %v, want %v", key, values[key], want)
		}
	}
}

func TestContextPanicIsIgnored(t *testing.T) {
	resetStream(t)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	SetContext(func() []Attr { panic("context") })
	ReportFault("app", "window", "boom", []byte("stack"))
	if len(events) != 1 {
		t.Fatalf("handler saw %d events", len(events))
	}
	if len(events[0].Attrs) != 2 {
		t.Errorf("attrs = %v, want only where and stack", events[0].Attrs)
	}
}

func TestAHandlerThatPanicsDoesNotStopTheCapture(t *testing.T) {
	resetStream(t)
	Capture(1)
	SetHandler(func(Event) { panic("handler") })
	Report(Info, "app", "test", "message")
	events := Recent(1)
	if len(events) != 1 || events[0].Message != "message" {
		t.Errorf("captured events = %v", events)
	}
}

func TestHandlerAndBufferBothReceive(t *testing.T) {
	resetStream(t)
	Capture(1)
	var seen int
	SetHandler(func(Event) { seen++ })
	Report(Info, "app", "test", "message")
	if seen != 1 || len(Recent(1)) != 1 {
		t.Errorf("handler saw %d, recent has %d", seen, len(Recent(1)))
	}
}

func TestSetHandlerNilStopsDelivery(t *testing.T) {
	resetStream(t)
	var seen int
	SetHandler(func(Event) { seen++ })
	Report(Info, "app", "test", "one")
	SetHandler(nil)
	Report(Info, "app", "test", "two")
	if seen != 1 {
		t.Errorf("handler saw %d events", seen)
	}
}

func TestConcurrentReports(t *testing.T) {
	resetStream(t)
	Capture(64)
	var mu sync.Mutex
	var seen int
	SetHandler(func(Event) {
		mu.Lock()
		seen++
		mu.Unlock()
	})
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; index < 100; index++ {
				Report(Info, "app", "test", "concurrent")
				_ = Recent(5)
			}
		}()
	}
	wait.Wait()
	if seen != 800 {
		t.Errorf("handler saw %d events, want 800", seen)
	}
}

func TestLevelString(t *testing.T) {
	for level, want := range map[Level]string{Debug: "debug", Info: "info", Warn: "warn", Error: "error", Level(9): "unknown"} {
		if got := level.String(); got != want {
			t.Errorf("Level(%d).String() = %q, want %q", level, got, want)
		}
	}
}
