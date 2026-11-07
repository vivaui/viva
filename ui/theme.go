package ui

const (
	WeightRegular  float32 = 0
	WeightMedium   float32 = 0.15
	WeightSemibold float32 = 0.23
	WeightBold     float32 = 0.40
)

// Density scales control geometry without touching colors.
type Density uint8

const (
	DensityRegular Density = iota
	DensityCompact
	DensityLarge
)

// Metrics carries the size tokens every component consumes. Derived themes
// replace these wholesale; components never hardcode geometry.
type Metrics struct {
	ControlHeight float32 // buttons, inputs, toggles
	RadiusControl float32 // control corner radius
	RadiusSurface float32 // card and container radius
	FontSizeS     float32 // captions, badges
	FontSizeM     float32 // body and controls
	FontSizeL     float32 // page titles
	PadControlX   float32 // horizontal control padding
	PadSurface    float32 // card interior padding
	GapItem       float32 // row-to-row spacing inside lists
}

// RadiusItem is the corner radius of a row inside a menu, list or popup: two
// pixels tighter than a control, so its corners sit inside the surface's.
func (m Metrics) RadiusItem() float32 { return max(m.RadiusControl-2, 0) }

// RadiusPopup is the corner radius of a floating surface that holds rows: a menu,
// a select list, a combobox or command panel. It is one pixel tighter than a
// dialog or card surface (RadiusSurface).
func (m Metrics) RadiusPopup() float32 { return max(m.RadiusSurface-1, 0) }

// RadiusFull is a corner radius no control is tall enough to reach: every
// rounded shape clamps its radius to half its shorter side, so this makes a
// pill (or a circle, on a square) of whatever it is given to.
const RadiusFull float32 = 1 << 14

// TextInsetX is where the text of a text-entry control starts, measured from its
// outer edge: the 1px border plus the control's horizontal padding less that
// pixel, as Vega's px-2.5 is at the default density (11).
func (m Metrics) TextInsetX() float32 { return max(m.PadControlX-1, 0) }

// DefaultMetrics returns the standard desktop density.
func DefaultMetrics() Metrics {
	return Metrics{
		ControlHeight: 32,
		RadiusControl: 7.5,
		RadiusSurface: 11,
		FontSizeS:     12,
		FontSizeM:     14,
		FontSizeL:     24,
		PadControlX:   12,
		PadSurface:    16,
		GapItem:       8,
	}
}

// WithDensity derives a metrics set at another density.
func (m Metrics) WithDensity(density Density) Metrics {
	switch density {
	case DensityCompact:
		return Metrics{
			ControlHeight: 26, RadiusControl: 5.5, RadiusSurface: 9,
			FontSizeS: 11, FontSizeM: 13, FontSizeL: 22,
			PadControlX: 10, PadSurface: 14, GapItem: 6,
		}
	case DensityLarge:
		return Metrics{
			ControlHeight: 40, RadiusControl: 9, RadiusSurface: 13,
			FontSizeS: 14, FontSizeM: 16, FontSizeL: 30,
			PadControlX: 18, PadSurface: 24, GapItem: 10,
		}
	default:
		return m
	}
}

// Theme contains semantic design tokens rather than runtime CSS.
type Theme struct {
	Background        Color
	Foreground        Color
	Card              Color
	CardForeground    Color
	CardElevated      Color
	Popover           Color
	PopoverForeground Color
	Muted             Color
	MutedForeground   Color
	Border            Color
	Hover             Color
	Input             Color
	Primary           Color
	PrimaryForeground Color
	// Selection and SelectionForeground are the text-selection pair. They are
	// separate from Primary because editors need a readable selection colour
	// even when a product uses a neutral primary action colour.
	Selection           Color
	SelectionForeground Color
	Secondary           Color
	SecondaryForeground Color
	Accent              Color
	AccentForeground    Color
	Ring                Color
	Success             Color
	Danger              Color
	DangerForeground    Color
	Chart               [5]Color
	Metrics             Metrics
	// Transition is the default state-change motion for controls. A zero
	// value disables animation for hand-built themes.
	Transition Motion
}

// WithRadius is the theme with another corner radius for its controls
// (Metrics.RadiusControl), for one control among others:
//
//	components.PaintInput(canvas, theme.WithRadius(ui.RadiusFull), bounds, snapshot)
//
// A theme is a value, so the other controls keep the radius they were painted
// with. What is derived from the control radius, such as a menu row's, follows;
// the radius of cards, dialogs and other surfaces (RadiusSurface) does not.
func (theme Theme) WithRadius(radius float32) Theme {
	theme.Metrics.RadiusControl = max(radius, 0)
	return theme
}

// WithControlHeight is the theme with another control height (Metrics.ControlHeight),
// for a control that is given a row of its own height: the heights of the button
// sizes, and the geometry that scales with the height, follow it. Like WithRadius
// it changes only the copy it returns.
func (theme Theme) WithControlHeight(height float32) Theme {
	theme.Metrics.ControlHeight = max(height, 0)
	return theme
}

// WithFontSize is the theme with another body font size (Metrics.FontSizeM), the
// size of a control's text. The small and large sizes keep their proportion to it,
// so a control's captions and titles stay in step. Like WithRadius it changes
// only the copy it returns:
//
//	components.PaintInput(canvas, theme.WithFontSize(16), bounds, snapshot)
func (theme Theme) WithFontSize(size float32) Theme {
	metrics := &theme.Metrics
	if size <= 0 || metrics.FontSizeM <= 0 {
		return theme
	}
	scale := size / metrics.FontSizeM
	metrics.FontSizeS *= scale
	metrics.FontSizeL *= scale
	metrics.FontSizeM = size
	return theme
}

// WithOpacity is the theme with the opacity of every color multiplied by opacity,
// 0 to 1, for a control painted faded: what it draws with the copy is as
// transparent as the copy's colors, so a faded Button, Input or Card needs no
// other change. Like WithRadius it changes only the copy it returns.
func (theme Theme) WithOpacity(opacity float32) Theme {
	opacity = max(0, min(opacity, 1))
	if opacity == 1 {
		return theme
	}
	for _, color := range [...]*Color{
		&theme.Background, &theme.Foreground, &theme.Card, &theme.CardForeground, &theme.CardElevated,
		&theme.Popover, &theme.PopoverForeground, &theme.Muted, &theme.MutedForeground, &theme.Border,
		&theme.Hover, &theme.Input, &theme.Primary, &theme.PrimaryForeground, &theme.Selection,
		&theme.SelectionForeground, &theme.Secondary, &theme.SecondaryForeground, &theme.Accent,
		&theme.AccentForeground, &theme.Ring, &theme.Success, &theme.Danger, &theme.DangerForeground,
	} {
		color.Alpha *= opacity
	}
	for index := range theme.Chart {
		theme.Chart[index].Alpha *= opacity
	}
	return theme
}

// DarkTheme returns the Vega Neutral + Blue preset in dark appearance.
// Colors are precomputed from preset b1Z5baiCO's OKLCH tokens so native
// applications do not link a runtime color parser.
func DarkTheme() Theme {
	return Theme{
		Background:          RGB(10, 10, 10),
		Foreground:          RGB(250, 250, 250),
		Card:                RGB(23, 23, 23),
		CardForeground:      RGB(250, 250, 250),
		CardElevated:        RGB(28, 28, 28),
		Popover:             RGB(23, 23, 23),
		PopoverForeground:   RGB(250, 250, 250),
		Muted:               RGB(38, 38, 38),
		MutedForeground:     RGB(161, 161, 161),
		Border:              RGBA(255, 255, 255, 0.10),
		Hover:               RGB(31, 31, 31),
		Input:               RGBA(255, 255, 255, 0.15),
		Primary:             RGB(25, 60, 184),
		PrimaryForeground:   RGB(239, 246, 255),
		Selection:           RGB(38, 79, 120),
		SelectionForeground: RGB(239, 246, 255),
		Secondary:           RGB(39, 39, 42),
		SecondaryForeground: RGB(250, 250, 250),
		Accent:              RGB(38, 38, 38),
		AccentForeground:    RGB(250, 250, 250),
		Ring:                RGB(115, 115, 115),
		Success:             RGB(34, 197, 94),
		Danger:              RGB(255, 100, 103),
		DangerForeground:    RGB(255, 255, 255),
		Chart: [5]Color{
			RGB(142, 197, 255), RGB(43, 127, 255), RGB(21, 93, 252),
			RGB(20, 71, 230), RGB(25, 60, 184),
		},
		Metrics:    DefaultMetrics(),
		Transition: DefaultMotion(),
	}
}

// LightTheme returns the Vega Neutral + Blue preset in light appearance.
func LightTheme() Theme {
	return Theme{
		Background:          RGB(255, 255, 255),
		Foreground:          RGB(10, 10, 10),
		Card:                RGB(255, 255, 255),
		CardForeground:      RGB(10, 10, 10),
		CardElevated:        RGB(247, 247, 247),
		Popover:             RGB(255, 255, 255),
		PopoverForeground:   RGB(10, 10, 10),
		Muted:               RGB(245, 245, 245),
		MutedForeground:     RGB(115, 115, 115),
		Border:              RGB(229, 229, 229),
		Hover:               RGB(250, 250, 250),
		Input:               RGB(229, 229, 229),
		Primary:             RGB(20, 71, 230),
		PrimaryForeground:   RGB(239, 246, 255),
		Selection:           RGB(181, 211, 255),
		SelectionForeground: RGB(17, 24, 39),
		Secondary:           RGB(244, 244, 245),
		SecondaryForeground: RGB(24, 24, 27),
		Accent:              RGB(245, 245, 245),
		AccentForeground:    RGB(23, 23, 23),
		Ring:                RGB(161, 161, 161),
		Success:             RGB(22, 163, 74),
		Danger:              RGB(231, 0, 11),
		DangerForeground:    RGB(255, 255, 255),
		Chart: [5]Color{
			RGB(142, 197, 255), RGB(43, 127, 255), RGB(21, 93, 252),
			RGB(20, 71, 230), RGB(25, 60, 184),
		},
		Metrics:    DefaultMetrics(),
		Transition: DefaultMotion(),
	}
}

// RGB is an opaque color from its 8-bit channels.
func RGB(red, green, blue uint8) Color {
	return RGBA(red, green, blue, 1)
}

// RGBA is a color from its 8-bit channels and an alpha between 0 and 1.
func RGBA(red, green, blue uint8, alpha float32) Color {
	const divisor = 255
	return Color{Red: float32(red) / divisor, Green: float32(green) / divisor, Blue: float32(blue) / divisor, Alpha: alpha}
}

// RGB24 is an opaque color from 0xRRGGBB, as a design tool or a stylesheet writes
// it: ui.RGB24(0x0a84ff). The number is an integer constant, so the compiler
// checks and folds it: there is no string to parse and no error to handle.
func RGB24(rgb uint32) Color {
	return RGB(uint8(rgb>>16), uint8(rgb>>8), uint8(rgb))
}

// WithAlpha is the color with another alpha between 0 and 1; its channels are
// kept.
func (color Color) WithAlpha(alpha float32) Color {
	color.Alpha = alpha
	return color
}
