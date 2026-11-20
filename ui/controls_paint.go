package ui

import (
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

// measureButtonNode sizes a button that paints its own label and icon.
func measureButtonNode(f *Frame, n *Node) { n.Pref = measureButton(f, n) }

// measureBadgeNode gives a Badge the extent its component wants.
func measureBadgeNode(f *Frame, n *Node) {
	w, h := components.BadgeExtent(f.Surface, f.Palette, n.Text)
	n.Pref = [2]float32{w, h}
}

// measurePaginationNode gives a Pagination the extent its items want.
func measurePaginationNode(f *Frame, n *Node) {
	w, h := components.PaginationExtent(f.Surface, f.Palette, f.PaginationItems(n))
	n.Pref = [2]float32{w, h}
}

// measureSplitButtonNode gives a SplitButton its label's width and its trigger.
func measureSplitButtonNode(f *Frame, n *Node) {
	button := components.BeginButton(f.Surface, f.Palette, gfx.ID{}, Rect{}).Size(n.ButtonSize).SizeToFit(n.Label)
	trigger := base.WidgetsOf(f).GetSplitButtons()[n.Aux].GetTriggerWidth()
	if trigger <= 0 {
		trigger = SplitButtonTrigger
	}
	n.Pref = [2]float32{button.BoundsWidth() + trigger, button.BoundsHeight()}
}

func measureCheckboxNode(f *Frame, n *Node) {
	w, h := components.CheckboxHitExtent(f.Palette)
	n.Pref = [2]float32{w, h}
	if n.Text != "" {
		n.Pref[AxisX] = components.CheckboxExtent(f.Palette) + CaptionGap + f.Surface.TextWidthWeight(n.Text, f.CaptionSize(n), gfx.WeightMedium)
	}
}

func measureSwitchNode(f *Frame, n *Node) {
	w, h := components.SwitchHitExtent(f.Palette, n.Flags&FlagSmall != 0)
	n.Pref = [2]float32{w, h}
	if n.Text != "" {
		control, _ := components.SwitchExtent(f.Palette, n.Flags&FlagSmall != 0)
		n.Pref[AxisX] = control + CaptionGap + f.Surface.TextWidthWeight(n.Text, f.CaptionSize(n), gfx.WeightMedium)
	}
}

func measureToggleNode(f *Frame, n *Node) {
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
}

func measureSegmentedNode(f *Frame, n *Node) {
	var width float32 = 8
	for _, option := range base.WidgetsOf(f).GetSegmented()[n.Aux].GetOptions() {
		width += f.Surface.TextWidthWeight(option.Label, f.Palette.Metrics.FontSizeM, gfx.WeightMedium) + 28
	}
	n.Pref = [2]float32{width, f.Palette.Metrics.ControlHeight}
}

func measureInputNode(f *Frame, n *Node) {
	n.Pref = [2]float32{n.PadX(), f.InputLine(n) + n.PadY()}
	if n.Flags&FlagThemed != 0 {
		n.Pref = [2]float32{0, f.Palette.Metrics.ControlHeight}
	}
}

func measureInputGroupNode(f *Frame, n *Node) {
	n.Pref = [2]float32{0, f.Palette.Metrics.ControlHeight}
	if base.WidgetsOf(f).GetGroups()[n.Aux].Multiline {
		n.Pref[AxisY] = 64
	}
}

func measureTextareaNode(f *Frame, n *Node) {
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
}

// measureWidgetNode gives a Table, Tabs or Select the size its widget wants.
func measureWidgetNode(f *Frame, n *Node) { f.MeasureWidget(n) }

func measureProgressNode(f *Frame, n *Node) {
	n.Pref = [2]float32{ProgressWidth, ProgressHeight}
}

func measureSpinnerNode(f *Frame, n *Node) { n.Pref = [2]float32{16, 16} }

func measureSplitterNode(f *Frame, n *Node) { f.MeasureSplitter(n) }

func measureSliderNode(f *Frame, n *Node) { f.MeasureSlider(n) }

// buttonWeight is the weight of a button's label: the one the element asks for, or the
// medium weight the control has.
func buttonWeight(n *Node) float32 {
	if n.WeightSet || n.Weight != 0 {
		return n.Weight
	}
	return gfx.WeightMedium
}

// measureButton is the size a button that paints its own label and icon is laid out at:
// its size's height, and its content's width inside the element's Pad (which is the
// recipe's, the size's padding, unless the caller set it). A Height of the element's
// is the button's height, and an icon alone is as wide as it is tall. Padded down the
// side by the caller, it grows to hold its label and the padding.
func measureButton(f *Frame, n *Node) [2]float32 {
	height, _, fontSize, _ := style.ButtonGeometry(f.Palette.Metrics, n.ButtonSize)
	if n.FontSize > 0 {
		fontSize = n.FontSize
	}
	if d := n.Size[AxisY]; d.Kind == DimPx {
		height = d.Value
	}
	if n.ButtonSize >= components.ButtonSizeIcon {
		return [2]float32{height, height} // an icon button is as wide as it is tall
	}
	width := n.PadX() + f.Surface.TextWidthWeight(n.Label, fontSize, buttonWeight(n))
	if icon := f.Exts[n.Ext].GetIcon(); len(icon.Commands) > 0 || len(icon.Layers) > 0 {
		width += style.ButtonIconExtent(height)
		if n.Label != "" {
			width += n.Gap
		}
	}
	leading := fontSize + 7
	if n.LineH > 0 {
		leading = n.LineH
	}
	height = max(height, leading+n.PadY())
	return [2]float32{width, height}
}

// buttonContentArea is the room a button's content is centered in: the height inside its
// Pad and Border, and across, the widest span centered where the content is, which is
// the whole button when it is padded the same on both sides. The padding is not taken out
// of a label's width, so a label wider than a narrow button still centers on it.
func buttonContentArea(f *Frame, n *Node) Rect {
	content := f.ContentRect(n)
	center := content.X + content.Width/2
	half := min(center-n.Rect.X, n.Rect.X+n.Rect.Width-center)
	return Rect{X: center - half, Y: content.Y, Width: 2 * half, Height: content.Height}
}

// paintButton paints the content of a button that paints its own: its icon and its label,
// in the room its Pad and Border leave. The surface is the element's, dressed by the
// button's recipe before this runs, and the label takes the color the recipe gives it in
// the state the button is in unless the element has one of its own. Content the caller
// makes wider than the room inside the padding is cut at the content edge.
func paintButton(f *Frame, n *Node) {
	if n.Flags&FlagChildren != 0 {
		return
	}
	extra := &f.Exts[n.Ext]
	_, _, fontSize, iconOnly := style.ButtonGeometry(f.Palette.Metrics, n.ButtonSize)
	if n.FontSize > 0 {
		fontSize = n.FontSize
	}
	area := buttonContentArea(f, n)
	room := f.ContentRect(n)
	need := f.Surface.TextWidthWeight(n.Label, fontSize, buttonWeight(n))
	if icon := extra.GetIcon(); len(icon.Commands) != 0 || len(icon.Layers) != 0 {
		need += style.ButtonIconExtent(n.Rect.Height)
		if n.Label != "" {
			need += n.Gap
		}
	}
	if room.Width > 0 && need > room.Width {
		f.Surface.Clip(room)
		defer f.Surface.Unclip()
	}
	components.PaintButtonContent(f.Surface, area, components.ButtonContent{
		Label: n.Label, Icon: extra.GetIcon(), IconTransform: extra.GetIconTransform(),
		FontSize: fontSize, Weight: buttonWeight(n), Color: n.Color, Gap: n.Gap, Height: n.Rect.Height,
		IconOnly: iconOnly, Underlined: f.GetButtonLook().Underlined,
	})
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
