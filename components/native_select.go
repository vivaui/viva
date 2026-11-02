package components

import (
	"github.com/vivaui/viva/ui"
)

// NativeSelectSize selects the official default or compact trigger height.
type NativeSelectSize uint8

const (
	NativeSelectDefault NativeSelectSize = iota
	NativeSelectSmall
)

// NativeSelectOption is one caller-owned native option. Order is stable for
// the lifetime of an open platform popup; the component retains no copy.
type NativeSelectOption struct {
	Label string
	Value string
	Hint  string
	// Group is the label of the optgroup the option belongs to. Consecutive
	// options with the same non-empty Group form one group; an empty Group
	// is a top-level option. The trigger paints none of it: a platform
	// adapter that builds the OS popup reads it to emit the group headings.
	Group    string
	Disabled bool
}

// NativeSelectSelection explicitly distinguishes option zero from no
// selection. Index addresses the current Options snapshot only.
type NativeSelectSelection struct {
	Index int
	Valid bool
}

// NativeSelectCommandKind identifies a state transition requested by a key.
type NativeSelectCommandKind uint8

const (
	NativeSelectCommandNone NativeSelectCommandKind = iota
	NativeSelectCommandOpen
	NativeSelectCommandClose
	NativeSelectCommandSelect
)

// NativeSelectCommand is returned to the owner; the component never opens an
// OS popup or mutates caller-owned selection itself.
type NativeSelectCommand struct {
	Kind      NativeSelectCommandKind
	Selection NativeSelectSelection
}

// NativeSelectSemantics is the typed semantic projection of a trigger.
type NativeSelectSemantics struct {
	ID          ui.ID
	Label       string
	Value       string
	Hint        string
	Selection   NativeSelectSelection
	OptionCount int
	Focused     bool
	Open        bool
	Disabled    bool
	ReadOnly    bool
	Invalid     bool
}

// NativeSelectSnapshot describes a closed or OS-open native select trigger.
// The application/platform owns options, selection, popup lifecycle, and input.
type NativeSelectSnapshot struct {
	ID          ui.ID
	Label       string
	Options     []NativeSelectOption
	Selection   NativeSelectSelection
	Value       string
	Display     string
	Placeholder string
	Hint        string
	Size        NativeSelectSize
	Open        bool
	Disabled    bool
	ReadOnly    bool
	Invalid     bool
	Focused     bool
}

// SelectedOption resolves the explicit caller-owned selection snapshot.
func (snapshot NativeSelectSnapshot) SelectedOption() (NativeSelectOption, bool) {
	if !snapshot.Selection.Valid || snapshot.Selection.Index < 0 || snapshot.Selection.Index >= len(snapshot.Options) {
		return NativeSelectOption{}, false
	}
	return snapshot.Options[snapshot.Selection.Index], true
}

// Semantics returns the allocation-free semantic state used by
// PaintNativeSelectSemantics and by platform adapters that need richer typed
// state than ui.Hit currently carries.
func (snapshot NativeSelectSnapshot) Semantics() NativeSelectSemantics {
	value := snapshot.Value
	selection := snapshot.Selection
	if option, ok := snapshot.SelectedOption(); ok {
		value = option.Value
	} else if selection.Valid {
		selection = NativeSelectSelection{}
		value = ""
	}
	return NativeSelectSemantics{
		ID: snapshot.ID, Label: snapshot.Label, Value: value, Hint: snapshot.Hint,
		Selection: selection, OptionCount: len(snapshot.Options),
		Focused: snapshot.Focused && !snapshot.Disabled, Open: snapshot.Open,
		Disabled: snapshot.Disabled, ReadOnly: snapshot.ReadOnly, Invalid: snapshot.Invalid,
	}
}

// NativeSelectCommandForKey maps trigger-owned keys to caller-applied state
// transitions. It does not open a native popup, synthesize pointer selection,
// wrap at list ends, or select disabled options.
func NativeSelectCommandForKey(snapshot NativeSelectSnapshot, key ui.Key) (NativeSelectCommand, bool) {
	if snapshot.Disabled || snapshot.ReadOnly {
		return NativeSelectCommand{}, false
	}
	switch key {
	case ui.KeyEnter, ui.KeySpace:
		if !snapshot.Open {
			return NativeSelectCommand{Kind: NativeSelectCommandOpen}, true
		}
		return NativeSelectCommand{}, false
	case ui.KeyEscape:
		if snapshot.Open {
			return NativeSelectCommand{Kind: NativeSelectCommandClose}, true
		}
		return NativeSelectCommand{}, false
	case ui.KeyDown:
		return nativeSelectSelectionCommand(snapshot, 1), true
	case ui.KeyUp:
		return nativeSelectSelectionCommand(snapshot, -1), true
	case ui.KeyHome:
		return nativeSelectEdgeCommand(snapshot, 1), true
	case ui.KeyEnd:
		return nativeSelectEdgeCommand(snapshot, -1), true
	default:
		return NativeSelectCommand{}, false
	}
}

// PaintNativeSelect paints the official Vega select trigger and declares
// combobox semantics without implementing a source-owned popup.
func PaintNativeSelect(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, snapshot NativeSelectSnapshot) ui.Rect {
	height := theme.Metrics.ControlHeight
	if snapshot.Size == NativeSelectSmall {
		height -= 4
	}
	if bounds.Height <= 0 {
		bounds.Height = height
	}
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return bounds
	}
	enabled := !snapshot.Disabled && !snapshot.ID.IsZero()
	paintTextEntrySurface(canvas, theme, bounds, textEntryState{
		Focused:  snapshot.Focused || enabled && canvas.FocusVisible() == snapshot.ID,
		Invalid:  snapshot.Invalid,
		Disabled: snapshot.Disabled,
		Hovered:  enabled && (canvas.Hovered() == snapshot.ID || canvas.Pressed() == snapshot.ID),
	})
	display := nativeSelectDisplay(snapshot)
	color := theme.Foreground
	if display == "" {
		display = snapshot.Placeholder
		color = theme.MutedForeground
	}
	iconColor := theme.MutedForeground
	if snapshot.Disabled {
		color.Alpha *= 0.50
		iconColor.Alpha *= 0.50
	}
	// Vega: pl-2.5 and pr-8 around the text, the chevron right-2.5.
	inset := theme.Metrics.TextInsetX()
	textBounds := ui.Rect{X: bounds.X + inset, Y: bounds.Y, Width: nativeSelectMax(0, bounds.Width-inset-33), Height: bounds.Height}
	canvas.Clip(textBounds)
	canvas.Label(display, textBounds, ui.TextStyle{Size: theme.Metrics.FontSizeM, Weight: ui.WeightRegular, Color: color})
	canvas.Unclip()
	paintChevron(canvas, &chevronDownIcon, ui.Rect{X: bounds.X + bounds.Width - 27, Y: bounds.Y, Width: 16, Height: bounds.Height}, iconColor)
	PaintNativeSelectSemantics(canvas, bounds, snapshot)
	return bounds
}

// PaintNativeSelectSemantics declares only the trigger hit and accessibility
// state. A platform adapter may pair it with an OS-owned popup without asking
// the component to draw or retain that popup.
func PaintNativeSelectSemantics(canvas *ui.Canvas, bounds ui.Rect, snapshot NativeSelectSnapshot) {
	if canvas == nil || snapshot.ID.IsZero() || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	semantics := snapshot.Semantics()
	canvas.HitWithOptions(semantics.ID, bounds, semantics.Label, ui.HitOptions{
		Role: ui.RoleCombobox, Value: semantics.Value, Hint: semantics.Hint,
		Disabled: semantics.Disabled, ReadOnly: semantics.ReadOnly, Expanded: semantics.Open, Expandable: true,
	})
}

func nativeSelectDisplay(snapshot NativeSelectSnapshot) string {
	if option, ok := snapshot.SelectedOption(); ok {
		return option.Label
	}
	if snapshot.Selection.Valid {
		return ""
	}
	if snapshot.Display != "" {
		return snapshot.Display
	}
	return snapshot.Value
}

func nativeSelectSelectionCommand(snapshot NativeSelectSnapshot, step int) NativeSelectCommand {
	if len(snapshot.Options) == 0 {
		return NativeSelectCommand{}
	}
	index := -1
	if snapshot.Selection.Valid && snapshot.Selection.Index >= 0 && snapshot.Selection.Index < len(snapshot.Options) {
		index = snapshot.Selection.Index
	}
	if index < 0 {
		return nativeSelectEdgeCommand(snapshot, step)
	}
	for candidate := index + step; candidate >= 0 && candidate < len(snapshot.Options); candidate += step {
		if !snapshot.Options[candidate].Disabled {
			return NativeSelectCommand{Kind: NativeSelectCommandSelect, Selection: NativeSelectSelection{Index: candidate, Valid: true}}
		}
	}
	return NativeSelectCommand{}
}

func nativeSelectEdgeCommand(snapshot NativeSelectSnapshot, step int) NativeSelectCommand {
	for offset := 0; offset < len(snapshot.Options); offset++ {
		index := offset
		if step < 0 {
			index = len(snapshot.Options) - 1 - offset
		}
		if !snapshot.Options[index].Disabled {
			return NativeSelectCommand{Kind: NativeSelectCommandSelect, Selection: NativeSelectSelection{Index: index, Valid: true}}
		}
	}
	return NativeSelectCommand{}
}

func nativeSelectMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
