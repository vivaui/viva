package ui

// One way to declare a window: an application's page and a component are both
// functions of a C, and every element is a function that takes it first, so a
// component of an application's own reads like the ones Viva ships.
//
//	func (m *model) page(c ui.C) {
//		ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
//			ui.TextField(c, &m.name, "Name", "")
//			ui.Switch(c, &m.sync, "Sync")
//			for i := range m.hosts {
//				h := &m.hosts[i]
//				ui.Row(c.KeyInt(h.ID)).Children(func() {
//					ui.Checkbox(c, &h.Done, h.Name)
//					if ui.Button(c, "Delete").Clicked() { c.After(func() { m.remove(h.ID) }) }
//				})
//			}
//		})
//	}
//
//	ui.Run(app.Config{Title: "Hosts", Width: 480, Height: 360}, m.page)
//
// Identity has one rule a user needs: an element is named by its place among the
// elements of its kind in its parent, and a Key names a row, so that the row and
// what is in it keep their focus and state as rows come, go and move. A control
// bound to a value keeps its name while the value moves (a field appearing above
// a password field does not take its typing), and a copy bound in every paint
// works as its place does. Names are exact paths in a gfx.Tree, such as
// "page/~k0/#0", which the headless driver and the logs print.
//
// The Frame underneath (C.Frame) is the layer with identities of the
// application's own, for what a central click dispatcher or a custom painter needs.

import (
	"context"
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// C is what a view declares with: the page being declared and where in it. Every
// element function takes it first, so a component of an application's own is a
// function of the same shape as the ones Viva ships.
type C = *Ctx

// Ctx is the declaration context of a Page. Use it through C. It is valid only
// while the page declares; a *Page lives across paints, a C and the Boxes of one
// paint do not.
type Ctx struct {
	page *Page
	f    *Frame
	// next names the next element instead of its place: a Key, an index or a part.
	next     protoName
	nextSet  bool
	stateSeq map[stateName]uint32 // reset each paint: the States of each name asked for in each scope
}

type protoName struct {
	key    keyKey
	part   string
	index  int
	kindOf uint8 // 1 key, 2 part, 3 index
}

// Key names the next element by key: a row of a list, so it and what is in it keep
// their names, focus and state as rows come, go and move. Keys are unique among
// their siblings in a paint.
func (c *Ctx) Key(key string) C {
	c.next, c.nextSet = protoName{key: keyKey{text: key, isText: true}, kindOf: 1}, true
	return c
}

// KeyInt is Key for a number, a record's ID.
func (c *Ctx) KeyInt(key int) C {
	c.next, c.nextSet = protoName{key: keyKey{number: key}, kindOf: 1}, true
	return c
}

// Part names the next element by a word of the program: the parts of a component
// (its trigger, its list), which a test can address by that word.
func (c *Ctx) Part(name string) C {
	checkName("Part", name)
	c.next, c.nextSet = protoName{part: name, kindOf: 2}, true
	return c
}

// After runs fn after the paint that delivers a click or a pointer event (a drop),
// outside any paint, then repaints: for a handler that changes the list being
// declared.
func (c *Ctx) After(fn func()) {
	if c.f.canvas != nil {
		c.f.canvas.AfterPaint(fn)
	}
}

// ID is the identity of a control of the application's own declared next (one
// made with the Frame, or a recipe taking an ID): named by its number among such
// controls in the container being filled, or by the Key or Part set before it.
func ID(c C) gfx.ID { return c.page.controlID(pkCustom) }

// Theme is the page's theme. Read it; do not keep it.
func (c *Ctx) Theme() *Theme { return &c.f.theme }

// Frame is the Frame underneath, for what the prototype does not wrap.
func (c *Ctx) Frame() *Frame { return c.f }

// element kinds, numbered apart in each parent.
const (
	pkContainer uint8 = iota
	pkButton
	pkCheckbox
	pkSwitch
	pkChoice
	pkField
	pkPopover
	pkModal
	pkMemo
	pkTabs
	pkSelect
	pkSlider
	pkSplitter
	pkTable
	pkCustom
	pkControl
	pkArea
	pkShortcut
	pkMenu
	pkCombobox
	pkToaster
	pkDrawer
	pkTree
	pkToggle
	pkRadio
	pkTab
	pkList
	pkKinds
)

// protoScope is what a page keeps for one scope between paints: its rows' keys and
// its buttons' labels for the guard.
type protoScope struct {
	epoch     uint32
	keySub    gfx.Scope
	keys      slotPool
	labels    []string
	prev      []string
	prevEpoch uint32
}

// nameUse is what a page knows of a name it gave a bound value: the paint it was
// last handed out in, and, for a place's name, when the value holding it was born.
type nameUse struct{ seen, born uint32 }

type protoBound struct {
	id          gfx.ID
	born, epoch uint32
	positional  bool
}

// PageConfig says how a Page paints.
type PageConfig struct {
	// View declares the page. It is what Paint draws.
	View func(C)
	// Tree is the identity tree the page names its elements in. Nil takes the one
	// of the app.Config given to Config, or else one of the page's own. A window
	// that paints more than one Page gives each its Tree and a Root of its own.
	Tree *gfx.Tree
	// Root is the name of the page's top scope in Tree, "page" when empty.
	Root string
	// Theme is what the page is painted with. Nil takes the light or dark theme
	// following the system's appearance when the page first paints a window (Paint);
	// a page that only Draws into a region another paint owns takes the light one.
	Theme *Theme
	// FollowSystemTheme, with a nil Theme, watches the system's appearance and
	// repaints in the other theme when it switches (app.WatchSystemAppearance). It
	// is off by default: watching costs a listener on some desktops.
	FollowSystemTheme bool
	// Background is the color Paint fills the window with, the theme's Background
	// when nil. Draw fills nothing.
	Background *Color
	// Invalidator wakes the window the page is drawn in when an Effect of the page
	// finishes. Config uses it instead of the app.Config's; a page the application
	// draws into a window of its own (Draw, not Paint) needs it, since only Config
	// would otherwise supply one.
	Invalidator *app.Invalidator
	// MemoCheck makes every Memo of this page declare its part and panic where a
	// copy of the last paint would have differed, as MemoCheck(true) does for every
	// page. It is for tests.
	MemoCheck bool
}

// Page is a window's declaration: one tree for the whole window. A Page lives
// across paints, as its C, Box and Frame values do not.
type Page struct {
	config     PageConfig
	checkMemos bool
	tree       *gfx.Tree
	root       gfx.Scope
	view       View
	ctx        Ctx
	theme      Theme
	themeSet   bool
	themeEpoch uint32
	themed     bool
	window     bool        // the page paints a window of its own, and so asks the system for its look
	stale      atomic.Bool // the system appearance changed since the theme was chosen
	epoch      uint32

	effects       []effectState      // the Effects of the last paint, which their work belongs to
	effectContext context.Context    // what their runs derive from; cancelled when the page closes
	effectCancel  context.CancelFunc // cancels effectContext
	invalidator   *app.Invalidator   // wakes the window when work finishes, when the page has one

	scopes      []*protoScope // by scope: Scopes are dense indexes into the Tree
	nodes       []protoNode   // each container of the frame, by node index
	keys        map[keyKey]keyEntry
	intKeys     map[intKey]keyEntry
	bound       map[boundKey]protoBound
	names       map[gfx.ID]nameUse // the names bound values were given
	fresh       slotPool
	freshAt     gfx.Scope
	kindName    [pkKinds]gfx.Name
	keyName     gfx.Name
	held        [2]armedButton
	states      map[protoStateKey]protoState
	fields      []fieldEntry
	focusFields map[any]any
	focusPaint  []focusState
	texts       map[gfx.ID]*boundText // the Fields of the TextFields bound to strings
	controls    []controlEntry        // the Controls of the last paint, which take their input
	shortcuts   []shortcutEntry       // the Shortcuts of the last paint
	menus       []*MenuItems          // the open Menus of the last paint
	rovers      []rover               // the Choices and Tabs of the last paint
	sliders     []*SliderState        // the Sliders of the last paint
	groups      []focusGroup          // the FocusGroups of the last paint
	groupIDs    []gfx.ID              // their members, in order
	groupStops  map[gfx.ID]groupStop  // the member each group has the Tab stop on
	contexts    []contextTarget       // the ContextMenus of the last paint, which Shift+F10 opens
	combos      []*comboState         // the Comboboxes of the last paint
	trees       []*TreeState          // the Trees of the last paint
	listStates  []*ListState          // the Lists of the last paint
	selects     []*SelectState        // the Selects of the last paint, which Wheel scrolls
	labels      []string              // scratch for Choice's labels
	memos       map[gfx.ID]*memoEntry
	memoBoxes   []memoBox
	memoChecks  []memoCheckOf
	recording   []*memoEntry // the Memos being declared, outermost first
	retry       bool         // a Memo's copy failed: the paint is done again
	overlay     bool         // the last paint declared an open Popover, Modal, Menu or Select
	lists       []listDecl
	listRows    []int32 // the row containers of this paint's Lists

	baseText     app.TextInputProvider
	baseScroll   func(gfx.ScrollEvent) gfx.EventResult
	baseKey      func(gfx.KeyEvent) gfx.EventResult
	basePointer  func(gfx.PointerEvent) gfx.EventResult
	baseShortcut func(gfx.KeyEvent) gfx.EventResult
	// keyRoute, pointerRoute, areaKey and shortcutRoute are set by the first
	// Control, TextArea or Shortcut a page declares, so a page without them does
	// not link what routes their input.
	keyRoute      func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	pointerRoute  func(*Page, gfx.PointerEvent) (gfx.EventResult, bool)
	areaKey       func(*Field, gfx.KeyEvent) gfx.EventResult
	shortcutRoute func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	menuRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	roveRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // arrow keys in a Choice or Tabs
	groupRoute    func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // arrow keys in a FocusGroup
	sliderRoute   func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // the keys of a Slider
	comboRoute    func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	treeRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	listRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // moves and chooses rows of a List
	wheelXRoute   func(*Page, gfx.ScrollEvent) (moved, only bool)   // the sideways wheel of a List with RowWidth
	typeRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // a Tree's typeahead, from OnShortcut
	wheelRoute    func(*Page, gfx.ScrollEvent) (gfx.EventResult, bool)
	textRoute     func(*Page, gfx.ID) *app.TextInput
}

// protoNode is a container of the frame: where it is named (in the container at
// parentNode, or in parentScope for a keyed row), how many elements of each kind it
// has, and the scope its name makes, made only once something needs it.
type protoNode struct {
	parentNode  int32
	parentScope gfx.Scope
	name        gfx.Name
	scope       gfx.Scope
	counts      [pkKinds]uint32
	named       bool
}

// place is where an element is named: in a container, by a name.
type place struct {
	node  int32
	scope gfx.Scope // set when known: a keyed or named element's
	name  gfx.Name
}

// intKey is a KeyInt row: compared without a string.
type intKey struct {
	parent gfx.Scope
	number int
}

// stateName is a State's name in a scope; protoStateKey adds which of that name it
// is, for a component declared twice in one container.
type stateName struct {
	scope gfx.Scope
	name  string
}

type protoStateKey struct {
	stateName
	n uint32
}

type protoState struct {
	value any
	epoch uint32
}

// NewPage makes the Page a window keeps. Give its Config to the window, or call
// Draw from a paint of your own to put the page in a region of it.
func NewPage(config PageConfig) *Page {
	if config.Root != "" {
		checkName("PageConfig.Root", config.Root)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Page{config: config, checkMemos: config.MemoCheck, effectContext: ctx, effectCancel: cancel, invalidator: config.Invalidator}
}

// Run opens a window that declares view, and blocks until it closes: an
// application in one call. The config's Tree, Paint, TextInput and OnScroll are
// the page's (see Page.Config).
func Run(config app.Config, view func(C)) error {
	page := NewPage(PageConfig{View: view})
	var canvas gfx.Canvas
	return app.RunCanvas(page.Config(config), &canvas)
}

func (p *Page) init(tree *gfx.Tree) {
	if p.tree != nil {
		return
	}
	switch {
	case p.config.Tree != nil:
		p.tree = p.config.Tree
	case tree != nil:
		p.tree = tree
	default:
		p.tree = new(gfx.Tree)
	}
	root := p.config.Root
	if root == "" {
		root = "page"
	}
	p.root = p.tree.Feature(root)
	for kind := range pkKinds {
		p.kindName[kind] = p.tree.Name(fmt.Sprintf("~k%d", kind))
	}
	p.keyName = p.tree.Name("~key")
	p.freshAt = p.tree.Sub(p.root, p.tree.Name("~new"))
	p.keys = map[keyKey]keyEntry{}
	p.intKeys = map[intKey]keyEntry{}
	p.bound = map[boundKey]protoBound{}
	p.names = map[gfx.ID]nameUse{}
	p.memos = map[gfx.ID]*memoEntry{}
	p.states = map[protoStateKey]protoState{}
	p.texts = map[gfx.ID]*boundText{}
	p.ctx.stateSeq = map[stateName]uint32{}
	p.epoch = 1
}

// Owns reports whether id is one the page named: one of its controls, or what
// its Popovers and Modals report a dismissal with.
func (p *Page) Owns(id gfx.ID) bool {
	return p.tree != nil && !id.IsZero() && p.tree.Within(id.Scope(), p.root)
}

// Overlay reports whether the last paint declared an open Popover, Modal, Menu,
// Select or Combobox list: while it does, the overlay has the pointer and the
// keyboard (see Pointer and Key).
func (p *Page) Overlay() bool { return p.overlay }

// Pointer is for a host that routes the window's pointer itself, a page being a
// pane or an overlay of a window it does not own. It reports whether the event
// is the page's business, and what the host returns for it then: a Control's go
// to its Handlers, the page's other targets are left unhandled, so the press
// completes its click in the page, and while an Overlay is open every other press
// is taken, so nothing behind it acts.
//
//	if result, ok := page.Pointer(event); ok {
//		return result
//	}
func (p *Page) Pointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if result, ok := p.routePointer(event); ok {
		return result, true
	}
	switch {
	case p.Owns(event.Target):
		return gfx.EventResult{}, true
	case p.overlay:
		return gfx.EventResult{Handled: true}, true
	}
	return gfx.EventResult{}, false
}

// Key is Pointer for the keyboard. A Shortcut the key presses acts first, then a
// Control with the focus takes its keys. Give it the window's OnShortcut too: the
// chords of printable keys (Primary+S) arrive there.
// While an Overlay is open, a key whose target is the page's (a field in it) is left
// to the page; of the others, Escape, Enter, Space and Tab are left to it (to close
// it, press its buttons and move between them) and the rest are taken, so the host's
// shortcuts do not act on what is behind it. With no Overlay it leaves the key to
// the host.
func (p *Page) Key(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if result, ok := p.routeKey(event); ok {
		return result, true
	}
	if !p.overlay {
		return gfx.EventResult{}, false
	}
	if p.Owns(event.Target) {
		return gfx.EventResult{}, true
	}
	switch event.Key {
	case gfx.KeyEscape, gfx.KeyEnter, gfx.KeySpace, gfx.KeyTab:
		return gfx.EventResult{}, true
	}
	return gfx.EventResult{Handled: true}, true
}

// routeKey gives a key to the Combobox it is typed in or the open Menu it moves in,
// else the Shortcut it presses, else the Control or TextArea it is for.
func (p *Page) routeKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if p.comboRoute != nil {
		if result, ok := p.comboRoute(p, event); ok {
			return result, true
		}
	}
	if p.treeRoute != nil {
		if result, ok := p.treeRoute(p, event); ok {
			return result, true
		}
	}
	if p.listRoute != nil {
		if result, ok := p.listRoute(p, event); ok {
			return result, true
		}
	}
	if p.menuRoute != nil {
		if result, ok := p.menuRoute(p, event); ok {
			return result, true
		}
	}
	if p.roveRoute != nil {
		if result, ok := p.roveRoute(p, event); ok {
			return result, true
		}
	}
	if p.sliderRoute != nil {
		if result, ok := p.sliderRoute(p, event); ok {
			return result, true
		}
	}
	if p.shortcutRoute != nil {
		if result, ok := p.shortcutRoute(p, event); ok {
			return result, true
		}
	}
	if h := p.controlOf(event.Target); h != nil && h.Key != nil {
		return h.Key(event), true
	}
	if p.areaKey != nil {
		for _, entry := range p.fields {
			if entry.id == event.Target {
				if result := p.areaKey(entry.field, event); result.Handled {
					return result, true
				}
				break
			}
		}
	}
	if p.groupRoute != nil {
		if result, ok := p.groupRoute(p, event); ok {
			return result, true
		}
	}
	return gfx.EventResult{}, false
}

// routePointer gives a pointer event to the Control it is for.
func (p *Page) routePointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if h := p.controlOf(event.Target); h != nil && h.Pointer != nil {
		return h.Pointer(event), true
	}
	return gfx.EventResult{}, false
}

// onKey and onPointer are the Config's: a Control's or TextArea's, then base's. A
// full-window page has nothing behind its overlays, so they do not take keys here.
func (p *Page) onKey(event gfx.KeyEvent) gfx.EventResult {
	if p.keyRoute != nil {
		if result, ok := p.keyRoute(p, event); ok {
			return result
		}
	}
	if p.baseKey != nil {
		return p.baseKey(event)
	}
	return gfx.EventResult{}
}

func (p *Page) onPointer(event gfx.PointerEvent) gfx.EventResult {
	if p.pointerRoute != nil {
		if result, ok := p.pointerRoute(p, event); ok {
			return result
		}
	}
	if p.basePointer != nil {
		return p.basePointer(event)
	}
	return gfx.EventResult{}
}

// currentTheme is the theme the page paints with: the one given, or the system's.
func (p *Page) currentTheme() Theme {
	theme := p.theme
	if p.config.Theme != nil {
		theme = *p.config.Theme
	} else if !p.themed || p.stale.Swap(false) {
		p.themed = true
		if p.window && app.SystemAppearance() == app.AppearanceDark {
			theme = gfx.DarkTheme()
		} else {
			theme = gfx.LightTheme()
		}
	}
	if !p.themeSet || theme != p.theme {
		p.theme = theme
		p.themeEpoch++
		p.themeSet = true
	}
	return theme
}

// Config returns base with the page's Tree, Paint, TextInput, OnScroll, OnKey,
// OnShortcut and OnPointer:
//
//   - Tree: base's is adopted when the page was given none; a different one panics.
//   - Paint paints View over the whole window.
//   - TextInput answers for the page's TextFields and asks base's for any other
//     focused field (a terminal of the application's, say).
//   - OnScroll scrolls an open Select, then the page's scroll areas, and gives
//     what they leave to base's.
//   - OnKey, OnShortcut and OnPointer give a Shortcut its chord and a Control its
//     own events, then base's the rest.
//
// Config gives the page an Invalidator when neither PageConfig.Invalidator nor
// base has one, for the page's Effects and, with FollowSystemTheme, the appearance
// watch. OnClosed cancels the page's effects and stops that watch before it runs
// base's OnClosed. Nothing else is touched.
func (p *Page) Config(base app.Config) app.Config {
	if p.config.View == nil {
		panic("ui: Page.Config needs PageConfig.View, the function that declares the page")
	}
	p.init(base.Tree)
	if base.Tree != nil && base.Tree != p.tree {
		panic("ui: Page.Config: the app.Config has another Tree than the page's; give NewPage the window's Tree")
	}
	config := base
	config.Tree, config.Paint = p.tree, p.Paint
	p.baseText, config.TextInput = base.TextInput, p.TextInput
	p.baseKey, config.OnKey = base.OnKey, p.onKey
	p.basePointer, config.OnPointer = base.OnPointer, p.onPointer
	p.baseShortcut, config.OnShortcut = base.OnShortcut, p.onShortcut
	p.baseScroll, config.OnScroll = base.OnScroll, p.Wheel
	invalidator := p.config.Invalidator
	if invalidator == nil {
		invalidator = base.Invalidator
	}
	if invalidator == nil {
		invalidator = new(app.Invalidator)
	}
	config.Invalidator = invalidator
	p.invalidator = invalidator
	if p.config.FollowSystemTheme && p.config.Theme == nil {
		stop := app.WatchSystemAppearance(func() {
			p.stale.Store(true)
			invalidator.Invalidate()
		})
		closed := config.OnClosed
		config.OnClosed = func() {
			stop()
			if closed != nil {
				closed()
			}
		}
	}
	closed := config.OnClosed
	config.OnClosed = func() {
		p.Close()
		if closed != nil {
			closed()
		}
	}
	return config
}

// Paint fills the window with the background and declares the view over it. It
// is the Config's Paint.
func (p *Page) Paint(canvas *gfx.Canvas, size Size) {
	if p.config.View == nil {
		panic("ui: Page.Paint needs PageConfig.View, the function that declares the page")
	}
	p.init(nil)
	p.window = true
	bounds := Rect{Width: size.Width, Height: size.Height}
	theme := p.currentTheme()
	background := theme.Background
	if p.config.Background != nil {
		background = *p.config.Background
	}
	canvas.Fill(bounds, background)
	p.Draw(canvas, bounds, p.config.View)
}

// Draw declares view in bounds of canvas. When a Memo's copy does not fit where
// the page put it, what the page painted is taken back and it is painted again,
// with that Memo declared. A page that is a pane of a window laid out with a
// Frame is declared with Frame.Page, which calls Draw once the frame knows the
// pane's rectangle; a page given no Tree names its controls in the canvas's.
func (p *Page) Draw(canvas *gfx.Canvas, bounds Rect, view func(C)) {
	if tree := canvas.Tree(); tree != nil && tree.Len() != 0 {
		p.init(tree) // a page inside a window names its controls in the window's Tree
	} else {
		p.init(nil)
	}
	if tree := canvas.Tree(); tree != p.tree {
		if tree.Len() != 0 {
			panic("ui: Page.Draw: the canvas names its identities in another Tree than the page's; give NewPage the canvas's Tree")
		}
		canvas.SetTree(p.tree)
	}
	start := canvas.Mark()
	for try := 0; ; try++ {
		p.paintOnce(canvas, bounds, view)
		if !p.retry || try > 0 || !canvas.Rewind(start) {
			break
		}
	}
	p.retry = false
	p.settleFocus()
	p.settleEffects()
	if len(p.memos) > 0 {
		canvas.KeepPrevious(true) // the next paint copies from this one
	}
}

func (p *Page) paintOnce(canvas *gfx.Canvas, bounds Rect, view func(C)) {
	p.epoch++
	p.retry = false
	p.overlay = false
	clear(p.fields)
	p.fields = p.fields[:0]
	clear(p.controls)
	p.controls = p.controls[:0]
	p.shortcuts = p.shortcuts[:0]
	clear(p.menus)
	p.menus = p.menus[:0]
	clear(p.rovers)
	p.rovers = p.rovers[:0]
	clear(p.sliders)
	p.sliders = p.sliders[:0]
	p.groups, p.groupIDs = p.groups[:0], p.groupIDs[:0]
	clear(p.contexts)
	p.contexts = p.contexts[:0]
	clear(p.combos)
	p.combos = p.combos[:0]
	clear(p.trees)
	p.trees = p.trees[:0]
	clear(p.listStates)
	p.listStates = p.listStates[:0]
	clear(p.selects)
	p.selects = p.selects[:0]
	clear(p.ctx.stateSeq)
	clear(p.labels)
	p.labels = p.labels[:0]
	p.focusPaint = p.focusPaint[:0]
	for i := range p.held {
		if id := p.held[i].id; !id.IsZero() && id != canvas.Pressed() && id != canvas.Focused() {
			p.held[i] = armedButton{}
		}
	}
	f := p.view.Begin(canvas, p.currentTheme(), bounds)
	f.page = p
	p.ctx.page, p.ctx.f, p.ctx.nextSet = p, f, false
	p.nodes = append(p.nodes[:0], protoNode{scope: p.root, named: true, parentNode: -1}) // node 0, the root column
	view(&p.ctx)
	f.End()
	p.measureLists()
	p.sweep()
	p.sweepTexts()
	p.sweepMemos()
	p.sweepGroups()
}

// current is the container being filled that the page named: a container the
// Frame made for itself (a Dialog's surface) is looked through.
func (p *Page) current() int32 {
	for i := p.ctx.f.parent; i >= 0; i = p.ctx.f.nodes[i].parent {
		if int(i) < len(p.nodes) && p.nodes[i].named {
			return i
		}
	}
	return 0
}

// scopeOf returns the scope of the container being filled.
func (p *Page) scopeOf() gfx.Scope { return p.resolve(p.current()) }

// resolve makes the scope of container i, and of the ones it is named in.
func (p *Page) resolve(i int32) gfx.Scope {
	node := &p.nodes[i]
	if node.scope == 0 {
		parent := node.parentScope
		if parent == 0 {
			parent = p.resolve(node.parentNode)
			node = &p.nodes[i]
		}
		node.scope = p.tree.Sub(parent, node.name)
	}
	return node.scope
}

// id is the identity of an element at at.
func (p *Page) id(at place) gfx.ID {
	if at.scope == 0 {
		at.scope = p.resolve(at.node)
	}
	return at.scope.ID(at.name)
}

// setNode records element i as a container the page named at.
func (p *Page) setNode(i int32, at place) {
	for len(p.nodes) <= int(i) {
		p.nodes = append(p.nodes, protoNode{})
	}
	p.nodes[i] = protoNode{parentNode: at.node, parentScope: at.scope, name: at.name, named: true}
}

// setScope records element i as a container with a scope of its own.
func (p *Page) setScope(i int32, scope gfx.Scope) {
	for len(p.nodes) <= int(i) {
		p.nodes = append(p.nodes, protoNode{})
	}
	p.nodes[i] = protoNode{scope: scope, named: true, parentNode: -1}
}

func (p *Page) record(scope gfx.Scope) *protoScope {
	for len(p.scopes) <= int(scope) {
		p.scopes = append(p.scopes, nil)
	}
	s := p.scopes[scope]
	if s == nil {
		s = &protoScope{epoch: p.epoch - 1}
		p.scopes[scope] = s
	}
	if s.epoch != p.epoch {
		clear(s.prev)
		s.prev, s.labels, s.prevEpoch = s.labels, s.prev[:0], s.epoch
		s.epoch = p.epoch
	}
	return s
}

// name gives the next element of kind its place in the container being filled: a
// Key, a Part or its number among its kind. named reports a Key or a Part.
func (p *Page) name(kind uint8) (at place, named bool) {
	node := p.current()
	c := &p.ctx
	if c.nextSet {
		c.nextSet = false
		scope := p.resolve(node)
		switch c.next.kindOf {
		case 1:
			s := p.record(scope)
			key := c.next.key
			key.parent = scope
			var entry keyEntry
			var ok bool
			if key.isText {
				entry, ok = p.keys[key]
			} else {
				entry, ok = p.intKeys[intKey{parent: scope, number: key.number}]
			}
			if ok && entry.epoch == p.epoch {
				panic("ui: two siblings have the key " + key.describe() + " in one paint; key rows by something unique")
			}
			if !ok {
				entry.slot = s.keys.take(p.epoch)
			}
			entry.epoch = p.epoch
			if key.isText {
				p.keys[key] = entry
			} else {
				p.intKeys[intKey{parent: scope, number: key.number}] = entry
			}
			if len(p.recording) > 0 {
				p.noteKey(key)
			}
			if s.keySub == 0 {
				s.keySub = p.tree.Sub(scope, p.keyName)
			}
			return place{node: node, scope: s.keySub, name: gfx.Index(int(entry.slot))}, true
		case 2:
			return place{node: node, scope: scope, name: p.tree.Name(c.next.part)}, true
		}
	}
	counts := &p.nodes[node].counts
	n := counts[kind]
	counts[kind]++
	return place{node: node, name: gfx.Index(int(kind)<<24 | int(n))}, false
}

// container makes element i a container with a scope of its own, named in its parent.
func (p *Page) container(b Box) Box {
	at, _ := p.name(pkContainer)
	p.setNode(b.i, at)
	return b
}

// controlID names an element bound to nothing: its place, a Key or a Part.
func (p *Page) controlID(kind uint8) gfx.ID {
	at, _ := p.name(kind)
	return p.id(at)
}

// boundScope is the scope of a control that numbers its own parts (the options of
// a Choice, the tabs of Tabs): the scope its bound name makes.
func (p *Page) boundScope(kind uint8, value unsafe.Pointer) gfx.Scope {
	id := p.boundID(kind, value)
	return p.tree.Sub(id.Scope(), id.Name())
}

// boundID names a control bound to value. A value declared before keeps the name it
// had, wherever it is now: a field appearing above a password field does not take
// its name. A new value takes its place's name, unless a value declared in an
// earlier paint than the last still holds it; a value that is new in every paint (a
// copy) is given the name the last paint's copy had, so a copy works as its place.
func (p *Page) boundID(kind uint8, value unsafe.Pointer) gfx.ID {
	at, named := p.name(kind)
	key := boundKey{value: value, kind: kind}
	for {
		entry, ok := p.bound[key]
		if ok && entry.epoch == p.epoch {
			key.n++
			continue
		}
		if ok && p.names[entry.id].seen != p.epoch {
			entry.epoch = p.epoch
			p.bound[key] = entry
			p.claim(entry.id, entry.born)
			p.noteBound(key, entry.id)
			return entry.id
		}
		place := p.id(at)
		id := place
		owner, held := p.names[place]
		if !named {
			copyChain := held && owner.born+1 == p.epoch && owner.seen+1 == p.epoch
			free := !held || owner.seen+1 < p.epoch
			if !copyChain && !free || held && owner.seen == p.epoch {
				id = p.freshAt.At(int(p.fresh.take(p.epoch)))
			}
		} else if held && owner.seen == p.epoch {
			id = p.freshAt.At(int(p.fresh.take(p.epoch)))
		}
		p.bound[key] = protoBound{id: id, born: p.epoch, epoch: p.epoch, positional: id == place}
		p.claim(id, p.epoch)
		p.noteBound(key, id)
		return id
	}
}

func (p *Page) claim(id gfx.ID, born uint32) {
	p.names[id] = nameUse{seen: p.epoch, born: born}
}

// sweep forgets what the paint did not declare.
func (p *Page) sweep() {
	for key, entry := range p.keys {
		if entry.epoch != p.epoch {
			delete(p.keys, key)
			p.record(key.parent).keys.give(entry.slot, p.epoch)
		}
	}
	for key, entry := range p.intKeys {
		if entry.epoch != p.epoch {
			delete(p.intKeys, key)
			p.record(key.parent).keys.give(entry.slot, p.epoch)
		}
	}
	for key, entry := range p.bound {
		if entry.epoch != p.epoch {
			delete(p.bound, key)
			if entry.id.Scope() == p.freshAt {
				slot, _ := entry.id.Name().Index()
				p.fresh.give(uint32(slot), p.epoch)
			}
		}
	}
	for id, use := range p.names {
		if use.seen+2 < p.epoch {
			delete(p.names, id)
		}
	}
	for key, state := range p.states {
		if state.epoch+2 < p.epoch {
			delete(p.states, key)
		}
	}
}

// guard is Screen's protection of a held button whose place now names another.
func (p *Page) guard(b Box, id gfx.ID, label string) {
	canvas := p.ctx.f.canvas
	if canvas == nil {
		return
	}
	s := p.record(id.Scope())
	number := len(s.labels)
	s.labels = append(s.labels, label)
	focused := canvas.Focused() == id
	if !focused && canvas.Pressed() != id {
		return
	}
	var held *armedButton
	for i := range p.held {
		if p.held[i].id == id {
			held = &p.held[i]
		}
	}
	switch {
	case held == nil:
		for i := range p.held {
			if p.held[i].id.IsZero() {
				p.held[i] = armedButton{id: id, label: label}
				return
			}
		}
		p.held[0] = armedButton{id: id, label: label}
	case held.moved:
		b.n().flags |= flagVeto
	case held.label == label:
	case s.prevEpoch+1 == p.epoch && movedFrom(s.prev, label, number):
		held.moved = true
		b.n().flags |= flagVeto
		if focused {
			canvas.ReleaseFocus(id)
		}
	default:
		held.label = label
	}
}

// TextInput gives the platform the editor of the focused TextField.
func (p *Page) TextInput(context app.TextInputContext) *app.TextInput {
	for _, entry := range p.fields {
		if entry.id == context.ID && !context.ID.IsZero() {
			entry.field.input.Editor = entry.field.ensure()
			return &entry.field.input
		}
	}
	if h := p.control(context.ID); h != nil && h.Text != nil {
		return h.Text(context)
	}
	if p.textRoute != nil {
		if input := p.textRoute(p, context.ID); input != nil {
			return input
		}
	}
	if p.baseText != nil {
		return p.baseText(context)
	}
	return nil
}

// Wheel scrolls an open Select, else the scroll area under the pointer, and gives
// an event neither takes to the OnScroll Config was given. It is the Config's OnScroll.
func (p *Page) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	if h := p.controlOf(event.Target); h != nil && h.Scroll != nil {
		if result := h.Scroll(event); result.Handled {
			return result
		}
	}
	for _, state := range p.selects {
		if state.Open {
			return state.Wheel(event)
		}
	}
	if p.wheelRoute != nil {
		if result, ok := p.wheelRoute(p, event); ok {
			return result
		}
	}
	moved := false
	if p.wheelXRoute != nil {
		var only bool
		if moved, only = p.wheelXRoute(p, event); only && moved {
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if result := p.view.Wheel(event); result.Handled || result.Repaint {
		result.Repaint = result.Repaint || moved
		return result
	}
	if moved {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if p.baseScroll != nil {
		return p.baseScroll(event)
	}
	return gfx.EventResult{}
}

// State returns the element-private state named name, of type T, of the container
// being filled: kept while the container is declared, dropped two paints after it is
// not. It is for a component's own small state (whether its list is open); the
// application's state stays in its model. The first call for a place allocates the
// value; later ones do not.
//
//	open := ui.State[bool](c, "open")
//
// The name is the state's in its container, so a State asked for only in some paints
// (behind an if) does not take another's value. A component declared twice in one
// container asks for the same name twice: those are told apart by their order, as
// two Buttons are, so give each its own container or a Key when one of them comes
// and goes.
func State[T any](c C, name string) *T {
	return stateIn[T](c.page, c.page.scopeOf(), name)
}

// stateIn is State in scope: a control's own scope, for state that follows the
// value it is bound to rather than its place.
func stateIn[T any](p *Page, scope gfx.Scope, name string) *T {
	c := &p.ctx
	at := stateName{scope: scope, name: name}
	n := c.stateSeq[at]
	c.stateSeq[at] = n + 1
	key := protoStateKey{stateName: at, n: n}
	entry, ok := p.states[key]
	value, typed := entry.value.(*T)
	if !ok || !typed {
		value = new(T)
		entry.value = value
	}
	entry.epoch = p.epoch
	p.states[key] = entry
	if len(p.recording) > 0 {
		p.noteState(key)
	}
	return value
}
