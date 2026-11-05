// Package diag is Viva's structured diagnostic stream.
//
// Framework code reports what it did and what went wrong into this package:
// caught panics, a lost and recreated graphics device, a web view environment
// that failed, a slow first frame. It is the base a program builds logging on,
// not a logger itself: it writes no file, opens no connection, starts no
// goroutine, and keeps nothing unless something asks for it.
//
// The public github.com/vivaui/viva/diagnostics package re-exports this API and
// adds the log/slog adapters. Framework packages import this one so that a
// program that never collects diagnostics links none of the adapters.
//
// # Cost when nobody listens
//
// A report first asks Enabled. With no handler, no capture buffer and no level
// allowance, Enabled is three atomic loads and a comparison, and a Report with
// no attributes returns after the same check without allocating. Attributes are
// built before the call, so a call that passes them should guard them too:
//
//	if diag.Enabled(diag.Debug) {
//		diag.Report(diag.Debug, "render", "first-frame", "first frame presented",
//			diag.Float("milliseconds", 12.5))
//	}
//
// # Threading
//
// Report and every setter may be called from any goroutine. A handler and the
// context function run on the calling goroutine, so they must be quick and must
// not call back into the stream. A handler that panics is ignored, and the
// capture buffer still receives the event.
package diag

import (
	"math"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Level orders events from least to most severe.
type Level uint8

const (
	// Debug is detail for a developer: a first frame, a cache miss, a device
	// recovery that succeeded.
	Debug Level = iota
	// Info is an ordinary lifecycle event.
	Info
	// Warn is something that went wrong and was worked around.
	Warn
	// Error is a fault the framework caught or a capability that failed.
	Error
)

// String returns the level's name.
func (level Level) String() string {
	switch level {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	}
	return "unknown"
}

// Kind is the type a Value holds.
type Kind uint8

const (
	// KindString is a text value.
	KindString Kind = iota
	// KindInt is a signed integer.
	KindInt
	// KindFloat is a floating-point number.
	KindFloat
	// KindBool is a boolean.
	KindBool
)

// Value is one attribute's value: a string, an integer, a float or a boolean.
// It is a small tagged union, so building one does not allocate and a handler
// reads it without a type switch on an interface.
type Value struct {
	kind Kind
	num  uint64
	text string
}

// Kind returns the type the value holds.
func (value Value) Kind() Kind { return value.kind }

// Text returns the string of a KindString value, or "" for any other kind.
func (value Value) Text() string {
	if value.kind != KindString {
		return ""
	}
	return value.text
}

// Int64 returns the integer of a KindInt value, or 0 for any other kind.
func (value Value) Int64() int64 {
	if value.kind != KindInt {
		return 0
	}
	return int64(value.num)
}

// Float64 returns the number of a KindFloat value, or 0 for any other kind.
func (value Value) Float64() float64 {
	if value.kind != KindFloat {
		return 0
	}
	return math.Float64frombits(value.num)
}

// Bool returns the boolean of a KindBool value, or false for any other kind.
func (value Value) Bool() bool {
	return value.kind == KindBool && value.num != 0
}

// Any returns the value as a string, int64, float64 or bool, for a consumer
// that wants an untyped value.
func (value Value) Any() any {
	switch value.kind {
	case KindInt:
		return value.Int64()
	case KindFloat:
		return value.Float64()
	case KindBool:
		return value.Bool()
	}
	return value.text
}

// String renders the value as text.
func (value Value) String() string {
	switch value.kind {
	case KindInt:
		return strconv.FormatInt(value.Int64(), 10)
	case KindFloat:
		return strconv.FormatFloat(value.Float64(), 'g', -1, 64)
	case KindBool:
		return strconv.FormatBool(value.Bool())
	}
	return value.text
}

// Attr is one key and value on an event.
type Attr struct {
	Key   string
	Value Value
}

// String returns an attribute holding text.
func String(key, value string) Attr {
	return Attr{Key: key, Value: Value{kind: KindString, text: value}}
}

// Int returns an attribute holding an integer.
func Int(key string, value int64) Attr {
	return Attr{Key: key, Value: Value{kind: KindInt, num: uint64(value)}}
}

// Float returns an attribute holding a floating-point number.
func Float(key string, value float64) Attr {
	return Attr{Key: key, Value: Value{kind: KindFloat, num: math.Float64bits(value)}}
}

// Bool returns an attribute holding a boolean.
func Bool(key string, value bool) Attr {
	var num uint64
	if value {
		num = 1
	}
	return Attr{Key: key, Value: Value{kind: KindBool, num: num}}
}

// Event is one thing the framework reported: when it happened, how severe it
// was, which part of the framework reported it, a stable code, a human message,
// and any attributes.
type Event struct {
	Time      time.Time
	Level     Level
	Subsystem string
	Code      string
	Message   string
	Attrs     []Attr
}

// Handler receives every event that passes the level and is not dropped.
// It runs on the reporting goroutine and must not retain the Attrs slice or
// call back into the stream.
type Handler func(Event)

// maxCapture bounds how many events Capture keeps.
const maxCapture = 4096

var (
	buffer atomic.Pointer[eventBuffer]
	// minimum holds the minimum level plus one; zero means Info.
	minimum    atomic.Uint32
	contextFn  atomic.Pointer[func() []Attr]
	activateFn atomic.Pointer[func()]
)

// SetLevel sets the least severe level that is delivered. Debug has to be
// asked for; the default is Info.
func SetLevel(level Level) {
	minimum.Store(uint32(level) + 1)
}

func minimumLevel() Level {
	if stored := minimum.Load(); stored != 0 {
		return Level(stored - 1)
	}
	return Info
}

// Enabled reports whether an event at level would be delivered: one is not
// below the configured level and something is listening.
func Enabled(level Level) bool {
	if level < minimumLevel() {
		return false
	}
	return handlerPointer.Load() != nil || buffer.Load() != nil
}

// SetHandler installs the function that receives every delivered event, or
// removes it when handler is nil. One handler at a time; a program with several
// destinations composes them itself (the diagnostics package's SlogHandler is
// one way).
func SetHandler(handler Handler) {
	if handler == nil {
		handlerPointer.Store(nil)
		return
	}
	handlerPointer.Store(&handler)
	notifyActivate()
}

// Capture keeps the most recent size events for Recent, replacing any earlier
// capture. A size of zero or less stops capturing. The buffer is bounded to
// 4096 events.
func Capture(size int) {
	if size <= 0 {
		buffer.Store(nil)
		return
	}
	capture := min(size, maxCapture)
	buffer.Store(newEventBuffer(capture))
	notifyActivate()
}

// Recent returns up to limit captured events, oldest first. It returns nil when
// Capture is not active.
func Recent(limit int) []Event {
	capture := buffer.Load()
	if capture == nil || limit <= 0 {
		return nil
	}
	return capture.recent(limit)
}

// SetContext installs a function that supplies attributes attached to every
// fault event, such as a build version, the document in use, or the graphics
// backend. It runs on the goroutine reporting the fault and must return
// quickly; a panic in it is ignored. Nil removes it.
func SetContext(context func() []Attr) {
	if context == nil {
		contextFn.Store(nil)
		return
	}
	contextFn.Store(&context)
}

// Context returns the attributes the context function supplies, or nil.
func Context() []Attr {
	context := contextFn.Load()
	if context == nil {
		return nil
	}
	var attrs []Attr
	func() {
		defer func() { _ = recover() }()
		attrs = (*context)()
	}()
	return slices.Clone(attrs)
}

// Report hands an event to the stream. It does nothing and allocates nothing
// when Enabled would answer false.
func Report(level Level, subsystem, code, message string, attrs ...Attr) {
	if !Enabled(level) {
		return
	}
	dispatch(Event{
		Time:      time.Now(),
		Level:     level,
		Subsystem: subsystem,
		Code:      code,
		Message:   message,
		Attrs:     attrs,
	})
}

// SetActivate installs a function the stream calls when a handler or a capture
// buffer starts listening, so a framework feature that is only worth paying for
// when someone listens (the fault bridge) can be turned on at that moment. It
// must be quick and must not call back into the stream; nil removes it.
func SetActivate(activate func()) {
	if activate == nil {
		activateFn.Store(nil)
		return
	}
	activateFn.Store(&activate)
}

func notifyActivate() {
	activate := activateFn.Load()
	if activate == nil {
		return
	}
	defer func() { _ = recover() }()
	(*activate)()
}

// ReportFault reports a panic the framework caught, with the place it was
// caught and the stack, plus the context attributes. It is what the app
// package's fault bridge calls.
func ReportFault(subsystem, where, message string, stack []byte) {
	if !Enabled(Error) {
		return
	}
	context := Context()
	attrs := make([]Attr, 0, 2+len(context))
	attrs = append(attrs, String("where", where), String("stack", string(stack)))
	attrs = append(attrs, context...)
	Report(Error, subsystem, "fault", message, attrs...)
}

// handlerPointer is the installed handler. It is separate from the exported
// SetHandler parameter so the atomic can hold a pointer to the function value.
var handlerPointer atomic.Pointer[Handler]

func dispatch(event Event) {
	if handler := handlerPointer.Load(); handler != nil {
		deliver(*handler, event)
	}
	if capture := buffer.Load(); capture != nil {
		capture.add(event)
	}
}

// deliver runs a handler without letting its panic take the reporting path
// down. Diagnostics must not be able to crash the program they describe.
func deliver(handler Handler, event Event) {
	defer func() { _ = recover() }()
	handler(event)
}

type eventBuffer struct {
	mu     sync.Mutex
	events []Event
	next   int
}

func newEventBuffer(size int) *eventBuffer {
	return &eventBuffer{events: make([]Event, 0, size)}
}

func (buffer *eventBuffer) add(event Event) {
	event.Attrs = slices.Clone(event.Attrs)
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if len(buffer.events) < cap(buffer.events) {
		buffer.events = append(buffer.events, event)
		return
	}
	buffer.events[buffer.next] = event
	buffer.next = (buffer.next + 1) % cap(buffer.events)
}

func (buffer *eventBuffer) recent(limit int) []Event {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if len(buffer.events) == 0 {
		return nil
	}
	if limit > len(buffer.events) {
		limit = len(buffer.events)
	}
	start := 0
	if len(buffer.events) == cap(buffer.events) {
		start = buffer.next
	}
	first := len(buffer.events) - limit
	events := make([]Event, limit)
	for index := 0; index < limit; index++ {
		event := buffer.events[(start+first+index)%len(buffer.events)]
		event.Attrs = slices.Clone(event.Attrs)
		events[index] = event
	}
	return events
}
