package windows

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/internal/platform"
)

// A page that appears replaces every one of its labels at once. Rasterizing each
// of them in its own Direct2D session costs about half a millisecond per label
// even for a short line, because the session's setup and flush do not depend on
// the line's width; a settings page of thirty-five labels would spend fifteen
// milliseconds in one present, miss the display's frame and visibly hitch. The
// batch below draws all of a frame's changed proportional labels into one
// bitmap in one session, then writes each label's pixels into its own texture.
//
// A line's own pixels are unchanged: the same shaped line is drawn at the same
// baseline offset, cut to the same rectangle the per-label path cut its bitmap
// to. Labels the batch cannot draw exactly as they were drawn before — fixed
// pitch text, images, and text whose channels need different ClearType
// backgrounds — are left to the per-label path, as is anything the batch bitmap
// cannot hold.

// labelBatchWidth bounds one batch bitmap's width. Wider labels still batch:
// each gets a shelf of its own.
const labelBatchWidth = 2048

// labelBatchHeight bounds one batch bitmap's height, keeping a batch bitmap at
// most eight megabytes at the width above. A frame with more changed text
// than this is drawn in several batches rather than one oversized bitmap;
// another session is far cheaper than the per-label sessions this replaces.
const labelBatchHeight = 1024

// batchPack places label rectangles in a shelf-packed bitmap.
type batchPack struct {
	x, y, shelf, width, height int
}

// full reports whether a width by height label would not fit the batch bitmap.
func (pack *batchPack) full(width, height int) bool {
	y := pack.y
	if pack.x > 0 && pack.x+width > labelBatchWidth {
		y += pack.shelf
	}
	return y+height > labelBatchHeight
}

// place reserves a width by height rectangle and returns its corner.
func (pack *batchPack) place(width, height int) (x, y int) {
	if pack.x > 0 && pack.x+width > labelBatchWidth {
		pack.x, pack.y, pack.shelf = 0, pack.y+pack.shelf, 0
	}
	x, y = pack.x, pack.y
	pack.x += width
	pack.shelf = max(pack.shelf, height)
	pack.width = max(pack.width, x+width)
	pack.height = max(pack.height, y+height)
	return x, y
}

// labelBatchItem is one label waiting to be drawn into a batch bitmap.
type labelBatchItem struct {
	label                  *labelTexture
	width, height          int
	skip                   int
	scale                  float32
	baseline               float32
	x, y                   int
	lcd                    bool
	drawLight              bool // the ClearType coverage is drawn white on black
	translucent            bool // the label is drawn with its own color and alpha
	background, foreground d2dColor
}

// labelBatch is one frame's changed proportional labels, grouped by the target
// that draws them.
type labelBatch struct {
	opaque, translucent         []labelBatchItem
	opaquePack, translucentPack batchPack
}

// add queues a label. It reports false when the label's pixels would differ
// from the per-label path, and full when the batch bitmap could not hold it;
// the caller then flushes and tries the label again.
func (batch *labelBatch) add(label *labelTexture, scale float32, allowLCD bool) (added, full bool, err error) {
	skip, width, ok := label.window(scale)
	if !ok {
		return false, false, nil
	}
	height := int(math.Ceil(float64(label.label.Height * scale)))
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || width*height > 16*1024*1024 {
		return false, false, nil // the per-label path reports the bounded-size error
	}
	size := label.label.FontSize * scale
	if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
		return false, false, fmt.Errorf("windows: invalid label font size")
	}
	item := labelBatchItem{
		label: label, width: width, height: height, skip: skip, scale: scale,
		baseline: float32(platform.TextBaseline(label.label, height, scale)),
	}
	color := label.label.Color
	item.lcd = allowLCD && !platform.ColorGlyphs(label.label.Value)
	if !item.lcd {
		// Grayscale coverage is drawn in the label's own color over transparent
		// pixels, as the per-label path draws it.
		item.translucent = true
		item.foreground = d2dColor{R: float32(color.Red), G: float32(color.Green), B: float32(color.Blue), A: float32(color.Alpha)}
	} else {
		// ClearType coverage depends on how bright each channel is: light text is
		// drawn white on black, dark text black on white. A color that mixes the
		// two needs two drawings and is left to the per-label path.
		var light [3]bool
		var anyLight, anyDark bool
		for channel, value := range [3]float64{color.Blue, color.Green, color.Red} {
			light[channel] = value > 0.5
			anyLight, anyDark = anyLight || light[channel], anyDark || !light[channel]
		}
		if anyLight && anyDark {
			return false, false, nil
		}
		item.drawLight = anyLight
		if anyLight {
			item.background = d2dColor{A: 1}
			item.foreground = d2dColor{R: 1, G: 1, B: 1, A: 1}
		} else {
			item.background = d2dColor{R: 1, G: 1, B: 1, A: 1}
			item.foreground = d2dColor{A: 1}
		}
	}
	group, pack := &batch.translucent, &batch.translucentPack
	if !item.translucent {
		group, pack = &batch.opaque, &batch.opaquePack
	}
	if pack.full(width, height) {
		return false, true, nil
	}
	item.x, item.y = pack.place(width, height)
	*group = append(*group, item)
	return true, false, nil
}

// flush draws and uploads every queued label.
func (batch *labelBatch) flush(renderer *renderer) error {
	if err := batch.draw(renderer, batch.opaque, batch.opaquePack, false); err != nil {
		return err
	}
	if err := batch.draw(renderer, batch.translucent, batch.translucentPack, true); err != nil {
		return err
	}
	batch.opaque, batch.translucent = batch.opaque[:0], batch.translucent[:0]
	batch.opaquePack, batch.translucentPack = batchPack{}, batchPack{}
	return nil
}

// draw rasterizes one group in a single Direct2D session and then uploads each
// label's rectangle.
func (batch *labelBatch) draw(renderer *renderer, items []labelBatchItem, pack batchPack, translucent bool) error {
	if len(items) == 0 {
		return nil
	}
	rasterizer, err := renderer.directRasterizer()
	if err != nil {
		return err
	}
	target, err := rasterizer.batchTarget(translucent)
	if err != nil {
		return err
	}
	scratch, err := target.bind(pack.width, pack.height)
	if err != nil {
		return err
	}
	renderer.labelBatches++
	// Each label's background is written into the bitmap before the Direct2D
	// session opens: the target owns the bitmap while drawing, and its flush at
	// the end of the session would overwrite pixels written underneath it.
	for index := range items {
		item := &items[index]
		fillIn(scratch, item.x, item.y, item.width, item.height, item.background)
	}
	mode, options := uint32(d2dAntialiasDefault), uint32(0)
	if translucent {
		mode, options = d2dAntialiasGrayscale, d2dEnableColorFont
	}
	target.begin(d2dColor{}, mode)
	var drawErr error
	for index := range items {
		item := &items[index]
		target.setColor(item.foreground)
		target.pushClipRect(item.x, item.y, item.x+item.width, item.y+item.height)
		drawErr = rasterizer.engine.drawStyle(item.label.label.Value, item.label.label.FontSize*item.scale, item.label.label.Weight, item.label.label.Italic, func(line *shapedLine) error {
			target.draw(float32(item.x)-line.left-float32(item.skip), float32(item.y)+item.baseline-line.ascent, line.layout, options)
			return nil
		})
		target.popClip()
		if drawErr != nil {
			break
		}
	}
	if err := target.end(); err != nil && drawErr == nil {
		drawErr = err
	}
	if drawErr != nil {
		return drawErr
	}
	for index := range items {
		item := &items[index]
		need := item.width * item.height * 4
		if cap(rasterizer.output) < need {
			rasterizer.output = make([]byte, need)
		}
		pixels := rasterizer.output[:need]
		if translucent {
			copyPixelsIn(pixels, scratch, item.x, item.y, item.width, item.height)
		} else {
			coverageOfPixelsIn(pixels, scratch, item.x, item.y, item.width, item.height, item.drawLight)
		}
		item.label.lcd = item.lcd
		item.label.spans = item.label.spans[:0]
		if err := renderer.uploadLabel(item.label, pixels, item.scale, item.skip, item.width, item.height); err != nil {
			return err
		}
	}
	return nil
}

// prepareLabels rasterizes the proportional labels that changed this frame in
// batches. Labels it leaves untouched are drawn by the per-label path in draw.
func (renderer *renderer) prepareLabels(scale float32) error {
	if renderer.noLabelBatch {
		return nil
	}
	allowLCD := (renderer.window != 0 && !renderer.usesComposition()) || renderer.color.Alpha >= 1
	var batch labelBatch
	flush := func() error {
		err := batch.flush(renderer)
		batch = labelBatch{}
		return err
	}
	for index := range renderer.labels {
		if index >= len(renderer.labelUsed) || !renderer.labelUsed[index] {
			continue
		}
		label := &renderer.labels[index]
		if label.label.Width <= 0 || label.label.Height <= 0 || label.label.Value == "" ||
			len(label.label.Bitmap) > 0 || label.label.Mono || len(label.label.Runs) > 0 {
			continue
		}
		skip, windowWidth, shown := label.window(scale)
		if !shown {
			continue
		}
		if !(label.view == 0 || label.stale || label.scale != scale || label.skip != skip || skip > 0 && label.width != windowWidth) {
			continue
		}
		_, full, err := batch.add(label, scale, allowLCD)
		if err != nil {
			return err
		}
		if full {
			// The batch bitmap is full: draw what it holds and try this label again.
			if err := flush(); err != nil {
				return err
			}
			if _, _, err := batch.add(label, scale, allowLCD); err != nil {
				return err
			}
		}
	}
	return flush()
}

// directRasterizer returns the process's DirectWrite/Direct2D rasterizer,
// making it on first use.
func (renderer *renderer) directRasterizer() (*directRasterizer, error) {
	if renderer.direct == nil {
		rasterizer, err := newDirectRasterizer()
		if err != nil {
			return nil, err
		}
		renderer.direct = rasterizer
	}
	return renderer.direct, nil
}
