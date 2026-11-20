package ui

import (
	"context"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/window"
)

// The engine and its declarations moved to package base; the functions here
// are their names in package ui, so a page keeps importing ui alone.

// EffectState is base's; a page names it when it holds an Effect's work.
type EffectState[T any, K comparable] = base.EffectState[T, K]

// The words of a page that base declares as constants.

const (
	Alt                = base.Alt
	Baseline           = base.Baseline
	Between            = base.Between
	Ctrl               = base.Ctrl
	DimFill            = base.DimFill
	DimPercent         = base.DimPercent
	DirectionColumn    = base.DirectionColumn
	DirectionCustom    = base.DirectionCustom
	DirectionFlow      = base.DirectionFlow
	DirectionGrid      = base.DirectionGrid
	DirectionRow       = base.DirectionRow
	DirectionStack     = base.DirectionStack
	DrawerRight        = base.DrawerRight
	FlagAlertDialog    = base.FlagAlertDialog
	FlagBasis          = base.FlagBasis
	FlagChildren       = base.FlagChildren
	FlagClip           = base.FlagClip
	FlagDrawer         = base.FlagDrawer
	FlagGrowSet        = base.FlagGrowSet
	FlagHasHit         = base.FlagHasHit
	FlagHasRadius      = base.FlagHasRadius
	FlagLabelFor       = base.FlagLabelFor
	FlagOpen           = base.FlagOpen
	FlagOutline        = base.FlagOutline
	FlagScroll         = base.FlagScroll
	FlagSelected       = base.FlagSelected
	FlagSetBackground  = base.FlagSetBackground
	FlagSetBorder      = base.FlagSetBorder
	FlagSetPad         = base.FlagSetPad
	FlagSetShadow      = base.FlagSetShadow
	FlagShrinkSet      = base.FlagShrinkSet
	FlagTextCenter     = base.FlagTextCenter
	FlagVeto           = base.FlagVeto
	FlagWrap           = base.FlagWrap
	FocusBoth          = base.FocusBoth
	FocusHorizontal    = base.FocusHorizontal
	FocusVertical      = base.FocusVertical
	KindContainer      = base.KindContainer
	KindToggle         = base.KindToggle
	KindIcon           = base.KindIcon
	KindImage          = base.KindImage
	KindSlot           = base.KindSlot
	KindSpacer         = base.KindSpacer
	KindText           = base.KindText
	ListCenter         = base.ListCenter
	ListEnd            = base.ListEnd
	ListStart          = base.ListStart
	PaginationEllipsis = base.PaginationEllipsis
	PaginationNext     = base.PaginationNext
	PaginationPage     = base.PaginationPage
	PaginationPrevious = base.PaginationPrevious
	Primary            = base.Primary
	Shift              = base.Shift
	Start              = base.Start
	Stretch            = base.Stretch
	ScrollAreaBarSize  = base.ScrollAreaBarSize
	ToBottom           = base.ToBottom
	ToBottomLeft       = base.ToBottomLeft
	ToBottomRight      = base.ToBottomRight
	ToLeft             = base.ToLeft
	ToRight            = base.ToRight
	ToTop              = base.ToTop
	ToTopLeft          = base.ToTopLeft
	ToTopRight         = base.ToTopRight
	ToastBottomCenter  = base.ToastBottomCenter
	ToastBottomLeft    = base.ToastBottomLeft
	ToastBottomRight   = base.ToastBottomRight
	ToastDefault       = base.ToastDefault
	ToastTopCenter     = base.ToastTopCenter
	ToastTopLeft       = base.ToastTopLeft
	ToastTopRight      = base.ToastTopRight
	TooltipBottom      = base.TooltipBottom
	TooltipLeft        = base.TooltipLeft
	TooltipRight       = base.TooltipRight
	TooltipTop         = base.TooltipTop
	TreeDropAfter      = base.TreeDropAfter
	TreeDropBefore     = base.TreeDropBefore
	TreeDropInside     = base.TreeDropInside
)

// Leaves and layout.

func Row(c C) Box { return base.Row(c) }

func Column(c C) Box { return base.Column(c) }

func Stack(c C) Box { return base.Stack(c) }

func Divider(c C) Box { return base.Divider(c) }

func Spacer(c C) Box { return base.Spacer(c) }

func Slot(c C) Box { return base.Slot(c) }

func Flow(c C) Box { return base.Flow(c) }

func Grid(c C, minCell float32) Box { return base.Grid(c, minCell) }

func CustomLayout(c C, layout Layout) Box { return base.CustomLayout(c, layout) }

func Scroll(c C) Box { useScroll(); return base.Scroll(c) }

func ScrollAt(c C, offset float32) Box { useScroll(); return base.ScrollAt(c, offset) }

func Textf(c C, format string, args ...any) Box { return base.Textf(c, format, args...) }

func Text(c C, Value string) Box { return base.Text(c, Value) }

func Paragraph(c C, Value string, MaxLines int) Box { return base.Paragraph(c, Value, MaxLines) }

func Wrap(c C, Value string) Box { return base.Wrap(c, Value) }

func Icon(c C, icon gfx.Icon) Box { return base.Icon(c, icon) }

func Image(c C, bitmap gfx.Bitmap) Box { return base.Image(c, bitmap) }

func Progress(c C, fraction float32) Box { useIndicators(); return base.Progress(c, fraction) }

func Spinner(c C) Box { useIndicators(); return base.Spinner(c) }

func Kbd(c C, keys ...string) Box { return base.Kbd(c, keys...) }

// Controls and their bases.

func Badge(c C, Label string) Box { useBadge(); return base.Badge(c, Label) }

func Button(c C, Label string) Box { useButton(); return base.Button(c, Label) }

func ButtonBase(c C, Label string) Box { useButton(); return base.ButtonBase(c, Label) }

func Checkbox(c C, Value *bool, Label string) Box {
	useCheckbox()
	return base.Checkbox(c, Value, Label)
}

func CheckboxBase(c C, Value *bool, Label string) Box {
	useCheckbox()
	return base.CheckboxBase(c, Value, Label)
}

func Switch(c C, Value *bool, Label string) Box { useSwitch(); return base.Switch(c, Value, Label) }

func SwitchBase(c C, Value *bool, Label string) Box {
	useSwitch()
	return base.SwitchBase(c, Value, Label)
}

func ToggleBase(c C, Value *bool, Label string) Box {
	useToggle()
	return base.ToggleBase(c, Value, Label)
}

func Toggle(c C, Value *bool, Label string) Box { useToggle(); return base.Toggle(c, Value, Label) }

func DisclosureBase(c C, Value *bool, Label string) Box {
	useToggle()
	return base.DisclosureBase(c, Value, Label)
}

func Control(c C, Label string, h *Handlers) Box { return base.Control(c, Label, h) }

func FormField(c C, state *FormFieldState, Label string) Box {
	return base.FormField(c, state, Label)
}

func Choice[T comparable](c C, Value *T, choices []T, Label func(T) string) Box {
	useSegmented()
	return base.Choice(c, Value, choices, Label)
}

func Radio[T comparable](c C, Value *T, option T, Label string) Box {
	useCheckbox()
	return base.Radio(c, Value, option, Label)
}

func RadioID[T comparable](c C, ID gfx.ID, Value *T, option T, Label string) Box {
	useCheckbox()
	return base.RadioID(c, ID, Value, option, Label)
}

func RadioBase[T comparable](c C, Value *T, option T, Label string) Box {
	useCheckbox()
	return base.RadioBase(c, Value, option, Label)
}

func RadioGroup(c C) Box { return base.RadioGroup(c) }

func TabBase[T comparable](c C, Value *T, tab T, Label string) Box {
	useTabs()
	return base.TabBase(c, Value, tab, Label)
}

func Tabs[T comparable](c C, Value *T, tabs []T, Label func(T) string) Box {
	useTabs()
	return base.Tabs(c, Value, tabs, Label)
}

func Select[T comparable](c C, Value *T, choices []T, Label func(T) string) Box {
	useSelect()
	return base.Select(c, Value, choices, Label)
}

func SelectID[T comparable](c C, ID gfx.ID, Value *T, choices []T, Label func(T) string) Box {
	useSelect()
	return base.SelectID(c, ID, Value, choices, Label)
}

func SelectBase[T comparable](c C, Value *T, choices []T, Label func(T) string) Box {
	useSelect()
	return base.SelectBase(c, Value, choices, Label)
}

func Combobox[T comparable](c C, Value *T, choices []T, nameOf func(T) string, Label string) Box {
	useCombo()
	return base.Combobox(c, Value, choices, nameOf, Label)
}

func ComboKey(Value string) (index int, isKey bool) { return base.ComboKey(Value) }

func ColorSwatch(c C, Color Color, selected bool, Label string) Box {
	useColor()
	return base.ColorSwatch(c, Color, selected, Label)
}

func ColorPicker(c C, state *ColorPickerState, Color *Color, Label string) Box {
	useColor()
	return base.ColorPicker(c, state, Color, Label)
}

func ColorPickerID(c C, ID gfx.ID, state *ColorPickerState, Color *Color, Label string) Box {
	useColor()
	return base.ColorPickerID(c, ID, state, Color, Label)
}

func Slider(c C, state *SliderState) Box { useSlider(); return base.Slider(c, state) }

func SliderBase(c C, state *SliderState) Box { useSlider(); return base.SliderBase(c, state) }

// NewRangeSliderState returns a range slider holding low and high.
func NewRangeSliderState(low, high float32) *RangeSliderState {
	return base.NewRangeSliderState(low, high)
}

// RangeSlider is a two-thumb slider over one track: Low and High are the
// values, Min, Max and Step the range. It is a Control, so its thumbs take the
// focus and the keyboard.
func RangeSlider(c C, state *RangeSliderState) Box {
	if state == nil {
		panic("ui: RangeSlider needs its RangeSliderState")
	}
	if state.Handlers.Key == nil {
		state.Handlers.Key = state.Key
		state.Handlers.Pointer = state.Pointer
	}
	if state.Paint == nil {
		state.Paint = func(d *base.DrawContext) { paintRangeSlider(d, state) }
	}
	return Control(c, "Range", &state.Handlers).
		Height(c.Theme().Metrics.ControlHeight).
		Draw(state.DrawFunc())
}

func Splitter(c C, state *SplitterState) Box { useSplitter(); return base.Splitter(c, state) }

func SplitButton(c C, spec SplitButtonSpec) Box { useSplitButton(); return base.SplitButton(c, spec) }

func Pagination(c C, Items []PaginationItem) Box { usePagination(); return base.Pagination(c, Items) }

// Fields and text entry.

func TextField[T TextValue](c C, Value T, Label, placeholder string) Box {
	useField()
	return base.TextField(c, Value, Label, placeholder)
}

func TextFieldID(c C, ID gfx.ID, field *Field, Label, placeholder string) Box {
	useField()
	return base.TextFieldID(c, ID, field, Label, placeholder)
}

func TextFieldBase[T TextValue](c C, Value T, Label, placeholder string) Box {
	useField()
	return base.TextFieldBase(c, Value, Label, placeholder)
}

func TextArea[T TextValue](c C, Value T, Label, placeholder string) Box {
	useField()
	return base.TextArea(c, Value, Label, placeholder)
}

func TextAreaID(c C, ID gfx.ID, field *Field, Label, placeholder string) Box {
	useField()
	return base.TextAreaID(c, ID, field, Label, placeholder)
}

func Fields(fields ...*Field) func(window.TextInputContext) *window.TextInput {
	useField()
	return base.Fields(fields...)
}

func FocusBind[T comparable](Box Box, field *T, Value T) Box {
	return base.FocusBind(Box, field, Value)
}

func FocusedValue[T comparable](c C, field *T) T { return base.FocusedValue(c, field) }

func BindIconHover(state *base.IconHoverState, ID gfx.ID) IconHover {
	return base.BindIconHover(state, ID)
}

// Dialogs, menus and other overlays.

func Dialog(c C, spec DialogSpec) Box { useDialog(); return base.Dialog(c, spec) }

func AlertDialog(c C, spec AlertDialogSpec) Box { useAlert(); return base.AlertDialog(c, spec) }

func Drawer(c C, open *bool, title string, spec DrawerSpec, body, footer func()) bool {
	useDrawer()
	return base.Drawer(c, open, title, spec, body, footer)
}

func Modal(c C, open *bool, title string, body, footer func()) bool {
	useDialog()
	return base.Modal(c, open, title, body, footer)
}

func ModalWith(c C, open *bool, title string, spec ModalSpec, body, footer func()) bool {
	useDialog()
	return base.ModalWith(c, open, title, spec, body, footer)
}

func Popover(c C, Anchor Box, open *bool, body func()) bool {
	usePopover()
	return base.Popover(c, Anchor, open, body)
}

func PopoverWith(c C, Anchor Box, spec PopoverSpec, open *bool, body func()) bool {
	usePopover()
	return base.PopoverWith(c, Anchor, spec, open, body)
}

func Menu(c C, Anchor Box, open *bool, body func(*MenuItems)) {
	useMenu()
	base.Menu(c, Anchor, open, body)
}

func ContextMenu(c C, target Box, body func(*MenuItems)) {
	useMenu()
	base.ContextMenu(c, target, body)
}

func ContextMenuAt(c C, target Box, open *bool, at gfx.Point, body func(*MenuItems)) {
	useMenu()
	base.ContextMenuAt(c, target, open, at, body)
}

func Overlay(c C, spec OverlaySpec) Box { return base.Overlay(c, spec) }

func PopupLayer(c C, layer uint16) { base.PopupLayer(c, layer) }

// Lists, tables, trees and effects.

func List(c C, state *ListState, n int, row func(ListRow)) Box {
	useScroll()
	return base.List(c, state, n, row)
}

func Table(c C, state *TableState, spec TableSpec) Box {
	useTable()
	return base.Table(c, state, spec)
}

func Tree(c C, state *TreeState, n int, Label func(i int) string, Content func(i int)) Box {
	useTree()
	return base.Tree(c, state, n, Label, Content)
}

func Memo[T comparable](c C, Value T, fn func()) Box { return base.Memo(c, Value, fn) }

func MemoCheck(on bool) { base.MemoCheck(on) }

func State[T any](c C, Name string) *T { return base.State[T](c, Name) }

func NewEffect[T any, K comparable](run func(context.Context) (T, error)) *EffectState[T, K] {
	return base.NewEffect[T, K](run)
}

func NewEffectKeyed[T any, K comparable](run func(context.Context, K) (T, error)) *EffectState[T, K] {
	return base.NewEffectKeyed[T, K](run)
}

func Effect[T any, K comparable](c C, state *EffectState[T, K], key K) {
	base.Effect(c, state, key)
}

// LayoutScrollArea computes a scroll area's bars without painting them: the
// component and ScrollState.BarGeometry use the same layout.
func LayoutScrollArea(bounds gfx.Rect, content gfx.Size, layout ScrollAreaLayout) ScrollAreaGeometry {
	return base.LayoutScrollArea(bounds, content, layout)
}

// Tree navigation.

type TreeNavigationItem = base.TreeNavigationItem

type TreeNavigationAction = base.TreeNavigationAction

const (
	TreeNavigationNone     = base.TreeNavigationNone
	TreeNavigationMove     = base.TreeNavigationMove
	TreeNavigationExpand   = base.TreeNavigationExpand
	TreeNavigationCollapse = base.TreeNavigationCollapse
)

func NavigateTree(items []TreeNavigationItem, current int, key gfx.Key, pageRows int) (int, TreeNavigationAction, bool) {
	return base.NavigateTree(items, current, key, pageRows)
}

// Identity and input.

func ID(c C) gfx.ID { return base.ID(c) }

func NewPage(config PageConfig) *Page { useTooltip(); return base.NewPage(config) }

func Shortcut(c C, modifiers gfx.Modifiers, key string) bool {
	return base.Shortcut(c, modifiers, key)
}
