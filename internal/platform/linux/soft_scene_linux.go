package linux

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/raster"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

// softScene is the retained scene of a window drawn by the CPU (ADR 0064): the rectangle
// instances and text labels the application sent, the picture of each label, and the part of the
// canvas that changed since it was last painted. A presenter (the GTK shell's drawing area, a
// Wayland surface) takes the canvas and the damaged region from it; nothing here depends on a
// display server.
//
// A frame names the instances that changed, so the damage is the pixels those covered before and
// cover now (raster.Box), and a label that is the same as the one in its slot keeps its picture
// and costs nothing. Typing in a text field repaints the field.
type softScene struct {
	services *textServices
	canvas   *raster.Canvas
	drawer   raster.Rasterizer
	scale    float32
	clear    scene.Color

	rects  []render.Rectangle
	labels []softLabel
	// views are the places native views fill; holes are the same in device pixels.
	views  []platform.NativeView
	holes  []raster.Hole
	drawn  []raster.Label // the labels painted this time, in slot order; scratch
	damage render.Scissor
}

// softLabel is one slot of the label list.
type softLabel struct {
	label platform.TextLabel
	runs  []ui.TextRun // backs label.Runs, which is only valid for the call that carried it
	// image is the label's picture at imageScale, if it has been drawn.
	image      labelImage
	imageScale float32
	hasImage   bool
	// placed is the picture on the canvas, visible says it draws anything.
	placed  raster.Label
	visible bool
}

// newSoftScene makes a scene for a canvas of the given device size. rectangles and labels are the
// window's first content.
func newSoftScene(services *textServices, width, height int, scale float32, clear platform.Color, rectangles []render.Rectangle, labels []platform.TextLabel) (*softScene, error) {
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("linux: invalid canvas size %dx%d", width, height)
	}
	s := &softScene{
		services: services,
		canvas:   raster.NewCanvas(width, height),
		scale:    max(scale, 1),
		clear:    scene.Color{Red: float32(clear.Red), Green: float32(clear.Green), Blue: float32(clear.Blue), Alpha: float32(clear.Alpha)},
	}
	s.rects = append(s.rects, rectangles...)
	if err := s.setLabels(labels); err != nil {
		return nil, err
	}
	s.damageAll()
	return s, nil
}

func (s *softScene) damageAll() { s.damage = s.canvas.Bounds() }

// addDamage adds a box of device pixels to what must be painted.
func (s *softScene) addDamage(box render.Scissor) {
	bounds := s.canvas.Bounds()
	box = render.Scissor{Left: max(box.Left, bounds.Left), Top: max(box.Top, bounds.Top), Right: min(box.Right, bounds.Right), Bottom: min(box.Bottom, bounds.Bottom)}
	if box.Right <= box.Left || box.Bottom <= box.Top {
		return
	}
	if s.damage.Right <= s.damage.Left || s.damage.Bottom <= s.damage.Top {
		s.damage = box
		return
	}
	s.damage = render.Scissor{Left: min(s.damage.Left, box.Left), Top: min(s.damage.Top, box.Top), Right: max(s.damage.Right, box.Right), Bottom: max(s.damage.Bottom, box.Bottom)}
}

// apply takes a frame into the scene and reports whether anything on the canvas changed.
func (s *softScene) apply(frame platform.Frame) (bool, error) {
	if frame.Update.Empty() && frame.TextLabels == nil && frame.NativeViews == nil {
		return false, nil
	}
	if !frame.Update.Empty() {
		if err := s.applyRectangles(frame); err != nil {
			return false, err
		}
	}
	if frame.NativeViews != nil {
		s.setViews(frame.NativeViews)
	}
	if frame.TextLabels != nil {
		if err := validateInitialTextLabels(frame.TextLabels); err != nil {
			return false, err
		}
		if err := s.setLabels(frame.TextLabels); err != nil {
			return false, err
		}
	}
	return s.pending(), nil
}

func (s *softScene) pending() bool {
	return s.damage.Right > s.damage.Left && s.damage.Bottom > s.damage.Top
}

func (s *softScene) applyRectangles(frame platform.Frame) error {
	if err := validateRectangles(frame.Rectangles); err != nil {
		return err
	}
	update := frame.Update
	if update.First < 0 || update.Count < 0 || update.First > len(frame.Rectangles) || update.Count > len(frame.Rectangles)-update.First {
		return fmt.Errorf("linux: invalid rectangle update")
	}
	if update.Rebuild {
		s.rects = append(s.rects[:0], frame.Rectangles...)
		s.damageAll()
		return nil
	}
	if len(frame.Rectangles) < len(s.rects) {
		return fmt.Errorf("linux: rectangle count shrank without rebuild")
	}
	if len(frame.Rectangles) > len(s.rects) {
		if update.First > len(s.rects) || update.First+update.Count != len(frame.Rectangles) {
			return fmt.Errorf("linux: rectangle count grew outside a suffix update")
		}
	}
	for i := update.First; i < update.First+update.Count; i++ {
		if i < len(s.rects) {
			s.addDamage(raster.Box(&s.rects[i], s.scale))
		}
		s.addDamage(raster.Box(&frame.Rectangles[i], s.scale))
	}
	if len(frame.Rectangles) > len(s.rects) {
		s.rects = append(s.rects, frame.Rectangles[len(s.rects):]...)
	}
	copy(s.rects[update.First:update.First+update.Count], frame.Rectangles[update.First:update.First+update.Count])
	return nil
}

// setLabels replaces the label list. A slot whose label is the same as before keeps its picture.
func (s *softScene) setLabels(labels []platform.TextLabel) error {
	for i := len(labels); i < len(s.labels); i++ {
		if s.labels[i].visible {
			s.addDamage(labelBox(&s.labels[i].placed))
		}
	}
	if len(labels) > cap(s.labels) {
		s.labels = append(s.labels[:cap(s.labels)], make([]softLabel, len(labels)-cap(s.labels))...)
	}
	s.labels = s.labels[:len(labels)]
	for i := range labels {
		if err := s.setLabel(&s.labels[i], labels[i]); err != nil {
			return fmt.Errorf("linux: label %d: %w", i, err)
		}
	}
	return nil
}

func (s *softScene) setLabel(slot *softLabel, next platform.TextLabel) error {
	previous := slot.placed
	wasVisible := slot.visible
	redraw := !slot.hasImage || slot.imageScale != s.scale || labelContentChanged(slot.label, next) || (len(next.Bitmap) > 0 && bitmapShownChanged(slot.label, next, s.scale))
	slot.runs = append(slot.runs[:0], next.Runs...)
	slot.label = next
	slot.label.Runs = slot.runs
	if redraw {
		image, ok, err := rasterizeLabel(s.services, slot.label, s.scale)
		if err != nil {
			return err
		}
		slot.image, slot.imageScale, slot.hasImage = image, s.scale, ok
		if !ok {
			slot.image = labelImage{}
		}
	}
	slot.visible = false
	if slot.hasImage && slot.image.width > 0 {
		slot.placed, slot.visible = placeLabel(slot.label, s.scale, slot.image, s.canvas.Width, s.canvas.Height)
	}
	switch {
	case !wasVisible && !slot.visible:
	case redraw || wasVisible != slot.visible || slot.placed.X != previous.X || slot.placed.Y != previous.Y ||
		slot.placed.Layer != previous.Layer || slot.placed.Clipped != previous.Clipped || slot.placed.Clip != previous.Clip:
		if wasVisible {
			s.addDamage(labelBox(&previous))
		}
		if slot.visible {
			s.addDamage(labelBox(&slot.placed))
		}
	}
	return nil
}

// labelBox is the device pixels a placed label may paint.
func labelBox(l *raster.Label) render.Scissor {
	box := render.Scissor{Left: l.X, Top: l.Y, Right: l.X + l.Width, Bottom: l.Y + l.Height}
	if l.Clipped {
		box = render.Scissor{Left: max(box.Left, l.Clip.Left), Top: max(box.Top, l.Clip.Top), Right: min(box.Right, l.Clip.Right), Bottom: min(box.Bottom, l.Clip.Bottom)}
	}
	return box
}

// labelContentChanged reports whether the two labels need different pictures. Where a label is
// and how it is clipped are not in it: scrolling text costs no layout.
func labelContentChanged(previous, next platform.TextLabel) bool {
	return previous.Value != next.Value || previous.Width != next.Width || previous.Height != next.Height || previous.FontSize != next.FontSize ||
		previous.Weight != next.Weight || previous.Color != next.Color || previous.Mono != next.Mono || previous.MonoFace != next.MonoFace || previous.CellWidth != next.CellWidth ||
		!sameTextRuns(previous.Runs, next.Runs) || previous.BitmapID != next.BitmapID || previous.BitmapVersion != next.BitmapVersion ||
		previous.BitmapWidth != next.BitmapWidth || previous.BitmapHeight != next.BitmapHeight || previous.BitmapStride != next.BitmapStride ||
		!sameBitmapBytes(previous.Bitmap, next.Bitmap)
}

// bitmapShownChanged reports whether an image covers a different number of device pixels than it
// did, which happens when it moves across a pixel boundary and is smaller than its bitmap.
func bitmapShownChanged(previous, next platform.TextLabel, scale float32) bool {
	return platform.DeviceExtent(previous.X, previous.Width, scale) != platform.DeviceExtent(next.X, next.Width, scale) ||
		platform.DeviceExtent(previous.Y, previous.Height, scale) != platform.DeviceExtent(next.Y, next.Height, scale)
}

// resize gives the scene a canvas of another device size. Everything is painted again.
func (s *softScene) resize(width, height int) error {
	if width < 1 || height < 1 {
		return fmt.Errorf("linux: invalid canvas size %dx%d", width, height)
	}
	if width == s.canvas.Width && height == s.canvas.Height {
		return nil
	}
	s.canvas.Resize(width, height)
	return s.replace()
}

// setScale draws everything at another scale.
func (s *softScene) setScale(scale float32) error {
	scale = max(scale, 1)
	if scale == s.scale {
		return nil
	}
	s.scale = scale
	return s.replace()
}

// setViews replaces the places of the native views. The pixels under a view and under its old place
// are painted again.
func (s *softScene) setViews(views []platform.NativeView) {
	for _, hole := range s.holes {
		s.addDamage(hole.Box)
	}
	s.views = append(s.views[:0], views...)
	s.placeHoles()
}

// placeHoles works out the holes in device pixels, on the grid the views are placed on.
func (s *softScene) placeHoles() {
	s.holes = s.holes[:0]
	for _, view := range s.views {
		b := view.Bounds
		box := render.Scissor{
			Left: int(math.Round(float64(b.X * s.scale))), Top: int(math.Round(float64(b.Y * s.scale))),
			Right: int(math.Round(float64((b.X + b.Width) * s.scale))), Bottom: int(math.Round(float64((b.Y + b.Height) * s.scale))),
		}
		s.holes = append(s.holes, raster.Hole{Box: box, Layer: view.Layer})
		s.addDamage(box)
	}
}

// overlays returns the parts of the view that something on a higher layer covers, in device pixels:
// where the pointer belongs to Viva although a page is under it.
func (s *softScene) overlays(index int) []render.Scissor {
	if index < 0 || index >= len(s.holes) {
		return nil
	}
	hole := s.holes[index]
	var covered []render.Scissor
	for i := range s.rects {
		r := &s.rects[i]
		if r.Layer <= hole.Layer || r.Kind == scene.KindShadow || r.Color.Alpha <= 0 {
			continue
		}
		box := render.Scissor{Left: max(hole.Box.Left, 0), Top: hole.Box.Top, Right: hole.Box.Right, Bottom: hole.Box.Bottom}
		if shown := raster.Box(r, s.scale); shown.Right > shown.Left && shown.Bottom > shown.Top {
			box = render.Scissor{Left: max(box.Left, shown.Left), Top: max(box.Top, shown.Top), Right: min(box.Right, shown.Right), Bottom: min(box.Bottom, shown.Bottom)}
			if box.Right > box.Left && box.Bottom > box.Top {
				covered = append(covered, box)
			}
		}
	}
	for i := range s.labels {
		l := &s.labels[i]
		if l.visible && l.placed.Layer > hole.Layer {
			box := labelBox(&l.placed)
			box = render.Scissor{Left: max(box.Left, hole.Box.Left), Top: max(box.Top, hole.Box.Top), Right: min(box.Right, hole.Box.Right), Bottom: min(box.Bottom, hole.Box.Bottom)}
			if box.Right > box.Left && box.Bottom > box.Top {
				covered = append(covered, box)
			}
		}
	}
	return covered
}

// refreshText draws every label again, after the fonts changed.
func (s *softScene) refreshText() error {
	for i := range s.labels {
		s.labels[i].hasImage = false
	}
	return s.replace()
}

// replace places every label again and paints all of the canvas.
func (s *softScene) replace() error {
	for i := range s.labels {
		if err := s.setLabel(&s.labels[i], s.labels[i].label); err != nil {
			return fmt.Errorf("linux: label %d: %w", i, err)
		}
	}
	s.placeHoles()
	s.damageAll()
	return nil
}

// paint draws what changed into the canvas and returns the region it drew, empty when nothing had
// changed.
func (s *softScene) paint() render.Scissor {
	if !s.pending() {
		return render.Scissor{}
	}
	damage := s.damage
	s.drawn = s.drawn[:0]
	for i := range s.labels {
		if s.labels[i].visible {
			s.drawn = append(s.drawn, s.labels[i].placed)
		}
	}
	s.drawer.Draw(s.canvas, raster.Params{Scale: s.scale, Clear: s.clear, Holes: s.holes, Damage: damage}, s.rects, s.drawn)
	s.damage = render.Scissor{}
	return damage
}

// close releases the pictures.
func (s *softScene) close() {
	s.canvas, s.rects, s.labels, s.drawn = nil, nil, nil, nil
}
