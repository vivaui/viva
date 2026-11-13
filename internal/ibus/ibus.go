//go:build unix

// Package ibus is the client half of the IBus input method protocol: what a
// window needs to let the user compose Chinese, Japanese or Korean text with an
// input method engine that runs in another process.
//
// IBus is a daemon with a private message bus. A client asks it for an input
// context, tells it about every key the user presses while a text field has
// focus, and tells it where the caret is so its panel can place the candidate
// window. The engine answers each key with "handled" or "not handled" and, as it
// composes, sends back preedit text (the underlined text still being composed)
// and committed text (the characters the user chose). Candidate lists are drawn
// by the daemon's own panel, so the client needs none.
//
// A Bus is driven by one thread. It exposes a file descriptor to wait on,
// Dispatch to read what arrived, and Deadline and Expire so a daemon that stops
// answering cannot hold the keyboard: after a bounded wait every pending key is
// released as "not handled" and the bus is closed, and the caller carries on
// without an input method. Nothing here starts a goroutine or a timer.
//
// Fcitx5 speaks this protocol through its IBus frontend, so one client serves
// both frameworks.
package ibus

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vivaui/viva/internal/dbus"
)

const (
	service          = "org.freedesktop.IBus"
	busPath          = "/org/freedesktop/IBus"
	contextInterface = "org.freedesktop.IBus.InputContext"

	// CapPreeditText says the client draws preedit text itself.
	CapPreeditText uint32 = 1 << 0
	// CapFocus says the client reports focus changes.
	CapFocus uint32 = 1 << 3
	// CapSurroundingText says the client tells the engine the text around the
	// caret and lets it delete some of it, as a reconversion needs.
	CapSurroundingText uint32 = 1 << 5

	// The kinds of IBusAttribute, which style a stretch of preedit text.
	AttrUnderline  uint32 = 1
	AttrForeground uint32 = 2
	AttrBackground uint32 = 3

	// ReleaseMask marks a key state as a key release.
	ReleaseMask uint32 = 1 << 30

	// keyTimeout is how long a key waits for the engine before the bus is given
	// up. An engine answers in milliseconds; this is for one that has hung.
	keyTimeout = 2 * time.Second
)

// ErrClosed is returned once the bus has failed or been closed.
var ErrClosed = errors.New("ibus: the connection is closed")

// transport is the part of a D-Bus connection the client uses.
type transport interface {
	Call(destination, path, iface, member, signature string, body []byte) (*dbus.Message, error)
	Send(destination, path, iface, member, signature string, body []byte) (uint32, error)
	Receive() ([]*dbus.Message, error)
	FD() int
	Close()
}

// Handler receives what the engine sends for one input context. Its methods
// run inside Dispatch, in the order the engine produced them, and a key's
// answer arrives after the text the key produced.
type Handler struct {
	// Commit delivers text the user has chosen.
	Commit func(text string)
	// Preedit preserves the signal's update/visibility provenance and optional
	// focus-out mode. Attributes are context-owned and must not be modified.
	Preedit func(PreeditEvent)
	// DeleteSurrounding asks the client to delete count characters of the text
	// around the caret, starting offset characters from it (a negative offset is
	// before it). The engine does so to replace text it is about to convert again.
	DeleteSurrounding func(offset, count int)
	// Forward asks the client to treat a key as if it had been typed, which an
	// engine does for a key it took apart (an Enter that ends a composition).
	Forward func(keysym, keycode, state uint32)
}

// PreeditKind distinguishes a text update from visibility-only signals.
type PreeditKind uint8

const (
	PreeditUpdate PreeditKind = iota
	PreeditShow
	PreeditHide
)

const (
	PreeditModeClear  uint32 = 0
	PreeditModeCommit uint32 = 1
)

// PreeditEvent is one native signal. An empty Text is not a terminal phase;
// HasMode distinguishes the old signal from UpdatePreeditTextWithMode. Unknown
// mode values are preserved for the caller to handle without guessing policy.
type PreeditEvent struct {
	Text       string
	Attributes []Attribute
	Cursor     int
	Mode       uint32
	Kind       PreeditKind
	Visible    bool
	HasMode    bool
}

// Attribute styles the characters [Start, End) of preedit text. Kind is one of
// the Attr constants and Value its argument: an underline style for
// AttrUnderline (0 none, 1 single, 2 double, 3 low, 4 error), a color for the
// others.
type Attribute struct {
	Kind, Value uint32
	Start, End  int
}

// Bus is a connection to the IBus daemon.
type Bus struct {
	conn     transport
	contexts map[string]*Context
	pending  []*pendingKey
	err      error
	now      func() time.Time
}

type pendingKey struct {
	serial   uint32
	done     func(handled bool)
	deadline time.Time
	answered bool
	handled  bool
}

// Context is one input context: the engine's view of a single text field.
type Context struct {
	bus     *Bus
	path    string
	handler Handler
	// preedit remembers the last preedit text, for a daemon that hides and later
	// shows it again without sending it twice.
	preedit PreeditEvent
}

// Address finds the IBus daemon's bus address the way libibus does: the
// IBUS_ADDRESS variable, or else the file the daemon writes under the user's
// configuration directory, named by the machine id, the display's host and its
// number. getenv and readFile are os.Getenv and os.ReadFile outside tests.
func Address(getenv func(string) string, readFile func(string) ([]byte, error)) (string, bool) {
	if address := getenv("IBUS_ADDRESS"); address != "" {
		return address, true
	}
	var machine string
	for _, name := range []string{"/var/lib/dbus/machine-id", "/etc/machine-id"} {
		if data, err := readFile(name); err == nil {
			if machine = strings.TrimSpace(string(data)); machine != "" {
				break
			}
		}
	}
	if machine == "" {
		return "", false
	}
	host, number := displayParts(getenv("DISPLAY"))
	config := getenv("XDG_CONFIG_HOME")
	if config == "" {
		home := getenv("HOME")
		if home == "" {
			return "", false
		}
		config = filepath.Join(home, ".config")
	}
	data, err := readFile(filepath.Join(config, "ibus", "bus", machine+"-"+host+"-"+number))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if address, ok := strings.CutPrefix(strings.TrimSpace(line), "IBUS_ADDRESS="); ok && address != "" {
			return address, true
		}
	}
	return "", false
}

// displayParts splits an X display name such as "host:1.0" into the host ("unix"
// when it is empty, as IBus names the file) and the display number.
func displayParts(display string) (host, number string) {
	if display == "" {
		display = ":0.0"
	}
	host, rest, found := strings.Cut(display, ":")
	if !found {
		return "unix", "0"
	}
	if host == "" {
		host = "unix"
	}
	number, _, _ = strings.Cut(rest, ".")
	if number == "" {
		number = "0"
	}
	return host, number
}

// Dial connects to the running daemon, or reports why it cannot. A machine
// without IBus is the usual reason, not an error worth showing.
func Dial() (*Bus, error) {
	address, ok := Address(os.Getenv, os.ReadFile)
	if !ok {
		return nil, errors.New("ibus: no daemon address")
	}
	conn, err := dbus.Dial(address)
	if err != nil {
		return nil, err
	}
	return newBus(conn), nil
}

func newBus(conn transport) *Bus {
	bus := &Bus{conn: conn, contexts: map[string]*Context{}, now: time.Now}
	// The daemon addresses its signals to the client, but a bus that routes by
	// match rule needs the request; a daemon that refuses it still delivers.
	var rule dbus.Encoder
	rule.String("type='signal',interface='" + contextInterface + "'")
	_, _ = conn.Call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "AddMatch", "s", rule.Bytes())
	return bus
}

// FD is the socket to wait on for readability.
func (bus *Bus) FD() int { return bus.conn.FD() }

// Err reports why the bus is no longer usable, or nil.
func (bus *Bus) Err() error { return bus.err }

// Close ends the connection. Keys still waiting are released as not handled.
func (bus *Bus) Close() {
	bus.fail(ErrClosed)
}

func (bus *Bus) fail(err error) {
	if bus.err != nil {
		return
	}
	bus.err = err
	bus.conn.Close()
	pending := bus.pending
	bus.pending = nil
	for _, key := range pending {
		if key.done != nil {
			key.done(false)
		}
	}
	clear(bus.contexts)
}

// NewContext creates an input context whose output goes to handler. The caller
// reports focus with FocusIn and FocusOut, keys with ProcessKey and the caret
// with SetCursorLocation.
func (bus *Bus) NewContext(name string, handler Handler) (*Context, error) {
	if bus.err != nil {
		return nil, ErrClosed
	}
	var body dbus.Encoder
	body.String(name)
	reply, err := bus.conn.Call(service, busPath, service, "CreateInputContext", "s", body.Bytes())
	if err != nil {
		bus.fail(err)
		return nil, err
	}
	path := reply.Args().String()
	if path == "" {
		return nil, errors.New("ibus: the daemon returned no input context")
	}
	context := &Context{bus: bus, path: path, handler: handler}
	bus.contexts[path] = context
	var capabilities dbus.Encoder
	capabilities.Uint32(CapPreeditText | CapFocus | CapSurroundingText)
	if err := context.send("SetCapabilities", "u", capabilities.Bytes()); err != nil {
		return nil, err
	}
	return context, nil
}

// Dispatch reads what the daemon has sent and runs the handlers it calls for.
func (bus *Bus) Dispatch() error {
	if bus.err != nil {
		return bus.err
	}
	messages, err := bus.conn.Receive()
	if err != nil {
		bus.fail(err)
		return err
	}
	for _, message := range messages {
		switch message.Type {
		case dbus.TypeSignal:
			if context := bus.contexts[message.Path]; context != nil && message.Interface == contextInterface {
				context.signal(message)
			}
		case dbus.TypeReturn, dbus.TypeError:
			bus.answer(message)
		}
	}
	return bus.err
}

// answer records the engine's verdict on a key and releases the verdicts that
// are ready, oldest first, so keys reach the application in the order typed.
func (bus *Bus) answer(message *dbus.Message) {
	for _, key := range bus.pending {
		if key.serial != message.ReplySerial {
			continue
		}
		key.answered = true
		if message.Type == dbus.TypeReturn && message.Signature == "b" {
			key.handled = message.Args().Bool()
		}
		break
	}
	for len(bus.pending) > 0 && bus.pending[0].answered {
		key := bus.pending[0]
		bus.pending = bus.pending[1:]
		if key.done != nil {
			key.done(key.handled)
		}
	}
}

// Deadline is when the oldest unanswered key gives up on the engine, or false
// when no key is waiting. A loop that waits on FD should wait no longer.
func (bus *Bus) Deadline() (time.Time, bool) {
	if len(bus.pending) == 0 {
		return time.Time{}, false
	}
	return bus.pending[0].deadline, true
}

// Expire gives up on an engine that has not answered in time: every waiting
// key is released as not handled and the bus closes.
func (bus *Bus) Expire() {
	if deadline, waiting := bus.Deadline(); waiting && !bus.now().Before(deadline) {
		bus.fail(errors.New("ibus: the daemon did not answer a key in time"))
	}
}

func (context *Context) send(member, signature string, body []byte) error {
	if context.bus.err != nil {
		return ErrClosed
	}
	if _, err := context.bus.conn.Send(service, context.path, contextInterface, member, signature, body); err != nil {
		context.bus.fail(err)
		return err
	}
	return nil
}

// ProcessKey gives the engine a key. done is called, in order with the other
// keys, with whether the engine consumed it; a key it did not consume is the
// application's to handle. Use keysym and keycode as the X server reports them
// (keycode less 8, as IBus counts), state as the X modifier mask, plus
// ReleaseMask for a release. done may be nil for a release.
func (context *Context) ProcessKey(keysym, keycode, state uint32, done func(handled bool)) error {
	if context.bus.err != nil {
		return ErrClosed
	}
	var body dbus.Encoder
	body.Uint32(keysym)
	body.Uint32(keycode)
	body.Uint32(state)
	serial, err := context.bus.conn.Send(service, context.path, contextInterface, "ProcessKeyEvent", "uuu", body.Bytes())
	if err != nil {
		context.bus.fail(err)
		return err
	}
	context.bus.pending = append(context.bus.pending, &pendingKey{serial: serial, done: done, deadline: context.bus.now().Add(keyTimeout)})
	return nil
}

// FocusIn tells the engine the text field has the keyboard.
func (context *Context) FocusIn() error { return context.send("FocusIn", "", nil) }

// FocusOut tells the engine the text field lost the keyboard.
func (context *Context) FocusOut() error { return context.send("FocusOut", "", nil) }

// Reset abandons the composition in progress.
func (context *Context) Reset() error { return context.send("Reset", "", nil) }

// SetSurroundingText tells the engine the text around the caret. cursor and
// anchor are character offsets into text: the caret, and the other end of the
// selection (the same as cursor when nothing is selected).
func (context *Context) SetSurroundingText(text string, cursor, anchor uint32) error {
	var body dbus.Encoder
	writeText(&body, text)
	body.Uint32(cursor)
	body.Uint32(anchor)
	return context.send("SetSurroundingText", "vuu", body.Bytes())
}

// SetCursorLocation tells the panel where the caret is, in root window
// coordinates, so the candidate window opens beside it.
func (context *Context) SetCursorLocation(x, y, width, height int32) error {
	var body dbus.Encoder
	body.Int32(x)
	body.Int32(y)
	body.Int32(width)
	body.Int32(height)
	return context.send("SetCursorLocation", "iiii", body.Bytes())
}

// Close destroys the input context.
func (context *Context) Close() {
	if context.bus.err == nil {
		_ = context.send("Destroy", "", nil)
	}
	delete(context.bus.contexts, context.path)
}

func (context *Context) signal(message *dbus.Message) {
	args := message.Args()
	switch message.Member {
	case "CommitText":
		if text, ok := readText(args, message.Signature); ok && context.handler.Commit != nil {
			context.handler.Commit(text)
		}
	case "UpdatePreeditText", "UpdatePreeditTextWithMode":
		withMode := message.Member == "UpdatePreeditTextWithMode"
		wantSignature := "vub"
		if withMode {
			wantSignature = "vubu"
		}
		if message.Signature != wantSignature {
			return
		}
		text, attrs, ok := readStyledText(args, message.Signature)
		if !ok {
			return
		}
		event := PreeditEvent{Text: text, Attributes: attrs, Cursor: int(args.Uint32()), Visible: args.Bool(), HasMode: withMode}
		if withMode {
			event.Mode = args.Uint32()
		}
		if args.Err() != nil || args.Pos() != len(message.Body) {
			return
		}
		context.preedit = event
		if context.handler.Preedit != nil {
			context.handler.Preedit(event)
		}
	case "ShowPreeditText":
		if message.Signature != "" || len(message.Body) != 0 {
			return
		}
		if context.handler.Preedit != nil {
			event := context.preedit
			event.Kind, event.Visible = PreeditShow, true
			context.handler.Preedit(event)
		}
	case "HidePreeditText":
		if message.Signature != "" || len(message.Body) != 0 {
			return
		}
		if context.handler.Preedit != nil {
			event := context.preedit
			event.Kind, event.Visible = PreeditHide, false
			context.handler.Preedit(event)
		}
	case "DeleteSurroundingText":
		offset, count := args.Int32(), args.Uint32()
		if args.Err() == nil && context.handler.DeleteSurrounding != nil {
			context.handler.DeleteSurrounding(int(offset), int(count))
		}
	case "ForwardKeyEvent":
		keysym, keycode, state := args.Uint32(), args.Uint32(), args.Uint32()
		if args.Err() == nil && context.handler.Forward != nil {
			context.handler.Forward(keysym, keycode, state)
		}
	}
}

// writeText writes an IBusText as the variant the daemon expects: the struct
// (name, attachments, text, attributes) with no attributes.
func writeText(e *dbus.Encoder, text string) {
	e.Variant("(sa{sv}sv)")
	e.Align(8)
	e.String("IBusText")
	attachments := e.BeginArray(8)
	e.EndArray(attachments)
	e.String(text)
	e.Variant("(sa{sv}av)")
	e.Align(8)
	e.String("IBusAttrList")
	attachments = e.BeginArray(8)
	e.EndArray(attachments)
	attributes := e.BeginArray(1)
	e.EndArray(attributes)
}

// readText reads an IBusText, which travels as a variant holding the struct
// (name, attachments, text, attributes), and returns its text alone.
func readText(args *dbus.Decoder, signature string) (string, bool) {
	text, _, ok := readStyledText(args, signature)
	return text, ok
}

// readStyledText reads an IBusText with its attribute list. Attributes of a kind
// this client does not know are kept: the caller picks what it can draw.
func readStyledText(args *dbus.Decoder, signature string) (string, []Attribute, bool) {
	if !strings.HasPrefix(signature, "v") {
		return "", nil, false
	}
	if args.Signature() != "(sa{sv}sv)" {
		return "", nil, false
	}
	args.Align(8)
	if name := args.String(); name != "IBusText" {
		return "", nil, false
	}
	args.Skip("a{sv}")
	text := args.String()
	var attrs []Attribute
	if variant := args.Signature(); variant == "(sa{sv}av)" {
		args.Align(8)
		_ = args.String() // IBusAttrList
		args.Skip("a{sv}")
		end := args.ArrayLength(1)
		for args.Err() == nil && args.Pos() < end {
			signature := args.Signature()
			if signature != "(sa{sv}uuuu)" {
				args.Skip(signature)
				continue
			}
			args.Align(8)
			_ = args.String() // IBusAttribute
			args.Skip("a{sv}")
			attribute := Attribute{Kind: args.Uint32(), Value: args.Uint32()}
			attribute.Start, attribute.End = int(args.Uint32()), int(args.Uint32())
			attrs = append(attrs, attribute)
		}
	} else {
		args.Skip(variant)
	}
	if args.Err() != nil {
		return "", nil, false
	}
	return text, attrs, true
}
