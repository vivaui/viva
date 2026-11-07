//go:build darwin || windows || linux

package headless

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const (
	// rippleLife is how long the ring of a pointer press takes to grow and fade.
	rippleLife = 450 * time.Millisecond
	// keysLife is how long the chip of typed keys stays, and keysFade the end of
	// it over which it fades.
	keysLife = 1200 * time.Millisecond
	keysFade = 300 * time.Millisecond

	inputLayer uint16 = 1<<16 - 1
)

// inputMark is one input the session delivered: where a pointer press landed, or
// what keys were typed, and when on the animation clock.
type inputMark struct {
	at      time.Duration
	pointer bool
	x, y    float32
	text    string
}

func (session *Session) markPointer(x, y float32) {
	session.mark(inputMark{pointer: true, x: x, y: y})
}

func (session *Session) markKeys(text string) {
	if text != "" {
		session.mark(inputMark{text: text})
	}
}

// mark remembers an input for the overlay of a recording. Marks too old for any
// overlay to show are dropped as new ones arrive, so a long session keeps a few.
func (session *Session) mark(mark inputMark) {
	now := session.driver.Now()
	mark.at = now
	kept := session.marks[:0]
	for _, old := range session.marks {
		if now-old.at < keysLife {
			kept = append(kept, old)
		}
	}
	session.marks = append(kept, mark)
}

// drawInput adds the overlay of the inputs still on screen at the snapshot's
// time to the primitives and labels the renderer draws, above everything the
// application painted. The overlay is drawn by the same renderer as the window,
// so it carries the platform's own text.
func (session *Session) drawInput(snapshot app.Snapshot) app.Snapshot {
	var chip *inputMark
	for index := range session.marks {
		mark := &session.marks[index]
		age := snapshot.Now - mark.at
		if age < 0 {
			continue
		}
		if !mark.pointer {
			if age < keysLife {
				chip = mark
			}
			continue
		}
		if age >= rippleLife {
			continue
		}
		progress := float32(age) / float32(rippleLife)
		radius := 8 + 16*progress
		snapshot.Primitives = append(snapshot.Primitives,
			app.Primitive{
				Bounds: ui.Rect{X: mark.x - radius, Y: mark.y - radius, Width: 2 * radius, Height: 2 * radius},
				Color:  ui.Color{Red: 0.20, Green: 0.52, Blue: 1, Alpha: 0.45 * (1 - progress)},
				Radius: radius, Layer: inputLayer,
			},
			app.Primitive{
				Bounds: ui.Rect{X: mark.x - 3, Y: mark.y - 3, Width: 6, Height: 6},
				Color:  ui.Color{Red: 0.20, Green: 0.52, Blue: 1, Alpha: 0.9 * (1 - progress)},
				Radius: 3, Layer: inputLayer,
			})
	}
	if chip == nil {
		return snapshot
	}
	fade := float32(1)
	if left := keysLife - (snapshot.Now - chip.at); left < keysFade {
		fade = float32(left) / float32(keysFade)
	}
	const size, height, margin, pad = 15, 32, 24, 14
	width := session.canvas.TextWidthWeight(chip.text, size, ui.WeightMedium)
	box := ui.Rect{Width: width + 2*pad, Height: height}
	box.X = (float32(snapshot.Width) - box.Width) / 2
	box.Y = float32(snapshot.Height) - margin - height
	snapshot.Primitives = append(snapshot.Primitives, app.Primitive{
		Bounds: box, Color: ui.Color{Red: 0.07, Green: 0.08, Blue: 0.10, Alpha: 0.9 * fade}, Radius: 8, Layer: inputLayer,
	})
	snapshot.Labels = append(snapshot.Labels, app.Label{
		Value: chip.text, Bounds: ui.Rect{X: box.X + pad, Y: box.Y + (height-size-4)/2, Width: width, Height: size + 4},
		FontSize: size, Weight: ui.WeightMedium, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: fade}, Layer: inputLayer,
	})
	return snapshot
}

// keysLabel is how a chord written for Keys reads on the chip: each part
// capitalised, "Cmd+Shift+P", while a lone key stays as it was typed.
func keysLabel(spec string) string {
	if spec == "+" || strings.HasSuffix(spec, "++") || !strings.Contains(spec, "+") {
		if first, size := utf8.DecodeRuneInString(spec); size < len(spec) {
			return strings.ToUpper(string(first)) + spec[size:]
		}
		return spec
	}
	parts := strings.Split(spec, "+")
	for index, part := range parts {
		first, size := utf8.DecodeRuneInString(part)
		parts[index] = strings.ToUpper(string(first)) + part[size:]
	}
	return strings.Join(parts, "+")
}
