package base

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
// The engine underneath is not part of the application model: a page does not
// expose it, and the frame and view types are unexported. A component is a
// function of C; a preview draws a Page.

import (
	"context"
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/window"
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
	Next     protoName
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
	c.Next, c.nextSet = protoName{key: keyKey{Text: key, isText: true}, kindOf: 1}, true
	return c
}

// KeyInt is Key for a number, a record's ID.
func (c *Ctx) KeyInt(key int) C {
	c.Next, c.nextSet = protoName{key: keyKey{number: key}, kindOf: 1}, true
	return c
}

// Part names the next element by a word of the program: the parts of a component
// (its trigger, its list), which a test can address by that word.
func (c *Ctx) Part(Name string) C {
	checkName("Part", Name)
	c.Next, c.nextSet = protoName{part: Name, kindOf: 2}, true
	return c
}

// After runs fn after the paint that delivers a click or a pointer event (a drop),
// outside any paint, then repaints: for a handler that changes the list being
// declared.
func (c *Ctx) After(fn func()) {
	if c.f.Surface != nil {
		c.f.Surface.AfterPaint(fn)
	}
}

// ID is the identity of a control of the application's own declared next (one
// made with the frame, or a recipe taking an ID): named by its number among such
// controls in the container being filled, or by the Key or Part set before it.
func ID(c C) gfx.ID { return c.page.ControlID(pkCustom) }

// Application is the application whose window the page was opened in by
// OpenWindow, so a handler can open more windows: a *app.Application. It is nil
// for a page run by Run or RunCanvas, or drawn into a window of someone else's,
// which have no Application.
func (c *Ctx) Application() any { return c.page.application }

// Theme is the page's theme. Read it; do not keep it.
func (c *Ctx) Theme() *Theme { return &c.f.Palette }

// Bounds is the rectangle the page is laid out in, as the window gave it.
func (c *Ctx) Bounds() Rect { return c.f.Bounds() }

// Canvas is the page's canvas, for measuring text or components before declaring
// them and for reading pointer/focus state. Declare elements through C and paint
// through Draw; do not paint from here.
func (c *Ctx) Canvas() *gfx.Canvas { return c.f.Canvas() }

// PopupLayer raises the popups the page declares (menus, combobox lists,
// dialogs) to layer, as frame.PopupLayer does for a frame's popups: a widget's
// popup uses the higher of its own layer plus two and this one.
func PopupLayer(c C, layer uint16) { c.f.PopupLayer(layer) }

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
	PKToaster
	pkDrawer
	pkTree
	pkToggle
	pkRadio
	pkTab
	pkList
	pkDisclosure
	pkProgress
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
	ID          gfx.ID
	born, epoch uint32
	positional  bool
}

// PageConfig says how a Page paints.
type PageConfig struct {
	// View declares the page. It is what Paint draws.
	View func(C)
	// Tree is the identity tree the page names its elements in. Nil takes the one
	// of the app.Config given to Configure, or else one of the page's own. A window
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
	// Appearance reports the system's appearance, for FollowSystemTheme. Nil is
	// the light theme; the facade sets it from the platform.
	Appearance func() window.Appearance
	// Background is the color Paint fills the window with, the theme's Background
	// when nil. Draw fills nothing.
	Background *Color
	// ScrollbarSize is the thickness of the scrollbars the page declares with
	// Scrollbars and of self-scrolling Tables, unless a bar sets its own; zero
	// keeps the thin hover default. It is the page's one place for the weight its
	// bars share.
	ScrollbarSize float32
	// Invalidator wakes the window the page is drawn in when an Effect of the page
	// finishes. Configure uses it instead of the app.Config's; a page the application
	// draws into a window of its own (Draw, not Paint) needs it, since only Config
	// would otherwise supply one. The interface lets a page depend on the window
	// contract rather than on the platform loop.
	Invalidator window.Invalidator
	// MemoCheck makes every Memo of this page declare its part and panic where a
	// copy of the last paint would have differed, as MemoCheck(true) does for every
	// page. It is for tests.
	MemoCheck bool
	// Styles replaces the recipes the page's styled controls paint with; nil uses
	// the theme's. See Styles. It is for an application with a look of its own
	// that keeps using Button, Checkbox, Switch, Input and Textarea rather than
	// their Bases, and it does not affect a control given a look of its own.
	Styles *Styles
}

// Page is a window's declaration: one tree for the whole window. A Page lives
// across paints, as its C, Box and frame values do not.
type Page struct {
	config     PageConfig
	checkMemos bool
	tree       *gfx.Tree
	root       gfx.Scope
	View       View
	ctx        Ctx
	Palette    Theme
	themeSet   bool
	themeEpoch uint32
	themed     bool
	window     bool        // the page paints a window of its own, and so asks the system for its look
	stale      atomic.Bool // the system appearance changed since the theme was chosen
	epoch      uint32

	effects       []effectState      // the Effects of the last paint, which their work belongs to
	effectContext context.Context    // what their runs derive from; cancelled when the page closes
	effectCancel  context.CancelFunc // cancels effectContext
	invalidator   window.Invalidator // wakes the window when work finishes, when the page has one
	application   any                // what OpenWindow opened the page's window in, else nil

	scopes      []*protoScope // by scope: Scopes are dense indexes into the Tree
	Nodes       []protoNode   // each container of the frame, by node index
	keys        map[keyKey]keyEntry
	intKeys     map[intKey]keyEntry
	Bound       map[boundKey]protoBound
	names       map[gfx.ID]nameUse // the names bound values were given
	fresh       slotPool
	freshAt     gfx.Scope
	kindName    [pkKinds]gfx.Name
	keyName     gfx.Name
	menuName    gfx.Name // the place of a ContextMenu inside its target (contextMenu)
	held        [2]armedButton
	States      map[protoStateKey]protoState
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
	combos      []*ComboState         // the Comboboxes of the last paint
	trees       []*TreeState          // the Trees of the last paint
	drags       []*DragState          // the DragStates of the last paint
	drag        *DragState            // the one following a press, if any
	listStates  []*ListState          // the Lists of the last paint
	tables      []tableRoute          // the Tables of the last paint, for the wheel and keys under the pointer
	pointer     gfx.Point             // where the pointer was at the last pointer event
	pointerIn   bool                  // a pointer event put it there, and no exit has taken it away
	hoverArea   gfx.ID                // the scroll area or self-scrolling table the pointer is on
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

	baseText     window.TextInputProvider
	baseScroll   func(gfx.ScrollEvent) gfx.EventResult
	baseKey      func(gfx.KeyEvent) gfx.EventResult
	basePointer  func(gfx.PointerEvent) gfx.EventResult
	baseShortcut func(gfx.KeyEvent) gfx.EventResult
	// keyRoute, pointerRoute, areaKey and shortcutRoute are set by the first
	// Control, TextArea or Shortcut a page declares, so a page without them does
	// not link what routes their input.
	keyRoute      func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	fieldKeyRoute func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // the focused field's Box.OnKey
	pointerRoute  func(*Page, gfx.PointerEvent) (gfx.EventResult, bool)
	areaKey       func(*Field, gfx.KeyEvent) gfx.EventResult
	shortcutRoute func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	menuRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	roveRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // arrow keys in a Choice or Tabs
	groupRoute    func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // arrow keys in a FocusGroup
	sliderRoute   func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // the keys of a Slider
	selectRoute   func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // opens and moves the list of a Select
	comboRoute    func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	treeRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool)
	listRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // moves and chooses rows of a List
	wheelXRoute   func(*Page, gfx.ScrollEvent) (moved, only bool)   // the sideways wheel of a List with RowWidth
	typeRoute     func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) // a Tree's typeahead, from OnShortcut
	wheelRoute    func(*Page, gfx.ScrollEvent) (gfx.EventResult, bool)
	textRoute     func(*Page, gfx.ID) *window.TextInput
}

// protoNode is a container of the frame: where it is named (in the container at
// parentNode, or in parentScope for a keyed row), how many elements of each kind it
// has, and the scope its name makes, made only once something needs it.
type protoNode struct {
	parentNode  int32
	parentScope gfx.Scope
	Name        gfx.Name
	scope       gfx.Scope
	counts      [pkKinds]uint32
	named       bool
}

// place is where an element is named: in a container, by a name.
type Place struct {
	Node  int32
	scope gfx.Scope // set when known: a keyed or named element's
	Name  gfx.Name
}

// intKey is a KeyInt row: compared without a string.
type intKey struct {
	Parent gfx.Scope
	number int
}

// stateName is a State's name in a scope; protoStateKey adds which of that name it
// is, for a component declared twice in one container.
type stateName struct {
	scope gfx.Scope
	Name  string
}

type protoStateKey struct {
	stateName
	n uint32
}

type protoState struct {
	Value any
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
	p.menuName = p.tree.Name("~menu")
	p.freshAt = p.tree.Sub(p.root, p.tree.Name("~new"))
	p.keys = map[keyKey]keyEntry{}
	p.intKeys = map[intKey]keyEntry{}
	p.Bound = map[boundKey]protoBound{}
	p.names = map[gfx.ID]nameUse{}
	p.memos = map[gfx.ID]*memoEntry{}
	p.States = map[protoStateKey]protoState{}
	p.texts = map[gfx.ID]*boundText{}
	p.ctx.stateSeq = map[stateName]uint32{}
	p.epoch = 1
}

// owns reports whether id is one the page named: one of its controls, or what
// its Popovers and Modals report a dismissal with.
func (p *Page) owns(ID gfx.ID) bool {
	return p.tree != nil && !ID.IsZero() && p.tree.Within(ID.Scope(), p.root)
}

// routeKey gives a key to the focused field's OnKey handler, then to the Combobox
// it is typed in or the open Menu it moves in, else the Shortcut it presses, else
// the Control or TextArea it is for.
func (p *Page) RouteKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if p.fieldKeyRoute != nil {
		if result, ok := p.fieldKeyRoute(p, event); ok {
			return result, true
		}
	}
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
	if p.selectRoute != nil {
		if result, ok := p.selectRoute(p, event); ok {
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
			if entry.ID == event.Target {
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

// fieldKey gives a key to the handler Box.OnKey put on the focused field. Keys
// that type text stay with the text system, so a handler cannot take typing away,
// and a key it leaves unhandled goes on to the page's other routes.
func (p *Page) FieldKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release || event.Text != "" || event.Key == gfx.KeyUnknown ||
		event.Key == gfx.KeySpace || event.Key.IsModifier() {
		return gfx.EventResult{}, false
	}
	for i := range p.fields {
		entry := &p.fields[i]
		if entry.key == nil || entry.ID != event.Target {
			continue
		}
		if result := entry.key(event); result.Handled {
			return result, true
		}
		return gfx.EventResult{}, false
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

// onKey and onPointer are the Config's: a drag's, a Control's or TextArea's,
// then base's. A full-window page has nothing behind its overlays, so they do
// not take keys here.
func (p *Page) onKey(event gfx.KeyEvent) gfx.EventResult {
	if result, ok := p.cancelDrag(event); ok {
		return result
	}
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
	switch event.Kind {
	case gfx.PointerMove, gfx.PointerDown, gfx.PointerUp:
		p.pointer, p.pointerIn = gfx.Point{X: event.X, Y: event.Y}, true
	case gfx.PointerExit:
		p.pointerIn = false
		// A press the platform took for itself (a window move) never gets its release:
		// the rows a List was carrying are not dropped.
		for _, s := range p.listStates {
			s.list.drag.reset()
		}
	}
	// A bar is revealed by the pointer being over its area, and an area with no
	// hit regions of its own would otherwise move no hover and never repaint to
	// show it.
	wasArea := p.hoverArea
	p.hoverArea = gfx.ID{}
	if p.pointerIn {
		p.hoverArea = p.hoverPlaceAt(p.pointer.X, p.pointer.Y)
	}
	entered := wasArea != p.hoverArea
	var result gfx.EventResult
	if done, ok := p.dropPointer(event); ok {
		result = done
	} else if p.pointerRoute != nil {
		if routed, ok := p.pointerRoute(p, event); ok {
			result = routed
		} else if p.basePointer != nil {
			result = p.basePointer(event)
		}
	} else if p.basePointer != nil {
		result = p.basePointer(event)
	}
	result.Repaint = result.Repaint || entered
	return result
}

// hoverPlaceAt names the innermost scroll area or self-scrolling table under a
// point; the zero ID means none.
func (p *Page) hoverPlaceAt(x, y float32) gfx.ID {
	best, bestArea := gfx.ID{}, float32(0)
	if f := p.View.Frame; f != nil {
		for i := range f.Nodes {
			node := &f.Nodes[i]
			if node.Ext == 0 {
				continue
			}
			state := f.Exts[node.Ext].scrollState
			if state == nil || state.ID.IsZero() || !pointInRect(x, y, node.Rect) {
				continue
			}
			if area := node.Rect.Width * node.Rect.Height; best.IsZero() || area < bestArea {
				best, bestArea = state.ID, area
			}
		}
	}
	for i := range p.tables {
		route := &p.tables[i]
		if (!route.Scrolls && !route.sideways) || (route.bar.IsZero() && route.horizontal.IsZero()) || !pointInRect(x, y, route.bounds) {
			continue
		}
		if area := route.bounds.Width * route.bounds.Height; best.IsZero() || area < bestArea {
			best, bestArea = route.bar, area
		}
	}
	return best
}

// dropPointer gives the pointer to the drag following a press, or to the
// DragState a press on it starts. It reports whether the drag handled the event;
// a press that does not move is left to the page's click.
func (p *Page) dropPointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if s := p.drag; s != nil {
		result, done := s.pointer(event)
		if done {
			p.drag = nil
		}
		return result, result.Handled
	}
	if event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}, false
	}
	for i := len(p.drags) - 1; i >= 0; i-- {
		if p.drags[i].press(event) {
			p.drag = p.drags[i]
			break
		}
	}
	return gfx.EventResult{}, false
}

// cancelDrag lets go of the drag an Escape pressed, and reports whether it did.
// The state keeps the pointer until its release, so the cancelled drag does not
// become a click.
func (p *Page) cancelDrag(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Key != gfx.KeyEscape || event.Release || p.drag == nil || !p.drag.active {
		return gfx.EventResult{}, false
	}
	p.drag.Cancel()
	return gfx.EventResult{Handled: true, Repaint: true}, true
}

// currentTheme is the theme the page paints with: the one given, or the system's.
func (p *Page) currentTheme() Theme {
	theme := p.Palette
	if p.config.Theme != nil {
		theme = *p.config.Theme
	} else if !p.themed || p.stale.Swap(false) {
		p.themed = true
		dark := p.config.Appearance != nil && p.config.Appearance() == window.AppearanceDark
		if p.window && dark {
			theme = gfx.DarkTheme()
		} else {
			theme = gfx.LightTheme()
		}
	}
	if !p.themeSet || theme != p.Palette {
		p.Palette = theme
		p.themeEpoch++
		p.themeSet = true
	}
	return theme
}

// Host is the set of hooks a window offers a page. Attach fills the ones the
// page answers and keeps the host's own, so the facade can hand them back to the
// platform's config.
type Host struct {
	Tree       *gfx.Tree
	Paint      func(*gfx.Canvas, Size)
	TextInput  window.TextInputProvider
	OnScroll   func(gfx.ScrollEvent) gfx.EventResult
	OnKey      func(gfx.KeyEvent) gfx.EventResult
	OnShortcut func(gfx.KeyEvent) gfx.EventResult
	OnPointer  func(gfx.PointerEvent) gfx.EventResult
}

// Attach wires the page into a host: the page adopts or keeps the tree, paints
// the view, answers text input, and routes scroll, keys, shortcuts and pointers
// before handing what is left to the host's own callbacks.
func (p *Page) Attach(h *Host) {
	h.Tree, h.Paint = p.tree, p.Paint
	p.baseText, h.TextInput = h.TextInput, p.TextInput
	p.baseKey, h.OnKey = h.OnKey, p.onKey
	p.basePointer, h.OnPointer = h.OnPointer, p.onPointer
	p.baseShortcut, h.OnShortcut = h.OnShortcut, p.onShortcut
	p.baseScroll, h.OnScroll = h.OnScroll, p.Wheel
}

// Init prepares the page's identity tree from the host's, or a new one.
func (p *Page) Init(tree *gfx.Tree) { p.init(tree) }

// IdentityTree is the tree the page names its elements in.
func (p *Page) IdentityTree() *gfx.Tree { return p.tree }

// SetInvalidator makes the page wake the window through invalidator.
func (p *Page) SetInvalidator(invalidator window.Invalidator) { p.invalidator = invalidator }

// MarkThemeStale makes the next paint read the system's appearance again.
func (p *Page) MarkThemeStale() { p.stale.Store(true) }

// SetApplication records the application whose window the page was opened in.
func (p *Page) SetApplication(application any) { p.application = application }

// Paint fills the window with the background and declares the view over it. It
// is the Config's Paint.
func (p *Page) Paint(Surface *gfx.Canvas, Size Size) {
	if p.config.View == nil {
		panic("ui: Page.Paint needs PageConfig.View, the function that declares the page")
	}
	p.init(nil)
	p.window = true
	bounds := Rect{Width: Size.Width, Height: Size.Height}
	theme := p.currentTheme()
	background := theme.Background
	if p.config.Background != nil {
		background = *p.config.Background
	}
	Surface.Fill(bounds, background)
	p.Draw(Surface, bounds, p.config.View)
}

// Draw declares view in bounds of canvas. When a Memo's copy does not fit where
// the page put it, what the page painted is taken back and it is painted again,
// with that Memo declared. A page drawn into a frame slot by Slot.Draw is laid
// out at the slot's rectangle; a page given no Tree names its controls in the
// canvas's. A host that draws a page this way routes its input with Key,
// Pointer, Wheel and TextInput; Config does that for a whole window.
func (p *Page) Draw(Surface *gfx.Canvas, bounds Rect, View func(C)) {
	if tree := Surface.Tree(); tree != nil && tree.Len() != 0 {
		p.init(tree) // a page inside a window names its controls in the window's Tree
	} else {
		p.init(nil)
	}
	if tree := Surface.Tree(); tree != p.tree {
		if tree.Len() != 0 {
			panic("ui: Page.Draw: the canvas names its identities in another Tree than the page's; give NewPage the canvas's Tree")
		}
		Surface.SetTree(p.tree)
	}
	start := Surface.Mark()
	for try := 0; ; try++ {
		p.paintOnce(Surface, bounds, View)
		if !p.retry || try > 0 || !Surface.Rewind(start) {
			break
		}
	}
	p.retry = false
	p.settleFocus()
	p.settleEffects()
	if len(p.memos) > 0 {
		Surface.KeepPrevious(true) // the next paint copies from this one
	}
}

func (p *Page) paintOnce(Surface *gfx.Canvas, bounds Rect, View func(C)) {
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
	clear(p.drags)
	p.drags = p.drags[:0]
	clear(p.listStates)
	p.listStates = p.listStates[:0]
	clear(p.tables)
	p.tables = p.tables[:0]
	clear(p.selects)
	p.selects = p.selects[:0]
	clear(p.ctx.stateSeq)
	clear(p.labels)
	p.labels = p.labels[:0]
	p.focusPaint = p.focusPaint[:0]
	for i := range p.held {
		if id := p.held[i].ID; !id.IsZero() && id != Surface.Pressed() && id != Surface.Focused() {
			p.held[i] = armedButton{}
		}
	}
	f := p.View.Begin(Surface, p.currentTheme(), bounds)
	f.styles = p.config.Styles
	f.page = p
	p.ctx.page, p.ctx.f, p.ctx.nextSet = p, f, false
	p.Nodes = append(p.Nodes[:0], protoNode{scope: p.root, named: true, parentNode: -1}) // node 0, the root column
	View(&p.ctx)
	f.End()
	if s := p.drag; s != nil && s.epoch != p.epoch {
		s.Cancel()
	}
	p.measureLists()
	p.sweep()
	p.sweepTexts()
	p.sweepMemos()
	p.sweepGroups()
}

// current is the container being filled that the page named: a container the
// frame made for itself (a Dialog's surface) is looked through.
func (p *Page) current() int32 {
	for i := p.ctx.f.Parent; i >= 0; i = p.ctx.f.Nodes[i].Parent {
		if int(i) < len(p.Nodes) && p.Nodes[i].named {
			return i
		}
	}
	return 0
}

// scopeOf returns the scope of the container being filled.
func (p *Page) scopeOf() gfx.Scope { return p.resolve(p.current()) }

// resolve makes the scope of container i, and of the ones it is named in.
func (p *Page) resolve(i int32) gfx.Scope {
	node := &p.Nodes[i]
	if node.scope == 0 {
		parent := node.parentScope
		if parent == 0 {
			parent = p.resolve(node.parentNode)
			node = &p.Nodes[i]
		}
		node.scope = p.tree.Sub(parent, node.Name)
	}
	return node.scope
}

// id is the identity of an element at at.
func (p *Page) ID(at Place) gfx.ID {
	if at.scope == 0 {
		at.scope = p.resolve(at.Node)
	}
	return at.scope.ID(at.Name)
}

// setNode records element i as a container the page named at.
func (p *Page) SetNode(i int32, at Place) {
	for len(p.Nodes) <= int(i) {
		p.Nodes = append(p.Nodes, protoNode{})
	}
	p.Nodes[i] = protoNode{parentNode: at.Node, parentScope: at.scope, Name: at.Name, named: true}
}

// setScope records element i as a container with a scope of its own.
func (p *Page) SetScope(i int32, scope gfx.Scope) {
	for len(p.Nodes) <= int(i) {
		p.Nodes = append(p.Nodes, protoNode{})
	}
	p.Nodes[i] = protoNode{scope: scope, named: true, parentNode: -1}
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
func (p *Page) Name(Kind uint8) (at Place, named bool) {
	node := p.current()
	c := &p.ctx
	if c.nextSet {
		c.nextSet = false
		scope := p.resolve(node)
		switch c.Next.kindOf {
		case 1:
			s := p.record(scope)
			key := c.Next.key
			key.Parent = scope
			var entry keyEntry
			var ok bool
			if key.isText {
				entry, ok = p.keys[key]
			} else {
				entry, ok = p.intKeys[intKey{Parent: scope, number: key.number}]
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
				p.intKeys[intKey{Parent: scope, number: key.number}] = entry
			}
			if len(p.recording) > 0 {
				p.noteKey(key)
			}
			if s.keySub == 0 {
				s.keySub = p.tree.Sub(scope, p.keyName)
			}
			return Place{Node: node, scope: s.keySub, Name: gfx.Index(int(entry.slot))}, true
		case 2:
			return Place{Node: node, scope: scope, Name: p.tree.Name(c.Next.part)}, true
		}
	}
	counts := &p.Nodes[node].counts
	n := counts[Kind]
	counts[Kind]++
	return Place{Node: node, Name: gfx.Index(int(Kind)<<24 | int(n))}, false
}

// container makes element i a container with a scope of its own, named in its parent.
func (p *Page) container(b Box) Box {
	at, _ := p.Name(pkContainer)
	p.SetNode(b.i, at)
	return b
}

// controlID names an element bound to nothing: its place, a Key or a Part.
func (p *Page) ControlID(Kind uint8) gfx.ID {
	at, _ := p.Name(Kind)
	return p.ID(at)
}

// boundScope is the scope of a control that numbers its own parts (the options of
// a Choice, the tabs of Tabs): the scope its bound name makes.
func (p *Page) BoundScope(Kind uint8, Value unsafe.Pointer) gfx.Scope {
	id := p.boundID(Kind, Value)
	return p.tree.Sub(id.Scope(), id.Name())
}

// boundID names a control bound to value. A value declared before keeps the name it
// had, wherever it is now: a field appearing above a password field does not take
// its name. A new value takes its place's name, unless a value declared in an
// earlier paint than the last still holds it; a value that is new in every paint (a
// copy) is given the name the last paint's copy had, so a copy works as its place.
func (p *Page) boundID(Kind uint8, Value unsafe.Pointer) gfx.ID {
	at, named := p.Name(Kind)
	key := boundKey{Value: Value, Kind: Kind}
	for {
		entry, ok := p.Bound[key]
		if ok && entry.epoch == p.epoch {
			key.n++
			continue
		}
		if ok && p.names[entry.ID].seen != p.epoch {
			entry.epoch = p.epoch
			p.Bound[key] = entry
			p.claim(entry.ID, entry.born)
			p.noteBound(key, entry.ID)
			return entry.ID
		}
		place := p.ID(at)
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
		p.Bound[key] = protoBound{ID: id, born: p.epoch, epoch: p.epoch, positional: id == place}
		p.claim(id, p.epoch)
		p.noteBound(key, id)
		return id
	}
}

func (p *Page) claim(ID gfx.ID, born uint32) {
	p.names[ID] = nameUse{seen: p.epoch, born: born}
}

// sweep forgets what the paint did not declare.
func (p *Page) sweep() {
	for key, entry := range p.keys {
		if entry.epoch != p.epoch {
			delete(p.keys, key)
			p.record(key.Parent).keys.give(entry.slot, p.epoch)
		}
	}
	for key, entry := range p.intKeys {
		if entry.epoch != p.epoch {
			delete(p.intKeys, key)
			p.record(key.Parent).keys.give(entry.slot, p.epoch)
		}
	}
	for key, entry := range p.Bound {
		if entry.epoch != p.epoch {
			delete(p.Bound, key)
			if entry.ID.Scope() == p.freshAt {
				slot, _ := entry.ID.Name().Index()
				p.fresh.give(uint32(slot), p.epoch)
			}
		}
	}
	for id, use := range p.names {
		if use.seen+2 < p.epoch {
			delete(p.names, id)
		}
	}
	for key, state := range p.States {
		if state.epoch+2 < p.epoch {
			delete(p.States, key)
		}
	}
}

// guard is Screen's protection of a held button whose place now names another.
func (p *Page) Guard(b Box, ID gfx.ID, Label string) {
	canvas := p.ctx.f.Surface
	if canvas == nil {
		return
	}
	s := p.record(ID.Scope())
	number := len(s.labels)
	s.labels = append(s.labels, Label)
	focused := canvas.Focused() == ID
	if !focused && canvas.Pressed() != ID {
		return
	}
	var held *armedButton
	for i := range p.held {
		if p.held[i].ID == ID {
			held = &p.held[i]
		}
	}
	switch {
	case held == nil:
		for i := range p.held {
			if p.held[i].ID.IsZero() {
				p.held[i] = armedButton{ID: ID, Label: Label}
				return
			}
		}
		p.held[0] = armedButton{ID: ID, Label: Label}
	case held.moved:
		b.N().Flags |= flagVeto
	case held.Label == Label:
	case s.prevEpoch+1 == p.epoch && movedFrom(s.prev, Label, number):
		held.moved = true
		b.N().Flags |= flagVeto
		if focused {
			canvas.ReleaseFocus(ID)
		}
	default:
		held.Label = Label
	}
}

// TextInput gives the platform the editor of the focused TextField.
func (p *Page) TextInput(context window.TextInputContext) *window.TextInput {
	for _, entry := range p.fields {
		if entry.ID == context.ID && !context.ID.IsZero() {
			entry.field.input.Alignment = context.Alignment
			entry.field.input.Editor = entry.field.ensure()
			return &entry.field.input
		}
	}
	if h := p.controlOf(context.ID); h != nil && h.Text != nil {
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

// Key routes a key event for a page the application draws into a region of its
// own (Draw, not Paint): its tabs, shortcuts, controls and drag states. A page
// that paints a whole window gets the same routing from Config's OnKey. Give the
// page the event before the host's own key handling, and do not call it for a
// page Configure already routes.
func (p *Page) Key(event gfx.KeyEvent) gfx.EventResult { return p.onKey(event) }

// Pointer routes a pointer event for a page the application draws into a region
// of its own (Draw, not Paint): its controls and drag states. A page that paints
// a whole window gets the same routing from Config's OnPointer, so a host that
// embeds a page calls this and does not also give the event to the window's
// OnPointer.
func (p *Page) Pointer(event gfx.PointerEvent) gfx.EventResult { return p.onPointer(event) }

// pointerOver reports whether the pointer is over r. A page whose input the
// application routes itself may never see a pointer event, so callers that paint
// hover bars pass the page's verdict and a view without one keeps its bars
// revealed.
func (p *Page) PointerOver(r Rect) bool {
	return p != nil && p.pointerIn && pointInRect(p.pointer.X, p.pointer.Y, r)
}

// Wheel routes the wheel through the page and gives an event no route takes to
// the OnScroll Config was given. It is the Config's OnScroll; a host that draws a
// page into a region calls it itself, and a host whose own OnScroll hands the
// wheel to the page gives RouteWheel instead, so the page does not fall back into
// the handler already routing it.
func (p *Page) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	if result := p.RouteWheel(event); result.Handled || result.Repaint {
		return result
	}
	if p.baseScroll != nil {
		return p.baseScroll(event)
	}
	return gfx.EventResult{}
}

// RouteWheel scrolls an open Select, a TextArea under the pointer, or the scroll
// area or sideways list under it, and reports an event none of them takes as not
// handled. It is the page's own routing without the OnScroll fallback: a host
// that forwards a wheel to the page from its own OnScroll calls this, so the page
// cannot hand the same event back to the handler that is already routing it.
func (p *Page) RouteWheel(event gfx.ScrollEvent) gfx.EventResult {
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
	// A TextArea under the pointer scrolls its own rows first; at its end the wheel goes on
	// to the page, as in a browser.
	for _, entry := range p.fields {
		if entry.field.rows && entry.field.area != nil && entry.field.area.Wheel(event) {
			return gfx.EventResult{Handled: true, Repaint: true}
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
	if result := p.View.Wheel(event); result.Handled || result.Repaint {
		result.Repaint = result.Repaint || moved
		return result
	}
	for i := len(p.tables) - 1; i >= 0; i-- {
		route := &p.tables[i]
		if !route.Scrolls && !route.sideways || !pointInRect(event.X, event.Y, route.bounds) {
			continue
		}
		state := &route.state.Scroll
		dx, dy := state.wheelSteps(event)
		if !state.canScroll(dx, dy) {
			continue
		}
		result := state.Wheel(event)
		result.Repaint = result.Repaint || moved
		return result
	}
	if moved {
		return gfx.EventResult{Handled: true, Repaint: true}
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
func State[T any](c C, Name string) *T {
	return stateIn[T](c.page, c.page.scopeOf(), Name)
}

// stateIn is State in scope: a control's own scope, for state that follows the
// value it is bound to rather than its place.
func stateIn[T any](p *Page, scope gfx.Scope, Name string) *T {
	c := &p.ctx
	at := stateName{scope: scope, Name: Name}
	n := c.stateSeq[at]
	c.stateSeq[at] = n + 1
	key := protoStateKey{stateName: at, n: n}
	entry, ok := p.States[key]
	value, typed := entry.Value.(*T)
	if !ok || !typed {
		value = new(T)
		entry.Value = value
	}
	entry.epoch = p.epoch
	p.States[key] = entry
	if len(p.recording) > 0 {
		p.noteState(key)
	}
	return value
}

// PageConfig returns the config the page was made with.
func (p *Page) PageConfig() PageConfig { return p.config }

// Overlay reports whether the last paint declared an overlay: an open Popover,
// Modal, Menu, Select or Combobox list.
func (p *Page) Overlay() bool { return p.overlay }
