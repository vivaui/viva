package ui

// The control layer: how each control kind looks and how large it is. The
// engine keeps the seam (controlPainter, controlMeasurer) and no longer names
// these kinds here, so the engine can move below the styled controls; this file
// moves with them.
import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// paintControls paints the controls: each kind's look, from package components
// for the ones that ship with a theme, and from the caller's own painter for
// the ones it declared.
func (f *frame) paintControls(n *node, r Rect) {
	if painter := controlPainters[n.kind]; painter != nil {
		painter(f, n, r)
	}
}

// controlPainters paints one control kind. It is an array, not a map: the kind
// is a small number and the lookup must not hash on the frame's path.
var controlPainters = [...]func(f *frame, n *node, r Rect){
	kindButton:      (*frame).paintButtonControl,
	kindCheckbox:    (*frame).paintCheckboxControl,
	kindSwitch:      (*frame).paintSwitchControl,
	kindInput:       (*frame).paintInputControl,
	kindInputGroup:  (*frame).paintInputGroupControl,
	kindTextarea:    (*frame).paintTextareaControl,
	kindSegmented:   (*frame).paintSegmentedControl,
	kindTable:       (*frame).paintWidgetControl,
	kindTabs:        (*frame).paintWidgetControl,
	kindSelect:      (*frame).paintWidgetControl,
	kindProgress:    (*frame).paintProgressControl,
	kindSpinner:     (*frame).paintSpinnerControl,
	kindSplitter:    (*frame).paintSplitterControl,
	kindSplitButton: (*frame).paintSplitButtonControl,
	kindBadge:       (*frame).paintBadgeControl,
	kindPagination:  (*frame).paintPaginationControl,
	kindSlider:      (*frame).paintSliderControl,
}

func (f *frame) paintButtonControl(n *node, _ Rect)      { f.paintButton(n) }
func (f *frame) paintInputControl(n *node, _ Rect)       { f.paintInput(n) }
func (f *frame) paintTextareaControl(n *node, _ Rect)    { f.paintTextarea(n) }
func (f *frame) paintProgressControl(n *node, _ Rect)    { f.paintProgress(n) }
func (f *frame) paintSpinnerControl(n *node, _ Rect)     { f.paintSpinner(n) }
func (f *frame) paintSplitterControl(n *node, _ Rect)    { f.paintSplitter(n) }
func (f *frame) paintSliderControl(n *node, _ Rect)      { f.paintSlider(n) }
func (f *frame) paintWidgetControl(n *node, _ Rect)      { f.paintWidget(n) }
func (f *frame) paintSplitButtonControl(n *node, r Rect) { f.paintSplitButton(viewOf(f, n)) }
func (f *frame) paintBadgeControl(n *node, _ Rect)       { f.paintBadge(viewOf(f, n)) }
func (f *frame) paintPaginationControl(n *node, _ Rect)  { f.paintPagination(viewOf(f, n)) }

func (f *frame) paintCheckboxControl(n *node, r Rect)   { f.paintCheckbox(viewOf(f, n)) }
func (f *frame) paintSwitchControl(n *node, r Rect)     { f.paintSwitch(viewOf(f, n)) }
func (f *frame) paintInputGroupControl(n *node, r Rect) { f.paintInputGroup(viewOf(f, n)) }
func (f *frame) paintSegmentedControl(n *node, r Rect)  { f.paintSegmented(viewOf(f, n)) }

func (f *frame) paintCheckbox(v ControlView) {
	canvas := f.canvas
	side := components.CheckboxExtent(f.theme)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-side)/2, Y: v.Rect.Y + (v.Rect.Height-side)/2, Width: side, Height: side}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.paintCaptionView(v, bounds)
	}
	checkbox := components.BeginCheckbox(canvas, f.theme, v.ID, bounds, v.Flags&flagChecked != 0)
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
	canvas := f.canvas
	small := v.Flags&flagSmall != 0
	width, height := components.SwitchExtent(f.theme, small)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-width)/2, Y: v.Rect.Y + (v.Rect.Height-height)/2, Width: width, Height: height}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.paintCaptionView(v, bounds)
	}
	control := components.BeginSwitch(canvas, f.theme, v.ID, bounds, v.Flags&flagChecked != 0)
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
	components.PaintInputGroup(f.canvas, f.theme, v.Rect, f.groups[v.Aux])
}

func (f *frame) paintSegmented(v ControlView) {
	choice := f.segmented[v.Aux]
	components.BeginSegmentedControl(f.canvas, f.theme, v.Rect, choice.options, choice.selected).Paint()
}

// measureControls measures the controls: each kind's geometry, from package
// components for the ones that ship with a theme.
func (f *frame) measureControls(n *node) {
	switch n.kind {
	case kindButton:
		button := components.BeginButton(f.canvas, f.theme, gfx.ID{}, Rect{}).Size(n.buttonSize).WithIcon(f.exts[n.ext].icon)
		if d := n.size[axisY]; d.kind == dimPx {
			button = button.Height(d.value) // an icon button is as wide as it is tall
		}
		button = button.SizeToFit(n.label)
		n.pref = [2]float32{button.BoundsWidth(), button.BoundsHeight()}
		if n.buttonSize >= components.ButtonSizeIcon {
			n.pref[axisX] = n.pref[axisY]
		}
	case kindBadge:
		w, h := components.BadgeExtent(f.canvas, f.theme, n.text)
		n.pref = [2]float32{w, h}
	case kindPagination:
		w, h := components.PaginationExtent(f.canvas, f.theme, f.paginationItems(n))
		n.pref = [2]float32{w, h}
	case kindSplitButton:
		button := components.BeginButton(f.canvas, f.theme, gfx.ID{}, Rect{}).Size(n.buttonSize).SizeToFit(n.label)
		trigger := widgetsOf(f).splitButtons[n.aux].triggerWidth
		if trigger <= 0 {
			trigger = splitButtonTrigger
		}
		n.pref = [2]float32{button.BoundsWidth() + trigger, button.BoundsHeight()}
	case kindCheckbox:
		w, h := components.CheckboxHitExtent(f.theme)
		n.pref = [2]float32{w, h}
		if n.text != "" {
			n.pref[axisX] = components.CheckboxExtent(f.theme) + captionGap + f.canvas.TextWidthWeight(n.text, f.captionSize(n), gfx.WeightMedium)
		}
	case kindSwitch:
		w, h := components.SwitchHitExtent(f.theme, n.flags&flagSmall != 0)
		n.pref = [2]float32{w, h}
		if n.text != "" {
			control, _ := components.SwitchExtent(f.theme, n.flags&flagSmall != 0)
			n.pref[axisX] = control + captionGap + f.canvas.TextWidthWeight(n.text, f.captionSize(n), gfx.WeightMedium)
		}
	case kindSegmented:
		var width float32 = 8
		for _, option := range f.segmented[n.aux].options {
			width += f.canvas.TextWidthWeight(option.Label, f.theme.Metrics.FontSizeM, gfx.WeightMedium) + 28
		}
		n.pref = [2]float32{width, f.theme.Metrics.ControlHeight}
	case kindInput:
		n.pref = [2]float32{n.padX(), f.inputLine(n) + n.padY()}
		if n.flags&flagThemed != 0 {
			n.pref = [2]float32{0, f.theme.Metrics.ControlHeight}
		}
	case kindInputGroup:
		n.pref = [2]float32{0, f.theme.Metrics.ControlHeight}
		if n.kind == kindInputGroup && f.groups[n.aux].Multiline {
			n.pref[axisY] = 64
		}
	case kindTextarea:
		n.pref = [2]float32{n.padX(), f.theme.Metrics.FontSizeM + 7 + n.padY()}
		if n.flags&flagThemed != 0 {
			n.pref = [2]float32{0, 64}
		}
	case kindTable, kindTabs, kindSelect:
		f.measureWidget(n)
	case kindProgress:
		n.pref = [2]float32{progressWidth, progressHeight}
	case kindSpinner:
		n.pref = [2]float32{16, 16}
	case kindSplitter:
		f.measureSplitter(n)
	case kindSlider:
		f.measureSlider(n)
	}
}
