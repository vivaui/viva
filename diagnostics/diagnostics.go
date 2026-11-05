// Package diagnostics exposes Viva's structured diagnostic stream and adapts it
// to and from log/slog.
//
// The framework reports what it did and what went wrong into the stream: a
// panic it caught at a native boundary, a graphics device that was lost and
// made again, a web view environment that failed to start, the time the first
// frame took. A program decides where those events go. This package writes no
// file, opens no connection and starts nothing on its own; a program that
// imports it without installing a handler pays for a few atomic loads per
// reported event and nothing else.
//
// # Both directions
//
// A handler receives framework events, so they can be written with log/slog:
//
//	file, _ := os.OpenFile("app.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
//	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}))
//	diagnostics.SetLevel(diagnostics.Debug)
//	diagnostics.SetHandler(diagnostics.SlogHandler(logger))
//
// NewLogger goes the other way: the application's own slog records enter the
// same stream, so one handler sees both what the application logged and what
// the framework reported.
//
//	diagnostics.SetHandler(someHandler)
//	logger := diagnostics.NewLogger(nil)
//	logger.Info("document saved", "path", path)
//
// # Faults
//
// SetContext installs a function whose attributes are attached to every fault
// (a panic the framework caught): a build version, the open document, the
// graphics backend. Capture keeps a small ring of recent events for a crash
// report; Recent returns it.
//
// # What this package does not do
//
// It does not write logs to disk, rotate files, upload anything, batch events,
// or run a background worker. A handler runs synchronously on the goroutine
// that reported the event, so a slow destination (a network client) should hand
// the event to its own queue rather than block a window thread.
package diagnostics

import (
	"context"
	"log/slog"
	"math"
	"runtime"
	"strconv"
	"time"

	"github.com/vivaui/viva/internal/diag"
)

// Level orders events from least to most severe.
type Level = diag.Level

// The levels an event may have.
const (
	Debug = diag.Debug
	Info  = diag.Info
	Warn  = diag.Warn
	Error = diag.Error
)

// Attr is one key and value on an event. Build one with String, Int, Float or
// Bool.
type Attr = diag.Attr

// Value is an attribute's value: a string, an integer, a float or a boolean.
type Value = diag.Value

// Kind is the type a Value holds.
type Kind = diag.Kind

// The kinds a Value may hold.
const (
	KindString = diag.KindString
	KindInt    = diag.KindInt
	KindFloat  = diag.KindFloat
	KindBool   = diag.KindBool
)

// String returns an attribute holding text.
func String(key, value string) Attr { return diag.String(key, value) }

// Int returns an attribute holding an integer.
func Int(key string, value int64) Attr { return diag.Int(key, value) }

// Float returns an attribute holding a floating-point number.
func Float(key string, value float64) Attr { return diag.Float(key, value) }

// Bool returns an attribute holding a boolean.
func Bool(key string, value bool) Attr { return diag.Bool(key, value) }

// Event is one thing the framework reported.
type Event = diag.Event

// Handler receives every delivered event.
type Handler = diag.Handler

// SetLevel sets the least severe level that is delivered. Debug has to be
// asked for; the default is Info.
func SetLevel(level Level) {
	diag.SetLevel(level)
}

// Enabled reports whether an event at level would be delivered.
func Enabled(level Level) bool {
	return diag.Enabled(level)
}

// SetHandler installs the function that receives every delivered event, or
// removes it when handler is nil.
func SetHandler(handler Handler) {
	diag.SetHandler(handler)
}

// Capture keeps the most recent size events for Recent. A size of zero or less
// stops capturing; the buffer is bounded to 4096 events.
func Capture(size int) {
	diag.Capture(size)
}

// Recent returns up to limit captured events, oldest first.
func Recent(limit int) []Event {
	return diag.Recent(limit)
}

// SetContext installs a function whose attributes are attached to every fault
// event. It runs on the reporting goroutine and must be quick; nil removes it.
func SetContext(context func() []Attr) {
	diag.SetContext(context)
}

// Context returns the attributes the context function supplies, or nil.
func Context() []Attr {
	return diag.Context()
}

// Report adds an event from the application to the stream. Framework code
// reports through the internal stream directly; an application that wants its
// own non-slog events in the same place uses this.
func Report(level Level, subsystem, code, message string, attrs ...Attr) {
	diag.Report(level, subsystem, code, message, attrs...)
}

// ReportFault adds a panic an application caught to the stream, with the
// context attributes attached, the same way the framework reports its own.
func ReportFault(subsystem, where, message, stack string) {
	if !diag.Enabled(Error) {
		return
	}
	diag.ReportFault(subsystem, where, message, []byte(stack))
}

// SlogHandler returns a Handler that writes every event to inner. The event's
// subsystem and code become attributes named "subsystem" and "code", and its
// attributes are added as they are. A nil inner returns nil, which removes the
// handler.
func SlogHandler(inner slog.Handler) Handler {
	if inner == nil {
		return nil
	}
	return func(event Event) {
		level := slogLevel(event.Level)
		ctx := context.Background()
		if !inner.Enabled(ctx, level) {
			return
		}
		when := event.Time
		if when.IsZero() {
			when = time.Now()
		}
		record := slog.NewRecord(when, level, event.Message, 0)
		hasSubsystem, hasCode := false, false
		for _, attr := range event.Attrs {
			hasSubsystem = hasSubsystem || attr.Key == "subsystem"
			hasCode = hasCode || attr.Key == "code"
		}
		if !hasSubsystem {
			record.AddAttrs(slog.String("subsystem", event.Subsystem))
		}
		if !hasCode {
			record.AddAttrs(slog.String("code", event.Code))
		}
		for _, attr := range event.Attrs {
			record.AddAttrs(slogAttr(attr))
		}
		_ = inner.Handle(ctx, record)
	}
}

// NewLogger returns a *slog.Logger whose records enter the diagnostic stream as
// subsystem "app" and code "log". Do not give SlogHandler a handler that leads
// back to a logger from NewLogger: the stream would feed itself. Its level is options.Level when that is set,
// and Info otherwise; the stream's own level still filters it.
func NewLogger(options *slog.HandlerOptions) *slog.Logger {
	var level slog.Leveler = slog.LevelInfo
	if options != nil && options.Level != nil {
		level = options.Level
	}
	return slog.New(&streamHandler{level: level})
}

// streamHandler is a slog.Handler that reports into the diagnostic stream.
type streamHandler struct {
	level  slog.Leveler
	attrs  []diag.Attr
	groups []string
}

func (handler *streamHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= handler.level.Level() && diag.Enabled(diagLevel(level))
}

func (handler *streamHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make([]diag.Attr, 0, len(handler.attrs)+record.NumAttrs())
	attrs = append(attrs, handler.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = appendSlogAttr(attrs, prefixedKey(handler.groups, attr.Key), attr.Value)
		return true
	})
	if record.PC != 0 {
		frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
		attrs = append(attrs, diag.String("source", frame.File+":"+strconv.Itoa(frame.Line)))
	}
	diag.Report(diagLevel(record.Level), "app", "log", record.Message, attrs...)
	return nil
}

func (handler *streamHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *handler
	next.attrs = append([]diag.Attr(nil), handler.attrs...)
	for _, attr := range attrs {
		next.attrs = appendSlogAttr(next.attrs, prefixedKey(handler.groups, attr.Key), attr.Value)
	}
	return &next
}

func (handler *streamHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return handler
	}
	next := *handler
	next.groups = append(append([]string(nil), handler.groups...), name)
	return &next
}

// appendSlogAttr flattens one slog attribute into the stream's key/value form.
// A group becomes dotted keys, which is what a flat event can represent.
func appendSlogAttr(dst []diag.Attr, key string, value slog.Value) []diag.Attr {
	value = value.Resolve()
	if value.Kind() == slog.KindGroup {
		for _, child := range value.Group() {
			dst = appendSlogAttr(dst, key+"."+child.Key, child.Value)
		}
		return dst
	}
	switch value.Kind() {
	case slog.KindString:
		return append(dst, diag.String(key, value.String()))
	case slog.KindInt64:
		return append(dst, diag.Int(key, value.Int64()))
	case slog.KindUint64:
		if number := value.Uint64(); number <= math.MaxInt64 {
			return append(dst, diag.Int(key, int64(number)))
		}
		return append(dst, diag.String(key, value.String()))
	case slog.KindFloat64:
		return append(dst, diag.Float(key, value.Float64()))
	case slog.KindBool:
		return append(dst, diag.Bool(key, value.Bool()))
	case slog.KindTime:
		return append(dst, diag.String(key, value.Time().Format(time.RFC3339Nano)))
	}
	// Durations, errors and every other value reach the stream as the text
	// slog itself would print for them.
	return append(dst, diag.String(key, value.String()))
}

// slogAttr converts one stream attribute to the slog form of the same type.
func slogAttr(attr diag.Attr) slog.Attr {
	switch attr.Value.Kind() {
	case diag.KindInt:
		return slog.Int64(attr.Key, attr.Value.Int64())
	case diag.KindFloat:
		return slog.Float64(attr.Key, attr.Value.Float64())
	case diag.KindBool:
		return slog.Bool(attr.Key, attr.Value.Bool())
	}
	return slog.String(attr.Key, attr.Value.Text())
}

func prefixedKey(groups []string, key string) string {
	for index := len(groups) - 1; index >= 0; index-- {
		key = groups[index] + "." + key
	}
	return key
}

func slogLevel(level diag.Level) slog.Level {
	switch level {
	case Debug:
		return slog.LevelDebug
	case Warn:
		return slog.LevelWarn
	case Error:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func diagLevel(level slog.Level) diag.Level {
	switch {
	case level >= slog.LevelError:
		return Error
	case level >= slog.LevelWarn:
		return Warn
	case level >= slog.LevelInfo:
		return Info
	default:
		return Debug
	}
}
