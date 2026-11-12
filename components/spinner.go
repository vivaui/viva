package components

import "github.com/vivaui/viva/gfx"

// SpinnerPhase selects one of the eight static frames in the official
// sixteen-pixel loading indicator. Advancing the phase and scheduling another
// frame are explicit caller responsibilities.
type SpinnerPhase uint8

const (
	spinnerPhaseCount SpinnerPhase = 8
	spinnerSize                    = float32(16)

	// Diagonal spokes are a staircase of spinnerDiagonalRows horizontal strips,
	// each spinnerDiagonalStep tall and shifted by the same amount, so the
	// edge runs at 45 degrees. A half-pixel step is one physical pixel on a 2x
	// display. The outermost strips are one pixel narrower to round the tips.
	spinnerDiagonalRows   = 6
	spinnerDiagonalStep   = float32(0.5)
	spinnerDiagonalWidth  = float32(2.8)
	spinnerDiagonalCenter = float32(4)

	// SpinnerLabel is the official accessible name for the status indicator.
	SpinnerLabel = "Loading"
)

// spinnerSpoke is one of the eight spokes in clockwise order from twelve
// o'clock. Straight spokes carry their rectangle on the sixteen-pixel grid;
// diagonal spokes carry only their outward direction because Canvas has no
// rotation or stroke primitive.
type spinnerSpoke struct {
	bounds gfx.Rect
	dx, dy float32
}

var spinnerSpokes = [8]spinnerSpoke{
	{bounds: gfx.Rect{X: 7, Y: 0, Width: 2, Height: 4}},
	{dx: 1, dy: -1},
	{bounds: gfx.Rect{X: 12, Y: 7, Width: 4, Height: 2}},
	{dx: 1, dy: 1},
	{bounds: gfx.Rect{X: 7, Y: 12, Width: 2, Height: 4}},
	{dx: -1, dy: 1},
	{bounds: gfx.Rect{X: 0, Y: 7, Width: 4, Height: 2}},
	{dx: -1, dy: -1},
}

// SpinnerLayout reports the fitted icon geometry and semantic label.
type SpinnerLayout struct {
	Bounds gfx.Rect
	Phase  SpinnerPhase
	Label  string
}

// PaintSpinner paints one caller-selected static frame of the official
// size-4 loader. The brightest spoke is the phase and the fade trails behind
// it, so advancing the phase turns the indicator clockwise. It never starts a
// timer, schedules a frame, or retains phase.
func PaintSpinner(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, phase SpinnerPhase) SpinnerLayout {
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return SpinnerLayout{}
	}
	size := min(bounds.Width, bounds.Height, spinnerSize)
	icon := gfx.Rect{
		X:      bounds.X + (bounds.Width-size)/2,
		Y:      bounds.Y + (bounds.Height-size)/2,
		Width:  size,
		Height: size,
	}
	phase %= spinnerPhaseCount
	scale := size / spinnerSize
	for index, spoke := range spinnerSpokes {
		distance := (int(phase) - index + len(spinnerSpokes)) % len(spinnerSpokes)
		color := theme.Foreground
		color.Alpha *= 1 - float32(distance)*0.10
		if spoke.dx != 0 {
			paintSpinnerDiagonal(canvas, icon, scale, spoke, color)
			continue
		}
		paint := gfx.Rect{
			X:      icon.X + spoke.bounds.X*scale,
			Y:      icon.Y + spoke.bounds.Y*scale,
			Width:  spoke.bounds.Width * scale,
			Height: spoke.bounds.Height * scale,
		}
		canvas.Rounded(paint, min(paint.Width, paint.Height)/2, color)
	}
	return SpinnerLayout{Bounds: icon, Phase: phase, Label: SpinnerLabel}
}

// paintSpinnerDiagonal paints one slanted spoke as abutting strips. They never
// overlap, so a translucent spoke keeps a single uniform alpha.
func paintSpinnerDiagonal(canvas *gfx.Canvas, icon gfx.Rect, scale float32, spoke spinnerSpoke, color gfx.Color) {
	step := spinnerDiagonalStep * scale
	centerX := icon.X + (spinnerSize/2+spoke.dx*spinnerDiagonalCenter)*scale
	centerY := icon.Y + (spinnerSize/2+spoke.dy*spinnerDiagonalCenter)*scale
	top := centerY - spinnerDiagonalRows*step/2
	for row := range spinnerDiagonalRows {
		y := top + float32(row)*step
		width := spinnerDiagonalWidth * scale
		if row == 0 || row == spinnerDiagonalRows-1 {
			width -= scale
		}
		// The spoke's axis crosses a strip's middle at this x.
		middle := centerX + (y+step/2-centerY)*spoke.dx*spoke.dy
		canvas.Fill(gfx.Rect{X: middle - width/2, Y: y, Width: width, Height: step}, color)
	}
}
