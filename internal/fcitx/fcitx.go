//go:build unix

// Package fcitx is the client half of the Fcitx 4 input method protocol: what a
// window needs to let the user compose Chinese, Japanese or Korean text with an
// input method engine that runs in another process.
//
// Fcitx 4 is a daemon on the user's session bus. A client asks it for an input
// context, tells it about every key the user presses while a text field has
// focus, and tells it where the caret is so its panel can place the candidate
// window. The engine answers each key with an integer saying whether it took
// the key and, as it composes, sends back the marked text still being composed
// (with the clause being converted) and the text the user chose. Candidate
// lists are drawn by the daemon's own panel, so the client needs none.
//
// Fcitx 5 speaks its own protocol and also offers an IBus frontend, which the
// ibus package covers; this package speaks only the Fcitx 4 interface.
//
// A Bus is driven by one thread. It exposes a file descriptor to wait on,
// Dispatch to read what arrived, and Deadline and Expire so a daemon that stops
// answering cannot hold the keyboard: after a bounded wait every pending key is
// released as "not handled" and the bus is closed, and the caller carries on
// without an input method. Nothing here starts a goroutine or a timer.
package fcitx

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/dbus"
)

const (
	service   = "org.fcitx.Fcitx"
	imPath    = "/inputmethod"
	imIface   = "org.fcitx.Fcitx.InputMethod"
	icIface   = "org.fcitx.Fcitx.InputContext"
	icPrefix  = "/inputcontext_"
	dbusIface = "org.freedesktop.DBus"
	dbusPath  = "/org/freedesktop/DBus"

	// Capacities this client declares. Preedit text means the client draws the
	// composition itself instead of in the daemon's input window; the
	// formatted form carries the clause being converted; unfocus commit means
	// the client settles a composition when focus leaves rather than the
	// daemon committing it on the client's behalf; surrounding text lets the
	// engine see and replace the text around the caret for a reconversion.
	CapPreedit             uint32 = 1 << 1
	CapFormattedPreedit    uint32 = 1 << 4
	CapClientUnfocusCommit uint32 = 1 << 5
	CapSurroundingText     uint32 = 1 << 6

	// preeditHighlight is set on the preedit segments the engine is converting.
	// The other format bits describe underline and color; marked text is
	// always drawn underlined, so only the highlighted clause is of use here.
	preeditHighlight int32 = 1 << 4

	// keyTimeout is how long a key waits for the engine before the bus is given
	// up. An engine answers in milliseconds; this is for one that has hung.
	keyTimeout = 2 * time.Second
)

// ErrClosed is returned once the bus has failed or been closed.
var ErrClosed = errors.New("fcitx: the connection is closed")

// transport is the part of a D-Bus connection the client uses.
type transport interface {
	Call(destination, path, iface, member, signature string, body []byte) (*dbus.Message, error)
	Send(destination, path, iface, member, signature string, body []byte) (uint32, error)
	Receive() ([]*dbus.Message, error)
	FD() int
	Close()
}

// Handler receives what the engine sends for one input context. Its methods run
// inside Dispatch, in the order the engine produced them, and a key's answer
// arrives after the text the key produced.
type Handler struct {
	// Commit delivers text the user has chosen.
	Commit func(text string)
	// Preedit reports the marked text and the clause being converted.
	Preedit func(PreeditEvent)
	// DeleteSurrounding asks the client to delete count characters of the text
	// around the caret, starting offset characters from it (a negative offset is
	// before it). The engine does so to replace text it is about to convert again.
	DeleteSurrounding func(offset, count int)
	// Forward asks the client to treat a key as if it had been typed, which an
	// engine does for a key it took apart (an Enter that ends a composition).
	Forward func(keysym, state uint32, release bool)
}

// Range is a span of characters: Start is inclusive and End exclusive.
type Range struct {
	Start, End int
}

// PreeditEvent is the marked text an engine is composing. Empty Text means the
// composition was cleared.
type PreeditEvent struct {
	Text string
	// Cursor is the caret within Text, counted in characters rather than bytes.
	Cursor int
	// Visible is whether Text is meant to be shown.
	Visible bool
	// Selection is the clause the engine is converting, in characters; a zero
	// width Selection is a caret alone.
	Selection Range
}

// Bus is a connection to the Fcitx daemon.
type Bus struct {
	conn        transport
	contexts    map[string]*Context
	pending     []*pendingKey
	err         error
	now         func() time.Time
	dispatching bool
	redispatch  bool
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
}

// Dial connects to the running daemon, or reports why it cannot. A machine
// without Fcitx is the usual reason, not an error worth showing.
func Dial() (*Bus, error) {
	conn, err := dbus.DialSession()
	if err != nil {
		return nil, err
	}
	var body dbus.Encoder
	body.String(service)
	reply, err := conn.Call(dbusIface, dbusPath, dbusIface, "NameHasOwner", "s", body.Bytes())
	if err != nil {
		conn.Close()
		return nil, err
	}
	owned := reply.Args().Bool()
	reply.CloseFDs()
	if !owned {
		conn.Close()
		return nil, errors.New("fcitx: no daemon on the session bus")
	}
	return newBus(conn), nil
}

func newBus(conn transport) *Bus {
	bus := &Bus{conn: conn, contexts: map[string]*Context{}, now: time.Now}
	// Fcitx broadcasts its context signals, so the bus needs a match rule for
	// them; the daemon addresses its replies to this connection already.
	var rule dbus.Encoder
	rule.String("type='signal',interface='" + icIface + "'")
	_, _ = conn.Call(dbusIface, dbusPath, dbusIface, "AddMatch", "s", rule.Bytes())
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
	body.Int32(int32(os.Getpid()))
	reply, err := bus.conn.Call(service, imPath, imIface, "CreateICv3", "si", body.Bytes())
	if err != nil {
		bus.fail(err)
		return nil, err
	}
	args := reply.Args()
	id := args.Int32()
	_ = args.Bool() // whether the engine starts with the method enabled
	args.Uint32()   // the two trigger keys the daemon suggests
	args.Uint32()
	args.Uint32()
	args.Uint32()
	reply.CloseFDs()
	if args.Err() != nil {
		bus.fail(args.Err())
		return nil, args.Err()
	}
	path := icPrefix + strconv.Itoa(int(id))
	context := &Context{bus: bus, path: path, handler: handler}
	bus.contexts[path] = context
	var capacities dbus.Encoder
	capacities.Uint32(CapPreedit | CapFormattedPreedit | CapClientUnfocusCommit | CapSurroundingText)
	if err := context.send("SetCapacity", "u", capacities.Bytes()); err != nil {
		return nil, err
	}
	// The daemon may have sent signals for this context before its answer was
	// read; deliver them now, while the context is still the caller's.
	if err := bus.Dispatch(); err != nil {
		return nil, err
	}
	return context, nil
}

// Dispatch reads what the daemon has sent and runs the handlers it calls for.
func (bus *Bus) Dispatch() error {
	if bus.err != nil {
		return bus.err
	}
	if bus.dispatching {
		bus.redispatch = true
		return nil
	}
	bus.dispatching = true
	defer func() { bus.dispatching = false }()
	for {
		bus.redispatch = false
		messages, err := bus.conn.Receive()
		if err != nil {
			bus.fail(err)
			return err
		}
		for _, message := range messages {
			switch message.Type {
			case dbus.TypeSignal:
				if context := bus.contexts[message.Path]; context != nil && message.Interface == icIface {
					context.signal(message)
				}
			case dbus.TypeReturn, dbus.TypeError:
				bus.answer(message)
			}
			if bus.err != nil {
				return bus.err
			}
		}
		// A synchronous call in a handler may have read newer messages. Finish
		// this batch before delivering them, even when Dispatch was reentered.
		if !bus.redispatch {
			return nil
		}
	}
}

// answer records the engine's verdict on a key and releases the verdicts that
// are ready, oldest first, so keys reach the application in the order typed.
func (bus *Bus) answer(message *dbus.Message) {
	for _, key := range bus.pending {
		if key.serial != message.ReplySerial {
			continue
		}
		key.answered = true
		if message.Type == dbus.TypeReturn && message.Signature == "i" {
			key.handled = message.Args().Int32() != 0
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
		bus.fail(errors.New("fcitx: the daemon did not answer a key in time"))
	}
}

func (context *Context) send(member, signature string, body []byte) error {
	if context.bus.err != nil || context.bus.contexts[context.path] != context {
		return ErrClosed
	}
	if _, err := context.bus.conn.Send(service, context.path, icIface, member, signature, body); err != nil {
		context.bus.fail(err)
		return err
	}
	return nil
}

// Key is one key press or release as the X server reports it: the keysym and
// keycode, the modifier state and the X event time, which an engine uses to tell
// a key it repeats from a second tap of the same key.
type Key struct {
	Keysym  uint32
	Keycode uint32
	State   uint32
	Time    uint32
	Release bool
}

// ProcessKey gives the engine a key. done is called, in order with the other
// keys, with whether the engine consumed it; a key it did not consume is the
// application's to handle.
func (context *Context) ProcessKey(key Key, done func(handled bool)) error {
	if context.bus.err != nil || context.bus.contexts[context.path] != context {
		return ErrClosed
	}
	var body dbus.Encoder
	body.Uint32(key.Keysym)
	body.Uint32(key.Keycode)
	body.Uint32(key.State)
	kind := int32(0)
	if key.Release {
		kind = 1
	}
	body.Int32(kind)
	body.Uint32(key.Time)
	serial, err := context.bus.conn.Send(service, context.path, icIface, "ProcessKeyEvent", "uuuiu", body.Bytes())
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
	body.String(text)
	body.Uint32(cursor)
	body.Uint32(anchor)
	return context.send("SetSurroundingText", "suu", body.Bytes())
}

// SetCursorLocation tells the panel where the caret is, in root window
// coordinates, so the candidate window opens beside it.
func (context *Context) SetCursorLocation(x, y, width, height int32) error {
	var body dbus.Encoder
	body.Int32(x)
	body.Int32(y)
	body.Int32(width)
	body.Int32(height)
	return context.send("SetCursorRect", "iiii", body.Bytes())
}

// Close destroys the input context.
func (context *Context) Close() {
	if context.bus.contexts[context.path] != context {
		return
	}
	delete(context.bus.contexts, context.path)
	if context.bus.err == nil {
		if _, err := context.bus.conn.Send(service, context.path, icIface, "DestroyIC", "", nil); err != nil {
			context.bus.fail(err)
		}
	}
}

func (context *Context) signal(message *dbus.Message) {
	args := message.Args()
	switch message.Member {
	case "CommitString":
		if message.Signature != "s" {
			return
		}
		value := args.String()
		if args.Err() != nil {
			return
		}
		if context.handler.Commit != nil {
			context.handler.Commit(value)
		}
	case "UpdateFormattedPreedit":
		if context.handler.Preedit == nil || message.Signature != "a(si)i" {
			return
		}
		event, ok := readFormattedPreedit(args)
		if ok {
			context.handler.Preedit(event)
		}
	case "UpdatePreedit":
		// The daemon sends this plain form instead of the formatted one when the
		// formatted capacity was refused.
		if context.handler.Preedit == nil || message.Signature != "si" {
			return
		}
		value := args.String()
		cursor := int(args.Int32())
		if args.Err() != nil {
			return
		}
		context.handler.Preedit(plainPreedit(value, cursor))
	case "DeleteSurroundingText":
		if context.handler.DeleteSurrounding == nil || message.Signature != "iu" {
			return
		}
		offset := int(args.Int32())
		count := int(args.Uint32())
		if args.Err() == nil {
			context.handler.DeleteSurrounding(offset, count)
		}
	case "ForwardKey":
		if context.handler.Forward == nil || message.Signature != "uui" {
			return
		}
		keysym, state := args.Uint32(), args.Uint32()
		release := args.Int32() != 0
		if args.Err() == nil {
			context.handler.Forward(keysym, state, release)
		}
	}
}

// readFormattedPreedit reads a(si) segments and the caret's byte offset, joining
// the segments and marking the clause the engine highlights. Fcitx counts the
// caret in UTF-8 bytes; characters are what a text field indexes.
func readFormattedPreedit(args *dbus.Decoder) (PreeditEvent, bool) {
	// An array of structs aligns its elements on eight bytes, as every struct is.
	end := args.ArrayLength(8)
	var builder strings.Builder
	selection := Range{}
	selected := false
	chars := 0
	for args.Err() == nil && args.Pos() < end {
		args.Align(8)
		segment := args.String()
		format := args.Int32()
		start := chars
		chars += utf8.RuneCountInString(segment)
		builder.WriteString(segment)
		if format&preeditHighlight != 0 && chars > start {
			if !selected {
				selection.Start = start
				selected = true
			}
			selection.End = chars
		}
	}
	caret := int(args.Int32())
	if args.Err() != nil || !args.Done() {
		return PreeditEvent{}, false
	}
	value := builder.String()
	cursor := runeOffset(value, caret)
	event := PreeditEvent{Text: value, Cursor: cursor, Visible: value != ""}
	if selected {
		event.Selection = selection
	} else if value != "" {
		event.Selection = Range{Start: cursor, End: cursor}
	}
	return event, true
}

// plainPreedit is the UpdatePreedit form: one string and a byte offset.
func plainPreedit(value string, caret int) PreeditEvent {
	cursor := runeOffset(value, caret)
	event := PreeditEvent{Text: value, Cursor: cursor, Visible: value != ""}
	if value != "" {
		event.Selection = Range{Start: cursor, End: cursor}
	}
	return event
}

// runeOffset converts a byte offset into value to a character offset, clamping
// it to the string.
func runeOffset(value string, bytes int) int {
	if bytes <= 0 {
		return 0
	}
	if bytes >= len(value) {
		return utf8.RuneCountInString(value)
	}
	return utf8.RuneCountInString(value[:bytes])
}
