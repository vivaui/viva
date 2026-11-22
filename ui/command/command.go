// Package command is the opt-in page control for the command palette of
// package components. It wraps components.BeginCommand in a ui.Control: the
// caller owns the query, the filtered results, the selection and the scroll
// position, and the wrapper paints the surface and routes its input.
//
//	// Once, with the model:
//	m.palette = command.State{
//		Filter: func(query string) { /* rebuild m.results */ },
//		Choose: func(index int) { /* run m.results[index] */ },
//	}
//
//	// Every paint:
//	palette := &m.palette
//	palette.Clear()
//	palette.Group("Suggestions")
//	palette.Item("Open workspace", "⌘O")
//	palette.Item("Create window", "⌘N")
//	command.View(c, "Command", palette).FillWidth().Height(180)
//
// The query is edited through the platform's text service: the paint that
// follows a native edit reads it, writes State.Query and calls Filter, so the
// caller filters in one place. Selected is the ordinal of an item row, not of
// Results: group headings, separators and the empty state take no selection.
// The wrapper moves it from the pointer and the Up and Down keys, and Enter
// calls Choose.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a State in the model beside the data it presents: its
// handlers and draw function are made once, so a warm paint allocates nothing.
// Do not copy a State after it has been declared.
package command

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// ResultKind selects what one row of Results paints.
type ResultKind uint8

const (
	// ResultItem is a command result. It is the zero kind, so a Result with only
	// Label and Shortcut set is an item.
	ResultItem ResultKind = iota
	// ResultGroup is a compact heading over the items that follow it.
	ResultGroup
	// ResultSeparator is the one-pixel divider between groups.
	ResultSeparator
	// ResultEmpty is the centered empty-result state.
	ResultEmpty
)

// Result is one caller-owned row of the result list. Filters, labels and
// shortcuts stay the application's; the wrapper only paints and hit-tests
// them.
type Result struct {
	Kind ResultKind
	// ID names the item's hit. A zero ID is named by the control's own part
	// scope, so a plain list needs no identities of its own. A non-zero ID is
	// the caller's, as for any canvas component.
	ID       gfx.ID
	Label    string
	Value    string
	Hint     string
	Shortcut string
	Icon     gfx.Icon
	// Checked paints the persistent check of a chosen command. Selected is the
	// transient row the pointer and keys move, and is State.Selected.
	Checked  bool
	Disabled bool
}

// Options is the visual and accessibility shape of a command surface. Its
// zero value is the compact embedded geometry.
type Options struct {
	Placeholder string
	Hint        string
	Disabled    bool
	Invalid     bool
	// Layer is the compositing layer of the popup surface.
	Layer uint16
	// Scope, when set, is the focus scope of the palette; DismissID, when set,
	// is the Escape target.
	Scope     gfx.ID
	DismissID gfx.ID

	variant components.CommandVariant
}

// Dialog returns o painting the roomier command-palette geometry used inside a
// dialog. It is the only visual variant; chain it rather than reaching for a
// second constructor:
//
//	command.Options = command.Options.Dialog()
func (o Options) Dialog() Options {
	o.variant = components.CommandDialog
	return o
}

// State is one command palette's caller-owned query, results, selection and
// scroll position, with the handlers that route its input. Keep it in the
// model beside the data it presents; its handlers and draw function point at
// this value, so do not copy a State after it has been declared.
type State struct {
	// Query is the text in the input. The wrapper reads every native edit at the
	// next paint, writes it here and calls Filter. Assigning it between paints
	// replaces the query without calling Filter; the wrapper then recreates the
	// text editor and resets ScrollState.Offset.
	Query string
	// Results are the rows painted under the input, in order: items, group
	// headings, separators and the empty state. Rebuild it when the query
	// changes or from Filter.
	Results []Result
	// Selected is the ordinal of the selected item row: items only, not group
	// headings or separators. Out of range selects no row. The wrapper moves it
	// from the pointer and the Up and Down keys and reports the new ordinal to
	// Select.
	Selected int
	// ScrollState is the result list's offset and scrollbar state. The wrapper
	// paints and scrolls it; the caller may set Offset and read the measured
	// geometry after a paint.
	ScrollState components.MenuScroll
	// Options are the surface's words, state and geometry.
	Options Options

	// Filter is called at the start of the paint that first sees a native edit
	// change Query, before Results are painted. Rebuild Results there. It is not
	// called for an assignment to Query.
	Filter func(query string)
	// Select is called after Selected changes from a key press, a pointer
	// hover or a click.
	Select func(index int)
	// Choose is called when Enter activates the selected item. A nil Choose
	// leaves Enter to the page.
	Choose func(index int)

	// Key receives the keys the wrapper leaves unhandled, after Up, Down and
	// Enter.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Pointer receives the pointer events aimed at the control or one of its
	// rows.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Scroll receives the wheel the result list did not scroll.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text, when set, replaces the wrapper's native query editor. Nil gives the
	// platform the wrapper's text.Editor through the control's role.
	Text func(window.TextInputContext) *window.TextInput

	handlers   ui.Handlers
	drawFn     func(*ui.DrawContext)
	prepared   bool
	editor     *text.Editor
	published  string
	input      window.TextInput
	controlID  gfx.ID
	bounds     gfx.Rect
	inputBound gfx.Rect
	itemBounds []gfx.Rect
}

// View declares the command palette at c. label names the control for
// assistive technology. Give it a height; it is a ui.Control painted through
// the component. Declare it on its own, not inside a Memo.
func View(c ui.C, label string, command *State) ui.Box {
	if command == nil {
		panic("command: View needs its State")
	}
	command.prepare()
	box := ui.Control(c, label, &command.handlers)
	command.controlID = box.N().ID
	return box.Draw(command.drawFn)
}

// View is View for a command the model keeps, so a page reads
// m.command.View(c, "State").
func (command *State) View(c ui.C, label string) ui.Box {
	return View(c, label, command)
}

// Clear removes every result, keeping the slice for the next declaration. It
// is how a caller rebuilds Results after Filter.
func (command *State) Clear() { command.Results = command.Results[:0] }

// Group appends a compact group heading.
func (command *State) Group(label string) {
	command.Results = append(command.Results, Result{Kind: ResultGroup, Label: label})
}

// Item appends one plain result. Append a Result to Results for a value, hint,
// icon, check or disabled state.
func (command *State) Item(label, shortcut string) {
	command.Results = append(command.Results, Result{Label: label, Shortcut: shortcut})
}

// Separator appends the divider between two groups.
func (command *State) Separator() {
	command.Results = append(command.Results, Result{Kind: ResultSeparator})
}

// Empty appends the centered empty-result state.
func (command *State) Empty(label string) {
	command.Results = append(command.Results, Result{Kind: ResultEmpty, Label: label})
}

// InputID is the identity of the query row, a part of the control. It is the
// hit the platform text service edits and the gallery resolves an editor for.
func (command *State) InputID() gfx.ID { return command.handlers.Parts().At(0) }

// Bounds returns the command surface geometry of the last paint.
func (command *State) Bounds() gfx.Rect { return command.bounds }

// InputBounds returns the input row's text geometry of the last paint.
func (command *State) InputBounds() gfx.Rect { return command.inputBound }

// ItemBounds returns the rectangle of item row index in the last paint, or the
// empty rectangle when the row was scrolled out or index is not an item.
// Selected is likewise an item ordinal.
func (command *State) ItemBounds(index int) gfx.Rect {
	if index < 0 || index >= len(command.itemBounds) {
		return gfx.Rect{}
	}
	return command.itemBounds[index]
}

func (command *State) prepare() {
	if command.prepared {
		return
	}
	command.prepared = true
	command.handlers.Key = command.key
	command.handlers.Pointer = command.pointer
	command.handlers.Scroll = command.scroll
	command.handlers.Text = command.text
	command.drawFn = command.draw
	command.editor = text.NewEditor(command.Query)
	command.published = command.Query
	command.input.Editor = command.editor
	command.input.NavigationKeys = true
}

// Editor returns the text.Editor the platform edits for Query. A host that
// resolves text input by identity, without calling Page.TextInput, gives this
// editor to its provider. It is nil before the first View.
func (command *State) Editor() *text.Editor { return command.editor }

// sync reads the query the platform edited since the last paint, or applies a
// Query the caller assigned between paints.
func (command *State) sync() {
	if command.Query != command.published {
		command.editor = text.NewEditor(command.Query)
		command.published = command.Query
		command.input.Editor = command.editor
		command.ScrollState.Offset = 0
		return
	}
	value := command.editor.String()
	if value == command.published {
		return
	}
	command.Query = value
	command.published = value
	command.ScrollState.Offset = 0
	if command.Filter != nil {
		command.Filter(value)
	}
}

func (command *State) text(context window.TextInputContext) *window.TextInput {
	if command.Text != nil {
		return command.Text(context)
	}
	command.input.Bounds = command.inputBound
	return &command.input
}

func (command *State) draw(d *ui.DrawContext) {
	command.sync()
	command.bounds = d.Rect
	command.itemBounds = command.itemBounds[:0]
	options := components.CommandOptions{
		InputID: command.handlers.Parts().At(0),
		Label:   d.Label, Query: command.Query, Placeholder: command.Options.Placeholder,
		Hint: command.Options.Hint, Disabled: command.Options.Disabled, Invalid: command.Options.Invalid,
		Variant: command.Options.variant, Layer: command.Options.Layer,
		Scope: command.Options.Scope, DismissID: command.Options.DismissID,
		Scroll: &command.ScrollState,
	}
	surface := components.BeginCommand(d.Canvas, *d.Theme, d.Rect, options)
	// The component lays the list under an input row of 36 pixels (48 for the
	// dialog geometry), and its label starts 36 pixels in, 50 short of the row.
	inputHeight := float32(36)
	if command.Options.variant == components.CommandDialog {
		inputHeight = 48
	}
	command.inputBound = gfx.Rect{
		X: d.Rect.X + 36, Y: d.Rect.Y,
		Width: max(0, d.Rect.Width-50), Height: inputHeight,
	}
	item := 0
	for index := range command.Results {
		result := &command.Results[index]
		switch result.Kind {
		case ResultGroup:
			surface.AddGroup(result.Label)
		case ResultSeparator:
			surface.AddSeparator()
		case ResultEmpty:
			surface.AddEmpty(result.Label)
		case ResultItem:
			id := result.ID
			if id.IsZero() {
				id = command.handlers.Parts().At(item)
			}
			row := surface.AddItem(components.CommandItem{
				ID: id, Label: result.Label, Value: result.Value, Hint: result.Hint,
				Shortcut: result.Shortcut, Selected: item == command.Selected,
				Checked: result.Checked, Disabled: result.Disabled, Icon: result.Icon,
			})
			if item < len(command.itemBounds) {
				command.itemBounds[item] = row
			} else {
				command.itemBounds = append(command.itemBounds, row)
			}
			item++
		}
	}
	surface.End()
	command.pick(d)
}

// pick moves the selection onto the row the pointer entered or clicked, as the
// component's pointer rows do, and keeps the keyboard and the text service on
// the control when a click lands on a result.
func (command *State) pick(d *ui.DrawContext) {
	if command.itemCount() == 0 {
		return
	}
	if id := d.Canvas.Hovered(); !id.IsZero() {
		if item, ok := command.ordinalOf(id); ok {
			command.selectItem(item)
		}
	}
	if id, ok := d.Canvas.ClickTarget(); ok {
		if item, ok := command.ordinalOf(id); ok {
			command.selectItem(item)
			d.Canvas.RequestFocus(d.Node.ID)
		}
	}
}

func (command *State) key(event gfx.KeyEvent) gfx.EventResult {
	switch event.Key {
	case gfx.KeyDown, gfx.KeyUp:
		if command.itemCount() == 0 {
			break
		}
		delta := 1
		if event.Key == gfx.KeyUp {
			delta = -1
		}
		moved, scrolled := command.move(delta)
		return gfx.EventResult{Handled: true, Repaint: moved || scrolled}
	case gfx.KeyEnter:
		if item, ok := command.itemAt(command.Selected); ok && !item.Disabled && command.Choose != nil {
			command.Choose(command.Selected)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if command.Key == nil {
		return gfx.EventResult{}
	}
	return command.Key(event)
}

func (command *State) pointer(event gfx.PointerEvent) gfx.EventResult {
	if command.Pointer == nil {
		return gfx.EventResult{}
	}
	return command.Pointer(event)
}

func (command *State) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if command.ScrollState.Wheel(event) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if command.Scroll == nil {
		return gfx.EventResult{}
	}
	return command.Scroll(event)
}

// move steps to the next enabled item, wrapping at the ends, and reports
// whether the selection changed and whether the list had to scroll.
func (command *State) move(delta int) (moved, scrolled bool) {
	count := command.itemCount()
	if count == 0 {
		return false, false
	}
	next := command.Selected
	if next < 0 || next >= count {
		if delta < 0 {
			next = count
		} else {
			next = -1
		}
	}
	for range count {
		next = (next + delta) % count
		if next < 0 {
			next += count
		}
		item, _ := command.itemAt(next)
		if item.Disabled {
			continue
		}
		if next != command.Selected {
			command.Selected = next
			moved = true
			if command.Select != nil {
				command.Select(next)
			}
		}
		scrolled = command.ScrollState.Reveal(command.itemID(next))
		return moved, scrolled
	}
	return false, false
}

func (command *State) selectItem(item int) {
	if item < 0 || item >= command.itemCount() || item == command.Selected {
		return
	}
	result, ok := command.itemAt(item)
	if !ok || result.Disabled {
		return
	}
	command.Selected = item
	if command.Select != nil {
		command.Select(item)
	}
}

func (command *State) itemCount() int {
	count := 0
	for index := range command.Results {
		if command.Results[index].Kind == ResultItem {
			count++
		}
	}
	return count
}

// itemAt returns item row ordinal, skipping headings, separators and the empty
// state.
func (command *State) itemAt(ordinal int) (Result, bool) {
	if ordinal < 0 {
		return Result{}, false
	}
	item := 0
	for index := range command.Results {
		if command.Results[index].Kind != ResultItem {
			continue
		}
		if item == ordinal {
			return command.Results[index], true
		}
		item++
	}
	return Result{}, false
}

func (command *State) itemID(ordinal int) gfx.ID {
	item, ok := command.itemAt(ordinal)
	if !ok {
		return gfx.ID{}
	}
	if !item.ID.IsZero() {
		return item.ID
	}
	return command.handlers.Parts().At(ordinal)
}

func (command *State) ordinalOf(id gfx.ID) (int, bool) {
	if id.IsZero() {
		return 0, false
	}
	item := 0
	for index := range command.Results {
		if command.Results[index].Kind != ResultItem {
			continue
		}
		named := command.Results[index].ID
		if named.IsZero() {
			named = command.handlers.Parts().At(item)
		}
		if named == id {
			return item, true
		}
		item++
	}
	return 0, false
}
