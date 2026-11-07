package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestButtonVectorIconKeepsAccessibleLabel(t *testing.T) {
	icon := checkIcon
	for _, disabled := range []bool{false, true} {
		var canvas ui.Canvas
		button := BeginButton(&canvas, ui.DarkTheme(), ui.Item(1), ui.Rect{Width: 80}).WithIcon(icon).Icon()
		if disabled {
			button = button.Disabled()
		}
		button.Ghost("Edit panel")
		if len(canvas.Texts()) != 0 || len(canvas.Triangles()) == 0 {
			t.Fatal("icon-only button must draw the vector without visible label text")
		}
		hit := canvas.Hits()[0]
		if hit.Label != "Edit panel" || hit.Hint != hit.Label || hit.Disabled != disabled || hit.Bounds.Width != 32 {
			t.Fatalf("icon button semantics: %+v", hit)
		}
	}
}

func TestButtonIconAndTextFitTogether(t *testing.T) {
	var canvas ui.Canvas
	button := BeginButton(&canvas, ui.LightTheme(), ui.Item(1), ui.Rect{X: 10}).WithIcon(checkIcon).SizeToFit("Save")
	button.Default("Save")
	if len(canvas.Triangles()) == 0 || len(canvas.Texts()) != 1 || canvas.Texts()[0].Value != "Save" {
		t.Fatal("leading icon button must retain its visible text")
	}
	want := canvas.TextWidthWeight("Save", ui.LightTheme().Metrics.FontSizeM, ui.WeightMedium) + 20 + 16 + 8
	if button.BoundsWidth() != want {
		t.Fatalf("icon and label fitted width = %v, want %v", button.BoundsWidth(), want)
	}
}

func TestNarrowIconButtonKeepsItsIconInside(t *testing.T) {
	bounds := ui.Rect{X: 40, Y: 10, Width: 12}
	var canvas ui.Canvas
	BeginButton(&canvas, ui.LightTheme(), ui.Item(1), bounds).WithIcon(checkIcon).Ghost("Edit panel")
	triangles := canvas.Triangles()
	if len(triangles) == 0 {
		t.Fatal("narrow icon button drew no icon")
	}
	for _, triangle := range triangles {
		for _, point := range triangle.Points {
			if point.X < bounds.X-.001 || point.X > bounds.X+bounds.Width+.001 {
				t.Fatalf("icon vertex X %.2f escaped %+v", point.X, bounds)
			}
		}
	}
}

func TestButtonOfficialVariants(t *testing.T) {
	theme := ui.LightTheme()
	tests := [...]struct {
		variant    ButtonVariant
		background ui.Color
		foreground ui.Color
		rectangles int
	}{
		{ButtonDefault, theme.Primary, theme.PrimaryForeground, 1},
		{ButtonDestructive, withAlpha(theme.Danger, 0.10), theme.Danger, 1},
		{ButtonSecondary, theme.Secondary, theme.SecondaryForeground, 1},
		{ButtonOutline, theme.Background, theme.Foreground, 2},
		{ButtonGhost, ui.Color{}, theme.Foreground, 0},
		{ButtonLink, ui.Color{}, theme.Primary, 0},
		{ButtonDestructiveOutline, theme.Background, theme.Danger, 2},
	}
	for _, test := range tests {
		var canvas ui.Canvas
		button := BeginButton(&canvas, theme, ui.Item(1), ui.Rect{Width: 120})
		button.paint("Action", test.variant)
		if len(canvas.Rectangles()) != test.rectangles {
			t.Fatalf("variant %d painted %d rectangles, want %d", test.variant, len(canvas.Rectangles()), test.rectangles)
		}
		if test.rectangles != 0 {
			fillIndex := len(canvas.Rectangles()) - 1
			if got := canvas.Rectangles()[fillIndex].Color; got != test.background {
				t.Fatalf("variant %d fill = %+v, want %+v", test.variant, got, test.background)
			}
		}
		if got := canvas.Texts()[0].Style.Color; got != test.foreground {
			t.Fatalf("variant %d foreground = %+v, want %+v", test.variant, got, test.foreground)
		}
	}
}

func TestButtonOfficialSizes(t *testing.T) {
	theme := ui.DarkTheme()
	tests := [...]struct {
		size   ButtonSize
		height float32
		icon   bool
	}{
		{ButtonSizeDefault, 32, false},
		{ButtonSizeXSmall, 24, false},
		{ButtonSizeSmall, 28, false},
		{ButtonSizeLarge, 36, false},
		{ButtonSizeIcon, 32, true},
		{ButtonSizeIconXSmall, 24, true},
		{ButtonSizeIconSmall, 28, true},
		{ButtonSizeIconLarge, 36, true},
	}
	for _, test := range tests {
		var canvas ui.Canvas
		BeginButton(&canvas, theme, ui.ID{}, ui.Rect{Width: 100}).Size(test.size).Default("A")
		bounds := canvas.Rectangles()[0].Bounds
		if bounds.Height != test.height {
			t.Fatalf("size %d height = %v, want %v", test.size, bounds.Height, test.height)
		}
		if test.icon && bounds.Width != test.height {
			t.Fatalf("icon size %d width = %v, want %v", test.size, bounds.Width, test.height)
		}
	}
}

func TestButtonTypedHitAndZeroIDPreview(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	BeginButton(&canvas, theme, ui.Item(7), ui.Rect{Width: 100}).Default("Save")
	BeginButton(&canvas, theme, ui.ID{}, ui.Rect{X: 120, Width: 100}).Default("Preview")
	if len(canvas.Hits()) != 1 {
		t.Fatalf("buttons declared %d hits, want only live declaration", len(canvas.Hits()))
	}
	hit := canvas.Hits()[0]
	if hit.ID != ui.Item(7) || hit.Label != "Save" || hit.Role != ui.RoleButton || !hit.Focusable || hit.Disabled {
		t.Fatalf("typed button hit = %+v", hit)
	}
}

func TestButtonDisabledKeepsTypedSemanticsAtHalfOpacity(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	BeginButton(&canvas, theme, ui.Item(9), ui.Rect{Width: 100}).Disabled().Default("Unavailable")
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable {
		t.Fatalf("disabled hit = %+v", canvas.Hits())
	}
	want := theme.Primary
	want.Alpha *= 0.50
	if got := canvas.Rectangles()[0].Color; got != want {
		t.Fatalf("disabled fill = %+v, want %+v", got, want)
	}
}

func TestButtonFocusAndInvalidRingsUseOfficialTokensInDark(t *testing.T) {
	theme := ui.DarkTheme()
	var focused, invalid ui.Canvas
	focused.SetInteraction(ui.ID{}, ui.ID{}, ui.Item(1), true)
	BeginButton(&focused, theme, ui.Item(1), ui.Rect{X: 4, Y: 4, Width: 100}).Default("Save")
	BeginButton(&invalid, theme, ui.Item(2), ui.Rect{X: 4, Y: 4, Width: 100}).Invalid().Outline("Save")

	focusRing := focused.Rectangles()[0]
	wantFocus := theme.Ring
	wantFocus.Alpha *= 0.50
	if focusRing.Color != wantFocus || focusRing.Bounds != (ui.Rect{X: 1, Y: 1, Width: 106, Height: 38}) {
		t.Fatalf("focus ring = %+v, want Ring/50 expanded 3px", focusRing)
	}
	invalidRing := invalid.Rectangles()[0]
	wantInvalid := withAlpha(theme.Danger, 0.40)
	wantBorder := invalidBorder(theme).Over(theme.Background)
	if invalidRing.Color != wantInvalid || invalid.Rectangles()[1].Color != wantBorder {
		t.Fatalf("invalid ring/border = %+v / %+v", invalidRing, invalid.Rectangles()[1])
	}
}

func TestButtonSizeToFitUsesSelectedPadding(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	defaultButton := BeginButton(&canvas, theme, ui.ID{}, ui.Rect{}).SizeToFit("Download")
	xsmallButton := BeginButton(&canvas, theme, ui.ID{}, ui.Rect{}).XSmall().SizeToFit("Download")
	wantDefault := canvas.TextWidthWeight("Download", theme.Metrics.FontSizeM, ui.WeightMedium) + 20
	wantXSmall := canvas.TextWidthWeight("Download", theme.Metrics.FontSizeS, ui.WeightMedium) + 16
	if defaultButton.BoundsWidth() != wantDefault || xsmallButton.BoundsWidth() != wantXSmall {
		t.Fatalf("fitted widths = %v and %v, want %v and %v", defaultButton.BoundsWidth(), xsmallButton.BoundsWidth(), wantDefault, wantXSmall)
	}
}

func TestButtonCentersInItsArea(t *testing.T) {
	var canvas ui.Canvas
	area := ui.Rect{X: 100, Y: 50, Width: 300, Height: 100}
	button := BeginButton(&canvas, ui.DarkTheme(), ui.ID{}, ui.Rect{}).
		SizeToFit("Download").CenterIn(area)
	if want := area.X + (area.Width-button.bounds.Width)/2; button.bounds.X != want {
		t.Fatalf("centered X = %v, want %v", button.bounds.X, want)
	}
	if want := area.Y + (area.Height-button.bounds.Height)/2; button.bounds.Y != want {
		t.Fatalf("centered Y = %v, want %v", button.bounds.Y, want)
	}
}

func TestButtonRepeatedPaintReusesCanvasStorage(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	BeginButton(&canvas, theme, ui.Item(1), ui.Rect{Width: 100}).Default("Save")
	allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		BeginButton(&canvas, theme, ui.Item(1), ui.Rect{Width: 100}).Default("Save")
	})
	if allocations != 0 {
		t.Fatalf("repeated button paint allocated %v times", allocations)
	}
}

// A transform moves the button's glyph inside its square, not the button, its
// label or the region that receives clicks.
func TestButtonMovesOnlyItsIcon(t *testing.T) {
	theme := ui.LightTheme()
	bounds := ui.Rect{X: 20, Y: 10, Width: 120, Height: 40}
	paint := func(transform ui.IconTransform) *ui.Canvas {
		var canvas ui.Canvas
		BeginButton(&canvas, theme, ui.Item(1), bounds).
			WithIcon(checkIcon).
			IconTransform(transform).
			Default("Save")
		return &canvas
	}
	rest := paint(ui.IconTransform{})
	moved := paint(ui.IconTransform{Translate: ui.Point{Y: -2}})
	if len(rest.Triangles()) == 0 || len(moved.Triangles()) != len(rest.Triangles()) {
		t.Fatalf("rest painted %d triangles, the moving icon %d", len(rest.Triangles()), len(moved.Triangles()))
	}
	restTop, movedTop := float32(1e9), float32(1e9)
	for _, triangle := range rest.Triangles() {
		for _, point := range triangle.Points {
			restTop = min(restTop, point.Y)
		}
	}
	for _, triangle := range moved.Triangles() {
		for _, point := range triangle.Points {
			movedTop = min(movedTop, point.Y)
		}
	}
	if movedTop >= restTop {
		t.Fatalf("the icon's top went from %v to %v, want it to rise", restTop, movedTop)
	}
	if got, want := rest.Rectangles()[0].Bounds, moved.Rectangles()[0].Bounds; got != want {
		t.Fatalf("the moving icon moved the button from %+v to %+v", want, got)
	}
	if got, want := moved.Hits()[0].Bounds, rest.Hits()[0].Bounds; got != want {
		t.Fatalf("the moving icon moved the hit region to %+v, want %+v", got, want)
	}
}

// A button in a row of its own height is painted that tall, its label centered in
// it, with the radius and text size of its size; without a height nothing changes.
func TestButtonHeightPaintsTheButtonThatTall(t *testing.T) {
	theme := ui.LightTheme()
	paint := func(button Button) (*ui.Canvas, ui.Rect) {
		canvas := &ui.Canvas{}
		button.canvas = canvas
		button.Default("Save")
		return canvas, canvas.Rectangles()[0].Bounds
	}
	plainCanvas, plain := paint(BeginButton(nil, theme, ui.Item(1), ui.Rect{Width: 80}))
	if plain.Height != theme.Metrics.ControlHeight {
		t.Fatalf("a button without a height is %v tall, want the size's %v", plain.Height, theme.Metrics.ControlHeight)
	}
	tallCanvas, tall := paint(BeginButton(nil, theme, ui.Item(1), ui.Rect{Y: 10, Width: 80}).Height(48))
	if tall.Height != 48 || tall.Y != 10 {
		t.Fatalf("a 48px button is %+v", tall)
	}
	if hit := tallCanvas.Hits()[0]; hit.Bounds.Height != 48 {
		t.Errorf("the hit region is %v tall, want 48", hit.Bounds.Height)
	}
	if got, want := tallCanvas.Rectangles()[0].Radius, plainCanvas.Rectangles()[0].Radius; got != want {
		t.Errorf("radius = %v, want the size's %v", got, want)
	}
	if got, want := tallCanvas.Texts()[0].Style.Size, plainCanvas.Texts()[0].Style.Size; got != want {
		t.Errorf("text size = %v, want the size's %v", got, want)
	}
	if label := tallCanvas.Texts()[0]; label.Bounds.Y+label.Bounds.Height/2 != 10+24 {
		t.Errorf("label %+v is not centered in the 48px button", label.Bounds)
	}
	// An icon button stays square, at the height.
	_, icon := paint(BeginButton(nil, theme, ui.Item(1), ui.Rect{Width: 10}).Icon().Height(44))
	if icon.Width != 44 || icon.Height != 44 {
		t.Errorf("a 44px icon button is %vx%v, want square", icon.Width, icon.Height)
	}
	// Zero or less is the size's own.
	if _, same := paint(BeginButton(nil, theme, ui.Item(1), ui.Rect{Width: 80}).Height(-5)); same != plain {
		t.Errorf("a negative height changed the button: %+v", same)
	}
}

// FontSize changes only the label's size: the button keeps the height and radius
// of its size, SizeToFit follows the label, and zero or less is the size's own.
func TestButtonFontSizeChangesOnlyTheLabel(t *testing.T) {
	theme := ui.LightTheme()
	paint := func(button Button) *ui.Canvas {
		canvas := &ui.Canvas{}
		button.canvas = canvas
		button.Default("Save")
		return canvas
	}
	base := ui.Rect{Width: 80}
	plain := paint(BeginButton(nil, theme, ui.Item(1), base).Small())
	sized := paint(BeginButton(nil, theme, ui.Item(1), base).Small().FontSize(13))
	if got := sized.Texts()[0].Style.Size; got != 13 {
		t.Errorf("label size = %v, want 13", got)
	}
	if sized.Rectangles()[0].Bounds != plain.Rectangles()[0].Bounds || sized.Rectangles()[0].Radius != plain.Rectangles()[0].Radius {
		t.Errorf("the box changed: %+v vs %+v", sized.Rectangles()[0], plain.Rectangles()[0])
	}
	if got, want := paint(BeginButton(nil, theme, ui.Item(1), base).Small().FontSize(-1)).Texts()[0].Style.Size, plain.Texts()[0].Style.Size; got != want {
		t.Errorf("a negative size gave %v, want the size's %v", got, want)
	}
	small := BeginButton(&ui.Canvas{}, theme, ui.Item(1), base).Small().SizeToFit("Layout").BoundsWidth()
	large := BeginButton(&ui.Canvas{}, theme, ui.Item(1), base).Small().FontSize(20).SizeToFit("Layout").BoundsWidth()
	if large <= small {
		t.Errorf("a bigger label fit in %v, no wider than %v", large, small)
	}
}
