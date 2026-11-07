//go:build darwin || windows || linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// Facts is the layout of one scene as the canvas captured it, before any
// rasterization: what Viva decided, in logical pixels.
type Facts struct {
	Scene    string `json:"scene"`
	Platform string `json:"platform"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Dark     bool   `json:"dark"`

	Texts      []TextFact     `json:"texts"`
	Rectangles []RectFact     `json:"rectangles"`
	Triangles  []TriangleFact `json:"triangles"`
	Shadows    []ShadowFact   `json:"shadows,omitempty"`
	Hits       []HitFact      `json:"hits"`
}

// Box is a rectangle in logical pixels.
type Box struct {
	X, Y, W, H float32
}

func box(r ui.Rect) Box { return Box{r.X, r.Y, r.Width, r.Height} }

// Hue is a color as four floats.
type Hue [4]float32

func hue(c ui.Color) Hue { return Hue{c.Red, c.Green, c.Blue, c.Alpha} }

// Measured is what the platform's text engine says about a text's string at
// its style: the line a label of that text would be if the platform laid it out.
// Viva's layout does not use it; the rasterizer draws the platform's own glyphs from it.
type Measured struct {
	Width   float32 `json:"width"`
	Height  float32 `json:"height"`
	Ascent  float32 `json:"ascent"`
	Descent float32 `json:"descent"`
	Leading float32 `json:"leading"`
}

func measured(m text.Metrics) *Measured {
	return &Measured{m.Width, m.Height, m.Ascent, m.Descent, m.Leading}
}

// TextFact is one canvas text.
type TextFact struct {
	Value    string    `json:"value"`
	Bounds   Box       `json:"bounds"`
	Size     float32   `json:"size"`
	Weight   float32   `json:"weight"`
	Mono     bool      `json:"mono,omitempty"`
	MonoFace uint16    `json:"monoFace,omitempty"`
	Color    Hue       `json:"color"`
	Layer    uint16    `json:"layer,omitempty"`
	Clip     *Box      `json:"clip,omitempty"`
	Measured *Measured `json:"measured,omitempty"`
	// Canvas is the width layout gave the text (the reference metrics, which the
	// layout golden is measured with).
	Canvas float32 `json:"canvasWidth"`
}

// RectFact is one solid rectangle.
type RectFact struct {
	Bounds Box     `json:"bounds"`
	Radius float32 `json:"radius,omitempty"`
	Color  Hue     `json:"color"`
	Layer  uint16  `json:"layer,omitempty"`
	Clip   *Box    `json:"clip,omitempty"`
}

// ShadowFact is one soft shadow: the rounded rectangle Bounds casts it, and its
// placement and blur are in logical pixels.
type ShadowFact struct {
	Bounds  Box     `json:"bounds"`
	Radius  float32 `json:"radius,omitempty"`
	Color   Hue     `json:"color"`
	OffsetX float32 `json:"offsetX,omitempty"`
	OffsetY float32 `json:"offsetY,omitempty"`
	Blur    float32 `json:"blur"`
	Spread  float32 `json:"spread,omitempty"`
	Layer   uint16  `json:"layer,omitempty"`
	Clip    *Box    `json:"clip,omitempty"`
}

// TriangleFact is one vector triangle (an icon, a check mark, a chevron).
type TriangleFact struct {
	Points [3][2]float32 `json:"points"`
	Color  Hue           `json:"color"`
	Layer  uint16        `json:"layer,omitempty"`
	Clip   *Box          `json:"clip,omitempty"`
}

// HitFact is one hit region.
type HitFact struct {
	ID     string `json:"id"`
	Bounds Box    `json:"bounds"`
	Label  string `json:"label,omitempty"`
	Role   int    `json:"role,omitempty"`
	Layer  uint16 `json:"layer,omitempty"`
}

func clipBox(r ui.Rect, has bool) *Box {
	if !has {
		return nil
	}
	b := box(r)
	return &b
}

// capture reads the canvas the way an application's painter would.
func capture(canvas *ui.Canvas, tree *ui.Tree, measure text.MeasureFunc) (texts []TextFact, rects []RectFact, triangles []TriangleFact, shadows []ShadowFact, hits []HitFact) {
	for _, t := range canvas.Texts() {
		fact := TextFact{
			Value: t.Value, Bounds: box(t.Bounds), Size: t.Style.Size, Weight: t.Style.Weight,
			Mono: t.Style.Mono, MonoFace: uint16(t.Style.MonoFace), Color: hue(t.Style.Color),
			Layer: t.Layer, Clip: clipBox(t.ClipOf, t.HasClip),
		}
		if !t.Style.Mono {
			fact.Canvas = canvas.TextWidthWeight(t.Value, t.Style.Size, t.Style.Weight)
			if measure != nil {
				if m, err := measure(t.Value, t.Style.Size, t.Style.Weight); err == nil {
					fact.Measured = measured(m)
				}
			}
		}
		texts = append(texts, fact)
	}
	for _, r := range canvas.Rectangles() {
		rects = append(rects, RectFact{Bounds: box(r.Bounds), Radius: r.Radius, Color: hue(r.Color), Layer: r.Layer, Clip: clipBox(r.ClipOf, r.HasClip)})
	}
	for _, t := range canvas.Triangles() {
		triangles = append(triangles, TriangleFact{
			Points: [3][2]float32{{t.Points[0].X, t.Points[0].Y}, {t.Points[1].X, t.Points[1].Y}, {t.Points[2].X, t.Points[2].Y}},
			Color:  hue(t.Color), Layer: t.Layer, Clip: clipBox(t.ClipOf, t.HasClip),
		})
	}
	for _, s := range canvas.Shadows() {
		shadows = append(shadows, ShadowFact{
			Bounds: box(s.Bounds), Radius: s.Radius, Color: hue(s.Color),
			OffsetX: s.Style.OffsetX, OffsetY: s.Style.OffsetY, Blur: s.Style.Blur, Spread: s.Style.Spread,
			Layer: s.Layer, Clip: clipBox(s.ClipOf, s.HasClip),
		})
	}
	for _, h := range canvas.Hits() {
		hits = append(hits, HitFact{ID: tree.Text(h.ID), Bounds: box(h.Bounds), Label: h.Label, Role: int(h.Role), Layer: h.Layer})
	}
	return
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
