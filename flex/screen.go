package flex

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// Screen declares an application's page without naming its controls, and keeps
// what they change in the application's own values:
//
//	type model struct {
//		name  flex.Field
//		sync  bool
//		theme int
//	}
//
//	func (m *model) page(s flex.Screen) {
//		s.Column().Fill().Pad(24).Gap(12).Children(func() {
//			s.Field(&m.name, "Name", "Your name")
//			s.Switch(&m.sync, "Sync")
//			s.Choice(&m.theme, themes)
//			if s.Button("Save").Clicked() {
//				m.save()
//			}
//		})
//	}
//
//	flex.Run(app.Config{Title: "Settings", Width: 480, Height: 320}, m.page)
//
// A Screen is a Frame underneath: the layout calls, sizes and looks are the Frame's
// and Box's, and Raw gives the Frame for anything the Screen does not forward. What
// it adds is identity (the controls are named for you, see screen_identity.go),
// binding (a control writes into the value it is given, in the paint that delivers
// the click), the text input of its Fields, and Popover and Modal, which open and
// close a *bool.
//
// The Screen handed to the body is a three-word value: pass it to the functions that
// declare parts of the page. NewScreen makes the one a window keeps, which paints
// the body (Paint, Draw) and answers the window (Config, TextInput, Wheel).
//
// A Screen's identities are names in a ui.Tree, under the Screen's Root: the
// headless driver and the logs show them as paths ("screen/~button/#0"), and Raw
// with ID gives a control a name of the application's for a test to click.
type Screen struct {
	f     *Frame
	sh    *screenShared
	scope ui.Scope
	rec   int32
}

// ScreenConfig says how a Screen paints.
type ScreenConfig struct {
	// Body declares the page. It is what Paint draws.
	Body func(Screen)
	// Tree is the identity tree the Screen names its controls in. Nil takes the one
	// of the app.Config given to Config, or else one of the Screen's own. An
	// application that paints more than one Screen in a window gives each the
	// window's Tree and a Root of its own.
	Tree *ui.Tree
	// Root is the name of the Screen's top scope in Tree, "screen" when empty.
	Root string
	// Theme is what the page is painted with. Nil takes the light or dark theme
	// following the system's appearance when the Screen first paints a window
	// (Paint); a Screen that only Draws into a page the application paints takes
	// the light theme, since that page has a theme of its own to give it.
	Theme *ui.Theme
	// FollowSystemTheme, with a nil Theme, watches the system's appearance and
	// repaints in the other theme when it switches (app.WatchSystemAppearance). It
	// is off by default: watching costs a listener on some desktops.
	FollowSystemTheme bool
	// Background is the color Paint fills the window with, the theme's Background
	// when nil. Draw, which paints into a region of a page, fills nothing.
	Background *ui.Color
}

// screenShared is what all the Screen values of one page share.
type screenShared struct {
	config ScreenConfig
	tree   *ui.Tree
	root   ui.Scope
	view   View
	theme  ui.Theme
	themed bool
	window bool        // the Screen paints a window of its own (Paint), and so asks the system for its look
	stale  atomic.Bool // the system appearance changed since the theme was chosen

	epoch      uint32
	records    map[ui.Scope]int32
	list       []scopeRecord
	rootRec    int32
	kindNames  [screenKinds]ui.Name
	keyName    ui.Name
	keys       map[keyKey]keyEntry
	bound      map[boundKey]boundEntry
	boundSlots [screenKinds]slotPool
	held       [2]armedButton
	lostWarned bool

	// fields are the Fields the last paint declared with their identities, which
	// TextInput answers from; selects are its Selects, which Wheel scrolls.
	fields  []screenField
	selects []*SelectState

	baseText   app.TextInputProvider
	baseScroll func(ui.ScrollEvent) ui.EventResult
	painting   bool
}

type screenField struct {
	id    ui.ID
	field *Field
}

// NewScreen makes the Screen a window keeps. Give its Config to the window, or call
// Draw from a Paint of your own to put the page in a region of it.
func NewScreen(config ScreenConfig) *Screen {
	if config.Root != "" {
		checkName("Root", config.Root)
	}
	return &Screen{sh: &screenShared{config: config}}
}

// Run opens a window that paints body, and blocks until it closes: a Screen's whole
// application in one call. The config's Tree, Paint, TextInput and OnScroll are the
// Screen's (see Screen.Config).
func Run(config app.Config, body func(Screen)) error {
	screen := NewScreen(ScreenConfig{Body: body})
	var canvas ui.Canvas
	return app.RunCanvas(screen.Config(config), &canvas)
}

// init settles the tree and the names, once: the Screen's own tree unless one was
// given or adopted.
func (sh *screenShared) init(tree *ui.Tree) {
	if sh.tree != nil {
		return
	}
	switch {
	case sh.config.Tree != nil:
		sh.tree = sh.config.Tree
	case tree != nil:
		sh.tree = tree
	default:
		sh.tree = new(ui.Tree)
	}
	root := sh.config.Root
	if root == "" {
		root = "screen"
	}
	sh.root = sh.tree.Feature(root)
	for kind := range screenKinds {
		sh.kindNames[kind] = sh.tree.Name(screenKindNames[kind])
	}
	sh.keyName = sh.tree.Name("~key")
	sh.epoch = 1
	sh.rootRec = sh.recordOf(sh.root, false)
}

// Config returns base with the Screen's Tree, Paint, TextInput and OnScroll:
//
//   - Tree: base's is adopted when the Screen was given none; a different one panics.
//   - Paint paints Body over the whole window.
//   - TextInput answers for the Fields of the page and asks base's for any other
//     focused field (a terminal of the application's, say).
//   - OnScroll scrolls an open Select, then the page's ScrollAreas, and gives what
//     they leave to base's.
//
// FollowSystemTheme also sets an Invalidator when base has none, and stops watching
// the appearance in OnClosed. Nothing else is touched.
func (s *Screen) Config(base app.Config) app.Config {
	sh := s.sh
	if sh.config.Body == nil {
		panic("flex: Screen.Config needs ScreenConfig.Body, the function that declares the page")
	}
	sh.init(base.Tree)
	if base.Tree != nil && base.Tree != sh.tree {
		panic("flex: Screen.Config: the app.Config has another Tree than the Screen's; give NewScreen the window's Tree")
	}
	config := base
	config.Tree = sh.tree
	config.Paint = s.Paint
	sh.baseText, config.TextInput = base.TextInput, s.TextInput
	sh.baseScroll, config.OnScroll = base.OnScroll, s.Wheel
	if sh.config.FollowSystemTheme && sh.config.Theme == nil {
		invalidator := config.Invalidator
		if invalidator == nil {
			invalidator = new(app.Invalidator)
			config.Invalidator = invalidator
		}
		stop := app.WatchSystemAppearance(func() {
			sh.stale.Store(true)
			invalidator.Invalidate()
		})
		closed := base.OnClosed
		config.OnClosed = func() {
			stop()
			if closed != nil {
				closed()
			}
		}
	}
	return config
}

// Paint fills the window with the background and draws Body over it. It is the
// Config's Paint.
func (s *Screen) Paint(canvas *ui.Canvas, size ui.Size) {
	sh := s.sh
	if sh.config.Body == nil {
		panic("flex: Screen.Paint needs ScreenConfig.Body, the function that declares the page")
	}
	sh.init(nil)
	sh.window = true
	bounds := ui.Rect{Width: size.Width, Height: size.Height}
	theme := sh.currentTheme()
	background := theme.Background
	if sh.config.Background != nil {
		background = *sh.config.Background
	}
	canvas.Fill(bounds, background)
	s.Draw(canvas, bounds, sh.config.Body)
}

// Draw declares body inside bounds, lays it out and paints it: the way to put a
// Screen in one region of a page another paint draws (a pane, a panel). It fills
// no background. The canvas must name its identities in the Screen's Tree.
func (s *Screen) Draw(canvas *ui.Canvas, bounds ui.Rect, body func(Screen)) {
	sh := s.sh
	if sh.painting {
		panic("flex: Screen.Draw was called inside the Screen's own body")
	}
	sh.init(nil)
	if canvas != nil {
		if tree := canvas.Tree(); tree != sh.tree {
			if tree.Len() != 0 {
				panic("flex: Screen.Draw: the canvas names its identities in another Tree than the Screen's; give NewScreen the canvas's Tree")
			}
			canvas.SetTree(sh.tree)
		}
	}
	sh.beginIdentity(canvas)
	clear(sh.fields)
	sh.fields = sh.fields[:0]
	clear(sh.selects)
	sh.selects = sh.selects[:0]
	theme := sh.currentTheme()
	f := sh.view.Begin(canvas, *theme, bounds)
	sh.painting = true
	defer func() { sh.painting = false }()
	body(Screen{f: f, sh: sh, scope: sh.root, rec: sh.rootRec})
	f.End()
	sh.endIdentity(canvas)
}

// currentTheme is the theme the page paints with: the one given, or the system's.
func (sh *screenShared) currentTheme() *ui.Theme {
	if sh.config.Theme != nil {
		return sh.config.Theme
	}
	if !sh.themed || sh.stale.Swap(false) {
		sh.themed = true
		if sh.window && app.SystemAppearance() == app.AppearanceDark {
			sh.theme = ui.DarkTheme()
		} else {
			sh.theme = ui.LightTheme()
		}
	}
	return &sh.theme
}

// TextInput gives the platform the editor of the focused Field of the last paint,
// and asks the provider Config was given for any other field. It is the Config's
// TextInput.
func (s *Screen) TextInput(context app.TextInputContext) *app.TextInput {
	sh := s.sh
	if !context.ID.IsZero() {
		for _, entry := range sh.fields {
			if entry.id == context.ID {
				entry.field.input.Editor = entry.field.ensure()
				return &entry.field.input
			}
		}
	}
	if sh.baseText != nil {
		return sh.baseText(context)
	}
	return nil
}

// Wheel scrolls an open Select, else the ScrollArea under the pointer, and gives an
// event neither takes to the OnScroll Config was given. It is the Config's OnScroll.
func (s *Screen) Wheel(event ui.ScrollEvent) ui.EventResult {
	sh := s.sh
	for _, state := range sh.selects {
		if state.Open {
			return state.Wheel(event)
		}
	}
	if result := sh.view.Wheel(event); result.Handled || result.Repaint {
		return result
	}
	if sh.baseScroll != nil {
		return sh.baseScroll(event)
	}
	return ui.EventResult{}
}

// frame returns the Frame being declared, and stops a Screen used outside its body.
func (s Screen) frame() *Frame {
	if s.f == nil {
		panic("flex: declare a Screen's controls inside its body, with the Screen the body is given")
	}
	return s.f
}

// Raw returns the Frame the Screen declares into, for what the Screen does not
// forward: a control with an identity of your own (ID), an InputGroup, a Textarea,
// a Base control. Its elements sit among the Screen's.
func (s Screen) Raw() *Frame { return s.frame() }

// Theme is the theme the page is painted with. It is the Screen's own: read it, and
// do not keep it past the body.
func (s Screen) Theme() *ui.Theme { return &s.frame().theme }

// Canvas is the canvas the page is painted on.
func (s Screen) Canvas() *ui.Canvas { return s.frame().canvas }

// AfterPaint runs fn after the paint that delivers a click, outside any paint, and
// repaints: the place for a handler that changes the list being declared (deleting
// its row) or opens a native dialog. In any other paint it does nothing, and the
// function literal given to it costs nothing until a click asks for it.
//
//	if r.Button("Delete").Clicked() {
//		r.AfterPaint(func() { m.remove(h.ID) })
//	}
func (s Screen) AfterPaint(fn func()) {
	if canvas := s.frame().canvas; canvas != nil {
		canvas.AfterPaint(fn)
	}
}

// Key returns the Screen of a row named by key: the controls declared with it are
// numbered within the row, so their focus, press and scrolling follow the row when
// the rows around it come, go or move. Use it for each row of a list whose rows have
// a control bound to nothing (a Button), or a value that is a copy (see Checkbox).
// Keys are unique among the rows of one scope in a paint; a duplicate panics. A key
// is kept while its row is declared and its number is given to another key two
// paints after, so a list costs what its rows on screen at once cost.
//
//	for i := range m.hosts {
//		h := &m.hosts[i]
//		r := s.Key(h.Name)
//		r.Row().Children(func() { ... r.Button("Delete") ... })
//	}
func (s Screen) Key(key string) Screen { return s.keyed(keyKey{text: key, isText: true}) }

// KeyIndex is Key for a number: a record's ID. It allocates nothing.
func (s Screen) KeyIndex(key int) Screen { return s.keyed(keyKey{number: key}) }

// Scoped declares fn in a scope named name under the Screen's: the same part of a
// page declared twice (two panels from one function) keeps its buttons apart, and a
// window-level handler can tell its identities by Tree.Within. name is a word of the
// program, not data: it is kept in the Tree for good.
func (s Screen) Scoped(name string, fn func(Screen)) {
	checkName("Scoped", name)
	sh := s.sh
	fn(s.child(sh.tree.Sub(s.scope, sh.tree.Name(name)), false))
}

// ID returns the identity name gives in the Screen's scope: a name of the program's
// for a control declared through Raw, which a test or the headless driver can click
// by its path. name is kept in the Tree for good.
func (s Screen) ID(name string) ui.ID {
	checkName("ID", name)
	return s.sh.tree.ID(s.scope, name)
}

// Layout: the Frame's, in the Screen's frame.

func (s Screen) Row() Box                 { return s.frame().Row() }
func (s Screen) Column() Box              { return s.frame().Column() }
func (s Screen) Stack() Box               { return s.frame().Stack() }
func (s Screen) Grid(minCell float32) Box { return s.frame().Grid(minCell) }
func (s Screen) Text(value string) Box    { return s.frame().Text(value) }
func (s Screen) Paragraph(value string, maxLines int) Box {
	return s.frame().Paragraph(value, maxLines)
}
func (s Screen) Spacer() Box                   { return s.frame().Spacer() }
func (s Screen) Divider() Box                  { return s.frame().Divider() }
func (s Screen) Icon(icon ui.Icon) Box         { return s.frame().Icon(icon) }
func (s Screen) Image(bitmap ui.Bitmap) Box    { return s.frame().Image(bitmap) }
func (s Screen) Slot() Box                     { return s.frame().Slot() }
func (s Screen) Progress(fraction float32) Box { return s.frame().Progress(fraction) }
func (s Screen) Spinner() Box                  { return s.frame().Spinner() }
func (s Screen) Popup() Box                    { return s.frame().Popup() }

// Textf is Text of fmt.Sprintf(format, args...). It allocates the string, and the
// arguments that are not pointers, in every paint; a page that must paint without
// allocating keeps the text in its model and uses Text.
func (s Screen) Textf(format string, args ...any) Box {
	return s.frame().Text(fmt.Sprintf(format, args...))
}

// ScrollArea is Frame.ScrollArea named name in the Screen's scope: the offset is
// kept by name, so give each scrolling region a name of its own.
func (s Screen) ScrollArea(name string) Box {
	return s.frame().ScrollArea(s.ID(name))
}

// Controls.

// Button adds a button; Clicked reports a click on it. It is named by its place
// among the Screen's buttons (rule 3 of screen_identity.go): in a list, declare it in
// a Key row.
func (s Screen) Button(label string) Box {
	id := s.anonID(skButton)
	b := s.frame().Button(id, label)
	s.guard(b, id, label)
	return b
}

// Checkbox adds a checkbox showing *value, with label beside it (none when empty;
// Label names it for accessibility then). A click flips *value. Changed reports the
// paint that did. The control is named by value's address: bind the model's field.
// A copy (v := m.done[id]; s.Checkbox(&v, ...)) has a new address in every paint, so
// declare it in a Key row, where controls are numbered, and write it back on Changed.
func (s Screen) Checkbox(value *bool, label string) Box {
	b := s.frame().Checkbox(s.boundID(skCheckbox, unsafe.Pointer(value)), *value)
	if label != "" {
		b.Caption(label)
	}
	s.f.bindBool(b, value)
	return b
}

// Switch is Checkbox's two-state switch.
func (s Screen) Switch(value *bool, label string) Box {
	b := s.frame().Switch(s.boundID(skSwitch, unsafe.Pointer(value)), *value)
	if label != "" {
		b.Caption(label)
	}
	s.f.bindBool(b, value)
	return b
}

// Choice adds a segmented control choosing among labels, *selected the index of the
// chosen one (negative for none); a click sets it.
func (s Screen) Choice(selected *int, labels []string) Box {
	f := s.frame()
	b := f.Choice(s.scopeOf(skChoice, unsafe.Pointer(selected)), labels, *selected)
	f.bindIndex(b, selected)
	return b
}

// Tabs adds a tab strip, *selected the index of the active tab; a click sets it.
// Declare the active tab's content after it.
func (s Screen) Tabs(selected *int, labels []string) Box {
	f := s.frame()
	b := f.Tabs(s.scopeOf(skTabs, unsafe.Pointer(selected)), labels, *selected)
	f.bindIndex(b, selected)
	return b
}

// Select adds a drop-down over labels, *selected the index of the chosen one
// (negative for none, and Placeholder shows); picking an option sets it. state keeps
// whether the list is open, and names the control: give each select its own.
func (s Screen) Select(state *SelectState, selected *int, labels []string) Box {
	f := s.frame()
	b := f.Select(s.scopeOf(skSelect, unsafe.Pointer(state)), state, labels, *selected)
	f.bindIndex(b, selected)
	s.sh.selects = append(s.sh.selects, state)
	return b
}

// Field adds a text field editing field, with an accessible label and a placeholder.
// Typing, the caret, selection, clipboard and input methods need nothing more: the
// Screen's Config gives the platform the field's editor. Changed reports the first
// paint after an edit; Submitted reports Enter.
func (s Screen) Field(field *Field, label, placeholder string) Box {
	f := s.frame()
	id := s.boundID(skField, unsafe.Pointer(field))
	b := f.Field(field, id, label, placeholder)
	f.bindField(b, field)
	s.sh.fields = append(s.sh.fields, screenField{id: id, field: field})
	return b
}

// Slider adds a slider over state.Value; Range, Step and Changed are the Frame's.
func (s Screen) Slider(state *SliderState) Box {
	return s.frame().Slider(s.boundID(skSlider, unsafe.Pointer(state)), state)
}

// Splitter adds a draggable divider between the panes around it, at state.Pos.
func (s Screen) Splitter(state *SplitterState) Box {
	return s.frame().Splitter(s.boundID(skSplitter, unsafe.Pointer(state)), state)
}

// Table adds a Table; its Rows and Header scopes are made from state when spec
// leaves them zero.
func (s Screen) Table(state *TableState, spec TableSpec) Box {
	if spec.Rows == 0 || spec.Header == 0 {
		scope := s.scopeOf(skTable, unsafe.Pointer(state))
		tree := s.sh.tree
		if spec.Rows == 0 {
			spec.Rows = tree.Sub(scope, ui.Index(0))
		}
		if spec.Header == 0 {
			spec.Header = tree.Sub(scope, ui.Index(1))
		}
	}
	return s.frame().Table(state, spec)
}

// scopeOf is the scope of a control that numbers its own parts (the options of a
// Choice, the rows of a Table): the scope of its identity.
func (s Screen) scopeOf(kind screenKind, value unsafe.Pointer) ui.Scope {
	id := s.boundID(kind, value)
	return s.sh.tree.Sub(id.Scope(), id.Name())
}
