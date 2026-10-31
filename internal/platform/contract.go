package platform

import (
	"errors"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// Color is a linear RGBA clear color.
type Color struct {
	Red   float64
	Green float64
	Blue  float64
	Alpha float64
}

// Config contains the data needed to create a native window.
type Config struct {
	Title string
	// Class names the application to the desktop environment where it keeps
	// windows of one application together under a launcher entry (the X11
	// WM_CLASS). Empty means the executable's file name.
	Class string
	// Icon is the application's picture where the desktop shows one for a
	// window: the taskbar and Alt+Tab on Windows, the taskbar and the title
	// bar on X11. Zero means none.
	Icon   WindowIcon
	Width  int
	Height int
	// MinWidth and MinHeight constrain the content area in logical pixels.
	MinWidth  int
	MinHeight int
	// Origin, when HasOrigin, is where the top-left of the content area opens, in
	// screen coordinates. Without one the system places the window.
	Origin     ui.Point
	HasOrigin  bool
	WindowMode WindowMode
	// TitlebarHeight is the height in logical pixels of the strip a custom
	// titlebar leaves for the application. Zero means DefaultTitlebarHeight.
	TitlebarHeight   float32
	ClearColor       Color
	WindowBackdrop   func() BackdropStyle
	OnBackdropChange func(bool)
	Rectangles       []render.Rectangle
	TextLabels       []TextLabel
	Accessibility    []AccessibilityNode
	OnPointer        PointerHandler
	// PressAt says what a press at a window point in logical pixels does, so a
	// custom-titlebar window knows whether to move there (see PressKind). Nil
	// means PressClient everywhere.
	PressAt  func(x, y float32) PressKind
	OnResize ResizeHandler
	// OnFrameChange, when set, runs on the window thread after the window's
	// content rectangle moved or changed size on screen. It receives the same
	// rectangle Frame reports, so an application can save where its window was.
	OnFrameChange FrameChangeHandler
	// OnMinimize, when set, runs on the window thread when the window is
	// minimized or restored. It is the platform's own state, not Hide.
	OnMinimize MinimizeHandler
	// OnMaximize, when set, runs on the window thread when the window is
	// maximized or restored.
	OnMaximize MaximizeHandler
	// OnReopen handles a native reopen request, such as a macOS Dock click.
	// When set, it must show the window; otherwise the adapter shows it directly.
	OnReopen   func()
	OnKey      KeyHandler
	OnShortcut ShortcutHandler
	// OnFocusLost, when set, runs when the window stops being the one the
	// keyboard types into (another window or application took the focus), so
	// the runtime can release the keys that were down: their releases will go
	// to the other window.
	OnFocusLost FocusLostHandler
	OnScroll    ScrollHandler
	// OnFileDrop, when set, makes the window a target for files dragged from the
	// system. The adapter reports hover and leave where the system does and
	// always the drop.
	OnFileDrop FileDropHandler
	// OnAccessibility, when set, receives what assistive technology asks of a
	// node: press it, focus it, set its value. The adapter calls it on the
	// event-loop thread and presents the frames it hands back.
	OnAccessibility AccessibilityHandler
	OnRedraw        RedrawHandler
	// Waker, when set together with OnRedraw, lets the runtime request one
	// redraw from any goroutine without a parked reader goroutine.
	Waker          *Waker
	OnCloseRequest CloseRequestHandler
	// OnClosed runs exactly once on the native event-loop thread after this
	// window has stopped receiving events and released its platform resources.
	OnClosed         func()
	TextInput        *TextInput
	DynamicTextInput bool
	// Actions, when set, is filled by the adapter with what the window controls
	// that Viva draws need: see WindowActions. It is set for a WindowCustomTitlebar
	// window on a platform whose system draws no controls for it.
	Actions *WindowActions
	// Anchor is where a WindowPanel appears: beside the status item that was
	// clicked. Without one the panel is centered on the screen.
	Anchor    ScreenRect
	HasAnchor bool
}

// WindowActions are the operations behind the minimize, maximize and close
// buttons Viva draws in a custom titlebar. The adapter assigns every function
// before the window is shown; they run on the event-loop thread.
type WindowActions struct {
	// Minimize puts the window in the taskbar or dock.
	Minimize func()
	// ToggleMaximize maximizes a window, or restores a maximized one.
	ToggleMaximize func()
	// RequestClose is the close button: it behaves as the window manager's own
	// would, which is to ask OnCloseRequest if there is one.
	RequestClose func()
	// Maximized reports whether the window is maximized, so the button can show
	// the glyph for the opposite action.
	Maximized func() bool
}

// Window is one native window owned by an Application. Close is thread-safe,
// idempotent, and posts the actual close to the native event loop. Hide and
// Show are thread-safe and post to the event loop as well.
type Window interface {
	Close()
	// Hide takes the window off the screen without releasing it: its state,
	// content and GPU resources stay, and it keeps the application running.
	Hide()
	// Show puts a hidden window back, in front, and brings the application to
	// the front with it. Showing a window that is on screen raises it.
	Show()
	// Frame is the content area in screen coordinates (logical pixels from the
	// primary display's top-left corner, y down), false when the window cannot
	// say. It is safe to call from any goroutine.
	Frame() (ui.Rect, bool)
}

// Application creates windows that share one native application event loop.
// OpenWindow, AddStatusItem and SetDockBadge are called on that event-loop
// thread.
type Application interface {
	OpenWindow(Config) (Window, error)
	// AddStatusItem puts an icon in the system's status area: the menu bar on
	// macOS, the notification area on Windows, a StatusNotifier host on Linux.
	AddStatusItem(StatusItemConfig) (StatusItem, error)
	// SetDockBadge marks the application's icon in the Dock with text, the red
	// badge macOS draws at its top corner. An empty text removes it. A desktop
	// without a Dock accepts it and does nothing.
	SetDockBadge(text string)
	// Quit closes every window, bypassing close requests, and ends the loop.
	// It is thread-safe.
	Quit()
}

// Secret store errors, shared by the three adapters. An adapter returns these (or
// an error wrapping them) so the application layer can name them.
var (
	// ErrSecretNotFound means the store holds nothing for the service and account.
	ErrSecretNotFound = errors.New("platform: secret not found")
	// ErrSecretStoreUnavailable means the system has no store to ask.
	ErrSecretStoreUnavailable = errors.New("platform: the system has no secret store")
	// ErrSecretDenied means the user or the system refused access to the store.
	ErrSecretDenied = errors.New("platform: access to the secret store was denied")
)

// ErrNoGraphics is returned by OpenWindow (wrapped, with what to install) when the
// machine has no graphics driver to draw a window with: no Vulkan on Linux, no
// Direct3D 11 device, not even the software one, on Windows. A cloud server with
// no GPU is the usual case, and it still runs with a software driver; a server with
// no display at all needs the headless build instead.
var ErrNoGraphics = errors.New("platform: no graphics driver")

// ErrStatusAreaUnavailable is returned by AddStatusItem when the desktop has no
// status area to put an icon in, as a Linux session without a StatusNotifier
// host. The application keeps running; it only has no icon.
var ErrStatusAreaUnavailable = errors.New("platform: the desktop has no status area")

// StatusItem is one icon in the system's status area.
type StatusItem interface {
	// SetIcon replaces the icon.
	SetIcon(StatusIcon) error
	// SetTooltip replaces the text shown while the pointer rests on the icon.
	SetTooltip(string)
	// Remove takes the icon away. It is idempotent and thread-safe.
	Remove()
}

// StatusIcon is a square image for the status area.
type StatusIcon struct {
	// Size is the width and height in pixels. 32 or more keeps it sharp on the
	// densest bars; the platform scales it to the size its bar draws.
	Size int
	// RGBA is Size*Size straight (not premultiplied) 8-bit pixels, top row
	// first.
	RGBA []byte
	// Template marks a shape that only has an alpha channel to say: the
	// platform colors it to suit the bar, light on dark and dark on light.
	Template bool
}

// Valid reports whether the icon's pixel data matches its size.
func (icon StatusIcon) Valid() bool {
	return icon.Size > 0 && icon.Size <= 1024 && len(icon.RGBA) == icon.Size*icon.Size*4
}

// ScreenRect is a rectangle on the desktop in the units and orientation of the
// adapter that produced it. Only that adapter interprets it: an application
// hands a StatusClick's anchor back to open a panel and reads nothing in it.
type ScreenRect struct {
	X, Y          float32
	Width, Height float32
}

// StatusClick is one press on a status item.
type StatusClick struct {
	Button ui.PointerButton
	// Anchor is where the item is, for a panel opened beside it.
	Anchor ScreenRect
	// PanelDismissed is true when a WindowPanel closed itself, for losing focus,
	// because of this very press. A click that toggles a panel must not open it
	// again then.
	PanelDismissed bool
}

// StatusItemConfig describes a status item. OnClick and Menu run on the native
// event-loop thread. With a Menu the system shows it for a press of the item.
// OnClick then runs only for a press the system does not turn into the menu: a
// primary press, which a Linux host reports as an activation of the item.
type StatusItemConfig struct {
	Tooltip string
	Icon    StatusIcon
	OnClick func(StatusClick)
	Menu    func() []MenuItem
}

// MenuItem is one entry of a status item's menu. A separator has no other
// field. OnSelect runs on the event-loop thread after the menu has closed.
type MenuItem struct {
	Title     string
	Separator bool
	Disabled  bool
	Checked   bool
	Submenu   []MenuItem
	OnSelect  func()
}

// ApplicationStart initializes the first windows before the event loop waits.
type ApplicationStart func(Application) error

// TextLabel is one immutable line of system-font text composited by the native GPU backend.
// A label with Runs is a fixed-pitch grid line: Mono selects the system
// fixed-pitch font, CellWidth is the column pitch and each run colors a stretch
// of Value (see ui.TextRun). Runs is only valid for the call that carries it.
type TextLabel struct {
	Value  string
	X      float32
	Y      float32
	Width  float32
	Height float32
	// OriginX and OriginY are where layout put the text, less X and Y (which the painter
	// rounds to whole logical pixels): within half a pixel either way. A backend that
	// places glyphs at device-pixel positions, as the platform's own text does, draws
	// the text OriginX, OriginY from the label's top left; the others ignore them.
	OriginX   float32
	OriginY   float32
	FontSize  float32
	Weight    float32
	Color     Color
	Layer     uint16
	Mono      bool
	MonoFace  ui.MonoFace
	CellWidth float32
	Runs      []ui.TextRun
	// Clip, when Clipped, is the logical-pixel region of the window this label
	// may paint into. It is set only for a label a canvas clip cuts partway:
	// a label wholly inside its clip, and one wholly outside it (which is
	// never sent), carry none. Renderers scissor the label's quad to it rather
	// than rasterize a different texture, so scrolling a clipped label costs no
	// text layout.
	Clip    ui.Rect
	Clipped bool
	// Bitmap is premultiplied BGRA, top row first. When non-empty this label
	// is an image quad rather than text; BitmapStride may exceed BitmapWidth*4.
	// BitmapID and BitmapVersion identify immutable content across frames.
	Bitmap        []byte
	BitmapWidth   int
	BitmapHeight  int
	BitmapStride  int
	BitmapID      uint64
	BitmapVersion uint32
	// BitmapUpload is true only when Bitmap content must cross a native
	// boundary. A false bitmap label references the matching retained texture.
	BitmapUpload bool
}

// PressKind is what a press at a window point does in a custom-titlebar window
// (see input.PressKind): the application takes it, a control takes it, the
// platform moves the window at once, or the application takes it and the
// platform moves the window if the pointer drags.
type PressKind = input.PressKind

const (
	PressClient     = input.PressClient
	PressControl    = input.PressControl
	PressMove       = input.PressMove
	PressMoveOnDrag = input.PressMoveOnDrag
)

// PointerEventKind identifies one native pointer event.
type PointerEventKind uint8

const (
	PointerDown PointerEventKind = iota + 1
	PointerMove
	PointerUp
	PointerExit
)

// PointerEvent is a pointer position in logical window pixels. Move and Exit
// events only fire while the pointer is inside the window.
type PointerEvent struct {
	Kind      PointerEventKind
	Button    ui.PointerButton
	Modifiers input.Modifiers
	X         float32
	Y         float32
	// ScreenX and ScreenY are the pointer in screen coordinates (see
	// ui.PointerEvent).
	ScreenX, ScreenY float32
	// ClickCount is the number of consecutive presses this Down or Up belongs
	// to (2 for a double click). Move and Exit events leave it 0.
	ClickCount int
}

// Size is a window client size in logical pixels.
type Size struct {
	Width  float32
	Height float32
	// Scale is the device pixels per logical pixel of the display the window is on
	// (1.25 at 125%), or zero when the platform does not say. The application paints
	// what must land on whole device pixels for it.
	Scale float32
}

// KeyEvent is one normalized keyboard event. Key identities mirror
// input.Key and modifiers mirror input.Modifiers.
type KeyEvent struct {
	Key       input.Key
	Modifiers input.Modifiers
	Repeat    bool
	// Release marks a key going up. Adapters send releases, and modifier keys
	// (input.KeyShiftLeft and the like), only while the focused terminal asked
	// for them (TextInput.KeyReleases).
	Release bool
	// Code names the physical key in the platform's own numbering, nonzero and
	// the same for a key's press and its release. The runtime pairs releases
	// with presses by it, so a release can carry the identity its press had
	// whatever the modifiers did in between. Zero means the key is not followed
	// and no release will be asked for.
	Code uint32
	// Text is the character of a chord without its modifiers, set for raw
	// terminal keys and shortcut offers (see ui.KeyEvent).
	Text string
	// Typed is the text a printable key types with the modifiers in effect,
	// set on the press of a terminal that asked for releases (see ui.KeyEvent).
	Typed string
	// Win32 is the console key identity on Windows; other platforms leave it zero.
	Win32 ui.Win32Key
}

// AccessibilityNode is one platform-neutral accessibility element derived
// from the semantic tree. The tree is flat in window order; a node's container
// is found by AccessibilityContainer.
type AccessibilityNode struct {
	// ID names the control the node stands for, so a request from assistive
	// technology reaches it whatever position it has in the next frame.
	ID        ui.ID
	Role      uint8
	X         float32
	Y         float32
	Width     float32
	Height    float32
	Label     string
	Value     string
	Hint      string
	Focused   bool
	Disabled  bool
	Checked   bool
	Mixed     bool
	Selected  bool
	Expanded  bool
	Multiline bool
	ReadOnly  bool
	Secure    bool
	// Focusable marks a node the keyboard can reach.
	Focusable bool
	// Expandable marks a node that opens and closes: Expanded says which.
	Expandable bool
	// Description is a longer explanation than Hint.
	Description string
	// Level is a heading's level, or a tree item's depth; zero when not stated.
	Level uint8
	// PosInSet and SetSize place the node among its siblings, counting from 1;
	// zero when not stated.
	PosInSet, SetSize int32
	// Live makes the node announce changes to its content.
	Live ui.LiveRegion
	// Range is the numeric value of a slider or progress bar.
	Range ui.ValueRange
	// Selection is the selected span of the focused text field's Value.
	Selection AccessibilitySelection
}

// AccessibilitySelection is a span of a text node's Value in Unicode code
// points: Start and End are the anchor and the caret, so End < Start for a
// selection made backwards and Start == End for a bare caret. Valid is false
// when the node has no caret, as for a field that does not have the keyboard.
type AccessibilitySelection struct {
	Start, End int32
	Valid      bool
}

// Frame is the complete renderer state for one repaint.
type Frame struct {
	Update        render.Update
	Rectangles    []render.Rectangle
	TextLabels    []TextLabel
	Accessibility []AccessibilityNode
	TextInput     TextInputUpdate
}

// TextInputUpdate explicitly preserves, replaces, or disables the focused
// native text editor. Changed=false preserves the current editor; Changed=true
// with Input=nil disables it.
type TextInputUpdate struct {
	Changed bool
	Input   *TextInput
}

// PointerHandler handles one pointer event and returns renderer state plus
// the exact changed range.
type PointerHandler func(PointerEvent) (Frame, error)

// ResizeHandler repaints window content at a new client size. The returned
// frame is presented with exactly one GPU submission.
type ResizeHandler func(Size) (Frame, error)

// FrameChangeHandler reports the window's content rectangle in screen
// coordinates after it moved or changed size. It runs on the window thread.
type FrameChangeHandler func(ui.Rect)

// MinimizeHandler reports that the window was minimized or restored. It runs
// on the window thread.
type MinimizeHandler func(minimized bool)

// MaximizeHandler reports that the window was maximized or restored. It runs
// on the window thread.
type MaximizeHandler func(maximized bool)

// KeyHandler applies one keyboard event and returns an optional repaint.
type KeyHandler func(KeyEvent) (Frame, error)

// FocusLostHandler reports that the window lost the keyboard and returns an
// optional repaint.
type FocusLostHandler func() (Frame, error)

// ShortcutHandler offers one Control- or Meta-modified printable chord before
// native text handling and reports whether the application consumed it.
type ShortcutHandler func(KeyEvent) (Frame, bool, error)

// ShortcutCandidate reports whether an event is a modified printable chord
// that may be offered before native text handling. Alt alone stays with the
// text service so dead keys and input methods retain ownership.
func ShortcutCandidate(event KeyEvent) bool {
	return event.Modifiers&(input.ModifierControl|input.ModifierMeta) != 0 &&
		utf8.RuneCountInString(event.Text) == 1
}

// ScrollEvent is one normalized scroll gesture at X,Y in logical window
// pixels, with the modifiers active for that gesture. Positive DeltaY reveals
// content below and positive DeltaX reveals content to the right, so a delta
// can be added directly to a scroll offset. Adapters honor the operating
// system's scroll-direction setting by passing through whatever direction the
// system already applied. Precise reports whether the deltas are device pixels
// (trackpad) or scaled line approximations (classic mouse wheels).
type ScrollEvent struct {
	Modifiers input.Modifiers
	X         float32
	Y         float32
	DeltaX    float32
	DeltaY    float32
	Precise   bool
}

// ScrollHandler applies one scroll gesture and returns an optional repaint.
type ScrollHandler func(ScrollEvent) (Frame, error)

// FileDropPhase is where a drag of files from the system is.
type FileDropPhase uint8

const (
	FileDropOver FileDropPhase = iota
	FileDropLeave
	FileDropDrop
)

// FileDropEvent is one step of a drag of files over a window, at X,Y in logical
// window pixels. Paths are set for FileDropDrop only.
type FileDropEvent struct {
	Phase FileDropPhase
	X, Y  float32
	Paths []string
}

// FileDropHandler takes one step of a drag and reports whether the point
// accepts files, plus an optional repaint.
type FileDropHandler func(FileDropEvent) (Frame, bool, error)

// RedrawHandler repaints in response to one explicit cross-thread wake-up.
// Platform adapters always invoke it on their native window thread.
type RedrawHandler func() (Frame, error)

// Waker posts one redraw event to the native event loop from any goroutine.
// An adapter binds its thread-safe post primitive when its loop starts and
// unbinds it before teardown; waking while unbound is a no-op. It needs no
// goroutine, so an idle application owns no parked reader.
type Waker struct {
	mu   sync.Mutex
	wake func()
}

// Bind installs the adapter's post primitive.
func (waker *Waker) Bind(wake func()) {
	waker.mu.Lock()
	waker.wake = wake
	waker.mu.Unlock()
}

// Unbind removes the post primitive and waits for an in-flight Wake.
func (waker *Waker) Unbind() {
	waker.mu.Lock()
	waker.wake = nil
	waker.mu.Unlock()
}

// Wake requests one OnRedraw call on the window thread.
func (waker *Waker) Wake() {
	waker.mu.Lock()
	defer waker.mu.Unlock()
	if waker.wake != nil {
		waker.wake()
	}
}

// WindowMode selects native window chrome.
type WindowMode uint8

const (
	// WindowTitled is the standard titled window with traffic lights.
	WindowTitled WindowMode = iota
	// WindowFrameless hides all window chrome including traffic lights.
	WindowFrameless
	// WindowCustomTitlebar keeps traffic lights but extends content full
	// height behind a transparent titlebar; the strip of Config.TitlebarHeight
	// at the top acts as a drag region, and on macOS the traffic lights are
	// centered in it.
	WindowCustomTitlebar
	// WindowPanel is a transient popup beside a status item: no chrome, not
	// listed among the application's windows, above other windows, taking
	// keyboard focus without bringing the rest of the application forward. It
	// closes itself when it loses focus, and OnClosed reports that.
	WindowPanel
)

// DefaultTitlebarHeight is the custom titlebar strip, in logical pixels, of a
// configuration that does not set one: the height of the system's own titlebar,
// so the traffic lights stay where the system puts them.
const DefaultTitlebarHeight = 28

// TitlebarStrip is the height of the custom titlebar strip in logical pixels.
func (config Config) TitlebarStrip() float32 {
	if config.TitlebarHeight <= 0 {
		return DefaultTitlebarHeight
	}
	return config.TitlebarHeight
}

// CloseRequestHandler decides whether the window may close. Call allow to
// close immediately (now or later); never calling it keeps the window open.
type CloseRequestHandler func(allow func())

// TextInput describes one native text editor surface. The current M2 adapter
// deliberately supports one focused editor while the multi-view runtime API is
// still being designed.
type TextInput struct {
	// Target identifies the focused Canvas hit. Geometry can change while an
	// input method is composing; a different target must end that composition
	// even when both fields have identical text and presentation. Standalone
	// platform clients may leave it zero when they have only one editor.
	Target   ui.ID
	X        float32
	Y        float32
	Width    float32
	Height   float32
	FontSize float32
	// Layer keeps native editor visuals in the component's compositing layer.
	Layer     uint16
	Multiline bool
	// Secure makes the platform draw masked characters and keep the real
	// text out of any service that would read it back, as a password field.
	Secure bool
	// Terminal makes the editor a preedit surface for a character-cell
	// terminal: text in the fixed-pitch font, no caret and no selection, and
	// keys that are not text delivered as raw key events (see KeyEvent.Text).
	Terminal bool
	MonoFace ui.MonoFace
	// AltSendsEscape routes printable Alt/Option chords in a Terminal input to
	// raw key delivery, with the base character in KeyEvent.Text.
	AltSendsEscape bool
	// KeyReleases makes a Terminal input deliver the whole keyboard as raw
	// presses and releases: modifier keys, printable keys (with Text, unless an
	// input method consumed them), and a release for each press. Release
	// events carry the Code of their press.
	KeyReleases bool
	// NavigationKeys asks Up, Down, PageUp and PageDown to reach the
	// application as raw key events while the field is focused, while
	// cursor placement, selection, Home/End and IME remain native.
	NavigationKeys bool
	// Placed says the field lays its rows out itself (ui.TextPlacement): the
	// application places the caret from the pointer, so the platform editor must
	// not (it knows one line only), and the keys that move between rows (Up,
	// Down, Home, End, Page Up and Down, Command with Left and Right) reach the
	// application as key events unless an input method is composing.
	// CaretX..CaretHeight is where the field painted its caret, in logical window
	// pixels, for input-method windows; a zero height means the platform's own
	// reckoning.
	Placed                                  bool
	CaretX, CaretY, CaretWidth, CaretHeight float32
	Color                                   Color
	State                                   TextState
	OnEvent                                 TextHandler
}

// TextState is the complete platform-neutral state mirrored by NSTextInputClient.
type TextState struct {
	// SelectionReversed preserves the moving end of a backwards selection.
	SelectionReversed bool
	Value             string
	Selection         text.Range
	Composition       text.Range
	Composing         bool
	// Redraw asks for a frame after this event although the editor itself looks
	// the same: the application changed something else on screen in response, a
	// terminal drawing typed text ahead of the server's echo. It is not part of
	// the mirrored editor state and is cleared again before the next event.
	Redraw bool
}

// TextInputPresentationChanged reports whether applying a native text event
// requires the application and native editor to be redrawn. A terminal uses
// the native editor only for IME preedit, so ordinary committed text that goes
// straight to the terminal program changes no local presentation.
func TextInputPresentationChanged(terminal bool, previous, next TextState) bool {
	return !terminal || next.Redraw || previous.WithoutRedraw() != next.WithoutRedraw()
}

// WithoutRedraw is the state with its redraw request cleared, for comparing what
// the editor shows.
func (state TextState) WithoutRedraw() TextState {
	state.Redraw = false
	return state
}

// TextEventKind identifies a native text-system mutation.
type TextEventKind uint8

const (
	TextInsert TextEventKind = iota + 1
	TextSetMarked
	TextUnmark
	TextCommandEvent
	TextSetSelection
	// TextPaste is one pasteboard edit. An ordinary editor keeps it as a
	// separate undo unit; a terminal decides how to forward it (for example
	// inside bracketed-paste markers).
	TextPaste
	// TextPasteNoText reports a paste asked for while the pasteboard holds no text.
	// It edits nothing: an editor that takes text only ignores it. An application
	// whose field accepts more than text (a chat box that takes a screenshot) reads
	// the other formats itself, with ClipboardImagePNG. Terminals never receive it.
	TextPasteNoText
)

// TextCommand identifies keyboard editing commands emitted by AppKit.
type TextCommand uint8

const (
	TextDeleteBackward TextCommand = iota + 1
	TextDeleteForward
	TextMoveLeft
	TextMoveRight
	TextMoveLeftAndModifySelection
	TextMoveRightAndModifySelection
	TextInsertNewline
	TextInsertTab
	// TextInsertLineBreak is a line break the user asked for without ending the
	// entry: Shift+Enter (and Option or Control+Enter on macOS). Where Enter means
	// "submit" in a multi-line field, this is the key that adds a line. An editor
	// that does not tell them apart inserts a newline for both.
	TextInsertLineBreak
	TextUndo
	TextRedo
	TextSelectAll
	TextMoveLineStart
	TextMoveLineEnd
	TextMoveLineStartAndModifySelection
	TextMoveLineEndAndModifySelection
	TextMoveDocumentStart
	TextMoveDocumentEnd
	TextMoveDocumentStartAndModifySelection
	TextMoveDocumentEndAndModifySelection
	TextMoveWordLeft
	TextMoveWordRight
	TextMoveWordLeftAndModifySelection
	TextMoveWordRightAndModifySelection
	TextDeleteWordBackward
	TextDeleteWordForward
	// TextDeleteToLineStart and TextDeleteToLineEnd remove the text between the
	// caret and the start or end of its line (Command+Backspace and
	// Command+Delete on macOS).
	TextDeleteToLineStart
	TextDeleteToLineEnd
)

// TextEvent contains rune-indexed ranges. MarkedSelection is relative to Text.
type TextEvent struct {
	Kind            TextEventKind
	Text            string
	Selection       text.Range
	MarkedSelection text.Range
	Replacement     text.Range
	HasReplacement  bool
	Command         TextCommand
}

// TextHandler applies one native edit and returns the complete new state.
type TextHandler func(TextEvent) (TextState, error)

// Apply routes one platform text event into editor: the way a runtime connects
// a platform text service straight to a Go-owned editor. A replacement range, if
// the event names one, is selected first, so the event acts on it.
func (event TextEvent) Apply(editor *text.Editor) {
	if event.HasReplacement {
		editor.SetSelection(event.Replacement)
	}
	switch event.Kind {
	case TextInsert:
		editor.Insert(event.Text)
	case TextPaste:
		editor.Paste(event.Text)
	case TextSetMarked:
		editor.SetMarkedText(event.Text, event.MarkedSelection)
	case TextUnmark:
		editor.Unmark()
	case TextSetSelection:
		editor.SetSelection(event.Selection)
	case TextCommandEvent:
		switch event.Command {
		case TextDeleteBackward:
			editor.DeleteBackward()
		case TextDeleteForward:
			editor.DeleteForward()
		case TextMoveLeft:
			editor.MoveLeft(false)
		case TextMoveRight:
			editor.MoveRight(false)
		case TextMoveLeftAndModifySelection:
			editor.MoveLeft(true)
		case TextMoveRightAndModifySelection:
			editor.MoveRight(true)
		case TextInsertNewline, TextInsertLineBreak:
			editor.Insert("\n")
		case TextInsertTab:
			editor.Insert("\t")
		case TextUndo:
			editor.Undo()
		case TextRedo:
			editor.Redo()
		case TextSelectAll:
			editor.SetSelection(text.Range{End: editor.RuneCount()})
		case TextMoveLineStart:
			editor.MoveLineBoundary(false, false)
		case TextMoveLineEnd:
			editor.MoveLineBoundary(true, false)
		case TextMoveLineStartAndModifySelection:
			editor.MoveLineBoundary(false, true)
		case TextMoveLineEndAndModifySelection:
			editor.MoveLineBoundary(true, true)
		case TextMoveDocumentStart:
			editor.MoveTo(0, false)
		case TextMoveDocumentEnd:
			editor.MoveTo(editor.RuneCount(), false)
		case TextMoveDocumentStartAndModifySelection:
			editor.MoveTo(0, true)
		case TextMoveDocumentEndAndModifySelection:
			editor.MoveTo(editor.RuneCount(), true)
		case TextMoveWordLeft:
			editor.MoveWord(false, false)
		case TextMoveWordRight:
			editor.MoveWord(true, false)
		case TextMoveWordLeftAndModifySelection:
			editor.MoveWord(false, true)
		case TextMoveWordRightAndModifySelection:
			editor.MoveWord(true, true)
		case TextDeleteWordBackward:
			editor.DeleteWord(false)
		case TextDeleteWordForward:
			editor.DeleteWord(true)
		case TextDeleteToLineStart:
			editor.DeleteToLineBoundary(false)
		case TextDeleteToLineEnd:
			editor.DeleteToLineBoundary(true)
		}
	}
}

// TextHistoryCommand recognizes the conventional primary-modifier undo and
// redo chords. Applications see shortcuts first; an unhandled chord reaches a
// focused ordinary text field, while terminals keep their own Control+Z.
func TextHistoryCommand(event KeyEvent) (TextCommand, bool) {
	primary := event.Modifiers & (input.ModifierControl | input.ModifierMeta)
	if primary != input.ModifierControl && primary != input.ModifierMeta || event.Modifiers&input.ModifierAlt != 0 {
		return 0, false
	}
	text := event.Text
	if text == "Z" {
		text = "z"
	} else if text == "Y" {
		text = "y"
	}
	switch text {
	case "z":
		if event.Modifiers&input.ModifierShift != 0 {
			return TextRedo, true
		}
		return TextUndo, true
	case "y":
		if event.Modifiers&input.ModifierShift == 0 {
			return TextRedo, true
		}
	}
	return 0, false
}

// EditorState snapshots a text.Editor for transfer to the native input system.
func EditorState(editor *text.Editor) TextState {
	composition, composing := editor.Composition()
	return TextState{
		Value:             editor.String(),
		Selection:         editor.Selection(),
		SelectionReversed: editor.Selection().Start != editor.Selection().End && editor.Caret() == editor.Selection().Start,
		Composition:       composition,
		Composing:         composing,
	}
}

// appearanceObserver is who hears that the system's light or dark preference may
// have changed. The application layer sets it once someone watches.
var appearanceObserver atomic.Pointer[func()]

// SetAppearanceObserver sets what NotifyAppearanceChanged calls. Nil stops it.
func SetAppearanceObserver(observer func()) {
	if observer == nil {
		appearanceObserver.Store(nil)
	} else {
		appearanceObserver.Store(&observer)
	}
	watchChanged()
}

// NotifyAppearanceChanged is for adapters: they call it on the event-loop
// thread when the system's appearance setting may have changed, as the system
// says so (a notification, a settings message) or, where it does not, when the
// window gains focus. The observer reads the setting itself and ignores a call
// that changed nothing, so an adapter may call it for more than it is sure of.
func NotifyAppearanceChanged() {
	if observer := appearanceObserver.Load(); observer != nil {
		(*observer)()
	}
}
