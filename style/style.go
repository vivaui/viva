// Package style is the vocabulary a control's look is chosen with: its variant
// and its size. The engine and the styled controls both speak it, so the engine
// does not depend on a particular design system; package components aliases
// these types for the buttons and other controls it paints.
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
