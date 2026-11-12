//go:build darwin || windows || linux

package headless

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// ErrNoRenderer is returned by Image and Screenshot on platforms without an
// offscreen renderer.
var ErrNoRenderer = errors.New("headless: no offscreen renderer on this platform")

// Options configures a Session.
type Options struct {
	// Scale is device pixels per logical pixel in screenshots. The default, 2,
	// matches a Retina display; use 1 to see how a standard display renders.
	Scale float64
	// ReferenceText lays the application out with the reference text metrics
	// (internal/textmetrics): the same widths on every platform, whatever fonts are
	// installed, for a test or a tool that compares layouts across machines. By
	// default a session measures with the platform's text engine, as a window does
	// through that engine, so a screenshot shows the layout the window would have.
	ReferenceText bool
	// Strict checks the frame after each settle: two controls that share a
	// non-zero identity, or two interactive controls that share a label, are a
	// mistake a page makes silently (the two identities name one state, and a
	// selector finds one control by chance). Labels may repeat by default.
	Strict bool
}

// Session is one application run without a window. It is not safe for
// concurrent use.
type Session struct {
	driver *app.Driver
	// config is the window the application opened with. It decides the window
	// chrome a screenshot carries over the scene (see drawWindowChrome).
	config   app.Config
	options  Options
	renderer *renderer
	// canvas measures the text of the input overlay a recording can show.
	canvas *gfx.Canvas
	// marks are the inputs delivered lately enough for the overlay to show, and
	// showInput says a recording is drawing them.
	marks     []inputMark
	showInput bool
}

// Open starts config's application in a driver and paints its first frame.
// canvas may be nil.
func Open(config app.Config, canvas *gfx.Canvas, options Options) (*Session, error) {
	if options.Scale == 0 {
		options.Scale = defaultScale
	}
	if options.Scale < 1 {
		return nil, fmt.Errorf("headless: scale must be at least 1, got %g", options.Scale)
	}
	if canvas == nil {
		canvas = &gfx.Canvas{}
	}
	// The canvas paints for the scale the screenshots are rendered at, as a window's
	// platform tells it: icons and strokes are rounded to whole pixels of it.
	canvas.SetPixelScale(float32(options.Scale))
	if !options.ReferenceText {
		if engine := app.PlatformTextEngine(); engine != nil {
			canvas.SetTextEngine(engine)
		}
	}
	driver, err := app.NewDriver(config, canvas)
	if err != nil {
		return nil, err
	}
	return &Session{driver: driver, config: config, options: options, canvas: canvas}, nil
}

// Close releases the session.
func (session *Session) Close() {
	if session.renderer != nil {
		session.renderer.close()
		session.renderer = nil
	}
	session.driver.Close()
}

// Driver exposes the underlying driver for operations the session does not wrap.
func (session *Session) Driver() *app.Driver { return session.driver }

// Snapshot captures the current frame.
func (session *Session) Snapshot() app.Snapshot { return session.driver.Snapshot() }

// Move moves the pointer to a logical-pixel point.
func (session *Session) Move(x, y float32) error { return session.driver.Move(x, y) }

// PressAt reports what a press at a point would do in a custom-titlebar window
// (see app.Driver.PressAt).
func (session *Session) PressAt(x, y float32) app.PressKind { return session.driver.PressAt(x, y) }

// Click clicks at a logical-pixel point.
func (session *Session) Click(x, y float32) error {
	session.markPointer(x, y)
	return session.driver.Click(x, y)
}

// RightClick presses and releases the secondary button at a logical-pixel point,
// which is what opens a context menu.
func (session *Session) RightClick(x, y float32) error {
	session.markPointer(x, y)
	return session.driver.ClickButton(x, y, gfx.PointerButtonSecondary)
}

// RightClickLabel right-clicks the center of the control a selector names: a label, or a role
// or identity (see Locate).
func (session *Session) RightClickLabel(label string) error {
	hit, err := session.Locate(label)
	if err != nil {
		return fmt.Errorf("headless: %w", err)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	session.markPointer(x, y)
	return session.driver.ClickButton(x, y, gfx.PointerButtonSecondary)
}

// DoubleClick double-clicks at a logical-pixel point.
func (session *Session) DoubleClick(x, y float32) error {
	session.markPointer(x, y)
	return session.driver.DoubleClick(x, y)
}

// DoubleClickLabel double-clicks the center of the control a selector names (see Locate).
func (session *Session) DoubleClickLabel(label string) error {
	hit, err := session.Locate(label)
	if err != nil {
		return fmt.Errorf("headless: %w", err)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	session.markPointer(x, y)
	return session.driver.DoubleClick(x, y)
}

// Press presses the primary button at a point; Release releases it.
func (session *Session) Press(x, y float32) error {
	session.markPointer(x, y)
	return session.driver.Press(x, y, gfx.PointerButtonPrimary)
}

// Release releases the primary button at a point.
func (session *Session) Release(x, y float32) error {
	return session.driver.Release(x, y, gfx.PointerButtonPrimary)
}

// Drop delivers files dragged from the system onto a logical-pixel window point.
// With gfx.FileDropOver it is the hover, with gfx.FileDropDrop the drop; the result
// is whether the hit under the point takes files.
func (session *Session) Drop(event gfx.FileDropEvent) (bool, error) {
	return session.driver.Drop(event)
}

// Scroll delivers a scroll gesture at a logical-pixel window point.
func (session *Session) Scroll(event gfx.ScrollEvent) error { return session.driver.Scroll(event) }

// Resize changes the content size in logical pixels.
func (session *Session) Resize(width, height int) error { return session.driver.Resize(width, height) }

// SetSystemAppearance changes the system appearance the run reports, and tells
// whoever watches it, as the system would.
func (session *Session) SetSystemAppearance(appearance app.Appearance) {
	session.driver.SetSystemAppearance(appearance)
}

// Type inserts text into the focused text field.
func (session *Session) Type(value string) error {
	session.markKeys(value)
	return session.driver.Type(value)
}

// Compose sets the input-method composition of the focused field; an empty
// value ends it. See app.Driver.Compose.
func (session *Session) Compose(value string) error { return session.driver.Compose(value) }

// Wait advances the animation clock.
func (session *Session) Wait(duration time.Duration) error { return session.driver.Advance(duration) }

// maxRest bounds how long Rest plays animations.
const maxRest = 5 * time.Second

// Rest advances the animation clock until no transition is in flight, so the
// frame shows the interface at rest: a dialog that has just been opened is in,
// not still fading. A slow display, such as a spinner, does not hold it. The
// clock moves at most five seconds.
func (session *Session) Rest() error {
	for elapsed := time.Duration(0); session.driver.Transitioning() && elapsed < maxRest; elapsed += time.Second / 120 {
		if err := session.driver.Advance(time.Second / 120); err != nil {
			return err
		}
	}
	return nil
}

// Hit returns the front-most reachable region whose accessibility label is
// exactly label, or failing that contains it (case-insensitively).
func (session *Session) Hit(label string) (gfx.Hit, bool) {
	hits := session.driver.Snapshot().Hits
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.Label == label && session.reachable(hit) {
			return hit, true
		}
	}
	lower := strings.ToLower(label)
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if strings.Contains(strings.ToLower(hit.Label), lower) && session.reachable(hit) {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func (session *Session) reachable(hit gfx.Hit) bool {
	if hit.Block || hit.MovesWindow || hit.ID.IsZero() {
		return false
	}
	target, ok := session.driver.HitTest(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2)
	return ok && target == hit.ID
}

// ClickLabel clicks the center of the control a selector names: a label, or a role or
// identity (see Locate).
func (session *Session) ClickLabel(label string) error {
	hit, err := session.Locate(label)
	if err != nil {
		return fmt.Errorf("headless: %w", err)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	session.markPointer(x, y)
	return session.driver.Click(x, y)
}

// Texts returns every text the application painted, in paint order.
func (session *Session) Texts() []string {
	painted := session.driver.Snapshot().Texts
	values := make([]string, len(painted))
	for index, value := range painted {
		values[index] = value.Value
	}
	return values
}

// HasText reports whether any painted text contains substring.
func (session *Session) HasText(substring string) bool {
	for _, value := range session.Texts() {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}

// WaitFor applies background wakes in real time until condition holds for the
// current frame, or timeout passes. It is for work that takes real time, such
// as a network connection. The real time it waits also passes for the
// application's timers: a frame the application asked for in 400 ms is due after
// 400 ms of waiting, as in a window, so a page that hands over after a delay
// does so here too. Animations that a script wants to step exactly are moved
// with Wait instead.
func (session *Session) WaitFor(condition func(app.Snapshot) bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	clock := realClock{last: time.Now()}
	for {
		if err := session.driver.Pump(); err != nil {
			return err
		}
		if err := clock.advance(session.driver); err != nil {
			return err
		}
		if condition(session.driver.Snapshot()) {
			return session.CheckStrict()
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("headless: condition not met within %v", timeout)
		}
		select {
		case <-session.driver.Wakes():
		case <-time.After(min(remaining, 25*time.Millisecond)):
		}
	}
}

// realClock carries the real time that went by during a wait over to the
// driver's virtual time, so frames the application scheduled come due.
type realClock struct{ last time.Time }

// maxClockStep bounds one step so a long stall (a suspended process) does not
// play a whole animation in one go.
const maxClockStep = 200 * time.Millisecond

func (clock *realClock) advance(driver *app.Driver) error {
	now := time.Now()
	elapsed := min(now.Sub(clock.last), maxClockStep)
	clock.last = now
	if elapsed <= 0 {
		return nil
	}
	return driver.Advance(elapsed)
}

// Settle applies background wakes until none arrives for quiet, or timeout
// passes, so work that was kicked off has landed before the next look.
func (session *Session) Settle(quiet, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	clock := realClock{last: time.Now()}
	for time.Now().Before(deadline) {
		if err := session.driver.Pump(); err != nil {
			return err
		}
		select {
		case <-session.driver.Wakes():
		case <-time.After(quiet):
			if err := clock.advance(session.driver); err != nil {
				return err
			}
			if err := session.driver.Pump(); err != nil {
				return err
			}
			return session.CheckStrict()
		}
		if err := clock.advance(session.driver); err != nil {
			return err
		}
	}
	return session.CheckStrict()
}

// CheckStrict reports the per-paint values a page gave two controls at once: a
// non-zero identity, or a label among interactive controls. It is a no-op unless
// Options.Strict is set.
func (session *Session) CheckStrict() error {
	if !session.options.Strict {
		return nil
	}
	hits := session.driver.Snapshot().Hits
	ids := map[gfx.ID]bool{}
	labels := map[string]bool{}
	for _, hit := range hits {
		if hit.ID.IsZero() || hit.IsLabel || hit.Backdrop || hit.Block {
			continue
		}
		if ids[hit.ID] {
			return fmt.Errorf("headless: two controls share the identity %v", hit.ID)
		}
		ids[hit.ID] = true
		if hit.Label == "" {
			continue
		}
		if labels[hit.Label] {
			return fmt.Errorf("headless: two controls share the label %q", hit.Label)
		}
		labels[hit.Label] = true
	}
	return nil
}

// Image renders the current frame at the session's scale, window chrome the
// platform draws over the scene included.
func (session *Session) Image() (*image.RGBA, error) {
	if err := session.driver.Pump(); err != nil {
		return nil, err
	}
	if session.renderer == nil {
		session.renderer = newRenderer()
	}
	snapshot := session.driver.Snapshot()
	if session.showInput {
		snapshot = session.drawInput(snapshot)
	}
	picture, err := session.renderer.render(snapshot, session.options.Scale)
	if err != nil {
		return nil, err
	}
	if !session.driver.Fullscreen() {
		drawWindowChrome(picture, session.config, session.options.Scale)
	}
	return picture, nil
}

// Screenshot renders the current frame to a PNG file.
func (session *Session) Screenshot(path string) error {
	picture, err := session.Image()
	if err != nil {
		return err
	}
	return writePNG(path, picture)
}

// writePNG writes picture to path as a PNG.
func writePNG(path string, picture *image.RGBA) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(file, picture)
	if closeErr := file.Close(); encodeErr == nil {
		encodeErr = closeErr
	}
	return encodeErr
}

// Keys delivers a key chord written as plus-separated parts, for example
// "tab", "shift+tab", "enter", "backspace", "left", "cmd+a", or a single
// character such as "x". Command chords are editing shortcuts in text fields;
// Control and a terminal-configured Alt chord are raw application input. A bare
// character is typed into the focused text field.
func (session *Session) Keys(spec string) error {
	session.markKeys(keysLabel(spec))
	prefix, name := "", spec
	if cut := strings.LastIndex(spec, "+"); cut > 0 {
		if cut == len(spec)-1 { // the plus key itself, as in "cmd++"
			prefix, name = strings.TrimSuffix(spec[:cut], "+"), "+"
		} else {
			prefix, name = spec[:cut], spec[cut+1:]
		}
	}
	modifiers, err := parseModifiers(prefix)
	if err != nil {
		return fmt.Errorf("%w in %q", err, spec)
	}
	if key, ok := namedKeys[strings.ToLower(name)]; ok && !key.IsModifier() {
		return session.driver.Key(key, modifiers)
	}
	if utf8.RuneCountInString(name) != 1 {
		return fmt.Errorf("headless: unknown key %q in %q", name, spec)
	}
	character, _ := utf8.DecodeRuneInString(name)
	if modifiers&gfx.ModifierMeta != 0 {
		handled, err := session.driver.ShortcutChord(modifiers, character|0x20) // shortcuts are lowercase
		if err == nil && !handled {
			err = fmt.Errorf("headless: Command+%s is not an editing shortcut or no text field is focused", name)
		}
		return err
	}
	if modifiers&(gfx.ModifierControl|gfx.ModifierAlt) != 0 {
		// Control and terminal-configured Alt chords are raw keys; Driver.Chord
		// keeps Alt on the text path when the terminal did not opt in.
		err := session.driver.Chord(modifiers, name)
		if errors.Is(err, app.ErrNoTerminal) {
			err = fmt.Errorf("headless: %q needs a focused terminal", spec)
		}
		return err
	}
	if modifiers&^gfx.ModifierShift != 0 {
		return fmt.Errorf("headless: %q has no text to type", spec)
	}
	if err := session.driver.Chord(modifiers, name); !errors.Is(err, app.ErrNoTerminal) {
		return err
	}
	return session.driver.Type(name)
}

// KeyDown presses and holds one key named as Keys names it, without
// modifiers: a key such as "left" or "x", or a modifier such as "shift",
// "ctrl", "alt", "cmd" (left side; "rshift", "rctrl", "ralt", "rcmd" for the
// right). Modifiers stay held for every pointer, scroll and key until KeyUp.
func (session *Session) KeyDown(name string) error {
	key, character, err := holdableKey(name)
	if err != nil {
		return err
	}
	return session.driver.KeyDown(key, character)
}

// KeyUp releases a key KeyDown pressed.
func (session *Session) KeyUp(name string) error {
	key, character, err := holdableKey(name)
	if err != nil {
		return err
	}
	return session.driver.KeyUp(key, character)
}

// LoseFocus makes the window lose the keyboard: held keys are released.
func (session *Session) LoseFocus() error { return session.driver.LoseFocus() }

var namedKeys = map[string]gfx.Key{
	"tab": gfx.KeyTab, "enter": gfx.KeyEnter, "return": gfx.KeyEnter, "space": gfx.KeySpace,
	"esc": gfx.KeyEscape, "escape": gfx.KeyEscape, "left": gfx.KeyLeft, "right": gfx.KeyRight,
	"up": gfx.KeyUp, "down": gfx.KeyDown, "home": gfx.KeyHome, "end": gfx.KeyEnd,
	"backspace": gfx.KeyBackspace, "delete": gfx.KeyDelete, "del": gfx.KeyDelete,
	"pageup": gfx.KeyPageUp, "pagedown": gfx.KeyPageDown, "insert": gfx.KeyInsert,
	"f1": gfx.KeyF1, "f2": gfx.KeyF2, "f3": gfx.KeyF3, "f4": gfx.KeyF4, "f5": gfx.KeyF5, "f6": gfx.KeyF6,
	"f7": gfx.KeyF7, "f8": gfx.KeyF8, "f9": gfx.KeyF9, "f10": gfx.KeyF10, "f11": gfx.KeyF11, "f12": gfx.KeyF12,
	"shift": gfx.KeyShiftLeft, "rshift": gfx.KeyShiftRight,
	"ctrl": gfx.KeyControlLeft, "control": gfx.KeyControlLeft, "rctrl": gfx.KeyControlRight,
	"alt": gfx.KeyAltLeft, "option": gfx.KeyAltLeft, "opt": gfx.KeyAltLeft, "ralt": gfx.KeyAltRight,
	"cmd": gfx.KeyMetaLeft, "command": gfx.KeyMetaLeft, "meta": gfx.KeyMetaLeft, "super": gfx.KeyMetaLeft, "rcmd": gfx.KeyMetaRight,
}

func holdableKey(name string) (gfx.Key, string, error) {
	if key, ok := namedKeys[strings.ToLower(name)]; ok {
		return key, "", nil
	}
	if utf8.RuneCountInString(name) != 1 {
		return 0, "", fmt.Errorf("headless: unknown key %q", name)
	}
	return gfx.KeyUnknown, name, nil
}

func parseModifiers(spec string) (gfx.Modifiers, error) {
	var modifiers gfx.Modifiers
	for _, part := range strings.Split(spec, "+") {
		switch strings.ToLower(part) {
		case "cmd", "command", "meta", "super":
			modifiers |= gfx.ModifierMeta
		case "ctrl", "control":
			modifiers |= gfx.ModifierControl
		case "alt", "option", "opt":
			modifiers |= gfx.ModifierAlt
		case "shift":
			modifiers |= gfx.ModifierShift
		case "":
		default:
			return 0, fmt.Errorf("headless: unknown modifier %q", part)
		}
	}
	return modifiers, nil
}
