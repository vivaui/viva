// Package navigationmenu is the opt-in page control for the top-level
// navigation menu of package components. It wraps
// components.BeginNavigationMenu in a ui.Control, so a page declares the
// triggers, their direct links and a panel of described destinations without
// calling the component itself:
//
//	m.navigation.Reset()
//	m.navigation.Trigger(productsID, "Products").Hint("Open product navigation")
//	m.navigation.Link(docsID, "Docs").Value("/docs").Active(m.page == "docs")
//	m.navigation.ContentLink(guideID, "Getting started").
//		Description("Build a native window and retained scene.").
//		Value("/guide").Active(m.page == "guide")
//	m.navigation.View(c, "Navigation menu").FillWidth().Height(36)
//	// After the paint, Selected names the link chosen last.
//	if m.navigation.Selected == docsID {
//		m.page = "docs"
//		m.navigation.Selected = gfx.ID{}
//	}
//
// The caller owns the open trigger, the focused entry or link, the chosen link
// and every label, description and identity. The wrapper paints through the
// component with exactly the identities the caller supplied, changes Open when
// a trigger is clicked, and reports the clicked or chosen identity with Clicked
// and Selected. Pointer and key events aimed at the control itself reach the
// optional callbacks.
//
// DirectContent declares a menu whose panel has no indicator of its own; it is
// an option, not a second constructor. The panel is open whenever Open names a
// trigger.
//
// Declarations are rebuilt every paint: call Reset, declare the entries in
// order, then View. The slices keep their capacity, so a warm paint allocates
// nothing. Keep a State in the model; its handlers and draw function are made
// once. Do not copy one after View has painted it.
//
// It is a separate package so an application that does not import it pays
// nothing for it.
package navigationmenu

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Direction is components.Direction, named here so a page that declares a
// navigation menu reads through this package alone.
type Direction = components.Direction

// State is one navigation menu's caller-owned state, its declarations
// and its page control. Fill Open and the callbacks from the model; declare the
// entries every paint; then call View.
type State struct {
	// Open is the trigger whose panel is open, or zero when it is closed. A
	// trigger click toggles it; choosing a link closes it unless DirectContent
	// is set.
	Open gfx.ID
	// Focus is the trigger, link or panel link the keyboard acts on. Clicking
	// an entry or link sets it; the arrow keys move it.
	Focus gfx.ID
	// Selected is the link chosen last by a click or by Enter, or zero before a
	// link is chosen. The wrapper writes it; the caller consumes and clears it.
	Selected gfx.ID
	// DirectContent paints the open panel without its indicator.
	DirectContent bool
	// Direction selects the arrow movement. The zero value is left to right.
	Direction components.Direction
	// Layer is the panel's layer. Zero uses layer 1.
	Layer uint16
	// Scope and DismissID are passed to the panel: DismissID is the identity an
	// Escape or an outside press reports.
	Scope, DismissID gfx.ID
	// Viewport, with DismissID, makes a press outside the panel report DismissID
	// and pass through, as a non-modal navigation menu does.
	Viewport gfx.Rect
	// Content optionally fixes the panel geometry. Zero Width and Height select
	// the size the links need; zero X and Y place the panel under the open
	// trigger.
	Content gfx.Rect

	// Pointer receives every pointer event aimed at the control itself.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives keys pressed while the control has the focus. The wrapper
	// moves Focus and updates Open first; the callback's Handled and Repaint are
	// merged.
	Key func(gfx.KeyEvent) gfx.EventResult

	entries []entryDecl
	links   []linkDecl

	currentEntry int
	currentLink  int
	lastTrigger  int

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool

	triggerModels []components.NavigationMenuTrigger
	entryModel    []int

	clicked  gfx.ID
	hadClick bool
}

type entryKind uint8

const (
	entryTrigger entryKind = iota
	entryLink
)

type entryDecl struct {
	Kind    entryKind
	Trigger components.NavigationMenuTrigger
	Link    components.NavigationMenuLink
}

type linkDecl struct {
	Entry     int
	Separator bool
	Link      components.NavigationMenuLink
}

const (
	// The panel hangs this far below the bar.
	panelOffset = float32(8)
	// The width the component uses when the caller names none.
	panelWidth = float32(420)
	// A content link's row heights, as the component lays them out.
	linkHeight        = float32(56)
	linkHeightShort   = float32(36)
	linkGap           = float32(4)
	panelPadding      = float32(8)
	separatorHeight   = float32(9)
	triggerLabelInset = float32(48)
	linkLabelInset    = float32(32)
)

// Reset forgets the last paint's declarations. Call it before declaring the
// entries again; the backing slices stay, so the next paint reuses them.
func (m *State) Reset() {
	m.entries = m.entries[:0]
	m.links = m.links[:0]
	m.currentEntry, m.currentLink, m.lastTrigger = -1, -1, -1
}

// Trigger declares one top-level disclosure. The content links declared after
// it belong to its panel until the next Trigger.
func (m *State) Trigger(id gfx.ID, label string) *State {
	m.entries = append(m.entries, entryDecl{Kind: entryTrigger, Trigger: components.NavigationMenuTrigger{ID: id, Label: label}})
	m.currentEntry, m.currentLink = len(m.entries)-1, -1
	m.lastTrigger = m.currentEntry
	return m
}

// Link declares one top-level navigation link.
func (m *State) Link(id gfx.ID, label string) *State {
	m.entries = append(m.entries, entryDecl{Kind: entryLink, Link: components.NavigationMenuLink{ID: id, Label: label}})
	m.currentEntry, m.currentLink = len(m.entries)-1, -1
	return m
}

// ContentLink declares one described destination in the current trigger's
// panel.
func (m *State) ContentLink(id gfx.ID, label string) *State {
	if m.lastTrigger < 0 {
		return m
	}
	m.links = append(m.links, linkDecl{
		Entry: m.lastTrigger,
		Link:  components.NavigationMenuLink{ID: id, Label: label},
	})
	m.currentEntry, m.currentLink = -1, len(m.links)-1
	return m
}

// Separator declares a divider between the current trigger's content links.
func (m *State) Separator() *State {
	if m.lastTrigger < 0 {
		return m
	}
	m.links = append(m.links, linkDecl{Entry: m.lastTrigger, Separator: true})
	m.currentLink = -1
	return m
}

// Width fixes the current trigger or link width. Zero lets the component
// measure it.
func (m *State) Width(width float32) *State {
	if m.currentLink >= 0 && m.currentLink < len(m.links) {
		m.links[m.currentLink].Link.Width = width
		return m
	}
	if m.currentEntry < 0 || m.currentEntry >= len(m.entries) {
		return m
	}
	entry := &m.entries[m.currentEntry]
	if entry.Kind == entryTrigger {
		entry.Trigger.Width = width
	} else {
		entry.Link.Width = width
	}
	return m
}

// Hint sets the current entry's accessible hint.
func (m *State) Hint(hint string) *State {
	if m.currentLink >= 0 && m.currentLink < len(m.links) {
		m.links[m.currentLink].Link.Hint = hint
		return m
	}
	if m.currentEntry < 0 || m.currentEntry >= len(m.entries) {
		return m
	}
	entry := &m.entries[m.currentEntry]
	if entry.Kind == entryTrigger {
		entry.Trigger.Hint = hint
	} else {
		entry.Link.Hint = hint
	}
	return m
}

// Disabled disables the current entry or content link.
func (m *State) Disabled() *State {
	if m.currentLink >= 0 && m.currentLink < len(m.links) {
		m.links[m.currentLink].Link.Disabled = true
		return m
	}
	if m.currentEntry < 0 || m.currentEntry >= len(m.entries) {
		return m
	}
	entry := &m.entries[m.currentEntry]
	if entry.Kind == entryTrigger {
		entry.Trigger.Disabled = true
	} else {
		entry.Link.Disabled = true
	}
	return m
}

// Value sets the current link's navigation value.
func (m *State) Value(value string) *State {
	if m.currentLink >= 0 && m.currentLink < len(m.links) {
		m.links[m.currentLink].Link.Value = value
		return m
	}
	if m.currentEntry >= 0 && m.currentEntry < len(m.entries) && m.entries[m.currentEntry].Kind == entryLink {
		m.entries[m.currentEntry].Link.Value = value
	}
	return m
}

// Description sets the current content link's second line.
func (m *State) Description(description string) *State {
	if m.currentLink >= 0 && m.currentLink < len(m.links) {
		m.links[m.currentLink].Link.Description = description
	}
	return m
}

// Active marks the current trigger or link as the active destination.
func (m *State) Active(active bool) *State {
	if m.currentLink >= 0 && m.currentLink < len(m.links) {
		m.links[m.currentLink].Link.Active = active
		return m
	}
	if m.currentEntry < 0 || m.currentEntry >= len(m.entries) {
		return m
	}
	entry := &m.entries[m.currentEntry]
	if entry.Kind == entryTrigger {
		entry.Trigger.Active = active
	} else {
		entry.Link.Active = active
	}
	return m
}

// View declares the navigation menu at c. label names the control for
// assistive technology. Give it the bar's size; the panel is painted above it.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, menu *State) ui.Box {
	if menu == nil {
		panic("navigationmenu: View needs its State")
	}
	menu.prepare()
	return ui.Control(c, label, &menu.handlers).Draw(menu.drawFn)
}

// View declares the navigation menu at c, for a page that keeps the
// State in its model.
func (m *State) View(c ui.C, label string) ui.Box { return View(c, label, m) }

// Clicked reports whether the paint just made was an event pass delivering a
// click on id, one of the caller's trigger or link identities. It is false in
// ordinary paints. Ask it after the paint.
func (m *State) Clicked(id gfx.ID) bool {
	return m.hadClick && !id.IsZero() && m.clicked == id
}

func (m *State) prepare() {
	if m.prepared {
		return
	}
	m.prepared = true
	m.handlers.Pointer = m.pointer
	m.handlers.Key = m.key
	m.drawFn = m.draw
}

func (m *State) direction() components.Direction {
	if m.Direction == components.RTL {
		return components.RTL
	}
	return components.LTR
}

func (m *State) layer() uint16 {
	if m.Layer == 0 {
		return 1
	}
	return m.Layer
}

func (m *State) draw(d *ui.DrawContext) {
	raw, had := d.Canvas.ClickTarget()
	theme := *d.Theme
	canvas := d.Canvas

	m.assignTriggerModels()
	open := m.triggerEntry(m.Open)

	options := components.NavigationMenuOptions{
		Open:          open >= 0,
		DirectContent: m.DirectContent,
		Content:       m.contentBounds(canvas, theme, d.Rect, open),
		Layer:         m.layer(),
		Scope:         m.Scope, DismissID: m.DismissID, Viewport: m.Viewport,
	}
	menu := components.BeginNavigationMenu(canvas, theme, d.Rect, options)
	for index := range m.entries {
		entry := &m.entries[index]
		if entry.Kind == entryTrigger {
			trigger := entry.Trigger
			trigger.Open = m.Open == trigger.ID
			menu.AddTrigger(trigger)
			continue
		}
		menu.AddLink(entry.Link)
	}
	if open >= 0 {
		content := menu.BeginContent()
		for index := range m.links {
			link := &m.links[index]
			if link.Entry != open {
				continue
			}
			if link.Separator {
				content.AddSeparator()
				continue
			}
			content.AddLink(link.Link)
		}
	}

	m.clicked, m.hadClick = raw, had
	if had {
		m.choose()
	}
}

func (m *State) assignTriggerModels() {
	m.triggerModels = m.triggerModels[:0]
	if cap(m.entryModel) < len(m.entries) {
		m.entryModel = make([]int, len(m.entries))
	} else {
		m.entryModel = m.entryModel[:len(m.entries)]
	}
	for index := range m.entryModel {
		m.entryModel[index] = -1
	}
	for index := range m.entries {
		entry := &m.entries[index]
		if entry.Kind != entryTrigger {
			continue
		}
		m.entryModel[index] = len(m.triggerModels)
		m.triggerModels = append(m.triggerModels, entry.Trigger)
	}
}

func (m *State) contentBounds(canvas *gfx.Canvas, theme gfx.Theme, bar gfx.Rect, open int) gfx.Rect {
	x, y := m.Content.X, m.Content.Y
	if x == 0 && open >= 0 {
		x = m.anchorX(canvas, theme, bar, open)
	}
	if x == 0 {
		x = bar.X
	}
	if y == 0 {
		y = bar.Y + bar.Height + panelOffset
	}
	width, height := m.Content.Width, m.Content.Height
	if width <= 0 {
		width = panelWidth
	}
	if height <= 0 {
		height = 2 * panelPadding
		if open >= 0 {
			for index := range m.links {
				link := &m.links[index]
				if link.Entry != open {
					continue
				}
				if link.Separator {
					height += separatorHeight
					continue
				}
				if link.Link.Description == "" {
					height += linkHeightShort + linkGap
				} else {
					height += linkHeight + linkGap
				}
			}
		}
	}
	return gfx.Rect{X: x, Y: y, Width: width, Height: height}
}

// anchorX simulates the component's horizontal layout up to the open trigger.
// The component has no NextTriggerBounds, so the same label measurement is
// repeated here to place the panel under its trigger.
func (m *State) anchorX(canvas *gfx.Canvas, theme gfx.Theme, bar gfx.Rect, open int) float32 {
	x := bar.X
	for index := range m.entries {
		entry := &m.entries[index]
		width := entryWidth(canvas, theme, entry)
		if width <= 0 {
			continue
		}
		if index == open {
			return x
		}
		if x+width > bar.X+bar.Width {
			break
		}
		x += width
	}
	return bar.X
}

func entryWidth(canvas *gfx.Canvas, theme gfx.Theme, entry *entryDecl) float32 {
	if entry.Kind == entryTrigger {
		if entry.Trigger.Width > 0 {
			return entry.Trigger.Width
		}
		return canvas.TextWidthWeight(entry.Trigger.Label, theme.Metrics.FontSizeM, gfx.WeightMedium) + triggerLabelInset
	}
	if entry.Link.Width > 0 {
		return entry.Link.Width
	}
	return canvas.TextWidthWeight(entry.Link.Label, theme.Metrics.FontSizeM, gfx.WeightMedium) + linkLabelInset
}

func (m *State) triggerEntry(id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for index := range m.entries {
		entry := &m.entries[index]
		if entry.Kind == entryTrigger && entry.Trigger.ID == id {
			return index
		}
	}
	return -1
}

func (m *State) entryIndex(id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for index := range m.entries {
		entry := &m.entries[index]
		if entry.Kind == entryTrigger && entry.Trigger.ID == id {
			return index
		}
		if entry.Kind == entryLink && entry.Link.ID == id {
			return index
		}
	}
	return -1
}

func (m *State) linkIndex(id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for index := range m.links {
		if !m.links[index].Separator && m.links[index].Link.ID == id {
			return index
		}
	}
	return -1
}

// choose applies the click this paint is delivering.
func (m *State) choose() {
	switch {
	case m.clicked == m.DismissID && !m.DismissID.IsZero():
		m.Open = gfx.ID{}
	default:
		if index := m.linkIndex(m.clicked); index >= 0 {
			m.Focus = m.clicked
			m.Selected = m.clicked
			if !m.DirectContent {
				m.Open = gfx.ID{}
				if entry := m.links[index].Entry; entry >= 0 && entry < len(m.entries) {
					m.Focus = m.entryID(entry)
				}
			}
			return
		}
		if index := m.entryIndex(m.clicked); index >= 0 {
			m.Focus = m.clicked
			if m.entries[index].Kind == entryLink {
				m.Selected = m.clicked
				if !m.DirectContent {
					m.Open = gfx.ID{}
				}
			} else if m.Open == m.clicked {
				m.Open = gfx.ID{}
			} else {
				m.Open = m.clicked
			}
		}
	}
}

func (m *State) pointer(event gfx.PointerEvent) gfx.EventResult {
	if m.Pointer != nil {
		return m.Pointer(event)
	}
	return gfx.EventResult{}
}

func (m *State) key(event gfx.KeyEvent) gfx.EventResult {
	var result gfx.EventResult
	if !event.Release && event.Text == "" && event.Key != gfx.KeyUnknown && !event.Key.IsModifier() {
		focus := event.Target
		if m.linkIndex(focus) < 0 && m.entryIndex(focus) < 0 {
			focus = m.Focus
		}
		if focus.IsZero() {
			focus = m.Open
		}
		if focus.IsZero() && len(m.entries) > 0 {
			focus = m.entryID(0)
		}
		if index := m.linkIndex(focus); index >= 0 {
			entry := m.links[index].Entry
			if entry >= 0 && entry < len(m.entries) && m.triggerEntry(m.Open) != entry {
				focus = m.entryID(entry)
			}
		}
		if index := m.linkIndex(focus); index >= 0 {
			result = m.contentLinkKey(event, index)
		} else if index := m.entryIndex(focus); index >= 0 {
			if m.entries[index].Kind == entryTrigger {
				result = m.triggerKey(event, index)
			} else {
				result = m.entryLinkKey(event, index)
			}
		}
	}
	if m.Key != nil {
		callback := m.Key(event)
		result.Handled = result.Handled || callback.Handled
		result.Repaint = result.Repaint || callback.Repaint
		result.Blur = result.Blur || callback.Blur
		result.ContinueTraversal = result.ContinueTraversal || callback.ContinueTraversal
		if result.Focus.IsZero() {
			result.Focus, result.FocusVisible = callback.Focus, callback.FocusVisible
		}
		if result.Activate.IsZero() {
			result.Activate = callback.Activate
		}
	}
	return result
}

func (m *State) triggerKey(event gfx.KeyEvent, entry int) gfx.EventResult {
	if entry < 0 || entry >= len(m.entries) {
		return gfx.EventResult{}
	}
	model := m.entryModel[entry]
	if model < 0 || model >= len(m.triggerModels) {
		return gfx.EventResult{}
	}
	var openPart gfx.ID
	if open := m.triggerEntry(m.Open); open >= 0 {
		openPart = m.entryID(open)
	}
	focus, nextOpen, ok := components.NavigationMenuKeyDown(m.entryID(entry), m.triggerModels, openPart, m.direction(), event.Key)
	if !ok {
		return gfx.EventResult{}
	}
	m.Open, m.Focus = nextOpen, focus
	result := gfx.EventResult{Handled: true, Repaint: true, Focus: focus, FocusVisible: true}
	switch event.Key {
	case gfx.KeyDown, gfx.KeyEnter, gfx.KeySpace:
		if opened := m.triggerEntry(m.Open); opened >= 0 {
			if link, ok := m.firstContentLink(opened); ok {
				m.Focus = link
				result.Focus = link
			}
		}
	}
	return result
}

func (m *State) entryLinkKey(event gfx.KeyEvent, entry int) gfx.EventResult {
	if entry < 0 || entry >= len(m.entries) {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyLeft, gfx.KeyRight, gfx.KeyHome, gfx.KeyEnd:
		next, ok := m.moveEntry(entry, event.Key)
		if !ok {
			return gfx.EventResult{Handled: true}
		}
		m.Focus = m.entryID(next)
		return gfx.EventResult{Handled: true, Repaint: true, Focus: m.Focus, FocusVisible: true}
	case gfx.KeyEnter, gfx.KeySpace:
		link := &m.entries[entry].Link
		if link.Disabled || link.ID.IsZero() {
			return gfx.EventResult{Handled: true}
		}
		m.Focus, m.Selected = link.ID, link.ID
		return gfx.EventResult{Handled: true, Repaint: true, Activate: link.ID}
	case gfx.KeyDown:
		if open := m.triggerEntry(m.Open); open >= 0 {
			if link, ok := m.firstContentLink(open); ok {
				m.Focus = link
				return gfx.EventResult{Handled: true, Repaint: true, Focus: link, FocusVisible: true}
			}
		}
		return gfx.EventResult{}
	case gfx.KeyEscape:
		if open := m.triggerEntry(m.Open); open >= 0 {
			m.Open = gfx.ID{}
			m.Focus = m.entryID(open)
			return gfx.EventResult{Handled: true, Repaint: true, Focus: m.Focus, FocusVisible: true}
		}
		return gfx.EventResult{}
	case gfx.KeyTab:
		if !m.Open.IsZero() {
			m.Open = gfx.ID{}
			return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
		}
	}
	return gfx.EventResult{}
}

func (m *State) contentLinkKey(event gfx.KeyEvent, link int) gfx.EventResult {
	if link < 0 || link >= len(m.links) || m.links[link].Separator {
		return gfx.EventResult{}
	}
	row := &m.links[link]
	switch event.Key {
	case gfx.KeyUp, gfx.KeyDown, gfx.KeyHome, gfx.KeyEnd:
		next, ok := m.moveContentLink(link, event.Key)
		if !ok {
			return gfx.EventResult{Handled: true}
		}
		m.Focus = m.links[next].Link.ID
		return gfx.EventResult{Handled: true, Repaint: true, Focus: m.Focus, FocusVisible: true}
	case gfx.KeyEnter, gfx.KeySpace:
		if row.Link.Disabled || row.Link.ID.IsZero() {
			return gfx.EventResult{Handled: true}
		}
		m.Focus, m.Selected = row.Link.ID, row.Link.ID
		return gfx.EventResult{Handled: true, Repaint: true, Activate: row.Link.ID}
	case gfx.KeyEscape:
		entry := row.Entry
		m.Open = gfx.ID{}
		if entry < 0 || entry >= len(m.entries) {
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		m.Focus = m.entryID(entry)
		return gfx.EventResult{Handled: true, Repaint: true, Focus: m.Focus, FocusVisible: true}
	case gfx.KeyLeft, gfx.KeyRight:
		entry := row.Entry
		if entry < 0 || entry >= len(m.entries) {
			return gfx.EventResult{}
		}
		model := m.entryModel[entry]
		if model < 0 || model >= len(m.triggerModels) {
			return gfx.EventResult{}
		}
		focus, nextOpen, ok := components.NavigationMenuKeyDown(m.entryID(entry), m.triggerModels, m.entryID(entry), m.direction(), event.Key)
		if !ok {
			return gfx.EventResult{}
		}
		m.Open, m.Focus = nextOpen, focus
		return gfx.EventResult{Handled: true, Repaint: true, Focus: focus, FocusVisible: true}
	case gfx.KeyTab:
		m.Open = gfx.ID{}
		return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
	}
	return gfx.EventResult{}
}

func (m *State) entryID(index int) gfx.ID {
	if index < 0 || index >= len(m.entries) {
		return gfx.ID{}
	}
	entry := &m.entries[index]
	if entry.Kind == entryTrigger {
		return entry.Trigger.ID
	}
	return entry.Link.ID
}

func (m *State) moveEntry(from int, key gfx.Key) (int, bool) {
	if key == gfx.KeyHome || key == gfx.KeyEnd {
		step, index := 1, 0
		if key == gfx.KeyEnd {
			step, index = -1, len(m.entries)-1
		}
		for at := index; at >= 0 && at < len(m.entries); at += step {
			if at != from && m.usableEntry(at) {
				return at, true
			}
		}
		return 0, false
	}
	step := 1
	if key == gfx.KeyLeft {
		step = -1
	}
	for offset := 1; offset <= len(m.entries); offset++ {
		at := from + step*offset
		for at < 0 {
			at += len(m.entries)
		}
		for at >= len(m.entries) {
			at -= len(m.entries)
		}
		if m.usableEntry(at) {
			return at, true
		}
	}
	return 0, false
}

func (m *State) usableEntry(index int) bool {
	if index < 0 || index >= len(m.entries) {
		return false
	}
	entry := &m.entries[index]
	if entry.Kind == entryTrigger {
		return !entry.Trigger.Disabled && !entry.Trigger.ID.IsZero()
	}
	return !entry.Link.Disabled && !entry.Link.ID.IsZero()
}

func (m *State) moveContentLink(from int, key gfx.Key) (int, bool) {
	entry := m.links[from].Entry
	if key == gfx.KeyHome || key == gfx.KeyEnd {
		step, index := 1, 0
		if key == gfx.KeyEnd {
			step, index = -1, len(m.links)-1
		}
		for at := index; at >= 0 && at < len(m.links); at += step {
			if at != from && m.usableContentLink(at, entry) {
				return at, true
			}
		}
		return 0, false
	}
	step := 1
	if key == gfx.KeyUp {
		step = -1
	}
	for offset := 1; offset <= len(m.links); offset++ {
		at := from + step*offset
		for at < 0 {
			at += len(m.links)
		}
		for at >= len(m.links) {
			at -= len(m.links)
		}
		if m.usableContentLink(at, entry) {
			return at, true
		}
	}
	return 0, false
}

func (m *State) usableContentLink(index, entry int) bool {
	link := &m.links[index]
	return link.Entry == entry && !link.Separator && !link.Link.Disabled && !link.Link.ID.IsZero()
}

func (m *State) firstContentLink(entry int) (gfx.ID, bool) {
	for index := range m.links {
		if m.usableContentLink(index, entry) {
			return m.links[index].Link.ID, true
		}
	}
	return gfx.ID{}, false
}
