# Diagnostics

Viva keeps a structured diagnostic stream inside the process. Framework code
reports what it did and what went wrong into it; an application decides whether
to keep those events and where they go. It is the base for logging, not a
logger: it writes no file, opens no connection, starts no goroutine and keeps
nothing unless the application asks.

Two packages make it up:

| Package | What it is |
| --- | --- |
| `github.com/vivaui/viva/internal/diag` | the stream the framework reports into: levels, events, a handler, an optional ring buffer, fault context. No `log/slog` dependency. |
| `github.com/vivaui/viva/diagnostics` | the same API for applications, plus the `log/slog` adapters and `ReportFault`. |

Framework code imports the internal package, so a program that does not import
`diagnostics` links none of the `slog` adapters.

## An event

```go
type Event struct {
	Time      time.Time
	Level     Level  // Debug, Info, Warn, Error
	Subsystem string // "app", "render", "web", "platform"
	Code      string // stable, machine-readable: "fault", "device-recovered", ...
	Message   string // human text
	Attrs     []Attr // key/value pairs
}
```

An `Attr` is a key and a `Value` that holds a string, an integer, a float or a
boolean. Build one with `diagnostics.String`, `Int`, `Float` or `Bool`; a
`Value` is a small tagged union, so building one does not allocate. An slog
attribute of any other type (an error, a duration, a struct) reaches the stream
as the text slog prints for it.

`Code` is stable across releases; `Message` may change. A collector should key
on `Subsystem` and `Code`, not on the wording.

## What the framework reports today

| Subsystem | Code | Level | When |
| --- | --- | --- | --- |
| `app` | `fault` | Error | a panic caught at a native boundary; attributes `where`, `stack`, and the context |
| `app` | `fatal` | Error | `app.ShowFatalError`, with `title` |
| `render` | `device-lost` | Error | the Windows graphics device was lost again and again |
| `render` | `device-recovery-failed` | Error | the Windows device was lost and could not be made again |
| `render` | `device-recovered` | Warn | the Windows device was lost and made again, with `software` |
| `render` | `first-frame` | Debug | the first frame on Linux, when `VIVA_PROFILE_STARTUP` measures it |
| `render` | `window-stats` | Debug | a Linux window closed, with its GPU counters |
| `web` | `environment` | Warn | a WebView2 environment could not be created, with `error` |
| `platform` | `reopen-panic` | Error | the macOS reopen handler panicked |

The list grows where a failure would otherwise be invisible. A successful
operation is reported only when it explains a failure or a measurement (a
recovered graphics device is a warning the user may notice), and a device that
keeps failing gives up rather than flooding the stream.

## Collecting

A handler receives every delivered event. The `slog` adapter is the shortest
route to a file, a journal, or whatever handler the program already has:

```go
file, err := os.OpenFile("app.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
if err != nil {
	log.Fatal(err)
}
logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}))
diagnostics.SetLevel(diagnostics.Debug)
diagnostics.SetHandler(diagnostics.SlogHandler(logger))
```

The other direction puts the application's own `slog` records in the same
stream, so one handler sees both:

```go
log := diagnostics.NewLogger(nil) // honors HandlerOptions.Level; other options are not used
log.Info("document saved", "path", path)
```

An application with several destinations composes handlers itself:

```go
diagnostics.SetHandler(func(event diagnostics.Event) {
	fileHandler(event)   // your writer
	metricsHandler(event) // your counters
})
```

A handler runs synchronously on the goroutine that reported the event. It must
return quickly: a slow destination (a network client) belongs behind its own
queue, not in a window callback. A handler that panics is ignored, and the
capture buffer still receives the event.

## Keeping the last events

`Capture` keeps a bounded ring of recent events, and `Recent` returns them
oldest first. A crash report can include them without the application writing
its own ring:

```go
diagnostics.Capture(256)
// ...
for _, event := range diagnostics.Recent(256) {
	report.WriteString(event.Message)
}
```

The buffer holds at most 4096 events and is allocated only when `Capture` is
called. `Capture(0)` stops it. The bound is a count, not bytes: an event with a
fault stack can be several kilobytes, so a full ring of stacks is bounded but
not small.

## Faults

`app.SetFaultHandler` receives panics the framework caught at native boundaries.
The same faults also enter the stream as `app`/`fault`, whether or not the
application installed a handler. `SetContext` adds attributes to every fault, so
a crash report says what the program was doing:

```go
diagnostics.SetContext(func() []diagnostics.Attr {
	return []diagnostics.Attr{
		diagnostics.String("version", buildVersion),
		diagnostics.String("document", model.Path),
		diagnostics.Bool("backend", app.PlatformTextEngine() != nil),
	}
})
```

The context function runs on the thread that panicked and must be quick. A panic
inside it is ignored.

A panic the application recovers itself enters the stream with
`diagnostics.ReportFault(subsystem, where, message, stack)`.

## Cost

- With no handler and no capture buffer, `Enabled` costs three atomic loads and
  a comparison, and `Report` returns without allocating. Attributes are built
  before `Report` is called, so a caller that passes them guards the whole call
  with `diagnostics.Enabled(level)`; an unguarded call with attributes allocates
  even when nobody listens.
- `Capture` allocates its ring once; a report copies the event's attributes only
  while capturing.
- No timer, goroutine, file descriptor or connection is created by either
  package.
- Nothing is written to stderr unless the application's handler does it. The
  few platform messages that still go to stderr are independent of this stream.

## What this is not

- Not a log file writer: rotation, retention and permissions belong to the
  application's `slog` handler.
- Not a collector or uploader: no endpoint, no consent, no identifier, no
  batching. Sending diagnostics somewhere is an application decision, made
  visible in its handler.
- Not a tracing system: no spans, no correlation IDs. A handler that wants them
  can add attributes of its own.
- Not backpressure-aware: a handler that blocks blocks the reporter.
