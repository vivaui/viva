package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/text"
)

// The words of package gfx a page uses, so a page imports ui alone. They are
// package base's names, the behavior layer's vocabulary; ui re-exports them as
// the permanent application facade.
type Color = base.Color

// Breakpoint is where a responsive change begins (Box.From, C.AtLeast): a window
// size class, or Width(n) for the nearest Container.
type Breakpoint = base.Breakpoint

// The window size classes, by the page's width in logical pixels.
const (
	Compact    = base.Compact    // from 0: phones held upright
	Medium     = base.Medium     // from 600: phones on their side, tablets held upright, narrow windows
	Expanded   = base.Expanded   // from 840: tablets on their side, most desktop windows
	Large      = base.Large      // from 1200: large desktop windows
	ExtraLarge = base.ExtraLarge // from 1600: wide screens
)

// Status is the outcome a control reports through color, one vocabulary for an
// alert, a tag and a field.
type Status = style.Status

const (
	StatusDefault = style.StatusDefault
	StatusSuccess = style.StatusSuccess
	StatusWarning = style.StatusWarning
	StatusDanger  = style.StatusDanger
)

type Rect = base.Rect
type Size = base.Size
type Point = base.Point
type Theme = base.Theme

// SliderMark is a value of a Slider's range marked on its track; see Box.Marks.
type SliderMark = base.SliderMark

// Option is a setting a control takes after its fixed arguments, as WithID.
type Option = base.Option

// LightTheme is the light theme.
func LightTheme() Theme { return base.LightTheme() }

// DarkTheme is the dark theme.
func DarkTheme() Theme { return base.DarkTheme() }

// RGB is an opaque color from 8-bit channels.
func RGB(red, green, blue uint8) Color { return base.RGB(red, green, blue) }

// RGBA is a color from 8-bit channels and an opacity from 0 to 1.
func RGBA(red, green, blue uint8, alpha float32) Color { return base.RGBA(red, green, blue, alpha) }

// Identity is the identity of a control: what ID, Key and Part name, what a
// Handlers target is, what a DialogSpec's buttons are.
type Identity = gfx.ID

// The values a custom control's Handlers and a Draw callback work with.
type (
	Canvas       = gfx.Canvas
	Bitmap       = gfx.Bitmap
	KeyEvent     = gfx.KeyEvent
	PointerEvent = gfx.PointerEvent
	ScrollEvent  = gfx.ScrollEvent
	EventResult  = gfx.EventResult
	Modifiers    = gfx.Modifiers
	Key          = gfx.Key
	Role         = gfx.Role
	// ControlState is the hovered, pressed, focused, selected and disabled state
	// a recipe and a Draw callback are given.
	ControlState = gfx.State
	// TextEditor is the editor a Field binds to.
	TextEditor = text.Editor
)

// Font weights, for Box.Weight.
const (
	WeightRegular  = gfx.WeightRegular
	WeightMedium   = gfx.WeightMedium
	WeightSemibold = gfx.WeightSemibold
	WeightBold     = gfx.WeightBold
)

// ShadowLevel is an elevation, for Box.Shadow.
type ShadowLevel = gfx.ShadowLevel

const (
	ShadowNone = gfx.ShadowNone
	ShadowXS   = gfx.ShadowXS
	ShadowSM   = gfx.ShadowSM
	ShadowMD   = gfx.ShadowMD
	ShadowLG   = gfx.ShadowLG
)

// TextDirection is a paragraph's direction, for Box.TextDirection.
type TextDirection = gfx.TextDirection

const (
	DirectionAuto = gfx.DirectionAuto
	DirectionLTR  = gfx.DirectionLTR
	DirectionRTL  = gfx.DirectionRTL
)

// TextAlignment places a field's text, for Field.Alignment.
type TextAlignment = gfx.Alignment

const (
	TextAlignLeft   = gfx.AlignLeft
	TextAlignCenter = gfx.AlignCenter
	TextAlignRight  = gfx.AlignRight
	TextAlignStart  = gfx.AlignStart
	TextAlignEnd    = gfx.AlignEnd
)

// ButtonVariant is a button's look among the theme's variants, for Box.Variant
// and the specs whose buttons take one.
type ButtonVariant = style.Variant

const (
	ButtonDefault            = style.VariantDefault
	ButtonDestructive        = style.VariantDestructive
	ButtonOutline            = style.VariantOutline
	ButtonSecondary          = style.VariantSecondary
	ButtonGhost              = style.VariantGhost
	ButtonLink               = style.VariantLink
	ButtonDestructiveOutline = style.VariantDestructiveOutline
)

// The looks a control's recipe returns, for Box.ButtonRecipe and its siblings.
type (
	ButtonLook    = style.ButtonLook
	SwitchLook    = style.SwitchLook
	CheckboxLook  = style.CheckboxLook
	TextEntryLook = style.TextEntryLook
)

// TableColumn is the width and alignment of one column of a Table.
type TableColumn = style.TableColumn

// TableAlignment places the text of a table column.
type TableAlignment = style.TableAlignment

const (
	TableAlignLeft   = style.TableAlignLeft
	TableAlignCenter = style.TableAlignCenter
	TableAlignRight  = style.TableAlignRight
)
