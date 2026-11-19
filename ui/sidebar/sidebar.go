// Package sidebar is the opt-in page control for the application sidebar.
// It wraps components.BeginSidebarWithOptions in a ui.Control, so a page
// declares a header, sections of items and a footer without calling the
// component itself:
//
//	m.nav.Reset().Title("Viva Studio").Fixed().
//		Group("Workspace").
//		Item(m.overviewID, "Overview").Icon("O").Badge("12").
//			Active(m.section == "overview").
//		Item(m.componentsID, "Components").Icon("C").
//			Active(m.section == "components").
//		Separator().
//		Account(m.accountID, "Skyler").Icon("S")
//	sidebar.View(c, "Application navigation", &m.nav).Width(240).FillHeight()
//
// The caller owns the selection, the collapsed/open state and the callbacks.
// Active marks the current destination, Collapsed asks for the icon-only rail,
// and OnSelect is told the caller's identity when one of the declared items is
// clicked. Selected and Clicked report the last paint's click for a page that
// would rather read it where the sidebar is declared.
//
// Declarations are rebuilt every paint: call Reset, declare the entries in
// order, then View. The slices keep their capacity, so a warm paint allocates
// nothing. Keep a State in the model; its handlers and draw function are made
// once. Do not copy one after View has painted it.
//
// It is a separate package so an application that does not import it pays
// nothing for it. It depends only on the public components, gfx and ui
// packages.
package sidebar

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Side is the desktop edge carrying the sidebar's separator.
type Side = components.SidebarSide

const (
	Left  Side = components.SidebarLeft
	Right Side = components.SidebarRight
)

type entryKind uint8

const (
	entryGroup entryKind = iota
	entryItem
	entrySeparator
)

// sidebarEntry is one declared group, item or separator. The slice is reused
// between paints, so a declaration that resets and rebuilds the same entries
// does not allocate.
type sidebarEntry struct {
	kind     entryKind
	label    string
	item     components.SidebarMenuItem
	sub      []components.SidebarSubItem
	subCount int
}

// State is one sidebar's declarations and its page control. Configure it
// with the chainable methods after Reset and before View; the caller's model
// owns the values the methods are given.
type State struct {
	title       string
	side        Side
	variant     components.SidebarVariant
	collapsible components.SidebarCollapsible
	collapsed   bool

	entries     []sidebarEntry
	entryCount  int
	footers     []components.SidebarMenuItem
	footerCount int

	currentEntry  int
	currentSub    int
	currentFooter int

	// OnSelect is told the caller's identity when one of the declared items,
	// sub items or footer items is clicked. It runs in the click's event pass,
	// where changing the model is allowed. It is optional.
	OnSelect func(gfx.ID)

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool

	clicked  gfx.ID
	hadClick bool
	selected gfx.ID
}

// New returns a sidebar with the component's fixed behavior and no
// declarations. The zero value also paints; choose Fixed, IconCollapse or
// Offcanvas before asking it to collapse.
func New() *State { return &State{collapsible: components.SidebarFixed} }

// Reset forgets the declared groups, items, sub items and footers and returns
// the sidebar to the start of a new declaration. The title, surface, collapse
// configuration, callback and storage are kept.
func (s *State) Reset() *State {
	for index := 0; index < s.entryCount; index++ {
		entry := &s.entries[index]
		entry.kind = entryGroup
		entry.label = ""
		entry.item = components.SidebarMenuItem{}
		entry.sub = entry.sub[:0]
		entry.subCount = 0
	}
	for index := 0; index < s.footerCount; index++ {
		s.footers[index] = components.SidebarMenuItem{}
	}
	s.entryCount, s.footerCount = 0, 0
	s.currentEntry, s.currentSub, s.currentFooter = -1, -1, -1
	return s
}

// Title is the product or workspace header. An empty title paints none.
func (s *State) Title(title string) *State { s.title = title; return s }

// Side selects the desktop edge carrying the separator.
func (s *State) Side(side Side) *State { s.side = side; return s }

// Standard paints the plain surface. It is the zero value's surface.
func (s *State) Standard() *State { s.variant = components.SidebarStandard; return s }

// Floating paints the rounded, ringed card inside a small margin.
func (s *State) Floating() *State { s.variant = components.SidebarFloating; return s }

// Inset paints the plain surface around a page card; paint the page side with
// components.PaintSidebarInset.
func (s *State) Inset() *State { s.variant = components.SidebarInset; return s }

// Fixed keeps the sidebar in place when it collapses. It is the default.
func (s *State) Fixed() *State { s.collapsible = components.SidebarFixed; return s }

// IconCollapse collapses to an icon rail; labels and groups are suppressed.
func (s *State) IconCollapse() *State {
	s.collapsible = components.SidebarIcon
	return s
}

// Offcanvas collapses to no commands at all.
func (s *State) Offcanvas() *State {
	s.collapsible = components.SidebarOffcanvas
	return s
}

// Collapsed asks for the collapsed state. Callers own it, so a toggle is the
// caller's own control. It only has an effect with IconCollapse or Offcanvas.
func (s *State) Collapsed(collapsed bool) *State {
	s.collapsed = collapsed
	return s
}

// Group declares a section caption. Items declared after it belong to it until
// the next Group.
func (s *State) Group(label string) *State {
	entry := s.appendEntry(entryGroup)
	entry.label = label
	return s
}

// Item declares one navigation destination with the caller's identity. A zero
// ID paints an inert item. Configure it with Icon, VectorIcon, Badge, Active,
// Disabled and Outline; declare an open sub menu with Sub.
func (s *State) Item(id gfx.ID, label string) *State {
	entry := s.appendEntry(entryItem)
	entry.item.ID, entry.item.Label = id, label
	return s
}

// Separator declares a divider between content items.
func (s *State) Separator() *State {
	s.appendEntry(entrySeparator)
	return s
}

// Sub declares one item of the current item's sub menu. Declare sub items only
// while the section is open; that open state is the caller's.
func (s *State) Sub(id gfx.ID, label string) *State {
	if s.currentEntry < 0 || s.currentEntry >= s.entryCount {
		return s
	}
	entry := &s.entries[s.currentEntry]
	if entry.kind != entryItem {
		return s
	}
	if entry.subCount < len(entry.sub) {
		entry.sub[entry.subCount] = components.SidebarSubItem{ID: id, Label: label}
	} else {
		entry.sub = append(entry.sub, components.SidebarSubItem{ID: id, Label: label})
	}
	s.currentSub = entry.subCount
	entry.subCount++
	return s
}

// Footer declares a menu item against the bottom edge, above footers declared
// before it.
func (s *State) Footer(id gfx.ID, label string) *State {
	item := s.appendFooter()
	item.ID, item.Label = id, label
	return s
}

// Account declares the signed-in account as a footer item. It is Footer for a
// row whose identity and label are the account's.
func (s *State) Account(id gfx.ID, label string) *State {
	item := s.appendFooter()
	item.ID, item.Label = id, label
	return s
}

// Icon sets the current item's text glyph.
func (s *State) Icon(icon string) *State {
	if item := s.currentItem(); item != nil {
		item.Icon = icon
	}
	return s
}

// VectorIcon sets the current item's vector icon, which paints instead of the
// text glyph.
func (s *State) VectorIcon(icon gfx.Icon) *State {
	if item := s.currentItem(); item != nil {
		item.VectorIcon = icon
	}
	return s
}

// Badge sets the current item's trailing count or short label.
func (s *State) Badge(badge string) *State {
	if item := s.currentItem(); item != nil {
		item.Trailing = badge
	}
	return s
}

// Active marks the current item as the selected destination. The caller owns
// the selection, so this is set from the model on every paint.
func (s *State) Active(active bool) *State {
	if item := s.currentItem(); item != nil {
		item.Active = active
	}
	return s
}

// Disabled paints the current item inert.
func (s *State) Disabled(disabled bool) *State {
	if item := s.currentItem(); item != nil {
		item.Disabled = disabled
	}
	return s
}

// Outline paints the current item with the outlined menu-button recipe.
func (s *State) Outline() *State {
	if item := s.currentItem(); item != nil {
		item.Variant = components.SidebarMenuOutline
	}
	return s
}

// SubActive marks the current sub item as the selected destination.
func (s *State) SubActive(active bool) *State {
	if sub := s.currentSubItem(); sub != nil {
		sub.Active = active
	}
	return s
}

// SubDisabled paints the current sub item inert.
func (s *State) SubDisabled(disabled bool) *State {
	if sub := s.currentSubItem(); sub != nil {
		sub.Disabled = disabled
	}
	return s
}

// View declares the sidebar at c. label names it for assistive technology. Give
// it a width and height; the component fills the box.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, sidebar *State) ui.Box {
	if sidebar == nil {
		panic("sidebar: View needs its State")
	}
	sidebar.prepare()
	return ui.Control(c, label, &sidebar.handlers).Draw(sidebar.drawFn)
}

// View declares the sidebar at c, for a page that keeps the State in its
// model.
func (s *State) View(c ui.C, label string) ui.Box { return View(c, label, s) }

// Selected is the identity clicked in the paint just made, or zero. It is not
// retained across paints: read it after View, where the click is handled.
func (s *State) Selected() gfx.ID { return s.selected }

// Clicked reports whether the paint just made delivered a click on id. It is
// false in ordinary paints. Ask it after View.
func (s *State) Clicked(id gfx.ID) bool {
	return s.hadClick && !id.IsZero() && s.clicked == id
}

func (s *State) prepare() {
	if s.prepared {
		return
	}
	s.prepared = true
	s.handlers.Role = gfx.RoleGroup
	s.drawFn = s.draw
}

func (s *State) draw(d *ui.DrawContext) {
	s.clicked, s.hadClick = d.Canvas.ClickTarget()
	s.selected = gfx.ID{}
	theme := *d.Theme
	options := components.SidebarOptions{
		Side: s.side, Variant: s.variant, Collapsible: s.collapsible, Collapsed: s.collapsed,
	}
	nav := components.BeginSidebarWithOptions(d.Canvas, theme, d.Rect, s.title, options)
	for index := 0; index < s.footerCount; index++ {
		nav.AddFooterMenuItem(s.footers[index])
	}
	for index := 0; index < s.entryCount; index++ {
		entry := &s.entries[index]
		switch entry.kind {
		case entryGroup:
			nav.AddGroup(entry.label)
		case entrySeparator:
			nav.AddSeparator()
		case entryItem:
			nav.AddMenuItem(entry.item)
			if entry.subCount > 0 {
				nav.AddSubMenu(entry.sub[:entry.subCount])
			}
		}
	}
	if s.hadClick && s.owns(s.clicked) {
		s.selected = s.clicked
		if s.OnSelect != nil {
			s.OnSelect(s.clicked)
		}
	}
}

// appendEntry starts one content declaration, reusing its storage.
func (s *State) appendEntry(kind entryKind) *sidebarEntry {
	if s.entryCount == len(s.entries) {
		s.entries = append(s.entries, sidebarEntry{})
	}
	entry := &s.entries[s.entryCount]
	entry.kind = kind
	entry.label = ""
	entry.item = components.SidebarMenuItem{}
	entry.sub = entry.sub[:0]
	entry.subCount = 0
	s.currentEntry = s.entryCount
	s.currentSub = -1
	s.currentFooter = -1
	s.entryCount++
	return entry
}

// appendFooter starts one footer declaration, reusing its storage.
func (s *State) appendFooter() *components.SidebarMenuItem {
	if s.footerCount == len(s.footers) {
		s.footers = append(s.footers, components.SidebarMenuItem{})
	}
	item := &s.footers[s.footerCount]
	*item = components.SidebarMenuItem{}
	s.footerCount++
	s.currentEntry = -1
	s.currentSub = -1
	s.currentFooter = s.footerCount - 1
	return item
}

// currentItem is the item or footer the value methods configure.
func (s *State) currentItem() *components.SidebarMenuItem {
	if s.currentFooter >= 0 && s.currentFooter < s.footerCount {
		return &s.footers[s.currentFooter]
	}
	if s.currentEntry < 0 || s.currentEntry >= s.entryCount {
		return nil
	}
	entry := &s.entries[s.currentEntry]
	if entry.kind != entryItem {
		return nil
	}
	return &entry.item
}

// currentSubItem is the sub item the sub value methods configure.
func (s *State) currentSubItem() *components.SidebarSubItem {
	if s.currentSub < 0 || s.currentEntry < 0 || s.currentEntry >= s.entryCount {
		return nil
	}
	entry := &s.entries[s.currentEntry]
	if entry.kind != entryItem || s.currentSub >= entry.subCount {
		return nil
	}
	return &entry.sub[s.currentSub]
}

// owns reports whether id names one of the declared items.
func (s *State) owns(id gfx.ID) bool {
	if id.IsZero() {
		return false
	}
	for index := 0; index < s.entryCount; index++ {
		entry := &s.entries[index]
		if entry.kind != entryItem {
			continue
		}
		if entry.item.ID == id {
			return true
		}
		for sub := 0; sub < entry.subCount; sub++ {
			if entry.sub[sub].ID == id {
				return true
			}
		}
	}
	for index := 0; index < s.footerCount; index++ {
		if s.footers[index].ID == id {
			return true
		}
	}
	return false
}
