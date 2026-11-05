package diagnostics

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func reset(t *testing.T) {
	t.Helper()
	SetHandler(nil)
	SetLevel(Info)
	Capture(0)
	SetContext(nil)
	t.Cleanup(func() {
		SetHandler(nil)
		SetLevel(Info)
		Capture(0)
		SetContext(nil)
	})
}

func TestSlogHandlerWritesAnEvent(t *testing.T) {
	reset(t)
	var output bytes.Buffer
	inner := slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})
	SetLevel(Debug)
	SetHandler(SlogHandler(inner))
	Report(Info, "render", "first-frame", "first frame presented", Float("milliseconds", 12.5))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("the slog handler wrote no JSON: %v (%q)", err, output.String())
	}
	if record["msg"] != "first frame presented" || record["subsystem"] != "render" || record["code"] != "first-frame" {
		t.Errorf("record = %v", record)
	}
	if record["milliseconds"] != 12.5 {
		t.Errorf("milliseconds = %v", record["milliseconds"])
	}
	if record["level"] != "INFO" {
		t.Errorf("level = %v", record["level"])
	}
}

func TestSlogHandlerHonorsTheInnerLevel(t *testing.T) {
	reset(t)
	var output bytes.Buffer
	inner := slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelWarn})
	SetHandler(SlogHandler(inner))
	Report(Info, "app", "test", "below the inner level")
	if output.Len() != 0 {
		t.Errorf("the handler wrote %q for a record it does not accept", output.String())
	}
}

func TestSlogHandlerNilRemoves(t *testing.T) {
	reset(t)
	if SlogHandler(nil) != nil {
		t.Error("SlogHandler(nil) returned a handler")
	}
}

func TestNewLoggerReportsApplicationRecords(t *testing.T) {
	reset(t)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	logger := NewLogger(&slog.HandlerOptions{Level: slog.LevelDebug})
	logger.Info("document saved", "path", "notes.md", "bytes", 42)

	if len(events) != 1 {
		t.Fatalf("stream saw %d events", len(events))
	}
	event := events[0]
	if event.Level != Info || event.Subsystem != "app" || event.Code != "log" || event.Message != "document saved" {
		t.Errorf("event = %+v", event)
	}
	values := make(map[string]any, len(event.Attrs))
	for _, attr := range event.Attrs {
		values[attr.Key] = attr.Value.Any()
	}
	if values["path"] != "notes.md" || values["bytes"] != int64(42) {
		t.Errorf("attrs = %v", values)
	}
}

func TestNewLoggerGroupsAndAttrs(t *testing.T) {
	reset(t)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	logger := NewLogger(nil).With("service", "api").WithGroup("http")
	logger.Info("request", "path", "/health")

	if len(events) != 1 {
		t.Fatalf("stream saw %d events", len(events))
	}
	values := make(map[string]any, len(events[0].Attrs))
	for _, attr := range events[0].Attrs {
		values[attr.Key] = attr.Value.Any()
	}
	if values["service"] != "api" {
		t.Errorf("service = %v", values["service"])
	}
	if values["http.path"] != "/health" {
		t.Errorf("http.path = %v", values["http.path"])
	}
}

func TestNewLoggerMapsLevels(t *testing.T) {
	reset(t)
	SetLevel(Debug)
	var levels []Level
	SetHandler(func(event Event) { levels = append(levels, event.Level) })
	logger := NewLogger(&slog.HandlerOptions{Level: slog.LevelDebug})
	logger.Debug("design detail")
	logger.Info("lifecycle")
	logger.Warn("worked around")
	logger.Error("failed")
	want := []Level{Debug, Info, Warn, Error}
	if len(levels) != len(want) {
		t.Fatalf("levels = %v", levels)
	}
	for index := range want {
		if levels[index] != want[index] {
			t.Errorf("levels = %v, want %v", levels, want)
		}
	}
}

func TestNewLoggerRespectsItsLevel(t *testing.T) {
	reset(t)
	SetLevel(Debug)
	var seen int
	SetHandler(func(Event) { seen++ })
	logger := NewLogger(&slog.HandlerOptions{Level: slog.LevelWarn})
	logger.Info("below the logger's level")
	if seen != 0 {
		t.Errorf("the stream saw %d records below the logger's level", seen)
	}
}

func TestFaultContextRoundTrip(t *testing.T) {
	reset(t)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	SetContext(func() []Attr { return []Attr{String("version", "1.4.2")} })
	if Context()[0].Value.Text() != "1.4.2" {
		t.Error("Context did not return the context")
	}
	ReportFault("app", "window", "boom", "stack")
	if len(events) != 1 {
		t.Fatalf("stream saw %d events", len(events))
	}
	values := make(map[string]any, len(events[0].Attrs))
	for _, attr := range events[0].Attrs {
		values[attr.Key] = attr.Value.Any()
	}
	if values["version"] != "1.4.2" || values["where"] != "window" || values["stack"] != "stack" {
		t.Errorf("attrs = %v", values)
	}
}

func TestCapturedEventsAreAvailable(t *testing.T) {
	reset(t)
	Capture(4)
	Report(Info, "app", "test", "one")
	Report(Info, "app", "test", "two")
	events := Recent(10)
	if len(events) != 2 || events[0].Message != "one" || events[1].Message != "two" {
		t.Errorf("events = %v", events)
	}
}

func TestNewLoggerReadsALevelVar(t *testing.T) {
	reset(t)
	SetLevel(Debug)
	var events []Event
	SetHandler(func(event Event) { events = append(events, event) })
	var level slog.LevelVar
	level.Set(slog.LevelError)
	logger := NewLogger(&slog.HandlerOptions{Level: &level})
	logger.Info("dropped")
	if len(events) != 0 {
		t.Fatalf("level var did not filter: %v", events)
	}
	level.Set(slog.LevelInfo)
	logger.Info("delivered")
	if len(events) != 1 {
		t.Fatalf("level var did not lower the level: %v", events)
	}
}

// Attribute kinds survive both adapters: a number stays a number on the way
// into the stream and on the way out to slog.
func TestSlogKeepsAttributeKinds(t *testing.T) {
	var got Event
	reset(t)
	SetHandler(func(event Event) { got = event })
	NewLogger(nil).Info("saved", "bytes", 42, "ratio", 0.5, "ok", true, "path", "a.txt")
	kinds := map[string]Kind{}
	for _, attr := range got.Attrs {
		kinds[attr.Key] = attr.Value.Kind()
	}
	want := map[string]Kind{"bytes": KindInt, "ratio": KindFloat, "ok": KindBool, "path": KindString}
	for key, kind := range want {
		if kinds[key] != kind {
			t.Errorf("%s has kind %v, want %v", key, kinds[key], kind)
		}
	}
}
