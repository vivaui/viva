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

// Ctx is the declaration context of a Page. Use it through C.
type Ctx struct {
	page *Page
	f    *Frame
	// next names the next element instead of its place: a Key, an index or a part.
	next     protoName
	nextSet  bool
	stateSeq map[gfx.Scope]uint32 // reset each paint: the States asked for in each scope
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

// After runs fn after the paint that delivers a click, outside any paint, then
// repaints: for a handler that changes the list being declared.
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
}

// Page is a window's declaration: one tree for the whole window.
type Page struct {
	config PageConfig
	tree   *gfx.Tree
	root   gfx.Scope
	view   View
	ctx    Ctx
	theme  Theme
	themed bool
	window bool        // the page paints a window of its own, and so asks the system for its look
	stale  atomic.Bool // the system appearance changed since the theme was chosen
	epoch  uint32

	scopes     []*protoScope // by scope: Scopes are dense indexes into the Tree
	nodes      []protoNode   // each container of the frame, by node index
	keys       map[keyKey]keyEntry
	intKeys    map[intKey]keyEntry
	bound      map[boundKey]protoBound
	names      map[gfx.ID]nameUse // the names bound values were given
	fresh      slotPool
	freshAt    gfx.Scope
	kindName   [pkKinds]gfx.Name
	keyName    gfx.Name
	held       [2]armedButton
	states     map[protoStateKey]protoState
	fields     []fieldEntry
	selects    []*SelectState // the Selects of the last paint, which Wheel scrolls
	labels     []string       // scratch for Choice's labels
	memos      map[gfx.ID]*memoEntry
	memoBoxes  []memoBox
	memoChecks []memoCheckOf
	recording  []*memoEntry // the Memos being declared, outermost first
	retry      bool         // a Memo's copy failed: the paint is done again
	lists      []listDecl
	listRows   []int32 // the row containers of this paint's Lists

	baseText   app.TextInputProvider
	baseScroll func(gfx.ScrollEvent) gfx.EventResult
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

type protoStateKey struct {
	scope gfx.Scope
	n     uint32
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
	return &Page{config: config}
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
	p.ctx.stateSeq = map[gfx.Scope]uint32{}
	p.epoch = 1
}

// currentTheme is the theme the page paints with: the one given, or the system's.
func (p *Page) currentTheme() Theme {
	if p.config.Theme != nil {
		return *p.config.Theme
	}
	if !p.themed || p.stale.Swap(false) {
		p.themed = true
		if p.window && app.SystemAppearance() == app.AppearanceDark {
			p.theme = gfx.DarkTheme()
		} else {
			p.theme = gfx.LightTheme()
		}
	}
	return p.theme
}

// Config returns base with the page's Tree, Paint, TextInput and OnScroll:
//
//   - Tree: base's is adopted when the page was given none; a different one panics.
//   - Paint paints View over the whole window.
//   - TextInput answers for the page's TextFields and asks base's for any other
//     focused field (a terminal of the application's, say).
//   - OnScroll scrolls an open Select, then the page's scroll areas, and gives
//     what they leave to base's.
//
// FollowSystemTheme also sets an Invalidator when base has none, and stops
// watching the appearance in OnClosed. Nothing else is touched.
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
	p.baseScroll, config.OnScroll = base.OnScroll, p.Wheel
	if p.config.FollowSystemTheme && p.config.Theme == nil {
		invalidator := config.Invalidator
		if invalidator == nil {
			invalidator = new(app.Invalidator)
			config.Invalidator = invalidator
		}
		stop := app.WatchSystemAppearance(func() {
			p.stale.Store(true)
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
// with that Memo declared.
func (p *Page) Draw(canvas *gfx.Canvas, bounds Rect, view func(C)) {
	p.init(nil)
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
	if len(p.memos) > 0 {
		canvas.KeepPrevious(true) // the next paint copies from this one
	}
}

func (p *Page) paintOnce(canvas *gfx.Canvas, bounds Rect, view func(C)) {
	p.epoch++
	p.retry = false
	clear(p.fields)
	p.fields = p.fields[:0]
	clear(p.selects)
	p.selects = p.selects[:0]
	clear(p.ctx.stateSeq)
	clear(p.labels)
	p.labels = p.labels[:0]
	for i := range p.held {
		if id := p.held[i].id; !id.IsZero() && id != canvas.Pressed() && id != canvas.Focused() {
			p.held[i] = armedButton{}
		}
	}
	f := p.view.Begin(canvas, p.currentTheme(), bounds)
	p.ctx.page, p.ctx.f, p.ctx.nextSet = p, f, false
	p.nodes = append(p.nodes[:0], protoNode{scope: p.root, named: true, parentNode: -1}) // node 0, the root column
	view(&p.ctx)
	f.End()
	p.measureLists()
	p.sweep()
	p.sweepMemos()
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
	if p.baseText != nil {
		return p.baseText(context)
	}
	return nil
}

// Wheel scrolls an open Select, else the scroll area under the pointer, and gives
// an event neither takes to the OnScroll Config was given. It is the Config's OnScroll.
func (p *Page) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	for _, state := range p.selects {
		if state.Open {
			return state.Wheel(event)
		}
	}
	if result := p.view.Wheel(event); result.Handled || result.Repaint {
		return result
	}
	if p.baseScroll != nil {
		return p.baseScroll(event)
	}
	return gfx.EventResult{}
}

// State returns the element-private state of type T for the container being
// filled, the n-th asked for in it in this paint: kept while the container is
// declared, dropped two paints after it is not. It is for a component's own small
// state (whether its list is open); the application's state stays in its model.
// The first call for a place allocates the value; later ones do not.
func State[T any](c C) *T {
	p := c.page
	scope := p.scopeOf()
	n := c.stateSeq[scope]
	c.stateSeq[scope] = n + 1
	key := protoStateKey{scope: scope, n: n}
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
