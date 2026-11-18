// Package style is the vocabulary a control's look is chosen with: variants and
// sizes, the resolved looks a recipe returns, and the values a painter reads
// (segment options, table columns, tooltip sides, and the like). It carries
// values only: behaviour, state machines and layout live in package base, whose
// recipes and painters both speak this vocabulary through package components.
//
// The direction is gfx -> style -> base -> ui, with components next to base
// above style. A type that changes state or measures with a canvas belongs in
// base, not here; if a value is only ever consumed by a painter, it belongs
// here.
package style

// Variant selects a control's look among the design system's variants.
type Variant uint8

const (
	// VariantDefault is the ordinary look.
	VariantDefault Variant = iota
	// VariantDestructive is for an action that cannot be undone.
	VariantDestructive
	// VariantOutline is only a rim.
	VariantOutline
	// VariantSecondary is a quiet fill.
	VariantSecondary
	// VariantGhost has no rim or fill at rest.
	VariantGhost
	// VariantLink looks like a link.
	VariantLink
	// VariantDestructiveOutline is a danger colored label on an outline button.
	VariantDestructiveOutline
)

// Size selects a control's height and padding.
type Size uint8

const (
	SizeDefault Size = iota
	SizeXSmall
	SizeSmall
	SizeLarge
	SizeIcon
	SizeIconXSmall
	SizeIconSmall
	SizeIconLarge
)

// ButtonVariant is Variant under the name a button's looks use.
type ButtonVariant = Variant
