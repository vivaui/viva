package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/style"
)

// The engine is base's; ui keeps it under the names applications use.

// PopoverSide is the edge of an anchor a popover opens on.
type PopoverSide = style.PopoverSide

const (
	PopoverBottom = style.PopoverBottom
	PopoverTop    = style.PopoverTop
	PopoverRight  = style.PopoverRight
	PopoverLeft   = style.PopoverLeft
)

type AlertDialogDecl = base.AlertDialogDecl
type AlertDialogSpec = base.AlertDialogSpec
type Align = base.Align
type Angle = base.Angle
type Box = base.Box
type C = base.C
type ColorPickerLook = base.ColorPickerLook
type ColorPickerState = base.ColorPickerState
type ComboState = base.ComboState
type ComboboxOption = base.ComboboxOption
type ComboboxPaint = base.ComboboxPaint
type ControlView = base.ControlView
type Ctx = base.Ctx
type DialogDecl = base.DialogDecl
type DialogSpec = base.DialogSpec
type DimKind = base.DimKind
type Dimension = base.Dimension
type Direction = base.Direction
type DragState = base.DragState
type DrawContext = base.DrawContext
type DrawerDeclaration = base.DrawerDeclaration
type DrawerRects = base.DrawerRects
type DrawerSide = base.DrawerSide
type DrawerSpec = base.DrawerSpec
type DrawerSwipe = base.DrawerSwipe
type Extra = base.Extra
type Field = base.Field
type FieldAddons = base.FieldAddons
type FocusAxis = base.FocusAxis
type FocusOrder = base.FocusOrder
type FormFieldState = base.FormFieldState
type Frame = base.Frame
type Handlers = base.Handlers
type Host = base.Host
type IconHover = base.IconHover
type Kind = base.Kind
type Layout = base.Layout
type LayoutContext = base.LayoutContext
type ListAlign = base.ListAlign
type ListRow = base.ListRow
type ListState = base.ListState
type Look = base.Look
type MenuItem = base.MenuItem
type MenuItemParts = base.MenuItemParts
type MenuItems = base.MenuItems
type MenuRow = base.MenuRow
type MenuRowKind = base.MenuRowKind
type ModalSpec = base.ModalSpec
type Node = base.Node
type OverlaySpec = base.OverlaySpec
type Page = base.Page
type PageConfig = base.PageConfig
type PaginationDecl = base.PaginationDecl
type PaginationItem = base.PaginationItem
type PaginationKind = base.PaginationKind
type Place = base.Place
type PopoverDecl = base.PopoverDecl
type PopoverSpec = base.PopoverSpec
type ScrollState = base.ScrollState

// RangeSliderState is the two values and range of a RangeSlider.
type RangeSliderState = base.RangeSliderState

// ScrollAreaGeometry is the complete deterministic layout of one scroll area:
// its viewport, translated content, tracks, thumbs and corner.
type ScrollAreaGeometry = base.ScrollAreaGeometry

// ScrollAreaLayout is how those bars are laid out, for LayoutScrollArea.
type ScrollAreaLayout = base.ScrollAreaLayout
type SelectDecl = base.SelectDecl
type SelectState = base.SelectState
type Selection = base.Selection
type SliderDecl = base.SliderDecl
type SliderState = base.SliderState
type SplitButtonDecl = base.SplitButtonDecl
type SplitButtonSpec = base.SplitButtonSpec
type SplitDecl = base.SplitDecl
type SplitterState = base.SplitterState
type Styles = base.Styles
type TableDecl = base.TableDecl
type TableRows = base.TableRows
type TableSource = base.TableSource
type TableSpec = base.TableSpec
type TableState = base.TableState
type TabsDecl = base.TabsDecl
type TextValue = base.TextValue
type ToastID = base.ToastID
type ToastKind = base.ToastKind
type ToastPosition = base.ToastPosition
type Toaster = base.Toaster
type TooltipSide = base.TooltipSide
type TreeDrop = base.TreeDrop
type TreeMove = base.TreeMove
type TreeNode = base.TreeNode
type TreeRow = base.TreeRow
type TreeState = base.TreeState
type View = base.View
type Widgets = base.Widgets

const (
	Auto               = base.Auto
	AxisX              = base.AxisX
	AxisY              = base.AxisY
	CaptionGap         = base.CaptionGap
	Center             = base.Center
	DimAuto            = base.DimAuto
	DimPx              = base.DimPx
	DrawerBottom       = base.DrawerBottom
	DrawerLeft         = base.DrawerLeft
	DrawerTop          = base.DrawerTop
	End                = base.End
	FlagChecked        = base.FlagChecked
	FlagDialog         = base.FlagDialog
	FlagDisabled       = base.FlagDisabled
	FlagHasColor       = base.FlagHasColor
	FlagInvalid        = base.FlagInvalid
	FlagMixed          = base.FlagMixed
	FlagMono           = base.FlagMono
	FlagPopover        = base.FlagPopover
	FlagSmall          = base.FlagSmall
	FlagThemed         = base.FlagThemed
	IndeterminateShare = base.IndeterminateShare
	KindBadge          = base.KindBadge
	KindButton         = base.KindButton
	KindCheckbox       = base.KindCheckbox
	KindInput          = base.KindInput
	KindPagination     = base.KindPagination
	KindProgress       = base.KindProgress
	KindSegmented      = base.KindSegmented
	KindSelect         = base.KindSelect
	KindSlider         = base.KindSlider
	KindSpinner        = base.KindSpinner
	KindSplitButton    = base.KindSplitButton
	KindSplitter       = base.KindSplitter
	KindSwitch         = base.KindSwitch
	KindTable          = base.KindTable
	KindTabs           = base.KindTabs
	KindTextarea       = base.KindTextarea
	MenuBar            = base.MenuBar
	MenuBeside         = base.MenuBeside
	MenuDismiss        = base.MenuDismiss
	MenuFocus          = base.MenuFocus
	MenuRowContent     = base.MenuRowContent
	MenuRowItem        = base.MenuRowItem
	MenuRowLabel       = base.MenuRowLabel
	MenuRowSeparator   = base.MenuRowSeparator
	MenuRowsPart       = base.MenuRowsPart
	PKToaster          = base.PKToaster
	ProgressHeight     = base.ProgressHeight
	ProgressWidth      = base.ProgressWidth
	ScrollbarLinger    = base.ScrollbarLinger
	SpinnerStep        = base.SpinnerStep
	TabIndicator       = base.TabIndicator
	ToastAction        = base.ToastAction
	ToastClose         = base.ToastClose
	ToastError         = base.ToastError
	ToastInfo          = base.ToastInfo
	ToastLayer         = base.ToastLayer
	ToastLoading       = base.ToastLoading
	ToastSuccess       = base.ToastSuccess
	ToastWarning       = base.ToastWarning
	TooltipLayer       = base.TooltipLayer
)

var stack = base.Stack

type DrawerDecl = base.DrawerDeclaration

const (
	MenuRowPageContent  = base.MenuRowPageContent
	SlotScrollbarFade   = base.SlotScrollbarFade
	KindInputGroup      = base.KindInputGroup
	SplitButtonTrigger  = base.SplitButtonTrigger
	IndeterminatePeriod = base.IndeterminatePeriod
)

var (
	PopoverFocusAskFor = base.PopoverFocusAskFor
	ClockEpoch         = base.ClockEpoch
)
