package ui

import (
	"math"

	"github.com/vivaui/viva/base"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

func paintCheckbox(f *Frame, v ControlView) {
	canvas := f.Surface
	side := components.CheckboxExtent(f.Palette)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-side)/2, Y: v.Rect.Y + (v.Rect.Height-side)/2, Width: side, Height: side}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.PaintCaptionView(v, bounds)
	}
	checkbox := components.BeginCheckbox(canvas, f.Palette, v.ID, bounds, v.Flags&FlagChecked != 0)
	if v.Exact {
		checkbox = checkbox.HitBounds(v.Rect)
	}
	if v.Flags&FlagMixed != 0 {
		checkbox = checkbox.Indeterminate()
	}
	if v.Flags&FlagInvalid != 0 {
		checkbox = checkbox.Invalid()
	}
	if f.GetStyles() != nil && f.GetStyles().Checkbox != nil {
		checkbox = checkbox.Recipe(f.GetStyles().Checkbox)
	}
	if v.Label != "" {
		checkbox = checkbox.Label(v.Label)
	}
	if v.Disabled {
		checkbox = checkbox.Disabled()
	}
	checkbox.Paint()
}

func paintSwitch(f *Frame, v ControlView) {
	canvas := f.Surface
	small := v.Flags&FlagSmall != 0
	width, height := components.SwitchExtent(f.Palette, small)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-width)/2, Y: v.Rect.Y + (v.Rect.Height-height)/2, Width: width, Height: height}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.PaintCaptionView(v, bounds)
	}
	control := components.BeginSwitch(canvas, f.Palette, v.ID, bounds, v.Flags&FlagChecked != 0)
	if v.Exact {
		control = control.HitBounds(v.Rect)
	}
	if recipe := v.SwitchRecipe; recipe != nil {
		control = control.Recipe(recipe)
	} else if f.GetStyles() != nil && f.GetStyles().Switch != nil {
		control = control.Recipe(f.GetStyles().Switch)
	}
	if small {
		control = control.Small()
	}
	if v.Flags&FlagInvalid != 0 {
		control = control.Invalid()
	}
	if v.Label != "" {
		control = control.Label(v.Label)
	}
	if v.Disabled {
		control = control.Disabled()
	}
	control.Paint()
}

func paintToggle(f *Frame, v ControlView) {
	toggle := components.BeginToggle(f.Surface, f.Palette, v.ID, v.Rect, v.Flags&FlagChecked != 0)
	if v.Size == style.SizeSmall {
		toggle = toggle.Small()
	} else if v.Size == style.SizeLarge {
		toggle = toggle.Large()
	}
	if v.Flags&FlagDisabled != 0 {
		toggle = toggle.Disabled()
	}
	if v.Flags&FlagInvalid != 0 {
		toggle = toggle.Invalid()
	}
	label := v.Text
	if label == "" {
		label = v.Label
	}
	if v.Variant == style.VariantOutline {
		toggle.Outline(label)
	} else {
		toggle.Default(label)
	}
}

func paintInputGroup(f *Frame, v ControlView) {
	components.PaintInputGroup(f.Surface, f.Palette, v.Rect, base.WidgetsOf(f).GetGroups()[v.Aux])
}

func paintSegmented(f *Frame, v ControlView) {
	choice := base.WidgetsOf(f).GetSegmented()[v.Aux]
	control := components.BeginSegmentedControl(f.Surface, f.Palette, v.Rect, choice.GetOptions(), choice.GetSelected())
	if v.Flags&FlagDisabled != 0 {
		control = control.Disabled()
	}
	control.Paint()
}

// measureControls measures the controls: each kind's geometry, from package
// components for the ones that ship with a theme.
func measureControls(f *Frame, n *Node) {
	switch n.Kind {
	case KindButton:
		button := components.BeginButton(f.Surface, f.Palette, gfx.ID{}, Rect{}).Size(n.ButtonSize).WithIcon(f.Exts[n.Ext].GetIcon())
		if d := n.Size[AxisY]; d.Kind == DimPx {
			button = button.Height(d.Value) // an icon button is as wide as it is tall
		}
		button = button.SizeToFit(n.Label)
		n.Pref = [2]float32{button.BoundsWidth(), button.BoundsHeight()}
		if n.ButtonSize >= components.ButtonSizeIcon {
			n.Pref[AxisX] = n.Pref[AxisY]
		}
	case KindBadge:
		w, h := components.BadgeExtent(f.Surface, f.Palette, n.Text)
		n.Pref = [2]float32{w, h}
	case KindPagination:
		w, h := components.PaginationExtent(f.Surface, f.Palette, f.PaginationItems(n))
		n.Pref = [2]float32{w, h}
	case KindSplitButton:
		button := components.BeginButton(f.Surface, f.Palette, gfx.ID{}, Rect{}).Size(n.ButtonSize).SizeToFit(n.Label)
		trigger := base.WidgetsOf(f).GetSplitButtons()[n.Aux].GetTriggerWidth()
		if trigger <= 0 {
			trigger = SplitButtonTrigger
		}
		n.Pref = [2]float32{button.BoundsWidth() + trigger, button.BoundsHeight()}
	case KindCheckbox:
		w, h := components.CheckboxHitExtent(f.Palette)
		n.Pref = [2]float32{w, h}
		if n.Text != "" {
			n.Pref[AxisX] = components.CheckboxExtent(f.Palette) + CaptionGap + f.Surface.TextWidthWeight(n.Text, f.CaptionSize(n), gfx.WeightMedium)
		}
	case KindSwitch:
		w, h := components.SwitchHitExtent(f.Palette, n.Flags&FlagSmall != 0)
		n.Pref = [2]float32{w, h}
		if n.Text != "" {
			control, _ := components.SwitchExtent(f.Palette, n.Flags&FlagSmall != 0)
			n.Pref[AxisX] = control + CaptionGap + f.Surface.TextWidthWeight(n.Text, f.CaptionSize(n), gfx.WeightMedium)
		}
	case KindToggle:
		toggle := components.BeginToggle(f.Surface, f.Palette, gfx.ID{}, Rect{}, false)
		if n.ButtonSize == style.SizeSmall {
			toggle = toggle.Small()
		} else if n.ButtonSize == style.SizeLarge {
			toggle = toggle.Large()
		}
		label := n.Text
		if label == "" {
			label = n.Label
		}
		toggle = toggle.SizeToFit(label)
		height := f.Palette.Metrics.ControlHeight
		if n.ButtonSize == style.SizeSmall {
			height -= 4
		} else if n.ButtonSize == style.SizeLarge {
			height += 4
		}
		n.Pref = [2]float32{toggle.BoundsWidth(), height}
	case KindSegmented:
		var width float32 = 8
		for _, option := range base.WidgetsOf(f).GetSegmented()[n.Aux].GetOptions() {
			width += f.Surface.TextWidthWeight(option.Label, f.Palette.Metrics.FontSizeM, gfx.WeightMedium) + 28
		}
		n.Pref = [2]float32{width, f.Palette.Metrics.ControlHeight}
	case KindInput:
		n.Pref = [2]float32{n.PadX(), f.InputLine(n) + n.PadY()}
		if n.Flags&FlagThemed != 0 {
			n.Pref = [2]float32{0, f.Palette.Metrics.ControlHeight}
		}
	case KindInputGroup:
		n.Pref = [2]float32{0, f.Palette.Metrics.ControlHeight}
		if n.Kind == KindInputGroup && base.WidgetsOf(f).GetGroups()[n.Aux].Multiline {
			n.Pref[AxisY] = 64
		}
	case KindTextarea:
		n.Pref = [2]float32{n.PadX(), f.Palette.Metrics.FontSizeM + 7 + n.PadY()}
		if n.Flags&FlagThemed != 0 {
			n.Pref = [2]float32{0, 64}
		}
		if n.MaxLines > 0 {
			// AutoGrow: the rows measured by the same layout the paint will use,
			// capped at n.MaxLines rows and never below the control's minimum.
			// The width is the one the last layout gave the box, which the first
			// paint records and then asks for before it is shown.
			snapshot := &base.WidgetsOf(f).GetTextareas()[n.Aux]
			if layout := snapshot.Layout; layout != nil {
				if width := layout.BoxWidth(); width > 0 {
					height := layout.ContentHeight(f.Surface, f.Palette, snapshot.LayoutText(), width, snapshot.NoWrap)
					maxHeight := float32(n.MaxLines)*(f.Palette.Metrics.FontSizeM+7) + 2*base.TextareaPadY
					n.Pref[AxisY] = max(64, min(height, maxHeight))
				}
			}
		}
	case KindTable, KindTabs, KindSelect:
		f.MeasureWidget(n)
	case KindProgress:
		n.Pref = [2]float32{ProgressWidth, ProgressHeight}
	case KindSpinner:
		n.Pref = [2]float32{16, 16}
	case KindSplitter:
		f.MeasureSplitter(n)
	case KindSlider:
		f.MeasureSlider(n)
	}
}

func paintButton(f *Frame, n *Node) {
	r := n.Rect
	extra := &f.Exts[n.Ext]
	button := components.BeginButton(f.Surface, f.Palette, n.ID, Rect{X: r.X, Y: r.Y, Width: r.Width}).Size(n.ButtonSize).
		WithIcon(extra.GetIcon()).IconTransform(extra.GetIconTransform())
	if f.GetStyles() != nil && f.GetStyles().Button != nil {
		button = button.Recipe(f.GetStyles().Button)
	}
	if extra.GetButtonRecipe() != nil {
		button = button.Recipe(extra.GetButtonRecipe())
	}
	// A button is as tall as its size says unless the element was given more or less
	// room: a Height, or a Grow along a column. It then fills that room.
	if h := r.Height; h > 0 && math.Abs(float64(h-button.BoundsHeight())) > 0.01 {
		button = button.Height(h)
	}
	if n.Flags&FlagDisabled != 0 {
		button = button.Disabled()
	}
	switch n.Variant {
	case components.ButtonSecondary:
		button.Secondary(n.Label)
	case components.ButtonOutline:
		button.Outline(n.Label)
	case components.ButtonGhost:
		button.Ghost(n.Label)
	case components.ButtonLink:
		button.Link(n.Label)
	case components.ButtonDestructive:
		button.Destructive(n.Label)
	case components.ButtonDestructiveOutline:
		button.DestructiveOutline(n.Label)
	default:
		button.Default(n.Label)
	}
}

func paintInput(f *Frame, n *Node) {
	// What the element says of its text (FontSize, Weight, Color) is the field's text style,
	// over the snapshot's own: the one value the glyphs, the caret and the platform follow.
	// What it says of its alignment is the same, but a field is start-aligned unless it says
	// otherwise, so the snapshot's own declaration is left alone only by an alignment it
	// declares. What the snapshot leaves at AlignLeft the element's TextEnd() takes.
	snapshot := base.WidgetsOf(f).GetInputs()[n.Aux]
	if n.TextEnd {
		if snapshot.Alignment == gfx.AlignLeft || snapshot.Alignment == gfx.AlignStart {
			snapshot.Alignment = gfx.AlignEnd
		}
	}
	if n.FontSize > 0 {
		snapshot.Text.Size = n.FontSize
	}
	if n.Weight > 0 {
		snapshot.Text.Weight = n.Weight
	}
	if n.Flags&FlagHasColor != 0 {
		snapshot.Text.Color = n.Color
	}
	if n.Flags&FlagMono != 0 {
		snapshot.Text.Mono, snapshot.Text.MonoFace = true, n.Face
	}
	if a := f.Exts[n.Ext].GetAddons(); a != 0 {
		f.PaintAddonsFn(f, n, snapshot, &f.GetAddons()[a-1])
		return
	}
	components.PaintInputContent(f.Surface, f.Palette, n.Rect, f.ContentRect(n), snapshot)
}

// paintTextarea paints the rows of a Textarea or TextareaBase inside the element's Pad and
// Border, as paintInput does for a line.
func paintTextarea(f *Frame, n *Node) {
	snapshot := base.WidgetsOf(f).GetTextareas()[n.Aux]
	if n.MaxLines > 0 {
		if layout := snapshot.Layout; layout != nil {
			// AutoGrow measured the last width the layout gave this box. The
			// first time the box is laid out there is no such width yet: record
			// it and ask the page to lay out once more so the displayed frame is
			// already the right height.
			if layout.BoxWidth() != n.Rect.Width {
				f.RequestRelayout()
			}
			layout.SetBoxWidth(n.Rect.Width)
		}
	}
	if n.Flags&FlagHasColor != 0 {
		theme := f.Palette
		theme.Foreground = n.Color
		components.PaintTextareaContent(f.Surface, theme, n.Rect, f.ContentRect(n), snapshot)
		return
	}
	components.PaintTextareaContent(f.Surface, f.Palette, n.Rect, f.ContentRect(n), snapshot)
}
