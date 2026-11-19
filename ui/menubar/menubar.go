// Package menubar is the opt-in page control for the persistent menu bar
// of package components. It wraps components.BeginMenubar and
// components.BeginMenubarContent in a ui.Control, so a page declares the bar,
// its triggers and their rows without calling the components themselves:
//
//	m.menubar.Reset()
//	m.menubar.Trigger(fileID, "File").
//		Item(newID, "New window").Shortcut("⌘N").
//		Item(gridID, "Show grid").Checkbox(m.grid).
//		Separator().
//		Item(deleteID, "Delete workspace").Destructive()
//	m.menubar.Trigger(viewID, "View").Item(reloadID, "Reload")
//	m.menubar.View(c, "Menubar").FillWidth().Height(36)
//	// After the paint, Selected names the row chosen last.
//	if m.menubar.Selected == gridID {
//		m.grid = !m.grid
//		m.menubar.Selected = gfx.ID{}
//	}
//
// The caller owns the open trigger, the focused trigger or row, the selected
// row and every label, description and identity. The wrapper paints through
// the components with exactly the identities the caller supplied, changes Open
// when a trigger is clicked, and reports the clicked or chosen identity with
// Clicked and Selected. Pointer and key events aimed at the control itself reach
// the optional callbacks; clicks on triggers and rows are read with Clicked
// after the paint, as for the component.
//
// Declarations are rebuilt every paint: call Reset, declare the triggers and
// rows in order, then View. The slices keep their capacity, so a warm paint
// allocates nothing. Keep a Menubar in the model beside its state; its handlers
// and draw function are made once. Do not copy a Menubar after View has painted
// it.
//
// It is a separate package so an application that does not import it pays
// nothing for it.
package menubar

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Direction is components.Direction, named here so a page that declares a bar
// reads through this package alone.
type Direction = components.Direction

// Menubar is one menubar's caller-owned state, its declarations and its page
// control. Fill Open and the callbacks from the model; declare the triggers and
// rows every paint; then call View.
type Menubar struct {
	// Open is the trigger whose menu is open, or zero when the bar is closed.
	// A trigger click toggles it.
	Open gfx.ID
	// Focus is the trigger or row the keyboard acts on. Clicking a trigger or
	// a row sets it; the arrow keys move it.
	Focus gfx.ID
	// Selected is the row chosen last by a click or by Enter, or zero before a
	// row is chosen. The wrapper writes it; the caller consumes and clears it.
	Selected gfx.ID
	// Direction selects the arrow movement. The zero value is left to right.
	Direction components.Direction
	// Layer is the popup's layer. Zero uses layer 1.
	Layer uint16
	// Scope and DismissID are passed to the popup: Scope traps focus and
	// DismissID is the identity an Escape or an outside press reports.
	Scope, DismissID gfx.ID
	// Viewport, with DismissID, makes a press outside the popup report DismissID
	// and pass through, as a menubar does.
	Viewport gfx.Rect
	// Content optionally fixes the popup geometry. Zero Width and Height select
	// the size the rows need; zero X and Y place the popup under the open
	// trigger. A nonzero width or height is used as given.
	Content gfx.Rect

	// Pointer receives every pointer event aimed at the control itself.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives the keys pressed while the control has the focus. The
	// wrapper moves Focus and updates Open first; the callback's Handled and
	// Repaint are merged.
	Key func(gfx.KeyEvent) gfx.EventResult

	triggers []triggerDecl
	items    []itemDecl

	currentTrigger int
	currentItem    int

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool

	triggerModels []components.MenubarTrigger
	triggerBounds []gfx.Rect

	clicked  gfx.ID
	hadClick bool
}

type triggerDecl struct {
	ID       gfx.ID
	Label    string
	Hint     string
	Disabled bool
}

type itemDecl struct {
	Trigger   int
	Separator bool
	Item      components.MenubarItem
}

const (
	// The popup hangs this far below the bar, as the component's own examples do.
	popupOffset = float32(8)
	// The menubar's minimum popup width, in the component.
	minPopupWidth = float32(144)
)

// Reset forgets the last paint's declarations. Call it before declaring the
// triggers and rows again; the backing slices stay, so the next paint reuses
// them.
func (m *Menubar) Reset() {
	m.triggers = m.triggers[:0]
	m.items = m.items[:0]
	m.currentTrigger, m.currentItem = -1, -1
}

// Trigger declares one top-level trigger. The rows declared after it belong to
// it until the next Trigger.
func (m *Menubar) Trigger(id gfx.ID, label string) *Menubar {
	m.triggers = append(m.triggers, triggerDecl{ID: id, Label: label})
	m.currentTrigger, m.currentItem = len(m.triggers)-1, -1
	return m
}

// Item declares one row of the current trigger. Rows are painted only while
// their trigger's menu is open.
func (m *Menubar) Item(id gfx.ID, label string) *Menubar {
	if m.currentTrigger < 0 {
		return m
	}
	m.items = append(m.items, itemDecl{
		Trigger: m.currentTrigger,
		Item:    components.MenubarItem{ID: id, Label: label},
	})
	m.currentItem = len(m.items) - 1
	return m
}

// Separator declares a divider between the current trigger's rows.
func (m *Menubar) Separator() *Menubar {
	if m.currentTrigger < 0 {
		return m
	}
	m.items = append(m.items, itemDecl{Trigger: m.currentTrigger, Separator: true})
	m.currentItem = -1
	return m
}

// Hint sets the accessible hint of the current row, or of the current trigger
// when no row has been declared since it.
func (m *Menubar) Hint(hint string) *Menubar {
	if m.currentItem >= 0 {
		m.items[m.currentItem].Item.Hint = hint
		return m
	}
	if m.currentTrigger >= 0 && m.currentTrigger < len(m.triggers) {
		m.triggers[m.currentTrigger].Hint = hint
	}
	return m
}

// Disabled disables the current row, or the current trigger when no row has
// been declared since it.
func (m *Menubar) Disabled() *Menubar {
	if m.currentItem >= 0 {
		m.items[m.currentItem].Item.Disabled = true
		return m
	}
	if m.currentTrigger >= 0 && m.currentTrigger < len(m.triggers) {
		m.triggers[m.currentTrigger].Disabled = true
	}
	return m
}

// Shortcut sets the current row's trailing shortcut text.
func (m *Menubar) Shortcut(shortcut string) *Menubar {
	if m.currentItem >= 0 {
		m.items[m.currentItem].Item.Shortcut = shortcut
	}
	return m
}

// Keys sets the current row's trailing keycaps, replacing Shortcut.
func (m *Menubar) Keys(keys ...string) *Menubar {
	if m.currentItem >= 0 {
		m.items[m.currentItem].Item.Keys = keys
	}
	return m
}

// Value sets the current row's accessible value.
func (m *Menubar) Value(value string) *Menubar {
	if m.currentItem >= 0 {
		m.items[m.currentItem].Item.Value = value
	}
	return m
}

// Checkbox makes the current row a checkbox with the given checked state.
func (m *Menubar) Checkbox(checked bool) *Menubar {
	if m.currentItem >= 0 {
		item := &m.items[m.currentItem].Item
		item.Kind, item.Checked = components.MenubarCheckbox, checked
	}
	return m
}

// Radio makes the current row a radio option with the given selected state.
func (m *Menubar) Radio(selected bool) *Menubar {
	if m.currentItem >= 0 {
		item := &m.items[m.currentItem].Item
		item.Kind, item.Selected = components.MenubarRadio, selected
	}
	return m
}

// Submenu makes the current row a submenu trigger with the given expanded
// state.
func (m *Menubar) Submenu(expanded bool) *Menubar {
	if m.currentItem >= 0 {
		item := &m.items[m.currentItem].Item
		item.Kind, item.Expanded = components.MenubarSubmenu, expanded
	}
	return m
}

// Destructive paints the current row with the destructive treatment.
func (m *Menubar) Destructive() *Menubar {
	if m.currentItem >= 0 {
		m.items[m.currentItem].Item.Variant = components.MenubarItemDestructive
	}
	return m
}

// Inset indents the current row's label, as a submenu row does.
func (m *Menubar) Inset() *Menubar {
	if m.currentItem >= 0 {
		m.items[m.currentItem].Item.Inset = true
	}
	return m
}

// View declares the menubar at c. label names the control for assistive
// technology. Give it the bar's size; the popup is painted above it.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, bar *Menubar) ui.Box {
	if bar == nil {
		panic("menubar: View needs its Menubar")
	}
	bar.prepare()
	return ui.Control(c, label, &bar.handlers).Draw(bar.drawFn)
}

// View declares the menubar at c, for a page that keeps the Menubar in its
// model.
func (m *Menubar) View(c ui.C, label string) ui.Box { return View(c, label, m) }

// Clicked reports whether the paint just made was an event pass delivering a
// click on id, a trigger or row identity the caller declared. It is false in
// ordinary paints. Ask it after the paint.
func (m *Menubar) Clicked(id gfx.ID) bool {
	return m.hadClick && !id.IsZero() && m.clicked == id
}

func (m *Menubar) prepare() {
	if m.prepared {
		return
	}
	m.prepared = true
	m.handlers.Pointer = m.pointer
	m.handlers.Key = m.key
	m.drawFn = m.draw
}

func (m *Menubar) direction() components.Direction {
	if m.Direction == components.RTL {
		return components.RTL
	}
	return components.LTR
}

func (m *Menubar) layer() uint16 {
	if m.Layer == 0 {
		return 1
	}
	return m.Layer
}

func (m *Menubar) draw(d *ui.DrawContext) {
	raw, had := d.Canvas.ClickTarget()
	theme := *d.Theme
	canvas := d.Canvas

	m.triggerModels = m.triggerModels[:0]
	for index := range m.triggers {
		trigger := &m.triggers[index]
		m.triggerModels = append(m.triggerModels, components.MenubarTrigger{
			ID:       trigger.ID,
			Label:    trigger.Label,
			Hint:     trigger.Hint,
			Open:     m.Open == trigger.ID,
			Disabled: trigger.Disabled,
		})
	}
	bar := components.BeginMenubar(canvas, theme, d.Rect)
	m.triggerBounds = m.triggerBounds[:0]
	for index := range m.triggerModels {
		m.triggerBounds = append(m.triggerBounds, bar.AddTrigger(m.triggerModels[index]))
	}
	if open := m.triggerIndex(m.Open); open >= 0 && open < len(m.triggerBounds) {
		m.paintContent(d, theme, open)
	}

	m.clicked, m.hadClick = raw, had
	if had {
		m.choose()
	}
}

func (m *Menubar) paintContent(d *ui.DrawContext, theme gfx.Theme, trigger int) {
	canvas := d.Canvas
	bounds := m.contentBounds(canvas, theme, d.Rect, trigger)
	options := components.MenubarOverlayOptions{
		Layer: m.layer(), Scope: m.Scope, DismissID: m.DismissID, Viewport: m.Viewport,
	}
	content := components.BeginMenubarContent(canvas, theme, bounds, options)
	for index := range m.items {
		item := &m.items[index]
		if item.Trigger != trigger {
			continue
		}
		if item.Separator {
			content.AddSeparator()
			continue
		}
		content.AddItem(item.Item)
	}
	content.End()
}

func (m *Menubar) contentBounds(canvas *gfx.Canvas, theme gfx.Theme, bar gfx.Rect, trigger int) gfx.Rect {
	width, height := m.Content.Width, m.Content.Height
	if width <= 0 || height <= 0 {
		neededWidth, neededHeight := float32(0), 2*components.DropdownMenuPadding
		for index := range m.items {
			item := &m.items[index]
			if item.Trigger != trigger {
				continue
			}
			if item.Separator {
				neededHeight += components.DropdownMenuSeparatorHeight
				continue
			}
			row := dropdownItem(item.Item)
			neededWidth = max(neededWidth, components.DropdownMenuItemWidth(canvas, theme, row))
			neededHeight += components.DropdownMenuItemHeight(theme, row)
		}
		if width <= 0 {
			width = max(neededWidth, minPopupWidth)
		}
		if height <= 0 {
			height = max(neededHeight, 2*components.DropdownMenuPadding)
		}
	}
	x, y := m.Content.X, m.Content.Y
	if x == 0 && trigger >= 0 && trigger < len(m.triggerBounds) {
		x = m.triggerBounds[trigger].X
	}
	if y == 0 {
		y = bar.Y + bar.Height + popupOffset
	}
	return gfx.Rect{X: x, Y: y, Width: width, Height: height}
}

func dropdownItem(item components.MenubarItem) components.DropdownMenuItem {
	return components.DropdownMenuItem{
		ID: item.ID, Label: item.Label, Shortcut: item.Shortcut, Keys: item.Keys, TrailingWidth: item.TrailingWidth,
		Value: item.Value, Hint: item.Hint,
		Kind:    components.DropdownMenuItemKind(item.Kind),
		Variant: components.DropdownMenuItemVariant(item.Variant),
		Checked: item.Checked, Selected: item.Selected, Disabled: item.Disabled, Inset: item.Inset,
		Expanded: item.Expanded, Icon: item.Icon,
	}
}

func (m *Menubar) triggerIndex(id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for index := range m.triggers {
		if m.triggers[index].ID == id {
			return index
		}
	}
	return -1
}

func (m *Menubar) itemIndex(id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for index := range m.items {
		if !m.items[index].Separator && m.items[index].Item.ID == id {
			return index
		}
	}
	return -1
}

// choose applies the click this paint is delivering.
func (m *Menubar) choose() {
	switch index := m.itemIndex(m.clicked); {
	case m.clicked == m.DismissID && !m.DismissID.IsZero():
		m.Open = gfx.ID{}
	case index >= 0:
		m.Focus = m.clicked
		m.Selected = m.clicked
		if m.items[index].Item.Kind == components.MenubarAction {
			m.Open = gfx.ID{}
			if trigger := m.items[index].Trigger; trigger >= 0 && trigger < len(m.triggers) {
				m.Focus = m.triggers[trigger].ID
			}
		}
	case m.triggerIndex(m.clicked) >= 0:
		m.Focus = m.clicked
		m.Open = gfx.ID(components.MenubarState(m.Open).Toggle(m.clicked))
	}
}

func (m *Menubar) pointer(event gfx.PointerEvent) gfx.EventResult {
	if m.Pointer != nil {
		return m.Pointer(event)
	}
	return gfx.EventResult{}
}

func (m *Menubar) key(event gfx.KeyEvent) gfx.EventResult {
	var result gfx.EventResult
	if !event.Release && event.Text == "" && event.Key != gfx.KeyUnknown && !event.Key.IsModifier() {
		focus := event.Target
		if m.itemIndex(focus) < 0 && m.triggerIndex(focus) < 0 {
			focus = m.Focus
		}
		if focus.IsZero() {
			focus = m.Open
		}
		if focus.IsZero() && len(m.triggers) > 0 {
			focus = m.triggers[0].ID
		}
		if index := m.itemIndex(focus); index >= 0 {
			result = m.itemKey(event, index)
		} else if index := m.triggerIndex(focus); index >= 0 {
			result = m.triggerKey(event, index)
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

func (m *Menubar) triggerKey(event gfx.KeyEvent, trigger int) gfx.EventResult {
	if trigger < 0 || trigger >= len(m.triggers) {
		return gfx.EventResult{}
	}
	var state components.MenubarState
	if open := m.triggerIndex(m.Open); open >= 0 {
		state = components.MenubarState(m.triggers[open].ID)
	}
	next, focus, ok := components.MenubarKeyDown(m.triggers[trigger].ID, m.triggerModels, state, m.direction(), event.Key)
	if !ok {
		return gfx.EventResult{}
	}
	m.Open, m.Focus = next.ActiveID(), focus
	result := gfx.EventResult{Handled: true, Repaint: true, Focus: focus, FocusVisible: true}
	switch event.Key {
	case gfx.KeyDown, gfx.KeyEnter, gfx.KeySpace:
		if open := m.triggerIndex(m.Open); open >= 0 {
			if item, ok := m.firstItem(open); ok {
				m.Focus = item
				result.Focus = item
			}
		}
	}
	return result
}

func (m *Menubar) itemKey(event gfx.KeyEvent, item int) gfx.EventResult {
	if item < 0 || item >= len(m.items) || m.items[item].Separator {
		return gfx.EventResult{}
	}
	row := &m.items[item]
	if row.Trigger < 0 || row.Trigger >= len(m.triggers) {
		return gfx.EventResult{}
	}
	if m.Open != m.triggers[row.Trigger].ID {
		return m.triggerKey(event, row.Trigger)
	}
	switch event.Key {
	case gfx.KeyUp, gfx.KeyDown, gfx.KeyHome, gfx.KeyEnd:
		next, ok := m.moveItem(item, event.Key)
		if !ok {
			return gfx.EventResult{Handled: true}
		}
		m.Focus = m.items[next].Item.ID
		return gfx.EventResult{Handled: true, Repaint: true, Focus: m.Focus, FocusVisible: true}
	case gfx.KeyEnter, gfx.KeySpace:
		if row.Item.Disabled || row.Item.ID.IsZero() {
			return gfx.EventResult{Handled: true}
		}
		m.Focus, m.Selected = row.Item.ID, row.Item.ID
		return gfx.EventResult{Handled: true, Repaint: true, Activate: row.Item.ID}
	case gfx.KeyEscape:
		m.Open = gfx.ID{}
		trigger := row.Trigger
		if trigger < 0 || trigger >= len(m.triggers) {
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		m.Focus = m.triggers[trigger].ID
		return gfx.EventResult{Handled: true, Repaint: true, Focus: m.Focus, FocusVisible: true}
	case gfx.KeyLeft, gfx.KeyRight:
		trigger := row.Trigger
		if trigger < 0 || trigger >= len(m.triggers) {
			return gfx.EventResult{}
		}
		state := components.MenubarState(m.triggers[trigger].ID)
		next, focus, ok := components.MenubarKeyDown(m.triggers[trigger].ID, m.triggerModels, state, m.direction(), event.Key)
		if !ok {
			return gfx.EventResult{}
		}
		m.Open, m.Focus = next.ActiveID(), focus
		return gfx.EventResult{Handled: true, Repaint: true, Focus: focus, FocusVisible: true}
	case gfx.KeyTab:
		m.Open = gfx.ID{}
		return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
	}
	return gfx.EventResult{}
}

// moveItem returns the next enabled row of the same menu, skipping separators
// and disabled rows, with wrapping.
func (m *Menubar) moveItem(from int, key gfx.Key) (int, bool) {
	trigger := m.items[from].Trigger
	if key == gfx.KeyHome || key == gfx.KeyEnd {
		step, index := 1, -1
		if key == gfx.KeyEnd {
			step, index = -1, len(m.items)
		}
		for at := index; at >= 0 && at < len(m.items); at += step {
			if at == from {
				continue
			}
			if m.usableItem(at, trigger) {
				return at, true
			}
		}
		return 0, false
	}
	step := 1
	if key == gfx.KeyUp {
		step = -1
	}
	for offset := 1; offset <= len(m.items); offset++ {
		at := from + step*offset
		for at < 0 {
			at += len(m.items)
		}
		for at >= len(m.items) {
			at -= len(m.items)
		}
		if m.usableItem(at, trigger) {
			return at, true
		}
	}
	return 0, false
}

func (m *Menubar) usableItem(index, trigger int) bool {
	item := &m.items[index]
	return item.Trigger == trigger && !item.Separator && !item.Item.Disabled && !item.Item.ID.IsZero()
}

func (m *Menubar) firstItem(trigger int) (gfx.ID, bool) {
	for index := range m.items {
		if m.usableItem(index, trigger) {
			return m.items[index].Item.ID, true
		}
	}
	return gfx.ID{}, false
}
