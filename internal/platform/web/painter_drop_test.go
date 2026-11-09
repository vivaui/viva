//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

type dropFile struct{}

func (dropFile) Name() string { return "dropped.txt" }
func (dropFile) Size() int64  { return 4 }
func (dropFile) MIME() string { return "text/plain" }
func (dropFile) IsDir() bool  { return false }
func (dropFile) Read(_ int64, length int, done func([]byte, error)) error {
	done([]byte("drop")[:min(length, 4)], nil)
	return nil
}

func TestFileDropResolvesTheHitAndCarriesBrowserFiles(t *testing.T) {
	var canvas gfx.Canvas
	var got gfx.FileDropEvent
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 90, Height: 40})
		},
		OnFileDrop: func(event gfx.FileDropEvent) gfx.EventResult {
			got = event
			return gfx.EventResult{Handled: true}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()

	handled, _ := painter.fileDrop(gfx.FileDropDrop, 20, 20, []platform.FileRef{dropFile{}})
	if !handled || got.Target != gfx.Item(1) || got.Phase != gfx.FileDropDrop || len(got.Files) != 1 {
		t.Fatalf("drop event = %+v handled=%t", got, handled)
	}
	var data []byte
	if err := got.Files[0].Read(0, 4, func(chunk []byte, err error) { data = chunk }); err != nil {
		t.Fatal(err)
	}
	if string(data) != "drop" || got.Files[0].Name() != "dropped.txt" {
		t.Fatalf("dropped file = %q %q", got.Files[0].Name(), data)
	}

	if _, _ = painter.fileDrop(gfx.FileDropLeave, 0, 0, nil); got.Phase != gfx.FileDropLeave || !got.Target.IsZero() || len(got.Files) != 0 {
		t.Fatalf("leave event = %+v", got)
	}
}
