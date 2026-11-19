// Package hierarchypicker is the opt-in page control for choosing a value from
// hierarchical paths, such as "School/Class A". It is a field trigger and a
// fly-out menu: the engine owns the paths, focus and keys, components paints
// the menu, and an application that does not import it pays nothing.
//
//	state := hierarchypicker.NewState([]string{"School", "School/Class A", "Office"})
//	picker := hierarchypicker.New(state)
//	...
//	ui.FormField(c, &m.groupField, "Group").Children(func() {
//		picker.View(c, hierarchypicker.Spec{Label: "Group", OptionIDs: groupOptions}).FillWidth()
//	})
package hierarchypicker

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// State is one picker's paths, chosen value and open state. Keep it in the
// model; SetOptions and SetValue update it, and Value reads the chosen path.
type State struct {
	Value string
	Open  bool

	Menu      base.HierarchyMenuState
	options   []string
	revision  uint64
	built     uint64
	separator string
}

// NewState returns a state for the full path values.
func NewState(options []string) *State {
	s := new(State)
	s.SetOptions(options)
	return s
}

// SetOptions replaces the paths. The chosen value stays; a value no longer in
// the paths remains selected until SetValue changes it.
func (s *State) SetOptions(options []string) {
	s.options = append(s.options[:0], options...)
	s.revision++
	s.built = 0
}

// SetValue selects the full path value, and closes the menu.
func (s *State) SetValue(value string) {
	s.Value, s.Open = value, false
	if s.built == s.revision {
		s.Menu.Seek(value)
	}
}

// Spec supplies the control's words and its identities. OptionIDs and ScrollID
// come from the same gfx.Tree the page uses; OptionIDs is the scope of the
// menu's rows.
type Spec struct {
	Label, Placeholder, EmptyLabel string
	Separator                      string // "/" when empty
	OptionIDs                      gfx.Scope
	ScrollID                       gfx.ID
	Scope, DismissID               gfx.ID
	MaxRows                        int
}

// Picker is one hierarchical picker's page control. Keep it in the model
// beside its State; one Picker belongs to one State and one place on a page.
type Picker struct {
	State *State

	spec      Spec
	handlers  ui.Handlers
	drawFn    func(*ui.DrawContext)
	controlID gfx.ID
}

// New returns a picker for state.
func New(state *State) *Picker {
	if state == nil {
		panic("hierarchypicker: New needs its State")
	}
	p := &Picker{State: state}
	p.handlers.Role = gfx.RoleCombobox
	p.handlers.Key = p.key
	p.handlers.Pointer = p.pointer
	p.handlers.Scroll = p.scroll
	p.drawFn = p.draw
	return p
}

// View declares the picker at c. Give it a width; it is as tall as the theme's
// control unless a Height follows. Declare it on its own, not inside a Memo.
func (p *Picker) View(c ui.C, spec Spec) ui.Box {
	if p.State == nil || p.drawFn == nil {
		panic("hierarchypicker: use New to make a Picker")
	}
	p.spec = spec
	return ui.Control(c, spec.Label, &p.handlers).
		Height(c.Theme().Metrics.ControlHeight).
		Draw(p.drawFn)
}

func (p *Picker) key(event gfx.KeyEvent) gfx.EventResult {
	result := p.State.Menu.Key(event, p.State.Open)
	if result.Opened {
		p.State.Open = true
	}
	if result.Picked {
		p.State.Value = result.Chosen
		p.State.Open = false
	}
	if event.Key == gfx.KeyEscape && p.State.Open && !result.Handled {
		p.State.Open = false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return result.EventResult
}

func (p *Picker) pointer(event gfx.PointerEvent) gfx.EventResult {
	return p.State.Menu.Pointer(event)
}

func (p *Picker) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if !p.State.Open {
		return gfx.EventResult{}
	}
	if p.State.Menu.Wheel(event) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

func (p *Picker) draw(d *ui.DrawContext) {
	p.controlID = d.Node.ID
	state := p.State
	separator := p.spec.Separator
	if separator == "" {
		separator = "/"
	}
	if state.built != state.revision || state.separator != separator {
		state.Menu.Build(separator, state.Value, nil, len(state.options), func(i int) base.HierarchyMenuItem {
			return base.HierarchyMenuItem{Value: state.options[i]}
		})
		state.built, state.separator = state.revision, separator
	}
	theme := *d.Theme
	trigger := d.Rect
	selectControl := components.BeginSelect(d.Canvas, theme, trigger, components.SelectOptions{
		Value: state.Value, Placeholder: p.spec.Placeholder, Open: false, Disabled: d.State.Disabled,
	})
	selectControl.End()
	if !state.Open {
		p.pick(d.Canvas)
		return
	}
	options := base.HierarchyMenuOptions{
		OptionIDs: p.spec.OptionIDs, ScrollID: p.spec.ScrollID,
		Scope: p.spec.Scope, Layer: d.Frame.WidgetLayer() + 2, Label: p.spec.Label,
		EmptyLabel: p.spec.EmptyLabel, Viewport: d.Frame.GetBounds(), MaxRows: p.spec.MaxRows,
		Overlay: true, DismissID: p.spec.DismissID,
	}
	components.PaintHierarchyMenu(d.Canvas, theme, trigger, &state.Menu, options)
	p.pick(d.Canvas)
}

func (p *Picker) pick(canvas *gfx.Canvas) {
	if canvas == nil {
		return
	}
	if canvas.Clicked(p.spec.DismissID) {
		p.State.Open = false
		return
	}
	if index, ok := canvas.ClickedIn(p.spec.OptionIDs); ok {
		if value, chosen := p.State.Menu.Choose(index); chosen {
			p.State.Value = value
			p.State.Open = false
		}
		return
	}
	// The trigger is the control's own hit; a press anywhere on it toggles.
	if canvas.Clicked(p.controlID) {
		p.State.Open = !p.State.Open
	}
}
