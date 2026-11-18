package ui

import (
	"math"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func (f *frame) paintCheckbox(v ControlView) {
	canvas := f.Surface
	side := components.CheckboxExtent(f.Palette)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-side)/2, Y: v.Rect.Y + (v.Rect.Height-side)/2, Width: side, Height: side}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.paintCaptionView(v, bounds)
	}
	checkbox := components.BeginCheckbox(canvas, f.Palette, v.ID, bounds, v.Flags&flagChecked != 0)
	if v.Flags&flagMixed != 0 {
		checkbox = checkbox.Indeterminate()
	}
	if f.styles != nil && f.styles.Checkbox != nil {
		checkbox = checkbox.Recipe(f.styles.Checkbox)
	}
	if v.Label != "" {
		checkbox = checkbox.Label(v.Label)
	}
	if v.Disabled {
		checkbox = checkbox.Disabled()
	}
	checkbox.Paint()
}

func (f *frame) paintSwitch(v ControlView) {
	canvas := f.Surface
	small := v.Flags&flagSmall != 0
	width, height := components.SwitchExtent(f.Palette, small)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-width)/2, Y: v.Rect.Y + (v.Rect.Height-height)/2, Width: width, Height: height}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.paintCaptionView(v, bounds)
	}
	control := components.BeginSwitch(canvas, f.Palette, v.ID, bounds, v.Flags&flagChecked != 0)
	if recipe := v.SwitchRecipe; recipe != nil {
		control = control.Recipe(recipe)
	} else if f.styles != nil && f.styles.Switch != nil {
		control = control.Recipe(f.styles.Switch)
	}
	if small {
		control = control.Small()
	}
	if v.Label != "" {
		control = control.Label(v.Label)
	}
	if v.Disabled {
		control = control.Disabled()
	}
	control.Paint()
}

func (f *frame) paintInputGroup(v ControlView) {
	components.PaintInputGroup(f.Surface, f.Palette, v.Rect, widgetsOf(f).groups[v.Aux])
}

func (f *frame) paintSegmented(v ControlView) {
	choice := widgetsOf(f).segmented[v.Aux]
	components.BeginSegmentedControl(f.Surface, f.Palette, v.Rect, choice.options, choice.selected).Paint()
}

// measureControls measures the controls: each kind's geometry, from package
// components for the ones that ship with a theme.
func (f *frame) measureControls(n *node) {
	switch n.Kind {
	case kindButton:
		button := components.BeginButton(f.Surface, f.Palette, gfx.ID{}, Rect{}).Size(n.ButtonSize).WithIcon(f.Exts[n.Ext].icon)
		if d := n.Size[axisY]; d.Kind == dimPx {
			button = button.Height(d.Value) // an icon button is as wide as it is tall
		}
		button = button.SizeToFit(n.Label)
		n.Pref = [2]float32{button.BoundsWidth(), button.BoundsHeight()}
		if n.ButtonSize >= components.ButtonSizeIcon {
			n.Pref[axisX] = n.Pref[axisY]
		}
	case kindBadge:
		w, h := components.BadgeExtent(f.Surface, f.Palette, n.Text)
		n.Pref = [2]float32{w, h}
	case kindPagination:
		w, h := components.PaginationExtent(f.Surface, f.Palette, f.PaginationItems(n))
		n.Pref = [2]float32{w, h}
	case kindSplitButton:
		button := components.BeginButton(f.Surface, f.Palette, gfx.ID{}, Rect{}).Size(n.ButtonSize).SizeToFit(n.Label)
		trigger := widgetsOf(f).splitButtons[n.Aux].triggerWidth
		if trigger <= 0 {
			trigger = splitButtonTrigger
		}
		n.Pref = [2]float32{button.BoundsWidth() + trigger, button.BoundsHeight()}
	case kindCheckbox:
		w, h := components.CheckboxHitExtent(f.Palette)
		n.Pref = [2]float32{w, h}
		if n.Text != "" {
			n.Pref[axisX] = components.CheckboxExtent(f.Palette) + captionGap + f.Surface.TextWidthWeight(n.Text, f.CaptionSize(n), gfx.WeightMedium)
		}
	case kindSwitch:
		w, h := components.SwitchHitExtent(f.Palette, n.Flags&flagSmall != 0)
		n.Pref = [2]float32{w, h}
		if n.Text != "" {
			control, _ := components.SwitchExtent(f.Palette, n.Flags&flagSmall != 0)
			n.Pref[axisX] = control + captionGap + f.Surface.TextWidthWeight(n.Text, f.CaptionSize(n), gfx.WeightMedium)
		}
	case kindSegmented:
		var width float32 = 8
		for _, option := range widgetsOf(f).segmented[n.Aux].options {
			width += f.Surface.TextWidthWeight(option.Label, f.Palette.Metrics.FontSizeM, gfx.WeightMedium) + 28
		}
		n.Pref = [2]float32{width, f.Palette.Metrics.ControlHeight}
	case kindInput:
		n.Pref = [2]float32{n.PadX(), f.InputLine(n) + n.PadY()}
		if n.Flags&flagThemed != 0 {
			n.Pref = [2]float32{0, f.Palette.Metrics.ControlHeight}
		}
	case kindInputGroup:
		n.Pref = [2]float32{0, f.Palette.Metrics.ControlHeight}
		if n.Kind == kindInputGroup && widgetsOf(f).groups[n.Aux].Multiline {
			n.Pref[axisY] = 64
		}
	case kindTextarea:
		n.Pref = [2]float32{n.PadX(), f.Palette.Metrics.FontSizeM + 7 + n.PadY()}
		if n.Flags&flagThemed != 0 {
			n.Pref = [2]float32{0, 64}
		}
	case kindTable, kindTabs, kindSelect:
		f.measureWidget(n)
	case kindProgress:
		n.Pref = [2]float32{progressWidth, progressHeight}
	case kindSpinner:
		n.Pref = [2]float32{16, 16}
	case kindSplitter:
		f.measureSplitter(n)
	case kindSlider:
		f.measureSlider(n)
	}
}

func (f *frame) paintButton(n *node) {
	r := n.Rect
	extra := &f.Exts[n.Ext]
	button := components.BeginButton(f.Surface, f.Palette, n.ID, Rect{X: r.X, Y: r.Y, Width: r.Width}).Size(n.ButtonSize).
		WithIcon(extra.icon).IconTransform(extra.iconTransform)
	if f.styles != nil && f.styles.Button != nil {
		button = button.Recipe(f.styles.Button)
	}
	if extra.buttonRecipe != nil {
		button = button.Recipe(extra.buttonRecipe)
	}
	// A button is as tall as its size says unless the element was given more or less
	// room: a Height, or a Grow along a column. It then fills that room.
	if h := r.Height; h > 0 && math.Abs(float64(h-button.BoundsHeight())) > 0.01 {
		button = button.Height(h)
	}
	if n.Flags&flagDisabled != 0 {
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

// Choice adds a segmented control choosing among labels, selected being the index of
// the current one (negative for none). Its options are identified by scope.At(0),
// scope.At(1), ...: declare a scope for it once (tree.Sub(page, tree.Name("theme"))),
// and read the click with Box.ChosenIndex.
//
//	if i, ok := f.Choice(themeScope, themeLabels, m.theme).ChosenIndex(); ok { m.theme = i }
func (f *frame) Choice(scope gfx.Scope, labels []string, selected int) Box {
	first := len(widgetsOf(f).choices)
	for index, label := range labels {
		widgetsOf(f).choices = append(widgetsOf(f).choices, components.SegmentOption{ID: scope.At(index), Label: label})
	}
	options := widgetsOf(f).choices[first:len(widgetsOf(f).choices):len(widgetsOf(f).choices)]
	chosen := gfx.ID{}
	if selected >= 0 && selected < len(labels) {
		chosen = scope.At(selected)
	}
	return f.Segmented(options, chosen)
}

func (f *frame) paintInput(n *node) {
	// What the element says of its text (FontSize, Weight, Color) is the field's text style,
	// over the snapshot's own: the one value the glyphs, the caret and the platform follow.
	// What it says of its alignment is the same, but a field is start-aligned unless it says
	// otherwise, so the snapshot's own declaration is left alone only by an alignment it
	// declares. What the snapshot leaves at AlignLeft the element's TextEnd() takes.
	snapshot := widgetsOf(f).inputs[n.Aux]
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
	if n.Flags&flagHasColor != 0 {
		snapshot.Text.Color = n.Color
	}
	if n.Flags&flagMono != 0 {
		snapshot.Text.Mono, snapshot.Text.MonoFace = true, n.Face
	}
	if a := f.Exts[n.Ext].addons; a != 0 {
		f.paintAddonsFn(f, n, snapshot, &f.addons[a-1])
		return
	}
	components.PaintInputContent(f.Surface, f.Palette, n.Rect, f.ContentRect(n), snapshot)
}

// paintTextarea paints the rows of a Textarea or TextareaBase inside the element's Pad and
// Border, as paintInput does for a line.
func (f *frame) paintTextarea(n *node) {
	if n.Flags&flagHasColor != 0 {
		theme := f.Palette
		theme.Foreground = n.Color
		components.PaintTextareaContent(f.Surface, theme, n.Rect, f.ContentRect(n), widgetsOf(f).textareas[n.Aux])
		return
	}
	components.PaintTextareaContent(f.Surface, f.Palette, n.Rect, f.ContentRect(n), widgetsOf(f).textareas[n.Aux])
}
