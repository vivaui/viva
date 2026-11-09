//go:build darwin || windows || linux

package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/gallery"
)

func main() {
	showcase := gallery.New()
	showcase.EnableTextEditing()
	theme := showcase.Theme()
	var canvas gfx.Canvas
	var nativeInput app.TextInput
	resolveTextInput := func(context app.TextInputContext) *app.TextInput {
		editor, ok := showcase.TextEditor(context.ID)
		if !ok {
			return nil
		}
		// Bounds, font size and color come from the field the component painted.
		nativeInput = app.TextInput{Editor: editor}
		return &nativeInput
	}
	showcase.Paint(&canvas, gfx.Size{Width: gallery.DefaultWidth, Height: gallery.DefaultHeight})
	if err := app.RunCanvas(app.Config{
		Title:  "Viva · component gallery",
		Width:  gallery.DefaultWidth,
		Height: gallery.DefaultHeight,
		ClearColor: app.Color{
			Red: float64(theme.Background.Red), Green: float64(theme.Background.Green),
			Blue: float64(theme.Background.Blue), Alpha: float64(theme.Background.Alpha),
		},
		Tree:      gallery.Tree(),
		Paint:     showcase.Paint,
		OnScroll:  showcase.Scroll,
		OnPointer: showcase.Pointer,
		OnKey:     showcase.Key,
		TextInput: resolveTextInput,
		Reload: app.ReloadConfig{
			Snapshot: func() ([]app.StateSlot, error) {
				return []app.StateSlot{{Key: 1, Version: gallery.ReloadStateVersion, Data: showcase.SnapshotReloadState()}}, nil
			},
			Restore: func(slots []app.StateSlot) error {
				for _, slot := range slots {
					if slot.Key != 1 {
						continue
					}
					if slot.Version != gallery.ReloadStateVersion {
						return fmt.Errorf("unsupported gallery reload state version %d", slot.Version)
					}
					return showcase.RestoreReloadState(slot.Data)
				}
				return nil
			},
		},
	}, &canvas); err != nil {
		log.Fatal(err)
	}
}
